package casdoorauth_test

import (
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humagin"
	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/kwhitestone/prism-fusion/global"
	"go.uber.org/zap"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	casbinMiddleware "top.whitestone/prism-fusion-site/addons/casbin-rbac/middleware"
	casbinService "top.whitestone/prism-fusion-site/addons/casbin-rbac/service"
	"top.whitestone/prism-fusion-site/addons/casdoor-auth/conf"
	"top.whitestone/prism-fusion-site/addons/casdoor-auth/middleware"
	"top.whitestone/prism-fusion-site/addons/casdoor-auth/model"
	"top.whitestone/prism-fusion-site/addons/casdoor-auth/router"
	"top.whitestone/prism-fusion-site/addons/casdoor-auth/service"
)

func TestLogoutL2(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	oldConfig, oldDB, oldLog := *conf.Get(), global.PRISM_DB, global.PRISM_LOG
	t.Cleanup(func() { *conf.Get() = oldConfig; global.PRISM_DB = oldDB; global.PRISM_LOG = oldLog })
	global.PRISM_LOG = zap.NewNop()
	casbinService.InvalidatePermCache()
	t.Cleanup(casbinService.InvalidatePermCache)
	dbPath := filepath.Join(t.TempDir(), "sessions.db")
	openDB := func() *gorm.DB {
		db, err := gorm.Open(sqlite.Open(dbPath+"?_busy_timeout=5000"), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
		if err != nil {
			t.Fatal(err)
		}
		sqlDB, _ := db.DB()
		sqlDB.SetMaxOpenConns(1)
		t.Cleanup(func() { _ = sqlDB.Close() })
		return db
	}
	global.PRISM_DB = openDB()
	if err := global.PRISM_DB.AutoMigrate(&model.CasdoorSession{}, &model.CasdoorTokenBlacklist{}); err != nil {
		t.Fatal(err)
	}
	var upstream *httptest.Server
	var sequence, refreshCalls, logoutCalls atomic.Int32
	var blockRefresh atomic.Bool
	entered, release := make(chan struct{}), make(chan struct{})
	pair := func(name string) (string, string) {
		id := fmt.Sprintf("%s-%d", name, sequence.Add(1))
		claims := jwt.MapClaims{"iss": upstream.URL, "aud": []string{"test-client"}, "sub": "test-user", "name": "tester", "owner": "test-org", "isAdmin": true, "exp": time.Now().Add(time.Hour).Unix(), "jti": id, "tokenType": "access-token", "nonce": id}
		if strings.HasPrefix(name, "no-jti") {
			delete(claims, "jti")
		}
		if strings.HasPrefix(name, "expired") {
			claims["exp"] = time.Now().Add(-time.Minute).Unix()
		}
		access, _ := jwt.NewWithClaims(jwt.SigningMethodRS256, claims).SignedString(key)
		claims["tokenType"] = "refresh-token"
		claims["exp"] = time.Now().Add(24 * time.Hour).Unix()
		refresh, _ := jwt.NewWithClaims(jwt.SigningMethodRS256, claims).SignedString(key)
		return access, refresh
	}
	upstream = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = r.ParseForm()
		switch r.URL.Path {
		case "/api/login/oauth/access_token", "/api/login/oauth/refresh_token":
			if r.URL.Path == "/api/login/oauth/refresh_token" {
				refreshCalls.Add(1)
				if blockRefresh.Load() {
					close(entered)
					<-release
				}
			}
			a, b := pair(r.Form.Get("code"))
			_ = json.NewEncoder(w).Encode(map[string]any{"access_token": a, "refresh_token": b, "token_type": "Bearer"})
		case "/api/get-permissions":
			_ = json.NewEncoder(w).Encode(map[string]any{"status": "ok", "data": []any{map[string]any{"users": []string{"test-org/*"}, "resources": []string{"/api/protected"}, "actions": []string{"GET"}, "effect": "Allow", "isEnabled": true}}})
		case "/api/logout":
			logoutCalls.Add(1)
			if r.Form.Get("id_token_hint") == "" {
				t.Error("missing logout hint")
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"status": "ok"})
		default:
			http.Error(w, "not found", 404)
		}
	}))
	defer upstream.Close()
	*conf.Get() = conf.CasdoorConfig{Endpoint: upstream.URL, ExternalEndpoint: upstream.URL, ClientID: "test-client", ClientSecret: "synthetic-secret", OrganizationName: "test-org", ApplicationName: "test-app", Certificate: string(pem.EncodeToMemory(&pem.Block{Type: "RSA PUBLIC KEY", Bytes: x509.MarshalPKCS1PublicKey(&key.PublicKey)}))}
	svc := &service.CasdoorService{}
	svc.InitSDK()
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.Use(middleware.CasdoorJwtMiddleware(), casbinMiddleware.CasbinAuthzMiddleware())
	engine.GET("/api/protected", func(c *gin.Context) { c.Status(200) })
	router.RegisterRoutes(humagin.New(engine, huma.DefaultConfig("test", "1")))
	request := func(path, access string, body map[string]string) *httptest.ResponseRecorder {
		method := http.MethodGet
		var payload []byte
		if body != nil {
			method = http.MethodPost
			payload, _ = json.Marshal(body)
		}
		r := httptest.NewRequest(method, path, bytes.NewReader(payload))
		if access != "" {
			r.Header.Set("Authorization", "Bearer "+access)
		}
		if body != nil {
			r.Header.Set("Content-Type", "application/json")
		}
		w := httptest.NewRecorder()
		engine.ServeHTTP(w, r)
		return w
	}
	assertStatus := func(w *httptest.ResponseRecorder, want int) {
		t.Helper()
		if w.Code != want {
			t.Fatalf("HTTP %d, want %d", w.Code, want)
		}
	}
	login := func(code string) (string, string) {
		t.Helper()
		w := request("/api/v1/addons/casdoor-auth/signin-callback", "", map[string]string{"code": code, "state": "test", "redirectUri": "https://example.test/login"})
		assertStatus(w, 200)
		var envelope struct {
			Data router.CasdoorTokenData `json:"data"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &envelope); err != nil {
			t.Fatal(err)
		}
		return envelope.Data.AccessToken, envelope.Data.RefreshToken
	}
	logout := func(a, b string) *httptest.ResponseRecorder {
		return request("/api/v1/addons/casdoor-auth/logout", a, map[string]string{"refreshToken": b})
	}
	refresh := func(b string) *httptest.ResponseRecorder {
		return request("/api/v1/addons/casdoor-auth/refresh-token", "", map[string]string{"refreshToken": b})
	}
	unregistered, _ := pair("direct-casdoor-token")
	assertStatus(request("/api/protected", unregistered, nil), 401)
	a, r := login("first")
	other, otherRefresh := login("other-session")
	assertStatus(request("/api/protected", a, nil), 200)
	assertStatus(logout(a, otherRefresh), 401) // same user, different family must not be revoked
	assertStatus(request("/api/protected", other, nil), 200)
	w := refresh(r)
	assertStatus(w, 200)
	var rotated struct {
		Data router.CasdoorTokenData `json:"data"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &rotated)
	assertStatus(logout(a, r), 200) // an old generation still revokes all descendants
	assertStatus(request("/api/protected", a, nil), 401)
	assertStatus(request("/api/protected", rotated.Data.AccessToken, nil), 401)
	before := refreshCalls.Load()
	assertStatus(refresh(r), 401)
	assertStatus(refresh(rotated.Data.RefreshToken), 401)
	if refreshCalls.Load() != before {
		t.Fatal("revoked refresh reached Casdoor")
	}
	assertStatus(logout(a, r), 200) // retry is idempotent
	assertStatus(request("/api/protected", other, nil), 200)
	if logoutCalls.Load() < 2 {
		t.Fatal("Casdoor logout was not attempted")
	}

	// Reopen the SQLite database: revoked credentials must remain rejected.
	global.PRISM_DB = openDB()
	assertStatus(request("/api/protected", a, nil), 401)
	assertStatus(refresh(rotated.Data.RefreshToken), 401)
	fresh, freshRefresh := login("relogin")
	assertStatus(request("/api/protected", fresh, nil), 200)
	assertStatus(refresh(freshRefresh), 200)

	// An access-only logout also finds and revokes its registered refresh family.
	assertStatus(logout(other, ""), 200)
	assertStatus(refresh(otherRefresh), 401)
	noJTI, noJTIRefresh := login("no-jti")
	assertStatus(logout(noJTI, noJTIRefresh), 200)
	assertStatus(request("/api/protected", noJTI, nil), 401)
	assertStatus(refresh(noJTIRefresh), 401)

	// Refresh is in flight at the IdP when logout commits; it cannot resurrect a family.
	raceAccess, raceRefresh := login("race")
	blockRefresh.Store(true)
	result := make(chan *httptest.ResponseRecorder, 1)
	go func() { result <- refresh(raceRefresh) }()
	select {
	case <-entered:
	case <-time.After(5 * time.Second):
		t.Fatal("refresh did not reach upstream")
	}
	assertStatus(logout(raceAccess, raceRefresh), 200)
	close(release)
	assertStatus(<-result, 401)
	blockRefresh.Store(false)
	assertStatus(request("/api/protected", raceAccess, nil), 401)
	assertStatus(refresh(raceRefresh), 401)

	// Older deployment tokens cannot refresh without a locally registered family,
	// but can log out even after access expiration.
	legacy, legacyRefresh := pair("expired-legacy")
	assertStatus(refresh(legacyRefresh), 401)
	assertStatus(logout(legacy, legacyRefresh), 200)
	assertStatus(refresh(legacyRefresh), 401)

	// An expired blacklist row must no longer block an otherwise active family.
	ttlAccess, _ := login("ttl")
	var ttlSession model.CasdoorSession
	if err := global.PRISM_DB.Where("subject = ?", "test-user").Order("rowid DESC").First(&ttlSession).Error; err != nil {
		t.Fatal(err)
	}
	if err := global.PRISM_DB.Create(&model.CasdoorTokenBlacklist{Key: ttlSession.AccessKey, ExpiresAt: time.Now().Add(-time.Second)}).Error; err != nil {
		t.Fatal(err)
	}
	assertStatus(request("/api/protected", ttlAccess, nil), 200)

	// Expiry is enforced in the read predicate, and startup cleanup removes records.
	var expired model.CasdoorTokenBlacklist
	if err := global.PRISM_DB.Where("expires_at < ?", time.Now()).First(&expired).Error; err != nil {
		t.Fatal(err)
	}
	if err := service.CleanupSessions(global.PRISM_DB, time.Now()); err != nil {
		t.Fatal(err)
	}
	var count int64
	global.PRISM_DB.Model(&model.CasdoorTokenBlacklist{}).Where("key = ?", expired.Key).Count(&count)
	if count != 0 {
		t.Fatal("expired blacklist entry not cleaned")
	}
	assertStatus(request("/api/protected", a, nil), 401) // unexpired entries survived cleanup

	outage, outageRefresh := login("outage")
	upstream.Close()
	assertStatus(logout(outage, outageRefresh), 200)
	assertStatus(request("/api/protected", outage, nil), 401)
	assertStatus(refresh(outageRefresh), 401)
	// Database failures fail closed, rather than silently skipping revocation checks.
	db := global.PRISM_DB
	global.PRISM_DB = nil
	assertStatus(request("/api/protected", fresh, nil), 401)
	global.PRISM_DB = db
}
