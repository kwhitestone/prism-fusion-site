package main

import (
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/casdoor/casdoor-go-sdk/casdoorsdk"
	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v4"
	"github.com/kwhitestone/prism-fusion/global"
	"github.com/kwhitestone/prism-fusion/initialize"
	"github.com/kwhitestone/prism-fusion/plugin"
	"github.com/spf13/viper"
	"go.uber.org/zap"
	casbinservice "top.whitestone/prism-fusion-site/addons/casbin-rbac/service"
	"top.whitestone/prism-fusion-site/addons/casdoor-auth/conf"
	"top.whitestone/prism-fusion-site/addons/casdoor-auth/model"
)

func TestPluginScopeIsolationThroughFrameworkRouter(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	publicKey, err := x509.MarshalPKIXPublicKey(&key.PublicKey)
	if err != nil {
		t.Fatal(err)
	}
	certificate := string(pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: publicKey}))
	// Only read-only local endpoints are allowed; bootstrap must never reach a real Casdoor.
	casdoor := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method != http.MethodGet {
			t.Errorf("unexpected Casdoor write: %s %s", r.Method, r.URL.Path)
			http.Error(w, "writes forbidden", http.StatusForbidden)
			return
		}
		switch r.URL.Path {
		case "/api/get-application":
			http.Error(w, "credential discovery disabled", http.StatusNotFound)
		case "/api/get-permissions":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"status": "ok",
				"data": []*casdoorsdk.Permission{
					{Users: []string{"built-in/*"}, Resources: []string{"/api/v1/*"}, Actions: []string{"GET", "POST"}, Effect: "Allow", IsEnabled: true},
					{Users: []string{"built-in/*"}, Resources: []string{"/api/v1/addons/messages"}, Actions: []string{"DELETE"}, Effect: "Allow", IsEnabled: true},
					{Users: []string{"built-in/*"}, Resources: []string{"/api/v1/addons/messages"}, Actions: []string{"DELETE"}, Effect: "Deny", IsEnabled: true},
				},
			})
		default:
			t.Errorf("unexpected Casdoor endpoint: %s", r.URL.Path)
			http.NotFound(w, r)
		}
	}))
	defer casdoor.Close()

	previousConfig, previousDB, previousLog, previousVP := global.PRISM_CONFIG, global.PRISM_DB, global.PRISM_LOG, global.PRISM_VP
	previousCasdoor := *conf.Get()
	t.Cleanup(func() {
		global.PRISM_CONFIG, global.PRISM_DB, global.PRISM_LOG, global.PRISM_VP = previousConfig, previousDB, previousLog, previousVP
		*conf.Get() = previousCasdoor
	})
	global.PRISM_LOG = zap.NewNop()
	global.PRISM_CONFIG.Auth.Provider = "casdoor"
	global.PRISM_CONFIG.RBAC.Provider = "casbin"
	global.PRISM_CONFIG.System.Env = "public"
	global.PRISM_CONFIG.Sqlite.Path = "file:plugin-scope-test?mode=memory&cache=shared"
	global.PRISM_CONFIG.Sqlite.MaxIdleConns = 1
	global.PRISM_CONFIG.Sqlite.MaxOpenConns = 1
	global.PRISM_VP = viper.New()
	global.PRISM_VP.Set("auth.casdoor", map[string]any{
		"endpoint": casdoor.URL, "certificate": certificate,
		"organization-name": "built-in", "application-name": "integration-test",
		"client-id": "test-client", "client-secret": "test-secret",
	})
	global.PRISM_DB = initialize.GormSqlite()
	if global.PRISM_DB == nil {
		t.Fatal("failed to initialize memory database")
	}
	sqlDB, err := global.PRISM_DB.DB()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	initialize.InitTables()
	gin.SetMode(gin.TestMode)
	router := initialize.Routers()
	casbinservice.InvalidatePermCache()

	for _, entry := range plugin.MustResolve() {
		if entry.Manifest.ID == "auth" || entry.Manifest.ID == "rbac" {
			t.Fatalf("builtin provider unexpectedly active: %s", entry.Manifest.ID)
		}
	}
	request := func(token, method, path, body string, want int, scoped bool) {
		t.Helper()
		req := httptest.NewRequest(method, path, bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}
		res := httptest.NewRecorder()
		router.ServeHTTP(res, req)
		if res.Code != want {
			t.Fatalf("%s %s = %d, want %d: %s", method, path, res.Code, want, res.Body.String())
		}
		if got := res.Header().Get("X-Example-Middleware") == "active"; got != scoped {
			t.Fatalf("%s: scoped header = %v, want %v", path, got, scoped)
		}
	}
	request("", "GET", "/health", "", http.StatusOK, false)
	request("", "GET", "/api/v1/addons/casdoor-auth/config", "", http.StatusOK, false)
	for _, path := range []string{"/api/v1/addons/example/items", "/api/v1/addons/messages", "/api/v1/addons/dashboard/stats", "/api/v1/addons/site-info/info", "/api/v1/addons/casbin-rbac/async-routes"} {
		request("", "GET", path, "", http.StatusUnauthorized, false)
	}
	request("invalid-token", "GET", "/api/v1/addons/example/items", "", http.StatusUnauthorized, false)
	claims := casdoorsdk.Claims{
		User:             casdoorsdk.User{Owner: "built-in", Name: "integration-user", Roles: []*casdoorsdk.Role{{Name: "user"}}},
		RegisteredClaims: jwt.RegisteredClaims{Issuer: casdoor.URL, Audience: jwt.ClaimStrings{"test-client"}, Subject: "integration-user", ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour))},
	}
	token, err := jwt.NewWithClaims(jwt.SigningMethodRS256, claims).SignedString(key)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := casdoorsdk.ParseJwtToken(token); err != nil {
		t.Fatalf("fixture must pass real SDK verification: %v", err)
	}
	// Authentication requires a local family, even for a correctly signed JWT.
	request(token, "GET", "/api/v1/addons/example/items", "", http.StatusUnauthorized, false)
	hash := fmt.Sprintf("%x", sha256.Sum256([]byte(token)))
	if err := global.PRISM_DB.Create(&model.CasdoorSession{
		TokenHash: hash, FamilyID: "scope-test", Subject: claims.Subject, AccessKey: hash,
		AccessExpiresAt: claims.ExpiresAt.Time, ExpiresAt: claims.ExpiresAt.Time, FamilyExpiresAt: claims.ExpiresAt.Time,
	}).Error; err != nil {
		t.Fatal(err)
	}
	request(token, "GET", "/api/v1/addons/example/items", "", http.StatusOK, true)
	request(token, "GET", "/api/v1/addons/site-info/info", "", http.StatusOK, false)
	request(token, "GET", "/api/v1/addons/casdoor-auth/user-info", "", http.StatusOK, false)
	request(token, "GET", "/api/v1/addons/casbin-rbac/async-routes", "", http.StatusOK, false)
	request(token, "POST", "/api/v1/addons/example/items", `{"name":"allowed","description":"integration"}`, http.StatusOK, true)
	request(token, "DELETE", "/api/v1/addons/example/items/1", "", http.StatusForbidden, false)
	request(token, "POST", "/api/v1/addons/messages", `{"content":"allowed","author":"integration-user"}`, http.StatusOK, false)
	request(token, "DELETE", "/api/v1/addons/messages", "", http.StatusForbidden, false)
	request(token, "GET", "/api/v1/addons/example-other/items", "", http.StatusNotFound, false)
}
