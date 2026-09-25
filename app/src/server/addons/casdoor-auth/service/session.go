package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/casdoor/casdoor-go-sdk/casdoorsdk"
	"github.com/google/uuid"
	"github.com/kwhitestone/prism-fusion/global"
	"go.uber.org/zap"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"top.whitestone/prism-fusion-site/addons/casdoor-auth/conf"
	"top.whitestone/prism-fusion-site/addons/casdoor-auth/model"
)

var errSessionInvalid = errors.New("Casdoor session is invalid, expired or revoked; sign in again")

func tokenHash(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}

// Scope jti by issuer and client. Older/custom JWTs without jti use the full JWT hash.
func accessKey(raw string, claims *casdoorsdk.Claims) string {
	if claims.ID == "" {
		return tokenHash(raw)
	}
	return tokenHash(claims.Issuer + "\x00" + conf.Get().ClientID + "\x00" + claims.ID)
}

func checkSessionActive(db *gorm.DB, raw string, claims *casdoorsdk.Claims) error {
	if db == nil {
		return errors.New("Casdoor session database unavailable")
	}
	var count int64
	now := time.Now()
	key := accessKey(raw, claims)
	// One indexed SQLite query. Require enrollment too: a token minted by calling
	// Casdoor directly after an IdP outage must not bypass local family revocation.
	err := db.Model(&model.CasdoorSession{}).
		Where("access_key = ? AND revoked_at IS NULL AND access_expires_at > ? AND family_expires_at > ?", key, now, now).
		Where("NOT EXISTS (SELECT 1 FROM casdoor_token_blacklists WHERE key = ? AND expires_at > ?)", key, now).
		Count(&count).Error
	if err != nil {
		return err
	}
	if count == 0 {
		return errSessionInvalid
	}
	return nil
}

func sessionForPair(access, refresh, family string, familyExpiry time.Time) (*model.CasdoorSession, error) {
	claims, err := verifyCasdoorToken(access, conf.Get())
	if err != nil {
		return nil, err
	}
	refreshClaims := claims
	if refresh != access {
		refreshClaims, err = verifyCasdoorCredential(refresh, conf.Get(), true, false)
		if err != nil {
			return nil, err
		}
		if refreshClaims.Subject != claims.Subject || refreshClaims.ID != claims.ID {
			return nil, errSessionInvalid
		}
	}
	if familyExpiry.IsZero() {
		familyExpiry = refreshClaims.ExpiresAt.Time
	}
	return &model.CasdoorSession{TokenHash: tokenHash(refresh), FamilyID: family, Subject: claims.Subject,
		AccessKey: accessKey(access, claims), AccessExpiresAt: claims.ExpiresAt.Time,
		ExpiresAt: refreshClaims.ExpiresAt.Time, FamilyExpiresAt: familyExpiry}, nil
}

func (s *CasdoorService) registerSession(access, refresh string) error {
	row, err := sessionForPair(access, refresh, uuid.NewString(), time.Time{})
	if err != nil {
		return err
	}
	if global.PRISM_DB == nil {
		return errors.New("Casdoor session database unavailable")
	}
	// Repeated authorization-code responses cannot resurrect an existing family.
	if err := global.PRISM_DB.Clauses(clause.OnConflict{DoNothing: true}).Create(row).Error; err != nil {
		return err
	}
	var stored model.CasdoorSession
	if err := global.PRISM_DB.First(&stored, "token_hash = ?", row.TokenHash).Error; err != nil {
		return err
	}
	if stored.RevokedAt != nil || stored.RotatedAt != nil || stored.AccessKey != row.AccessKey {
		return errSessionInvalid
	}
	return nil
}

// Refresh locally admits only a registered, unrevoked generation. The final CAS
// and insertion share a transaction so logout wins even while Casdoor is responding.
func (s *CasdoorService) refreshSession(raw string) (string, string, error) {
	db := global.PRISM_DB
	if db == nil {
		return "", "", errSessionInvalid
	}
	var current model.CasdoorSession
	now := time.Now()
	if err := db.Where("token_hash = ? AND revoked_at IS NULL AND rotated_at IS NULL AND expires_at > ? AND family_expires_at > ?", tokenHash(raw), now, now).First(&current).Error; err != nil {
		return "", "", errSessionInvalid
	}
	newToken, err := casdoorsdk.RefreshOAuthToken(raw, casdoorsdk.WithHTTPClient(&http.Client{Timeout: 8 * time.Second}))
	if err != nil {
		return "", "", errors.New("Casdoor token refresh failed")
	}
	refresh := newToken.RefreshToken
	if refresh == "" {
		refresh = raw
	}
	next, err := sessionForPair(newToken.AccessToken, refresh, current.FamilyID, current.FamilyExpiresAt)
	if err != nil || next.Subject != current.Subject {
		s.logoutUpstream(newToken.AccessToken)
		return "", "", errSessionInvalid
	}
	err = db.Transaction(func(tx *gorm.DB) error {
		now := time.Now()
		changed := tx.Model(&model.CasdoorSession{}).Where("token_hash = ? AND revoked_at IS NULL AND rotated_at IS NULL AND expires_at > ? AND family_expires_at > ?", current.TokenHash, now, now).Update("rotated_at", now)
		if changed.Error != nil {
			return changed.Error
		}
		if changed.RowsAffected != 1 {
			return errSessionInvalid
		}
		return tx.Create(next).Error
	})
	if err != nil {
		s.logoutUpstream(newToken.AccessToken)
		return "", "", errSessionInvalid
	}
	return newToken.AccessToken, refresh, nil
}

func blacklist(tx *gorm.DB, key string, expiry time.Time) error {
	// Never shorten an existing revocation window on repeated logout.
	return tx.Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "key"}}, DoUpdates: clause.Assignments(map[string]interface{}{"expires_at": gorm.Expr("MAX(expires_at, ?)", expiry)})}).Create(&model.CasdoorTokenBlacklist{Key: key, ExpiresAt: expiry}).Error
}

// Logout permits expired access credentials only for revocation. An optional
// refresh must belong to the same family; it is never trusted from body claims.
func (s *CasdoorService) Logout(ctx context.Context, access, refresh string) error {
	claims, err := verifyCasdoorCredential(access, conf.Get(), false, true)
	if err != nil {
		return errSessionInvalid
	}
	db := global.PRISM_DB
	if db == nil {
		return errors.New("Casdoor session database unavailable")
	}
	key := accessKey(access, claims)
	err = db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var rows []model.CasdoorSession
		if err := tx.Where("access_key = ? AND subject = ?", key, claims.Subject).Find(&rows).Error; err != nil {
			return err
		}
		if refresh != "" {
			var supplied model.CasdoorSession
			lookup := tx.First(&supplied, "token_hash = ?", tokenHash(refresh)).Error
			if lookup == nil {
				matched := false
				for _, row := range rows {
					matched = matched || row.FamilyID == supplied.FamilyID
				}
				if !matched || supplied.Subject != claims.Subject {
					return errSessionInvalid
				}
			} else if errors.Is(lookup, gorm.ErrRecordNotFound) {
				// Pre-deployment sessions may log out, but may not refresh without re-login.
				refreshClaims := claims
				if refresh != access {
					var err error
					refreshClaims, err = verifyCasdoorCredential(refresh, conf.Get(), true, true)
					if err != nil || refreshClaims.Subject != claims.Subject || claims.ID == "" || refreshClaims.ID != claims.ID {
						return errSessionInvalid
					}
				}
				now := time.Now()
				legacy := model.CasdoorSession{TokenHash: tokenHash(refresh), FamilyID: uuid.NewString(), Subject: claims.Subject,
					AccessKey: key, AccessExpiresAt: claims.ExpiresAt.Time, ExpiresAt: refreshClaims.ExpiresAt.Time,
					FamilyExpiresAt: refreshClaims.ExpiresAt.Time, RevokedAt: &now}
				if err := tx.Create(&legacy).Error; err != nil {
					return err
				}
			} else {
				return lookup
			}
		}
		for _, row := range rows {
			now := time.Now()
			if err := tx.Model(&model.CasdoorSession{}).Where("family_id = ?", row.FamilyID).Update("revoked_at", now).Error; err != nil {
				return err
			}
			var family []model.CasdoorSession
			if err := tx.Where("family_id = ?", row.FamilyID).Find(&family).Error; err != nil {
				return err
			}
			for _, member := range family {
				if err := blacklist(tx, member.AccessKey, member.AccessExpiresAt); err != nil {
					return err
				}
			}
		}
		return blacklist(tx, key, claims.ExpiresAt.Time)
	})
	if err != nil {
		return err
	}
	// Commit local revocation first. No Casdoor outage or request cancellation can undo it.
	s.logoutUpstream(access)
	s.InvalidateUserCache(claims.Subject)
	return nil
}

func (s *CasdoorService) logoutUpstream(access string) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	form := url.Values{"id_token_hint": {access}}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(conf.Get().Endpoint, "/")+"/api/logout", strings.NewReader(form.Encode()))
	if err != nil {
		global.PRISM_LOG.Warn("Casdoor logout request failed; local revocation retained")
		return
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	client := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	resp, err := client.Do(req)
	if err != nil {
		global.PRISM_LOG.Warn("Casdoor logout unavailable; local revocation retained")
		return
	}
	defer resp.Body.Close()
	var result struct {
		Status string `json:"status"`
	}
	err = json.NewDecoder(io.LimitReader(resp.Body, 65536)).Decode(&result)
	if resp.StatusCode != http.StatusOK || err != nil || result.Status != "ok" {
		// Do not log upstream bodies or URL errors: either can echo credentials.
		global.PRISM_LOG.Warn("Casdoor logout rejected; local revocation retained", zap.Int("status", resp.StatusCode))
	}
}

// Cleanup runs after startup migration and can be reused by maintenance. Indexed
// expiry predicates make expired entries ineffective immediately, before deletion.
func CleanupSessions(db *gorm.DB, now time.Time) error {
	return db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("expires_at <= ?", now).Delete(&model.CasdoorTokenBlacklist{}).Error; err != nil {
			return err
		}
		return tx.Where("family_expires_at <= ? AND access_expires_at <= ? AND expires_at <= ?", now, now, now).Delete(&model.CasdoorSession{}).Error
	})
}
