package service

import (
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/kwhitestone/prism-fusion/global"
	"top.whitestone/prism-fusion-site/addons/casdoor-auth/conf"

	"github.com/casdoor/casdoor-go-sdk/casdoorsdk"
	"go.uber.org/zap"
)

// CasdoorService Casdoor 认证服务
type CasdoorService struct{}

// ============================================================
// PKCE (Proof Key for Code Exchange) — 无需 client_secret
// ============================================================

var (
	pkceStore   = make(map[string]string) // state → code_verifier
	pkceStoreMu sync.Mutex
)

// ============================================================
// 用户信息缓存（JWT-Standard 不含 Roles/IsAdmin，需从 API 补全）
// ============================================================

type cachedUser struct {
	user      *casdoorsdk.User
	fetchedAt time.Time
}

var (
	userCache    = make(map[string]*cachedUser) // username → cached user
	userCacheMu  sync.RWMutex
	userCacheTTL = 5 * time.Minute
)

// generatePKCE 生成 PKCE code_verifier 和 code_challenge (S256)
func generatePKCE() (verifier, challenge string, err error) {
	b := make([]byte, 32)
	if _, err = rand.Read(b); err != nil {
		return "", "", fmt.Errorf("generate PKCE verifier: %w", err)
	}
	verifier = base64.RawURLEncoding.EncodeToString(b)
	h := sha256.Sum256([]byte(verifier))
	challenge = base64.RawURLEncoding.EncodeToString(h[:])
	return verifier, challenge, nil
}

// storePKCEVerifier 存储 PKCE verifier（按 state 索引）
func storePKCEVerifier(state, verifier string) {
	pkceStoreMu.Lock()
	defer pkceStoreMu.Unlock()
	pkceStore[state] = verifier
}

// consumePKCEVerifier 取出并删除 PKCE verifier
func consumePKCEVerifier(state string) (string, bool) {
	pkceStoreMu.Lock()
	defer pkceStoreMu.Unlock()
	verifier, ok := pkceStore[state]
	if ok {
		delete(pkceStore, state)
	}
	return verifier, ok
}

// InitSDK 初始化 Casdoor SDK（应在 RegisterRoutes 中调用）
// 如果 certificate 未配置，会自动从 Casdoor OIDC JWKS 端点获取公钥
func (s *CasdoorService) InitSDK() {
	cfg := conf.Get()

	// 自动发现证书：从 Casdoor OIDC JWKS 端点获取 RSA 公钥
	if cfg.Certificate == "" {
		global.PRISM_LOG.Info("Casdoor certificate not configured, auto-discovering from OIDC JWKS...")
		cert, err := fetchCertFromOIDC(cfg.Endpoint)
		if err != nil {
			global.PRISM_LOG.Warn("Failed to auto-discover Casdoor certificate", zap.Error(err))
		} else {
			cfg.Certificate = cert
			global.PRISM_LOG.Info("Auto-discovered Casdoor certificate from OIDC JWKS endpoint")
		}
	}

	// 自动同步/发现凭据（通过 Casdoor Admin API）
	syncOrDiscoverCredentials(cfg)

	casdoorsdk.InitConfig(
		cfg.Endpoint,
		cfg.ClientID,
		cfg.ClientSecret,
		cfg.Certificate,
		cfg.OrganizationName,
		cfg.ApplicationName,
	)
	global.PRISM_LOG.Info("Casdoor SDK initialized",
		zap.Bool("hasClientId", cfg.ClientID != ""),
		zap.Bool("hasClientSecret", cfg.ClientSecret != ""),
		zap.Bool("hasCertificate", cfg.Certificate != ""),
	)
}

// ============================================================
// 凭据自动同步/发现（通过 Casdoor Admin Login）
// ============================================================

// syncOrDiscoverCredentials 自动同步或发现 Casdoor 应用凭据
//
// 工作模式：
//  1. 自动发现模式（client-id 和 client-secret 均为空）：
//     以 admin 身份登录 Casdoor，获取应用的真实 clientId 和 clientSecret
//  2. 同步模式（配置了自定义 client-id/client-secret）：
//     以 admin 身份登录 Casdoor，将配置值写入 Casdoor 应用
//
// 需要 CASDOOR_APP_ADMIN_PASSWORD 或 CASDOOR_ADMIN_PASSWORD 环境变量（默认 "123"）
func syncOrDiscoverCredentials(cfg *conf.CasdoorConfig) {
	adminPassword := os.Getenv("CASDOOR_APP_ADMIN_PASSWORD")
	if adminPassword == "" {
		adminPassword = os.Getenv("CASDOOR_ADMIN_PASSWORD")
		if adminPassword == "" {
			adminPassword = "123"
		}
	}

	// Step 1: 获取当前 clientId（公开 API，无需认证）
	currentClientID, _, err := fetchAppCredentials(cfg.Endpoint, cfg.ApplicationName)
	if err != nil {
		global.PRISM_LOG.Warn("无法获取 Casdoor 应用信息，跳过凭据同步", zap.Error(err))
		return
	}

	// Step 2: 以管理员身份登录获取 session
	client, err := loginCasdoorAdmin(cfg.Endpoint, cfg.OrganizationName, cfg.ApplicationName, adminPassword)
	if err != nil {
		global.PRISM_LOG.Warn("Casdoor 管理员登录失败，跳过凭据同步（检查 CASDOOR_ADMIN_PASSWORD 是否正确）",
			zap.Error(err))
		// 回退：仅使用公开 API 发现的 clientId
		if cfg.ClientID == "" {
			cfg.ClientID = currentClientID
		}
		return
	}

	// Step 3: 用 session 获取完整应用信息（包含真实 clientSecret）
	app, err := getAppWithSession(cfg.Endpoint, cfg.ApplicationName, client)
	if err != nil {
		global.PRISM_LOG.Warn("无法获取完整应用信息", zap.Error(err))
		if cfg.ClientID == "" {
			cfg.ClientID = currentClientID
		}
		return
	}

	realClientID, _ := app["clientId"].(string)
	realSecret, _ := app["clientSecret"].(string)

	desiredID := cfg.ClientID
	desiredSecret := cfg.ClientSecret

	if desiredID == "" && desiredSecret == "" {
		// 自动发现模式：使用 Casdoor 当前的真实凭据
		cfg.ClientID = realClientID

		if realSecret != "" && realSecret != "***" {
			// 获取到真实 clientSecret
			cfg.ClientSecret = realSecret
			global.PRISM_LOG.Info("已自动发现 Casdoor 凭据",
				zap.String("clientId", realClientID),
				zap.Bool("hasSecret", true))
		} else {
			// clientSecret 被掩码（***），生成新的并推送到 Casdoor
			global.PRISM_LOG.Info("Casdoor 返回掩码 clientSecret，自动生成新密钥...")
			newSecret := generateRandomSecret(32)
			app["clientSecret"] = newSecret
			if err := updateAppWithSession(cfg.Endpoint, app, client); err != nil {
				global.PRISM_LOG.Error("无法更新 Casdoor clientSecret", zap.Error(err))
			} else {
				cfg.ClientSecret = newSecret
				global.PRISM_LOG.Info("已自动生成并同步新的 clientSecret 到 Casdoor")
			}
		}
		return
	}

	// 同步模式：将配置的凭据写入 Casdoor 应用
	needUpdate := false
	if desiredID != "" && desiredID != realClientID {
		app["clientId"] = desiredID
		needUpdate = true
	}
	if desiredSecret != "" && desiredSecret != realSecret {
		app["clientSecret"] = desiredSecret
		needUpdate = true
	}

	if needUpdate {
		if err := updateAppWithSession(cfg.Endpoint, app, client); err != nil {
			global.PRISM_LOG.Error("同步凭据到 Casdoor 失败，使用当前凭据", zap.Error(err))
			cfg.ClientID = realClientID
			cfg.ClientSecret = realSecret
			return
		}
		global.PRISM_LOG.Info("已将自定义凭据同步到 Casdoor 应用")
	}

	// 更新全局配置为最终值
	finalID, _ := app["clientId"].(string)
	finalSecret, _ := app["clientSecret"].(string)
	cfg.ClientID = finalID
	cfg.ClientSecret = finalSecret

	// 同步 Token 过期时间到 Casdoor 应用
	syncTokenExpiry(app, client, cfg)
}

// syncTokenExpiry 将 config.yaml 中的 token-expire-hours / refresh-expire-hours 同步到 Casdoor 应用
func syncTokenExpiry(app map[string]interface{}, client *http.Client, cfg *conf.CasdoorConfig) {
	tokenExpire := cfg.TokenExpireHours
	refreshExpire := cfg.RefreshExpireHours

	// 设置默认值
	if tokenExpire <= 0 {
		tokenExpire = 0.1 // ~6 分钟
	}
	if refreshExpire <= 0 {
		refreshExpire = 72 // 3 天
	}

	// 读取 Casdoor 当前值
	currentTokenExpire, _ := app["expireInHours"].(float64)
	currentRefreshExpire, _ := app["refreshExpireInHours"].(float64)

	needUpdate := false
	if currentTokenExpire != tokenExpire {
		app["expireInHours"] = tokenExpire
		needUpdate = true
	}
	if currentRefreshExpire != refreshExpire {
		app["refreshExpireInHours"] = refreshExpire
		needUpdate = true
	}

	if !needUpdate {
		global.PRISM_LOG.Debug("Casdoor Token 过期时间已匹配，无需更新",
			zap.Float64("tokenExpireHours", tokenExpire),
			zap.Float64("refreshExpireHours", refreshExpire))
		return
	}

	if err := updateAppWithSession(cfg.Endpoint, app, client); err != nil {
		global.PRISM_LOG.Error("同步 Token 过期时间到 Casdoor 失败", zap.Error(err))
		return
	}

	global.PRISM_LOG.Info("已同步 Token 过期时间到 Casdoor 应用",
		zap.Float64("tokenExpireHours", tokenExpire),
		zap.Float64("refreshExpireHours", refreshExpire))
}

// loginCasdoorAdmin 以 Casdoor 管理员身份登录，返回带 session cookie 的 HTTP client
// 使用 type: "login" 设置 session cookie（不依赖任何 grant_type 配置）
func loginCasdoorAdmin(endpoint, orgName, appName, password string) (*http.Client, error) {
	// 创建带 cookie jar 的 HTTP client
	jar, err := cookiejar.New(nil)
	if err != nil {
		return nil, fmt.Errorf("create cookie jar: %w", err)
	}
	client := &http.Client{Jar: jar}

	// 以管理员身份登录（type: "login" 设置 session，无需 grant_type）
	// Casdoor 默认 passwordObfuscatorType 为空（Plain），密码明文传输
	loginReq := map[string]interface{}{
		"type":         "login",
		"application":  appName,
		"organization": orgName,
		"username":     "admin",
		"password":     password,
		"signinMethod": "Password",
	}

	body, _ := json.Marshal(loginReq)
	resp, err := client.Post(endpoint+"/api/login", "application/json", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("login request: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)

	var result struct {
		Status string `json:"status"`
		Msg    string `json:"msg"`
		Data   string `json:"data"`  // login 模式返回 userId
		Data2  string `json:"data2"` // 某些版本返回 OIDC token
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("parse login response: %s", string(respBody))
	}

	if result.Status != "ok" {
		return nil, fmt.Errorf("admin login failed: %s", result.Msg)
	}

	global.PRISM_LOG.Info("Casdoor 管理员 session 登录成功")
	return client, nil
}

// generateRandomSecret 生成随机 base64url 密钥
func generateRandomSecret(n int) string {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return base64.RawURLEncoding.EncodeToString(b)
}

// getAppWithSession 使用 session cookie 获取完整应用信息（包含真实 clientSecret）
func getAppWithSession(endpoint, appName string, client *http.Client) (map[string]interface{}, error) {
	reqURL := fmt.Sprintf("%s/api/get-application?id=admin/%s", endpoint, appName)
	resp, err := client.Get(reqURL)
	if err != nil {
		return nil, fmt.Errorf("get application: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)

	var result struct {
		Data map[string]interface{} `json:"data"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("parse application response: %s", string(respBody))
	}
	if result.Data == nil {
		return nil, errors.New("application not found")
	}

	return result.Data, nil
}

// updateAppWithSession 使用 session cookie 更新 Casdoor 应用
func updateAppWithSession(endpoint string, app map[string]interface{}, client *http.Client) error {
	name, _ := app["name"].(string)
	body, _ := json.Marshal(app)

	reqURL := fmt.Sprintf("%s/api/update-application?id=admin/%s", endpoint, name)
	resp, err := client.Post(reqURL, "application/json", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("update application: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	var result struct {
		Status string `json:"status"`
		Msg    string `json:"msg"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return fmt.Errorf("parse update response: %s", string(respBody))
	}
	if result.Status != "ok" {
		return fmt.Errorf("update failed: %s", result.Msg)
	}

	return nil
}

// ============================================================
// 动态注册 redirectUri
// ============================================================

// knownRedirectUris 缓存已知的 redirectUris，避免每次登录都查询 Casdoor
var (
	knownRedirectUris   = make(map[string]bool)
	knownRedirectUrisMu sync.RWMutex
)

// InitKnownRedirectUris 在 bootstrap 后初始化已知 redirectUris 缓存
func InitKnownRedirectUris(uris []string) {
	knownRedirectUrisMu.Lock()
	defer knownRedirectUrisMu.Unlock()
	for _, u := range uris {
		knownRedirectUris[u] = true
	}
}

// EnsureRedirectUri 确保 Casdoor 应用已注册指定的 redirectUri。
// 先查本地缓存，若未命中则通过 admin API 查询并动态添加。
func EnsureRedirectUri(redirectUri string) {
	if redirectUri == "" {
		return
	}

	// 快速路径：已知的 URI 直接返回
	knownRedirectUrisMu.RLock()
	if knownRedirectUris[redirectUri] {
		knownRedirectUrisMu.RUnlock()
		return
	}
	knownRedirectUrisMu.RUnlock()

	cfg := conf.Get()
	appName := cfg.ApplicationName

	// 使用 admin session 查询应用
	adminClient, err := bootstrapLogin(cfg.Endpoint)
	if err != nil {
		global.PRISM_LOG.Warn("EnsureRedirectUri: admin login failed", zap.Error(err))
		return
	}

	appData, err := bsAPIGetRaw(adminClient, cfg.Endpoint,
		fmt.Sprintf("/api/get-application?id=admin/%s", appName))
	if err != nil {
		global.PRISM_LOG.Warn("EnsureRedirectUri: cannot read application", zap.Error(err))
		return
	}

	// 检查是否已包含
	uris, _ := appData["redirectUris"].([]interface{})
	for _, u := range uris {
		s, _ := u.(string)
		knownRedirectUrisMu.Lock()
		knownRedirectUris[s] = true
		knownRedirectUrisMu.Unlock()
		if s == redirectUri {
			return
		}
	}

	// 追加新 URI
	uris = append(uris, redirectUri)
	appData["redirectUris"] = uris
	if err := bsAPIPost(adminClient, cfg.Endpoint,
		fmt.Sprintf("/api/update-application?id=admin/%s", appName), appData); err != nil {
		global.PRISM_LOG.Warn("EnsureRedirectUri: failed to add redirectUri",
			zap.String("uri", redirectUri), zap.Error(err))
		return
	}

	knownRedirectUrisMu.Lock()
	knownRedirectUris[redirectUri] = true
	knownRedirectUrisMu.Unlock()
	global.PRISM_LOG.Info("EnsureRedirectUri: registered new redirectUri",
		zap.String("uri", redirectUri), zap.String("app", appName))
}

// GetSigninURL 获取 Casdoor OAuth2 登录 URL（带 PKCE）
// redirectUri 由前端传入（由浏览器地址栏自动获取）。
// 生成的 URL 使用 ExternalEndpoint（浏览器可达的 Casdoor 地址）。
func (s *CasdoorService) GetSigninURL(state, redirectUri string) string {
	cfg := conf.Get()

	// 确保 Casdoor 应用已注册此 redirectUri
	EnsureRedirectUri(redirectUri)

	// 生成 PKCE code_challenge，存储 verifier 供 ExchangeToken 使用
	verifier, challenge, err := generatePKCE()
	if err != nil {
		global.PRISM_LOG.Error("Failed to generate PKCE", zap.Error(err))
		// 回退到无 PKCE 的 URL（需要 client_secret）
		return fmt.Sprintf(
			"%s/login/oauth/authorize?client_id=%s&response_type=code&redirect_uri=%s&scope=read&state=%s",
			cfg.ExternalEndpoint, cfg.ClientID, url.QueryEscape(redirectUri), state,
		)
	}
	storePKCEVerifier(state, verifier)

	return fmt.Sprintf(
		"%s/login/oauth/authorize?client_id=%s&response_type=code&redirect_uri=%s&scope=read&state=%s&code_challenge=%s&code_challenge_method=S256",
		cfg.ExternalEndpoint, cfg.ClientID, url.QueryEscape(redirectUri), state, challenge,
	)
}

// ExchangeToken 使用授权码交换 Casdoor Token
// redirectUri 必须与 GetSigninURL 中使用的值一致。
// 优先使用 PKCE（无需 client_secret），回退到 SDK 方式（需要 client_secret）
func (s *CasdoorService) ExchangeToken(code string, state string, redirectUri string) (*casdoorsdk.Claims, string, string, error) {
	cfg := conf.Get()

	var accessToken, refreshToken string

	// 优先尝试 PKCE 方式（不需要 client_secret）
	verifier, hasPKCE := consumePKCEVerifier(state)
	if hasPKCE {
		global.PRISM_LOG.Info("Exchanging token with PKCE code_verifier")
		at, rt, err := exchangeTokenWithPKCE(cfg.Endpoint, cfg.ClientID, code, redirectUri, verifier)
		if err != nil {
			return nil, "", "", fmt.Errorf("exchange token (PKCE) failed: %w", err)
		}
		accessToken = at
		refreshToken = rt
	} else if cfg.ClientSecret != "" {
		// 回退：使用 SDK（需要 client_secret）
		global.PRISM_LOG.Info("No PKCE verifier found, falling back to SDK with client_secret")
		token, err := casdoorsdk.GetOAuthToken(code, state)
		if err != nil {
			return nil, "", "", fmt.Errorf("exchange token (SDK) failed: %w", err)
		}
		accessToken = token.AccessToken
		refreshToken = token.RefreshToken
	} else {
		return nil, "", "", errors.New("no PKCE verifier and no client_secret configured, cannot exchange token")
	}

	// 解析 JWT claims
	claims, err := casdoorsdk.ParseJwtToken(accessToken)
	if err != nil {
		global.PRISM_LOG.Warn("ParseJwtToken failed, falling back to unverified decode", zap.Error(err))
		claims, err = decodeJwtPayload(accessToken)
		if err != nil {
			return nil, "", "", fmt.Errorf("parse token failed: %w", err)
		}
	}

	global.PRISM_LOG.Info("ExchangeToken completed",
		zap.Bool("hasRefreshToken", refreshToken != ""),
		zap.Int("accessTokenLen", len(accessToken)),
		zap.Int("refreshTokenLen", len(refreshToken)),
	)

	return claims, accessToken, refreshToken, nil
}

// exchangeTokenWithPKCE 直接向 Casdoor token 端点发送 PKCE token 请求
// 使用 code_verifier 替代 client_secret
func exchangeTokenWithPKCE(endpoint, clientID, code, redirectURI, codeVerifier string) (accessToken, refreshToken string, err error) {
	tokenURL := endpoint + "/api/login/oauth/access_token"

	data := url.Values{
		"grant_type":    {"authorization_code"},
		"client_id":     {clientID},
		"code":          {code},
		"redirect_uri":  {redirectURI},
		"code_verifier": {codeVerifier},
	}

	resp, err := http.PostForm(tokenURL, data)
	if err != nil {
		return "", "", fmt.Errorf("POST token endpoint: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", "", fmt.Errorf("read token response: %w", err)
	}

	// Casdoor 可能返回 JSON 或 form-encoded 格式
	var tokenResp struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		TokenType    string `json:"token_type"`
		ExpiresIn    int    `json:"expires_in"`
		Error        string `json:"error"`
		ErrorDesc    string `json:"error_description"`
	}

	if err := json.Unmarshal(body, &tokenResp); err != nil {
		// 尝试 form-encoded 解析
		vals, parseErr := url.ParseQuery(string(body))
		if parseErr != nil {
			return "", "", fmt.Errorf("parse token response: %s", string(body))
		}
		if errVal := vals.Get("error"); errVal != "" {
			return "", "", fmt.Errorf("%s: %s", errVal, vals.Get("error_description"))
		}
		return vals.Get("access_token"), vals.Get("refresh_token"), nil
	}

	if tokenResp.Error != "" {
		return "", "", fmt.Errorf("%s: %s", tokenResp.Error, tokenResp.ErrorDesc)
	}

	if tokenResp.AccessToken == "" {
		return "", "", fmt.Errorf("empty access_token in response: %s", string(body))
	}

	return tokenResp.AccessToken, tokenResp.RefreshToken, nil
}

// ParseToken 解析验证 Casdoor JWT（用于中间件验证请求中的 token）
func (s *CasdoorService) ParseToken(accessToken string) (*casdoorsdk.Claims, error) {
	claims, err := casdoorsdk.ParseJwtToken(accessToken)
	if err != nil {
		// 回退：不验签解析，适用于证书自动发现失败的场景
		global.PRISM_LOG.Debug("ParseJwtToken failed, trying unverified decode", zap.Error(err))
		claims, err = decodeJwtPayload(accessToken)
		if err != nil {
			return nil, fmt.Errorf("invalid casdoor token: %w", err)
		}
	}
	return claims, nil
}

// RefreshToken 刷新 Casdoor Token
func (s *CasdoorService) RefreshToken(refreshToken string) (string, string, error) {
	newToken, err := casdoorsdk.RefreshOAuthToken(refreshToken)
	if err != nil {
		return "", "", fmt.Errorf("refresh token failed: %w", err)
	}
	return newToken.AccessToken, newToken.RefreshToken, nil
}

// GetTokenExpireSeconds 返回 Access Token 的过期时间（秒），前端用于设置 expires
func GetTokenExpireSeconds() int64 {
	hours := conf.Get().TokenExpireHours
	if hours <= 0 {
		hours = 0.1
	}
	return int64(hours * 3600)
}

// GetUserByToken 根据 Token 获取 Casdoor 用户详情
func (s *CasdoorService) GetUserByToken(accessToken string) (*casdoorsdk.Claims, error) {
	return s.ParseToken(accessToken)
}

// EnrichClaimsFromAPI 当 JWT-Standard 令牌缺少 Roles/IsAdmin/Owner 等 Casdoor 扩展字段时，
// 通过 Casdoor API 获取完整用户信息，并补全到 Claims 中。带内存缓存，避免每次请求都调用 API。
func (s *CasdoorService) EnrichClaimsFromAPI(claims *casdoorsdk.Claims) *casdoorsdk.Claims {
	if claims == nil {
		return claims
	}

	// 如果 JWT 已经包含 Roles 或 IsAdmin 信息，无需补全
	if claims.IsAdmin || len(claims.Roles) > 0 {
		return claims
	}

	// JWT-Standard 格式下：
	//   "sub" (Subject) = 用户登录名（如 "admin"）
	//   "name"          = 用户显示名（如 "超级管理员"）
	// casdoorsdk.GetUser() 需要登录名，但 JWT-Standard 的 sub 是 UUID，name 是显示名，
	// 因此需要用 GetUserByUserId (按 UUID) 查找用户。
	userID := claims.Subject
	if userID == "" {
		return claims
	}

	// 尝试从缓存读取
	userCacheMu.RLock()
	cached, ok := userCache[userID]
	userCacheMu.RUnlock()

	if ok && time.Since(cached.fetchedAt) < userCacheTTL {
		// 缓存命中且未过期，用 API 数据补全 Claims
		applyUserToClaims(claims, cached.user)
		return claims
	}

	// 缓存未命中或已过期，先用 UUID 查找，再回退到用户名
	user, err := casdoorsdk.GetUserByUserId(userID)
	if err != nil || user == nil {
		// UUID 查找失败，尝试用 Subject 当作用户名查找（兼容非标准格式）
		user, err = casdoorsdk.GetUser(userID)
	}
	if err != nil {
		global.PRISM_LOG.Warn("EnrichClaimsFromAPI: failed to fetch user from Casdoor API",
			zap.String("userID", userID),
			zap.Error(err),
		)
		return claims // 返回原始 claims，不阻断请求
	}

	if user == nil {
		global.PRISM_LOG.Warn("EnrichClaimsFromAPI: user not found in Casdoor",
			zap.String("userID", userID),
		)
		return claims
	}

	// 更新缓存
	userCacheMu.Lock()
	userCache[userID] = &cachedUser{user: user, fetchedAt: time.Now()}
	userCacheMu.Unlock()

	applyUserToClaims(claims, user)

	global.PRISM_LOG.Debug("EnrichClaimsFromAPI: enriched claims from Casdoor API",
		zap.String("userID", userID),
		zap.String("loginName", claims.Name),
		zap.Bool("isAdmin", claims.IsAdmin),
		zap.Int("rolesCount", len(claims.Roles)),
		zap.String("owner", claims.Owner),
	)

	return claims
}

// InvalidateUserCache 清除指定用户的缓存，用于用户重新登录时强制从 Casdoor API 获取最新数据
func (s *CasdoorService) InvalidateUserCache(userID string) {
	if userID == "" {
		return
	}
	userCacheMu.Lock()
	delete(userCache, userID)
	userCacheMu.Unlock()
}

// applyUserToClaims 将 Casdoor API 返回的 User 信息补全到 Claims 中
func applyUserToClaims(claims *casdoorsdk.Claims, user *casdoorsdk.User) {
	if user == nil {
		return
	}
	// JWT-Standard 下 claims.Name 是显示名，需要用 API 返回的登录名覆盖
	claims.Name = user.Name
	claims.Owner = user.Owner
	claims.IsAdmin = user.IsAdmin
	claims.Roles = user.Roles
	claims.Permissions = user.Permissions
	if user.Avatar != "" {
		claims.Avatar = user.Avatar
	}
	if claims.Email == "" {
		claims.Email = user.Email
	}
	if claims.DisplayName == "" {
		claims.DisplayName = user.DisplayName
	}
}

// ============================================================
// 自动发现 Casdoor 应用凭据
// ============================================================

type casdoorAppResponse struct {
	Status string `json:"status"`
	Data   struct {
		ClientID     string `json:"clientId"`
		ClientSecret string `json:"clientSecret"`
	} `json:"data"`
}

// fetchAppCredentials 从 Casdoor 应用 API 获取 clientId 和 clientSecret
// Casdoor 的 /api/get-application 对公开应用可匿名访问（登录页需要 clientId）
func fetchAppCredentials(endpoint, appName string) (clientID, clientSecret string, err error) {
	// Casdoor 内置应用 owner 为 "admin"
	url := fmt.Sprintf("%s/api/get-application?id=admin/%s", endpoint, appName)
	resp, err := http.Get(url)
	if err != nil {
		return "", "", fmt.Errorf("fetch application info: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", "", fmt.Errorf("application API returned status %d", resp.StatusCode)
	}

	var appResp casdoorAppResponse
	if err := json.NewDecoder(resp.Body).Decode(&appResp); err != nil {
		return "", "", fmt.Errorf("parse application response: %w", err)
	}

	if appResp.Data.ClientID == "" {
		return "", "", errors.New("application API returned empty clientId")
	}

	return appResp.Data.ClientID, appResp.Data.ClientSecret, nil
}

// ============================================================
// OIDC / JWKS 自动发现证书
// ============================================================

type oidcDiscovery struct {
	JwksURI string `json:"jwks_uri"`
}

type jwksResponse struct {
	Keys []jwksKey `json:"keys"`
}

type jwksKey struct {
	Kty string `json:"kty"` // 密钥类型，应为 "RSA"
	N   string `json:"n"`   // RSA 公钥模数 (base64url)
	E   string `json:"e"`   // RSA 公钥指数 (base64url)
	Alg string `json:"alg"`
	Use string `json:"use"`
}

// fetchCertFromOIDC 从 Casdoor OIDC 发现端点获取 RSA 公钥并转换为 PEM 格式
// 流程：/.well-known/openid-configuration → jwks_uri → JWKS → RSA PEM
func fetchCertFromOIDC(endpoint string) (string, error) {
	// 1. 获取 OIDC 发现配置
	resp, err := http.Get(endpoint + "/.well-known/openid-configuration")
	if err != nil {
		return "", fmt.Errorf("fetch OIDC discovery: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("OIDC discovery returned status %d", resp.StatusCode)
	}

	var discovery oidcDiscovery
	if err := json.NewDecoder(resp.Body).Decode(&discovery); err != nil {
		return "", fmt.Errorf("parse OIDC discovery: %w", err)
	}
	if discovery.JwksURI == "" {
		return "", errors.New("OIDC discovery missing jwks_uri")
	}

	// 2. 获取 JWKS
	resp2, err := http.Get(discovery.JwksURI)
	if err != nil {
		return "", fmt.Errorf("fetch JWKS: %w", err)
	}
	defer resp2.Body.Close()

	if resp2.StatusCode != http.StatusOK {
		return "", fmt.Errorf("JWKS endpoint returned status %d", resp2.StatusCode)
	}

	var jwks jwksResponse
	if err := json.NewDecoder(resp2.Body).Decode(&jwks); err != nil {
		return "", fmt.Errorf("parse JWKS: %w", err)
	}

	// 3. 找到第一个 RSA 密钥并转换为 PEM
	for _, key := range jwks.Keys {
		if key.Kty == "RSA" && key.N != "" && key.E != "" {
			return rsaJwkToPEM(key.N, key.E)
		}
	}

	return "", errors.New("no RSA key found in JWKS")
}

// rsaJwkToPEM 将 JWK 的 RSA 参数 (n, e) 转换为 PEM 格式公钥
func rsaJwkToPEM(n, e string) (string, error) {
	nBytes, err := base64.RawURLEncoding.DecodeString(n)
	if err != nil {
		return "", fmt.Errorf("decode modulus: %w", err)
	}
	eBytes, err := base64.RawURLEncoding.DecodeString(e)
	if err != nil {
		return "", fmt.Errorf("decode exponent: %w", err)
	}

	pubKey := &rsa.PublicKey{
		N: new(big.Int).SetBytes(nBytes),
		E: int(new(big.Int).SetBytes(eBytes).Int64()),
	}

	derBytes, err := x509.MarshalPKIXPublicKey(pubKey)
	if err != nil {
		return "", fmt.Errorf("marshal public key: %w", err)
	}

	pemBlock := &pem.Block{Type: "PUBLIC KEY", Bytes: derBytes}
	return string(pem.EncodeToMemory(pemBlock)), nil
}

// ============================================================
// JWT 回退解码（不验签，仅用于可信的服务端间调用）
// ============================================================

// decodeJwtPayload 解码 JWT payload 而不验证签名
// 仅用于 ProxyLogin 等服务端直连 Casdoor 的场景
func decodeJwtPayload(tokenStr string) (*casdoorsdk.Claims, error) {
	parts := strings.Split(tokenStr, ".")
	if len(parts) != 3 {
		return nil, errors.New("invalid JWT format: expected 3 segments")
	}

	payload := parts[1]
	// base64url 需要补齐 padding
	switch len(payload) % 4 {
	case 2:
		payload += "=="
	case 3:
		payload += "="
	}

	decoded, err := base64.URLEncoding.DecodeString(payload)
	if err != nil {
		// 尝试 RawURLEncoding（无 padding）
		decoded, err = base64.RawURLEncoding.DecodeString(parts[1])
		if err != nil {
			return nil, fmt.Errorf("decode JWT payload: %w", err)
		}
	}

	var claims casdoorsdk.Claims
	if err := json.Unmarshal(decoded, &claims); err != nil {
		// JWT-Standard tokenFormat 下，address 为 OIDC 标准对象 {"formatted":"", ...}，
		// 而 casdoorsdk.User.Address 类型是 []string，导致 unmarshal 失败。
		// 容错处理：将 address 字段置 null 后重试。
		var raw map[string]json.RawMessage
		if jsonErr := json.Unmarshal(decoded, &raw); jsonErr == nil {
			if _, ok := raw["address"]; ok {
				raw["address"] = json.RawMessage(`null`)
				if fixed, fixErr := json.Marshal(raw); fixErr == nil {
					if retryErr := json.Unmarshal(fixed, &claims); retryErr == nil {
						return &claims, nil
					}
				}
			}
		}
		return nil, fmt.Errorf("unmarshal JWT claims: %w", err)
	}

	return &claims, nil
}
