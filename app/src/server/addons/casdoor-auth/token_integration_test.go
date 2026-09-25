package casdoorauth_test

import (
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"math/big"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"
	"top.whitestone/prism-fusion-site/addons/casdoor-auth/model"

	"github.com/casdoor/casdoor-go-sdk/casdoorsdk"
	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humagin"
	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/kwhitestone/prism-fusion/global"
	"go.uber.org/zap"
	"top.whitestone/prism-fusion-site/addons/casdoor-auth/conf"
	"top.whitestone/prism-fusion-site/addons/casdoor-auth/middleware"
	"top.whitestone/prism-fusion-site/addons/casdoor-auth/router"
	"top.whitestone/prism-fusion-site/addons/casdoor-auth/service"
)

// Exercise production middleware AND the OAuth callback against a mock Casdoor.
// No real credentials or Casdoor configuration writes are needed.
func TestDualClaimsAuthentication(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	wrongKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	oldConfig, oldLog := *conf.Get(), global.PRISM_LOG
	t.Cleanup(func() { *conf.Get() = oldConfig; global.PRISM_LOG = oldLog })
	global.PRISM_LOG = zap.NewNop()
	oldDB := global.PRISM_DB
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "auth.db")), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&model.CasdoorSession{}, &model.CasdoorTokenBlacklist{}); err != nil {
		t.Fatal(err)
	}
	global.PRISM_DB = db
	t.Cleanup(func() { global.PRISM_DB = oldDB; sqlDB, _ := db.DB(); _ = sqlDB.Close() })
	tokens := map[string]string{}
	jwksCalls := 0
	var upstream *httptest.Server
	upstream = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/.well-known/openid-configuration":
			_ = json.NewEncoder(w).Encode(map[string]string{"jwks_uri": upstream.URL + "/.well-known/jwks"})
		case "/.well-known/jwks":
			jwksCalls++
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"keys": []interface{}{map[string]string{
				"kty": "RSA", "alg": "RS256", "use": "sig", "n": base64.RawURLEncoding.EncodeToString(key.N.Bytes()),
				"e": base64.RawURLEncoding.EncodeToString(big.NewInt(int64(key.E)).Bytes()),
			}}})
		case "/api/login/oauth/access_token":
			_ = r.ParseForm()
			access := tokens[r.Form.Get("code")]
			rc := jwt.MapClaims{}
			_, _, _ = jwt.NewParser().ParseUnverified(access, rc)
			rc["tokenType"] = "refresh-token"
			refresh, _ := jwt.NewWithClaims(jwt.SigningMethodRS256, rc).SignedString(key)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"access_token": access, "refresh_token": refresh, "token_type": "Bearer"})
		case "/api/get-user":
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"status": "ok", "data": map[string]interface{}{"name": "tester", "owner": "test-org", "id": "test-user"}})
		default:
			http.Error(w, "not found", http.StatusNotFound)
		}
	}))
	defer upstream.Close()
	*conf.Get() = conf.CasdoorConfig{Endpoint: upstream.URL, ExternalEndpoint: "https://account.example.test", OrganizationName: "test-org", ClientID: "test-client", ClientSecret: "synthetic-secret", ApplicationName: "test-app"}
	svc := &service.CasdoorService{}
	svc.InitSDK() // discovers and caches the public key; credential discovery gets 404
	if conf.Get().Certificate == "" {
		t.Fatal("JWKS discovery failed")
	}
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.Use(middleware.CasdoorJwtMiddleware())
	engine.GET("/api/protected", func(c *gin.Context) { c.Status(http.StatusOK) })
	router.RegisterRoutes(humagin.New(engine, huma.DefaultConfig("test", "1")))
	redirect := "https://site.example.test/login"
	service.InitKnownRedirectUris([]string{redirect})
	signed := func(c jwt.MapClaims, k *rsa.PrivateKey, method jwt.SigningMethod) string {
		s, e := jwt.NewWithClaims(method, c).SignedString(k)
		if e != nil {
			t.Fatal(e)
		}
		return s
	}
	base := func() jwt.MapClaims {
		return jwt.MapClaims{"iss": upstream.URL, "sub": "test-user", "aud": []string{"test-client"}, "exp": time.Now().Add(time.Hour).Unix(), "name": "tester", "owner": "test-org", "address": []string{"street"}, "tokenType": "access-token"}
	}
	cases := []struct {
		name   string
		modify func(jwt.MapClaims)
		want   int
	}{
		{"legacy", func(jwt.MapClaims) {}, 200},
		{"standard", func(c jwt.MapClaims) {
			c["address"] = map[string]string{"street_address": "street\nsecond"}
			delete(c, "owner")
		}, 200},
		{"external-issuer", func(c jwt.MapClaims) { c["iss"] = conf.Get().ExternalEndpoint }, 200},
		{"wrong-issuer", func(c jwt.MapClaims) { c["iss"] = "https://attacker.invalid" }, 401},
		{"issuer-prefix", func(c jwt.MapClaims) { c["iss"] = upstream.URL + ".attacker.invalid" }, 401},
		{"missing-issuer", func(c jwt.MapClaims) { delete(c, "iss") }, 401},
		{"expired", func(c jwt.MapClaims) { c["exp"] = time.Now().Add(-time.Minute).Unix() }, 401},
		{"missing-exp", func(c jwt.MapClaims) { delete(c, "exp") }, 401},
		{"future-nbf", func(c jwt.MapClaims) { c["nbf"] = time.Now().Add(time.Hour).Unix() }, 401},
		{"wrong-audience", func(c jwt.MapClaims) { c["aud"] = "other-app" }, 401},
		{"wrong-organization", func(c jwt.MapClaims) { c["owner"] = "other-org" }, 401},
		{"refresh-token", func(c jwt.MapClaims) { c["tokenType"] = "refresh-token" }, 401},
		{"bad-address", func(c jwt.MapClaims) { c["address"] = "street" }, 401},
		{"wrong-signature", func(jwt.MapClaims) {}, 401},
		{"tampered-payload", func(jwt.MapClaims) {}, 401},
		{"wrong-algorithm", func(jwt.MapClaims) {}, 401},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := base()
			tc.modify(c)
			k := key
			method := jwt.SigningMethodRS256
			if tc.name == "wrong-signature" {
				k = wrongKey
			}
			if tc.name == "wrong-algorithm" {
				method = jwt.SigningMethodRS512
			}
			token := signed(c, k, method)
			if tc.name == "tampered-payload" {
				parts := strings.Split(token, ".")
				c["isAdmin"] = true
				b, _ := json.Marshal(c)
				parts[1] = base64.RawURLEncoding.EncodeToString(b)
				token = strings.Join(parts, ".")
			}
			tokens[tc.name] = token
			// PKCE path obtains a verifier using the production signin URL method.
			_ = svc.GetSigninURL(tc.name, redirect)
			body, _ := json.Marshal(map[string]string{"code": tc.name, "state": tc.name, "redirectUri": redirect})
			req := httptest.NewRequest("POST", "/api/v1/addons/casdoor-auth/signin-callback", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			engine.ServeHTTP(w, req)
			if w.Code != tc.want {
				t.Fatalf("PKCE callback status %d, want %d", w.Code, tc.want)
			}
			// Same callback without a stored verifier exercises the SDK exchange path.
			req = httptest.NewRequest("POST", "/api/v1/addons/casdoor-auth/signin-callback", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			w = httptest.NewRecorder()
			engine.ServeHTTP(w, req)
			if w.Code != tc.want {
				t.Fatalf("SDK callback status %d, want %d", w.Code, tc.want)
			}
			req = httptest.NewRequest("GET", "/api/protected", nil)
			req.Header.Set("Authorization", "Bearer "+token)
			w = httptest.NewRecorder()
			engine.ServeHTTP(w, req)
			if w.Code != tc.want {
				t.Fatalf("middleware status %d, want %d", w.Code, tc.want)
			}
			if tc.want == 401 && strings.Contains(w.Body.String(), token) {
				t.Fatal("token leaked in error")
			}
		})
	}
	// Pin the SDK defects from B-v2 as regression evidence.
	if _, err := casdoorsdk.ParseJwtToken(tokens["standard"]); err == nil {
		t.Fatal("SDK standard-claims regression premise changed")
	}
	if _, err := casdoorsdk.ParseJwtToken(tokens["wrong-issuer"]); err != nil {
		t.Fatal("SDK wrong-issuer regression premise changed")
	}
	claims, err := svc.ParseToken(tokens["standard"])
	if err != nil || len(claims.Address) != 2 {
		t.Fatal("standard address not preserved")
	}
	if jwksCalls != 1 {
		t.Fatalf("JWKS fetched %d times; expected initialization only", jwksCalls)
	}
	for _, certificate := range []string{"", "invalid"} {
		conf.Get().Certificate = certificate
		if _, err := svc.ParseToken(tokens["legacy"]); err == nil {
			t.Fatal("missing/broken certificate accepted")
		}
	}
}
