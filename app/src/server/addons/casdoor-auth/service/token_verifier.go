package service

import (
	"crypto/rsa"
	"encoding/json"
	"errors"
	"strings"
	"sync"

	"github.com/casdoor/casdoor-go-sdk/casdoorsdk"
	"github.com/golang-jwt/jwt/v5"
	"top.whitestone/prism-fusion-site/addons/casdoor-auth/conf"
)

// Cache only the configured public key, never authentication decisions. JWKS
// discovery runs during InitSDK; a missing/broken key always fails closed.
var verificationKeyCache struct {
	sync.Mutex
	certificate string
	key         *rsa.PublicKey
}

func verificationKey(certificate string) (*rsa.PublicKey, error) {
	verificationKeyCache.Lock()
	defer verificationKeyCache.Unlock()
	if certificate == "" {
		return nil, errors.New("Casdoor verification key unavailable")
	}
	if verificationKeyCache.certificate != certificate || verificationKeyCache.key == nil {
		key, err := jwt.ParseRSAPublicKeyFromPEM([]byte(certificate))
		if err != nil {
			return nil, errors.New("invalid Casdoor verification key")
		}
		verificationKeyCache.certificate, verificationKeyCache.key = certificate, key
	}
	return verificationKeyCache.key, nil
}

func verifyCasdoorToken(raw string, cfg *conf.CasdoorConfig) (*casdoorsdk.Claims, error) {
	key, err := verificationKey(cfg.Certificate)
	if err != nil {
		return nil, err
	}
	if cfg.Endpoint == "" || cfg.OrganizationName == "" || cfg.ClientID == "" {
		return nil, errors.New("incomplete Casdoor verification configuration")
	}
	claims := jwt.MapClaims{}
	token, err := jwt.ParseWithClaims(raw, claims, func(*jwt.Token) (interface{}, error) {
		return key, nil
	}, jwt.WithValidMethods([]string{"RS256"}), jwt.WithExpirationRequired(),
		jwt.WithAudience(cfg.ClientID), jwt.WithJSONNumber())
	if err != nil || token == nil || !token.Valid {
		// Parser errors can contain attacker-controlled claim/header values.
		return nil, errors.New("invalid Casdoor token signature or claims")
	}
	// Casdoor generateJwtToken uses getOriginFromHost, not an organization URL.
	// Both origins are trusted configuration (internal code exchange / public login).
	issuer, err := claims.GetIssuer()
	if err != nil || issuer == "" || (issuer != strings.TrimRight(cfg.Endpoint, "/") &&
		issuer != strings.TrimRight(cfg.ExternalEndpoint, "/")) {
		return nil, errors.New("invalid Casdoor token issuer")
	}
	if owner, exists := claims["owner"]; exists && owner != cfg.OrganizationName {
		return nil, errors.New("invalid Casdoor token organization")
	}
	if subject, err := claims.GetSubject(); err != nil || subject == "" {
		return nil, errors.New("missing Casdoor token subject")
	}
	if claims["tokenType"] == "refresh-token" || claims["TokenType"] == "refresh-token" {
		return nil, errors.New("refresh token cannot authenticate requests")
	}

	// Only map claims after validating the ORIGINAL compact JWT. Shadow the SDK's
	// []string address so OIDC address objects cannot break signature verification.
	data, err := json.Marshal(claims)
	if err != nil {
		return nil, errors.New("invalid Casdoor claims")
	}
	var dual struct {
		casdoorsdk.Claims
		Address json.RawMessage `json:"address"`
	}
	if err := json.Unmarshal(data, &dual); err != nil {
		return nil, errors.New("invalid Casdoor claims")
	}
	if len(dual.Address) != 0 && string(dual.Address) != "null" {
		switch dual.Address[0] {
		case '[':
			if err := json.Unmarshal(dual.Address, &dual.Claims.Address); err != nil {
				return nil, errors.New("invalid legacy Casdoor address")
			}
		case '{':
			var address map[string]string
			if err := json.Unmarshal(dual.Address, &address); err != nil {
				return nil, errors.New("invalid standard Casdoor address")
			}
			if street := strings.TrimSpace(address["street_address"]); street != "" {
				dual.Claims.Address = strings.Split(street, "\n")
			} else if formatted := address["formatted"]; formatted != "" {
				dual.Claims.Address = []string{formatted}
			}
		default:
			return nil, errors.New("invalid Casdoor address format")
		}
	}
	return &dual.Claims, nil
}
