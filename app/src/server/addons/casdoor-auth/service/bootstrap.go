package service

// bootstrap.go — 首次启动时自动引导 Casdoor 组织、应用、超管角色/权限/用户
//
// 执行时机：casdoor-auth 插件 InitSDK()，在 SDK 初始化 **之前**。
// 幂等设计：若资源已存在则跳过，不会重复创建。
// 全部使用 HTTP API + admin session，不依赖 Casdoor SDK（因为 SDK 尚未初始化）。

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/kwhitestone/prism-fusion/global"
	"top.whitestone/prism-fusion-site/addons/casdoor-auth/conf"

	"go.uber.org/zap"
)

// bootstrapDone 本进程内只尝试一次 Bootstrap，防止多次重启耗尽 Casdoor 登录次数
var bootstrapDone sync.Once

// BootstrapOrganization 自动引导：创建独立组织、应用、超管角色/权限/用户。
// 仅当 organization-name != "built-in" 时执行。
// 每个进程生命周期内只执行一次，避免反复失败登录耗尽 Casdoor 防暴力破解配额。
func BootstrapOrganization() {
	cfg := conf.Get()
	orgName := cfg.OrganizationName
	appName := cfg.ApplicationName

	if orgName == "built-in" {
		global.PRISM_LOG.Info("Bootstrap skipped: using built-in organization")
		return
	}

	bootstrapDone.Do(func() {
		doBootstrap(cfg.Endpoint, orgName, appName)
	})
}

// ForceBootstrap 强制重新执行引导流程，绕过 sync.Once 限制。
// 用于"重置组织"场景：旧资源已删除，需重建并重新发现 clientId/clientSecret。
// 会同时重置已缓存的 clientId/clientSecret，确保 InitSDK 重新发现新凭据。
func ForceBootstrap() {
	cfg := conf.Get()
	orgName := cfg.OrganizationName
	appName := cfg.ApplicationName

	if orgName == "built-in" {
		global.PRISM_LOG.Info("ForceBootstrap skipped: using built-in organization")
		return
	}

	// 重置 sync.Once，允许下次 BootstrapOrganization() 再次执行
	bootstrapDone = sync.Once{}

	// 清空已缓存的凭据，确保 InitSDK 重新从 Casdoor 发现新 clientId/clientSecret
	cfg.ClientID = ""
	cfg.ClientSecret = ""

	global.PRISM_LOG.Info("ForceBootstrap: 强制重新引导组织...",
		zap.String("org", orgName), zap.String("app", appName))
	doBootstrap(cfg.Endpoint, orgName, appName)
}

func doBootstrap(endpoint, orgName, appName string) {
	global.PRISM_LOG.Info("Bootstrap: checking organization & application...",
		zap.String("org", orgName), zap.String("app", appName))

	// ---- 需要 built-in admin session 来操作跨组织资源 ----
	adminClient, err := bootstrapLogin(endpoint)
	if err != nil {
		global.PRISM_LOG.Warn("Bootstrap: admin login failed, skipping auto-bootstrap",
			zap.Error(err),
			zap.String("hint", "若 Casdoor 管理员密码已修改，请设置环境变量 CASDOOR_ADMIN_PASSWORD；"+
				"若账户被锁定请重置 Casdoor 数据卷: docker-compose down -v && docker-compose up -d casdoor"))
		return
	}

	ep := endpoint

	// 1. 确保组织存在
	if err := ensureOrganization(adminClient, ep, orgName); err != nil {
		global.PRISM_LOG.Error("Bootstrap: failed to ensure organization", zap.Error(err))
		return
	}

	// 2. 确保应用存在并绑定到组织
	if err := ensureApplication(adminClient, ep, orgName, appName); err != nil {
		global.PRISM_LOG.Error("Bootstrap: failed to ensure application", zap.Error(err))
		return
	}

	// 3. 确保应用挂载 Default Captcha Provider（注册页面人机验证必需）
	ensureCaptchaProvider(adminClient, ep, appName)

	// 4. 确保 Email Provider 存在并挂载到应用（QQ 邮箱 SMTP）
	ensureEmailProvider(adminClient, ep, appName)

	// 5. 生成 Logo URL（优先 S3，回退 localhost）并更新组织/应用图标
	logoURL := LogoURL()
	if logoURL == "" {
		logoURL = selfHostedLogoURL() // S3 未配置时回退
	}
	applyLogoToResources(adminClient, ep, orgName, appName, logoURL)

	// 6. 加固 built-in 组织：禁止注册
	hardenBuiltInApp(adminClient, ep)

	// 7. 角色 / 权限 / 用户（全部通过 HTTP API）
	ensureSuperRole(adminClient, ep, orgName)
	ensureSuperPermission(adminClient, ep, orgName)
	ensureSuperAdmin(adminClient, ep, orgName)

	// 8. 注册外部服务的 OIDC 回调地址
	ensureExternalRedirectUris()

	// 9. 初始化 S3 头像 Bucket & Casdoor Storage Provider
	initAvatarBucket(adminClient, ep, orgName, appName)

	// 10. 确保注册页面包含头像上传（依赖步骤 9 的 Storage Provider 已绑定）
	ensureSignupAvatarItem(adminClient, ep, appName)

	global.PRISM_LOG.Info("Bootstrap: organization setup complete",
		zap.String("org", orgName))
}

// ============================================================
// Admin Session（使用 built-in/admin 登录）
// ============================================================

// casdoorDefaultPassword Casdoor 出厂管理员密码
const casdoorDefaultPassword = "123"

// bootstrapLogin 以 built-in/admin 登录 Casdoor，返回带 session cookie 的 HTTP client。
//
// 策略：
//  1. 先用 CASDOOR_ADMIN_PASSWORD 尝试登录（生产环境密码已被修改过的情况）
//  2. 若失败且 CASDOOR_ADMIN_PASSWORD != "123"，则回退用出厂密码 "123" 再试
//  3. 若回退成功，自动将密码修改为 CASDOOR_ADMIN_PASSWORD 并用新密码重新登录
func bootstrapLogin(endpoint string) (*http.Client, error) {
	desiredPassword := os.Getenv("CASDOOR_ADMIN_PASSWORD")
	if desiredPassword == "" {
		desiredPassword = casdoorDefaultPassword
	}

	// 第一次尝试：用期望密码登录
	client, err := doLogin(endpoint, desiredPassword)
	if err == nil {
		return client, nil
	}

	// 若期望密码就是默认密码，无需回退
	if desiredPassword == casdoorDefaultPassword {
		return nil, err
	}

	global.PRISM_LOG.Info("Bootstrap: 期望密码登录失败，尝试出厂密码回退...",
		zap.String("firstError", err.Error()))

	// 第二次尝试：用出厂密码登录
	client, err2 := doLogin(endpoint, casdoorDefaultPassword)
	if err2 != nil {
		return nil, fmt.Errorf("admin login failed with both configured (%v) and default (%v) passwords", err, err2)
	}

	// 回退成功 → 修改密码为期望值
	global.PRISM_LOG.Info("Bootstrap: 出厂密码登录成功，正在修改为配置密码...")
	if chgErr := changeAdminPassword(client, endpoint, casdoorDefaultPassword, desiredPassword); chgErr != nil {
		global.PRISM_LOG.Warn("Bootstrap: 修改管理员密码失败，继续使用出厂密码 session",
			zap.Error(chgErr))
		return client, nil // 仍然返回已登录的 client
	}

	global.PRISM_LOG.Info("Bootstrap: 管理员密码已修改，使用新密码重新登录...")
	// 用新密码重新登录（修改密码后旧 session 可能失效）
	client2, err3 := doLogin(endpoint, desiredPassword)
	if err3 != nil {
		global.PRISM_LOG.Warn("Bootstrap: 新密码重新登录失败，回退到旧 session", zap.Error(err3))
		return client, nil
	}
	return client2, nil
}

// doLogin 以指定密码登录 Casdoor built-in/admin，返回带 session 的 client
func doLogin(endpoint, password string) (*http.Client, error) {
	jar, _ := cookiejar.New(nil)
	client := &http.Client{Jar: jar, Timeout: 15 * time.Second}

	loginReq := map[string]interface{}{
		"type":         "login",
		"application":  "app-built-in",
		"organization": "built-in",
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
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("parse login response: %s", string(respBody))
	}
	if result.Status != "ok" {
		return nil, fmt.Errorf("admin login failed: %s", result.Msg)
	}

	return client, nil
}

// changeAdminPassword 通过 Casdoor /api/set-password 修改 built-in/admin 的密码
func changeAdminPassword(client *http.Client, endpoint, oldPassword, newPassword string) error {
	form := url.Values{
		"userOwner":   {"built-in"},
		"userName":    {"admin"},
		"oldPassword": {oldPassword},
		"newPassword": {newPassword},
	}
	resp, err := client.PostForm(endpoint+"/api/set-password", form)
	if err != nil {
		return fmt.Errorf("set-password request: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	var result struct {
		Status string `json:"status"`
		Msg    string `json:"msg"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return fmt.Errorf("parse set-password response: %s", string(respBody))
	}
	if result.Status != "ok" {
		return fmt.Errorf("set-password failed: %s", result.Msg)
	}
	return nil
}

// syncAppAdminPassword 使用 built-in/admin 的全局管理员权限，
// 将应用组织管理员的密码同步为期望值。
// Casdoor 的 SetPassword 对 isAdmin 用户允许省略 oldPassword。
func syncAppAdminPassword(client *http.Client, endpoint, orgName, userName, desiredPassword string) {
	form := url.Values{
		"userOwner":   {orgName},
		"userName":    {userName},
		"oldPassword": {""}, // 全局管理员可省略
		"newPassword": {desiredPassword},
	}
	resp, err := client.PostForm(endpoint+"/api/set-password", form)
	if err != nil {
		global.PRISM_LOG.Warn("Bootstrap: sync app admin password request failed", zap.Error(err))
		return
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	var result struct {
		Status string `json:"status"`
		Msg    string `json:"msg"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		global.PRISM_LOG.Warn("Bootstrap: parse sync password response failed",
			zap.String("raw", string(respBody)))
		return
	}
	if result.Status != "ok" {
		global.PRISM_LOG.Warn("Bootstrap: sync app admin password failed",
			zap.String("msg", result.Msg))
		return
	}
	global.PRISM_LOG.Info("Bootstrap: app admin password synced to configured value",
		zap.String("org", orgName), zap.String("user", userName))
}

// ============================================================
// 自托管 Logo URL
// ============================================================

// selfHostedLogoURL 返回前端托管的 logo.svg 的绝对 URL。
// 使用系统监听端口生成 localhost URL。
func selfHostedLogoURL() string {
	origin := fmt.Sprintf("http://localhost:%d", global.PRISM_CONFIG.System.Addr)
	logoURL := origin + "/logo.svg"
	global.PRISM_LOG.Info("Bootstrap: using self-hosted logo", zap.String("url", logoURL))
	return logoURL
}

// applyLogoToResources 将已上传的 Logo URL 更新到组织（favicon/defaultAvatar）和应用（logo）
func applyLogoToResources(client *http.Client, endpoint, orgName, appName, logoURL string) {
	// 更新组织的 favicon（defaultAvatar 由 initAvatarBucket 步骤单独管理）
	orgData, err := bsAPIGetRaw(client, endpoint,
		fmt.Sprintf("/api/get-organization?id=admin/%s", orgName))
	if err == nil {
		if orgData["favicon"] != logoURL {
			orgData["favicon"] = logoURL
			if err := bsAPIPost(client, endpoint,
				fmt.Sprintf("/api/update-organization?id=admin/%s", orgName), orgData); err != nil {
				global.PRISM_LOG.Warn("Bootstrap: failed to update organization favicon", zap.Error(err))
			} else {
				global.PRISM_LOG.Info("Bootstrap: updated organization favicon")
			}
		}
	}

	// 更新应用的 logo
	appData, err := bsAPIGetRaw(client, endpoint,
		fmt.Sprintf("/api/get-application?id=admin/%s", appName))
	if err == nil && appData["logo"] != logoURL {
		appData["logo"] = logoURL
		if err := bsAPIPost(client, endpoint,
			fmt.Sprintf("/api/update-application?id=admin/%s", appName), appData); err != nil {
			global.PRISM_LOG.Warn("Bootstrap: failed to update application logo", zap.Error(err))
		} else {
			global.PRISM_LOG.Info("Bootstrap: updated application logo")
		}
	}
}

// ============================================================
// 确保组织存在
// ============================================================

func ensureOrganization(client *http.Client, endpoint, orgName string) error {
	exists, _ := bsAPIExists(client, endpoint,
		fmt.Sprintf("/api/get-organization?id=admin/%s", orgName))
	if exists {
		global.PRISM_LOG.Info("Bootstrap: organization already exists", zap.String("org", orgName))
		return nil
	}

	global.PRISM_LOG.Info("Bootstrap: creating organization...", zap.String("org", orgName))

	newOrg := map[string]interface{}{
		"owner":              "admin",
		"name":               orgName,
		"createdTime":        time.Now().UTC().Format(time.RFC3339),
		"displayName":        orgName,
		"websiteUrl":         fmt.Sprintf("https://%s.example.com", orgName),
		"passwordType":       "plain",
		"passwordOptions":    []string{"AtLeast6"},
		"countryCodes":       []string{"CN", "US"},
		"defaultAvatar":      "",
		"tags":               []string{},
		"languages":          []string{"zh", "en"},
		"masterPassword":     "",
		"initScore":          2000,
		"enableSoftDeletion": false,
		"isProfilePublic":    false,
		"accountItems": []map[string]interface{}{
			{"name": "Organization", "visible": true, "viewRule": "Public", "modifyRule": "Admin"},
			{"name": "ID", "visible": true, "viewRule": "Public", "modifyRule": "Immutable"},
			{"name": "Name", "visible": true, "viewRule": "Public", "modifyRule": "Admin"},
			{"name": "Display name", "visible": true, "viewRule": "Public", "modifyRule": "Self"},
			{"name": "Avatar", "visible": true, "viewRule": "Public", "modifyRule": "Self"},
			{"name": "User type", "visible": true, "viewRule": "Public", "modifyRule": "Admin"},
			{"name": "Password", "visible": true, "viewRule": "Self", "modifyRule": "Self"},
			{"name": "Email", "visible": true, "viewRule": "Public", "modifyRule": "Self"},
			{"name": "Phone", "visible": false, "viewRule": "Admin", "modifyRule": "Admin"},
			{"name": "Country/Region", "visible": false, "viewRule": "Admin", "modifyRule": "Admin"},
			{"name": "Country code", "visible": false, "viewRule": "Admin", "modifyRule": "Admin"},
		},
	}

	return bsAPIPost(client, endpoint,
		fmt.Sprintf("/api/add-organization?id=admin/%s", orgName), newOrg)
}

// ============================================================
// 确保 Default Captcha Provider 存在并挂载到应用
// ============================================================

const captchaProviderName = "provider-captcha-default"
const emailProviderName = "provider-email-smtp-qq"

// emailVerificationTitle 验证码邮件主题
const emailVerificationTitle = "您的验证码"

// emailVerificationContent 验证码邮件 HTML 正文模板。
// Casdoor 替换规则：%s → 验证码，%{user.friendlyName} → 用户昵称。
// 注意：使用 backtick 原始字符串，必须用 UTF-8 中文而非 \u 转义（Go 原始字符串不解释转义）。
// 注意：Casdoor Provider.Content 字段为 varchar(2000)，模板必须控制在 2000 字符以内。
// 此模板使用纯内联样式（无 <style> 块）以兼容更多邮件客户端并缩减字符数。
const emailVerificationContent = `<!DOCTYPE html>
<html lang="zh-CN"><head><meta charset="UTF-8"></head>
<body style="margin:0;padding:0;background:#f5f7fa;font-family:-apple-system,BlinkMacSystemFont,sans-serif">
<div style="padding:40px 20px"><div style="max-width:520px;margin:0 auto;background:#fff;border-radius:12px;box-shadow:0 4px 24px rgba(0,0,0,.08);overflow:hidden">
<div style="background:linear-gradient(135deg,#667eea,#764ba2);padding:32px 40px;text-align:center">
<h1 style="margin:0;color:#fff;font-size:22px;font-weight:600">身份验证</h1></div>
<div style="padding:36px 40px">
<p style="font-size:15px;color:#374151;margin-bottom:16px">你好，<strong>%{user.friendlyName}</strong></p>
<p style="font-size:14px;color:#6b7280;line-height:1.6;margin-bottom:28px">您正在进行身份验证，请使用以下验证码完成操作。验证码 5 分钟内有效，请勿泄露给任何人。</p>
<div style="background:#f3f4f6;border-radius:8px;padding:20px;text-align:center;margin-bottom:28px">
<div style="font-size:36px;font-weight:700;letter-spacing:10px;color:#4f46e5;font-family:Courier New,monospace">%s</div>
<div style="font-size:12px;color:#9ca3af;margin-top:8px">5 分钟内有效</div></div>
<reset-link><p style="text-align:center;margin-bottom:20px">或者点击 <a href="%link" style="color:#4f46e5;font-weight:600">此链接</a> 重设密码</p></reset-link>
<p style="font-size:13px;color:#9ca3af;line-height:1.6;border-top:1px solid #f0f0f0;padding-top:20px">如果这不是您本人的操作，请忽略此邮件。</p>
</div>
<div style="background:#f9fafb;padding:20px 40px;text-align:center;font-size:12px;color:#9ca3af">由系统自动发送，请勿回复</div>
</div></div></body></html>`

// ensureCaptchaProvider 确保 Default 类型 Captcha Provider 存在，并挂载到 appName 的 providers 列表。
// Casdoor 要求应用必须配置 Captcha Provider，否则注册页面提示「无效的 ClientId」。
// Default 类型的验证码不显示滑块，仅作为占位 Provider 满足 Casdoor 内部校验。
func ensureCaptchaProvider(client *http.Client, endpoint, appName string) {
	// 1. 确保全局 Default Captcha Provider 存在（归属 built-in/admin）
	exists, _ := bsAPIExists(client, endpoint,
		fmt.Sprintf("/api/get-provider?id=admin/%s", captchaProviderName))
	if !exists {
		global.PRISM_LOG.Info("Bootstrap: creating Default Captcha provider...")
		newProvider := map[string]interface{}{
			"owner":        "admin",
			"name":         captchaProviderName,
			"createdTime":  time.Now().UTC().Format(time.RFC3339),
			"displayName":  "Default Captcha",
			"category":     "Captcha",
			"type":         "Default",
			"clientId":     "",
			"clientSecret": "",
		}
		if err := bsAPIPost(client, endpoint,
			fmt.Sprintf("/api/add-provider?id=admin/%s", captchaProviderName), newProvider); err != nil {
			global.PRISM_LOG.Warn("Bootstrap: failed to create Default Captcha provider", zap.Error(err))
			return
		}
		global.PRISM_LOG.Info("Bootstrap: Default Captcha provider created")
	} else {
		global.PRISM_LOG.Info("Bootstrap: Default Captcha provider already exists")
	}

	// 2. 获取应用当前 providers 列表，若已包含则跳过
	appData, err := bsAPIGetRaw(client, endpoint,
		fmt.Sprintf("/api/get-application?id=admin/%s", appName))
	if err != nil {
		global.PRISM_LOG.Warn("Bootstrap: cannot read application to attach captcha provider", zap.Error(err))
		return
	}

	providers, _ := appData["providers"].([]interface{})
	for _, p := range providers {
		pm, ok := p.(map[string]interface{})
		if !ok {
			continue
		}
		if pm["name"] == captchaProviderName {
			global.PRISM_LOG.Info("Bootstrap: captcha provider already attached to application")
			return
		}
	}

	// 3. 追加 Provider Item 到应用
	newItem := map[string]interface{}{
		"name":      captchaProviderName,
		"canSignUp": false,
		"canSignIn": false,
		"canUnlink": false,
		"prompted":  false,
		"alertType": "None",
		"rule":      "None",
	}
	appData["providers"] = append(providers, newItem)

	if err := bsAPIPost(client, endpoint,
		fmt.Sprintf("/api/update-application?id=admin/%s", appName), appData); err != nil {
		global.PRISM_LOG.Warn("Bootstrap: failed to attach captcha provider to application", zap.Error(err))
		return
	}
	global.PRISM_LOG.Info("Bootstrap: Default Captcha provider attached to application",
		zap.String("app", appName))
}

// ensureEmailProvider 确保 QQ 邮箱 SMTP Email Provider 存在，并挂载到 appName 的 providers 列表。
// 依赖环境变量：
//   - CASDOOR_QQ_MAIL_SMTP_ACCOUNT  — SMTP 账号（如 xxx@qq.com）
//   - CASDOOR_QQ_MAIL_SMTP_SECRET   — SMTP 授权码
//
// 若两个变量均未设置，则跳过（避免创建无效的空凭据 Provider）。
func ensureEmailProvider(client *http.Client, endpoint, appName string) {
	smtpAccount := os.Getenv("CASDOOR_QQ_MAIL_SMTP_ACCOUNT")
	smtpSecret := os.Getenv("CASDOOR_QQ_MAIL_SMTP_SECRET")
	if smtpAccount == "" || smtpSecret == "" {
		global.PRISM_LOG.Info("Bootstrap: CASDOOR_QQ_MAIL_SMTP_ACCOUNT/SECRET not set, skipping email provider")
		return
	}

	// 1. 确保全局 Email Provider 存在（归属 admin，所有应用共享）
	exists, _ := bsAPIExists(client, endpoint,
		fmt.Sprintf("/api/get-provider?id=admin/%s", emailProviderName))
	if !exists {
		global.PRISM_LOG.Info("Bootstrap: creating QQ SMTP Email provider...")
		newProvider := map[string]interface{}{
			"owner":         "admin",
			"name":          emailProviderName,
			"createdTime":   time.Now().UTC().Format(time.RFC3339),
			"displayName":   "QQ Mail SMTP",
			"category":      "Email",
			"type":          "Default",
			"host":          "smtp.qq.com",
			"port":          465,
			"clientId":      smtpAccount,
			"clientSecret":  smtpSecret,
			"clientSecret2": smtpAccount, // Casdoor 用 clientSecret2 作为 fromName（发件人显示名）
			"sslMode":       "SSL",       // QQ SMTP 465 端口使用 SSL（替代已弃用的 disableSsl）
			"title":         emailVerificationTitle,
			"content":       emailVerificationContent,
		}
		if err := bsAPIPost(client, endpoint,
			fmt.Sprintf("/api/add-provider?id=admin/%s", emailProviderName), newProvider); err != nil {
			global.PRISM_LOG.Warn("Bootstrap: failed to create QQ SMTP Email provider", zap.Error(err))
			return
		}
		global.PRISM_LOG.Info("Bootstrap: QQ SMTP Email provider created")
	} else {
		// Provider 已存在：同步账号/密码/模板（密钥可能已轮换，模板可能待更新）
		providerData, err := bsAPIGetRaw(client, endpoint,
			fmt.Sprintf("/api/get-provider?id=admin/%s", emailProviderName))
		if err == nil {
			needUpdate := false
			if providerData["clientId"] != smtpAccount {
				providerData["clientId"] = smtpAccount
				needUpdate = true
			}
			// clientSecret 在 Casdoor 中可能返回掩码 "***"，始终写入确保一致
			if providerData["clientSecret"] != smtpSecret {
				providerData["clientSecret"] = smtpSecret
				needUpdate = true
			}
			if providerData["title"] != emailVerificationTitle {
				providerData["title"] = emailVerificationTitle
				needUpdate = true
			}
			if providerData["content"] != emailVerificationContent {
				providerData["content"] = emailVerificationContent
				needUpdate = true
			}
			if needUpdate {
				if err := bsAPIPost(client, endpoint,
					fmt.Sprintf("/api/update-provider?id=admin/%s", emailProviderName), providerData); err != nil {
					global.PRISM_LOG.Warn("Bootstrap: failed to sync QQ SMTP provider", zap.Error(err))
				} else {
					global.PRISM_LOG.Info("Bootstrap: QQ SMTP Email provider synced (credentials + template)")
				}
			} else {
				global.PRISM_LOG.Info("Bootstrap: QQ SMTP Email provider already up-to-date")
			}
		}
	}

	// 2. 确保 Provider 挂载到应用
	appData, err := bsAPIGetRaw(client, endpoint,
		fmt.Sprintf("/api/get-application?id=admin/%s", appName))
	if err != nil {
		global.PRISM_LOG.Warn("Bootstrap: cannot read application to attach email provider", zap.Error(err))
		return
	}

	providers, _ := appData["providers"].([]interface{})
	for _, p := range providers {
		pm, ok := p.(map[string]interface{})
		if !ok {
			continue
		}
		if pm["name"] == emailProviderName {
			global.PRISM_LOG.Info("Bootstrap: email provider already attached to application")
			return
		}
	}

	newItem := map[string]interface{}{
		"name":      emailProviderName,
		"canSignUp": false,
		"canSignIn": false,
		"canUnlink": false,
		"prompted":  false,
		"alertType": "None",
		"rule":      "None",
	}
	appData["providers"] = append(providers, newItem)

	if err := bsAPIPost(client, endpoint,
		fmt.Sprintf("/api/update-application?id=admin/%s", appName), appData); err != nil {
		global.PRISM_LOG.Warn("Bootstrap: failed to attach email provider to application", zap.Error(err))
		return
	}
	global.PRISM_LOG.Info("Bootstrap: QQ SMTP Email provider attached to application",
		zap.String("app", appName))
}

// ============================================================
// 注册外部服务的 OIDC 回调地址
// ============================================================

// ensureExternalRedirectUris 注册外部服务的 OIDC 回调地址到 Casdoor 应用。
// 通过环境变量 EXTERNAL_OIDC_REDIRECT_URIS（逗号分隔）获取需要注册的回调地址列表。
// 幂等：已存在的 URI 不会重复添加。
func ensureExternalRedirectUris() {
	raw := strings.TrimSpace(os.Getenv("EXTERNAL_OIDC_REDIRECT_URIS"))
	if raw == "" {
		return
	}
	for _, uri := range strings.Split(raw, ",") {
		uri = strings.TrimSpace(uri)
		if uri != "" {
			EnsureRedirectUri(uri)
		}
	}
}

// ============================================================
// 加固 built-in 组织：禁止注册
// ============================================================

// hardenBuiltInApp 将 app-built-in 的 enableSignUp 设置为 false，
// 防止外部用户在 built-in 组织中注册新管理员账号。
// 此操作幂等，每次 bootstrap 都会检查并修正。
func hardenBuiltInApp(client *http.Client, endpoint string) {
	const builtInApp = "app-built-in"

	appData, err := bsAPIGetRaw(client, endpoint,
		fmt.Sprintf("/api/get-application?id=admin/%s", builtInApp))
	if err != nil {
		global.PRISM_LOG.Warn("Bootstrap: cannot read app-built-in, skipping hardening", zap.Error(err))
		return
	}

	// 检查当前值，避免不必要的写入
	if enabled, ok := appData["enableSignUp"].(bool); ok && !enabled {
		global.PRISM_LOG.Info("Bootstrap: app-built-in sign-up already disabled")
		return
	}

	appData["enableSignUp"] = false
	if err := bsAPIPost(client, endpoint,
		fmt.Sprintf("/api/update-application?id=admin/%s", builtInApp), appData); err != nil {
		global.PRISM_LOG.Warn("Bootstrap: failed to disable sign-up for app-built-in", zap.Error(err))
		return
	}
	global.PRISM_LOG.Info("Bootstrap: disabled sign-up for app-built-in")
}

// ============================================================
// 确保应用存在（绑定到组织，配置 OAuth 回调）
// ============================================================

func ensureApplication(client *http.Client, endpoint, orgName, appName string) error {
	exists, _ := bsAPIExists(client, endpoint,
		fmt.Sprintf("/api/get-application?id=admin/%s", appName))
	if exists {
		global.PRISM_LOG.Info("Bootstrap: application already exists", zap.String("app", appName))
		// 确保 tokenFormat 为 JWT-Standard（防止 Casdoor 默认 tokenFormat 导致
		// 嵌入 Go User 全量字段，其中 address:[] 等非标结构会让第三方 OIDC 客户端解析失败）
		ensureAppTokenFormat(client, endpoint, appName)
		return nil
	}

	global.PRISM_LOG.Info("Bootstrap: creating application...", zap.String("app", appName))

	// 初始 redirectUri 使用 localhost + 系统监听端口，实际生产环境的 URI 由 EnsureRedirectUri 动态注册
	defaultRedirectUri := fmt.Sprintf("http://localhost:%d/login", global.PRISM_CONFIG.System.Addr)

	newApp := map[string]interface{}{
		"owner":                "admin",
		"name":                 appName,
		"createdTime":          time.Now().UTC().Format(time.RFC3339),
		"displayName":          appName,
		"logo":                 "",
		"organization":         orgName,
		"cert":                 "cert-built-in",
		"enablePassword":       true,
		"enableSignUp":         true,
		"enableSigninSession":  true,
		"enableAutoSignin":     false,
		"enableCodeSignin":     false,
		"grantTypes":           []string{"authorization_code", "refresh_token"},
		"tokenFormat":          "JWT-Standard", // 使用标准 OIDC claims，避免嵌入 Go User 全量字段导致 address:[] 等非标结构
		"redirectUris":         []string{defaultRedirectUri},
		"expireInHours":        168, // 7 days
		"refreshExpireInHours": 336, // 14 days
		// signupItems 控制注册页面显示的字段和验证方式。
		// 不设置则 Casdoor 使用默认值（包含手机号+手机验证码）。
		// 此处仅保留：用户名、显示名、密码、邮箱、邮箱验证码、协议。
		"signupItems": []map[string]interface{}{
			{"name": "ID", "visible": false, "required": true, "prompted": false, "rule": "Random"},
			{"name": "Username", "visible": true, "required": true, "prompted": false, "rule": "None"},
			{"name": "Display name", "visible": true, "required": true, "prompted": false, "rule": "None"},
			{"name": "Password", "visible": true, "required": true, "prompted": false, "rule": "None"},
			{"name": "Confirm password", "visible": true, "required": true, "prompted": false, "rule": "None"},
			{"name": "Email", "visible": true, "required": true, "prompted": false, "rule": "Normal"},
			{"name": "Email code", "visible": true, "required": true, "prompted": false, "rule": "None"},
			{"name": "Agreement", "visible": true, "required": true, "prompted": false, "rule": "None"},
		},
	}

	return bsAPIPost(client, endpoint,
		fmt.Sprintf("/api/add-application?id=admin/%s", appName), newApp)
}

// ensureSignupAvatarItem 确保应用的 signupItems 中包含 Avatar 项。
// 若 Storage Provider 已绑定但 signupItems 缺少 Avatar，注册页面不会显示头像上传。
func ensureSignupAvatarItem(client *http.Client, endpoint, appName string) {
	appData, err := bsAPIGetRaw(client, endpoint,
		fmt.Sprintf("/api/get-application?id=admin/%s", appName))
	if err != nil {
		global.PRISM_LOG.Warn("Bootstrap: cannot read application for signupItems check",
			zap.String("app", appName), zap.Error(err))
		return
	}

	// 检查 signupItems 中是否已有 Avatar
	signupItems, _ := appData["signupItems"].([]interface{})
	for _, item := range signupItems {
		if m, ok := item.(map[string]interface{}); ok {
			if name, _ := m["name"].(string); name == "Avatar" {
				global.PRISM_LOG.Info("Bootstrap: signupItems already contains Avatar")
				return
			}
		}
	}

	// 在 Password 之前插入 Avatar（Display name 之后）
	avatarItem := map[string]interface{}{
		"name": "Avatar", "visible": true, "required": false, "prompted": false, "rule": "None",
	}

	inserted := false
	newItems := make([]interface{}, 0, len(signupItems)+1)
	for _, item := range signupItems {
		if m, ok := item.(map[string]interface{}); ok {
			if name, _ := m["name"].(string); name == "Password" && !inserted {
				newItems = append(newItems, avatarItem)
				inserted = true
			}
		}
		newItems = append(newItems, item)
	}
	if !inserted {
		newItems = append(newItems, avatarItem)
	}

	appData["signupItems"] = newItems
	if err := bsAPIPost(client, endpoint,
		fmt.Sprintf("/api/update-application?id=admin/%s", appName), appData); err != nil {
		global.PRISM_LOG.Warn("Bootstrap: failed to add Avatar to signupItems",
			zap.String("app", appName), zap.Error(err))
		return
	}
	global.PRISM_LOG.Info("Bootstrap: Avatar added to signupItems",
		zap.String("app", appName))
}

// ensureAppTokenFormat 确保已有应用的 tokenFormat 为 JWT-Standard。
// Casdoor 默认 tokenFormat 为空（嵌入 Go User 全量字段），其中 address 字段
// 被序列化为 []（空数组），不符合 OIDC 规范（应为 JSON 对象），
// 导致 openidconnect 等标准 OIDC 客户端解析 id_token 失败。
func ensureAppTokenFormat(client *http.Client, endpoint, appName string) {
	const desiredFormat = "JWT-Standard"

	appData, err := bsAPIGetRaw(client, endpoint,
		fmt.Sprintf("/api/get-application?id=admin/%s", appName))
	if err != nil {
		global.PRISM_LOG.Warn("Bootstrap: cannot read application for tokenFormat check",
			zap.String("app", appName), zap.Error(err))
		return
	}

	current, _ := appData["tokenFormat"].(string)
	if current == desiredFormat {
		global.PRISM_LOG.Info("Bootstrap: application tokenFormat already JWT-Standard",
			zap.String("app", appName))
		return
	}

	global.PRISM_LOG.Info("Bootstrap: updating application tokenFormat to JWT-Standard",
		zap.String("app", appName), zap.String("old", current))

	appData["tokenFormat"] = desiredFormat
	if err := bsAPIPost(client, endpoint,
		fmt.Sprintf("/api/update-application?id=admin/%s", appName), appData); err != nil {
		global.PRISM_LOG.Warn("Bootstrap: failed to update tokenFormat",
			zap.String("app", appName), zap.Error(err))
		return
	}
	global.PRISM_LOG.Info("Bootstrap: application tokenFormat updated to JWT-Standard",
		zap.String("app", appName))
}

// ============================================================
// 确保超级管理员角色
// ============================================================

func ensureSuperRole(client *http.Client, endpoint, orgName string) {
	roleName := "role-super-admin"

	exists, _ := bsAPIExists(client, endpoint,
		fmt.Sprintf("/api/get-role?id=%s/%s", orgName, roleName))
	if exists {
		global.PRISM_LOG.Info("Bootstrap: super admin role already exists")
		return
	}

	global.PRISM_LOG.Info("Bootstrap: creating super admin role...")
	role := map[string]interface{}{
		"owner":       orgName,
		"name":        roleName,
		"createdTime": time.Now().UTC().Format(time.RFC3339),
		"displayName": "超级管理员",
		"description": "拥有系统全部权限的超级管理员角色",
		"isEnabled":   true,
		"users":       []string{},
		"roles":       []string{},
	}

	err := bsAPIPost(client, endpoint,
		fmt.Sprintf("/api/add-role?id=%s/%s", orgName, roleName), role)
	if err != nil {
		global.PRISM_LOG.Warn("Bootstrap: failed to create super admin role", zap.Error(err))
	}
}

// ============================================================
// 确保超级管理员权限（全路径 + 全方法）
// ============================================================

func ensureSuperPermission(client *http.Client, endpoint, orgName string) {
	permName := "permission-super-admin"

	exists, _ := bsAPIExists(client, endpoint,
		fmt.Sprintf("/api/get-permission?id=%s/%s", orgName, permName))
	if exists {
		global.PRISM_LOG.Info("Bootstrap: super admin permission already exists")
		return
	}

	global.PRISM_LOG.Info("Bootstrap: creating super admin permission...")
	perm := map[string]interface{}{
		"owner":       orgName,
		"name":        permName,
		"createdTime": time.Now().UTC().Format(time.RFC3339),
		"displayName": "超级管理员权限",
		"description": "拥有所有 API 路径和方法的访问权限",
		"resources":   []string{"/api/v1/*"},
		"actions":     []string{"GET", "POST", "PUT", "DELETE", "PATCH"},
		"roles":       []string{fmt.Sprintf("%s/role-super-admin", orgName)},
		"effect":      "Allow",
		"isEnabled":   true,
	}

	err := bsAPIPost(client, endpoint,
		fmt.Sprintf("/api/add-permission?id=%s/%s", orgName, permName), perm)
	if err != nil {
		global.PRISM_LOG.Warn("Bootstrap: failed to create super admin permission", zap.Error(err))
	}
}

// ============================================================
// 确保超级管理员用户（并分配角色）
// ============================================================

func ensureSuperAdmin(client *http.Client, endpoint, orgName string) {
	adminName := "admin"

	appPassword := os.Getenv("CASDOOR_APP_ADMIN_PASSWORD")
	if appPassword == "" {
		// 回退到 CASDOOR_ADMIN_PASSWORD，再回退到默认值
		appPassword = os.Getenv("CASDOOR_ADMIN_PASSWORD")
		if appPassword == "" {
			appPassword = casdoorDefaultPassword
		}
	}

	exists, _ := bsAPIExists(client, endpoint,
		fmt.Sprintf("/api/get-user?id=%s/%s", orgName, adminName))
	if exists {
		global.PRISM_LOG.Info("Bootstrap: super admin user already exists in org")
		ensureAdminHasRole(client, endpoint, orgName, adminName)
		// 同步密码：使用 built-in/admin 全局管理员权限，无需 oldPassword
		syncAppAdminPassword(client, endpoint, orgName, adminName, appPassword)
		return
	}

	global.PRISM_LOG.Info("Bootstrap: creating super admin user...")

	user := map[string]interface{}{
		"owner":       orgName,
		"name":        adminName,
		"createdTime": time.Now().UTC().Format(time.RFC3339),
		"displayName": "超级管理员",
		"password":    appPassword,
		"avatar":      DefaultAvatarURL(),
		"isAdmin":     true,
		"type":        "normal-user",
		"roles": []map[string]interface{}{
			{
				"owner": orgName,
				"name":  "role-super-admin",
			},
		},
	}

	err := bsAPIPost(client, endpoint,
		fmt.Sprintf("/api/add-user?id=%s/%s", orgName, adminName), user)
	if err != nil {
		global.PRISM_LOG.Warn("Bootstrap: failed to create super admin user", zap.Error(err))
		return
	}

	// 分配角色
	ensureAdminHasRole(client, endpoint, orgName, adminName)
}

// ensureAdminHasRole 确保 admin 用户绑定了 super-admin 角色
func ensureAdminHasRole(client *http.Client, endpoint, orgName, adminName string) {
	roleName := "role-super-admin"

	// 获取角色
	roleData, err := bsAPIGetRaw(client, endpoint,
		fmt.Sprintf("/api/get-role?id=%s/%s", orgName, roleName))
	if err != nil || roleData == nil {
		global.PRISM_LOG.Warn("Bootstrap: cannot find super admin role to bind user")
		return
	}

	// 检查 Users 列表
	userPath := fmt.Sprintf("%s/%s", orgName, adminName)
	users, _ := roleData["users"].([]interface{})
	for _, u := range users {
		if uStr, ok := u.(string); ok && uStr == userPath {
			return // 已绑定
		}
	}

	// 追加用户到角色
	users = append(users, userPath)
	roleData["users"] = users

	err = bsAPIPost(client, endpoint,
		fmt.Sprintf("/api/update-role?id=%s/%s", orgName, roleName), roleData)
	if err != nil {
		global.PRISM_LOG.Warn("Bootstrap: failed to bind super admin role to user", zap.Error(err))
		return
	}
	global.PRISM_LOG.Info("Bootstrap: bound super admin role to admin user")
}

// ============================================================
// 通用 HTTP 辅助函数（使用 admin session）
// ============================================================

// bsAPIDelete 发起 POST 请求删除 Casdoor 资源（Casdoor 的 delete API 使用 POST 方法）
func bsAPIDelete(client *http.Client, endpoint, path string, payload interface{}) error {
	return bsAPIPost(client, endpoint, path, payload)
}

// bsAPIGetList 获取 Casdoor 列表数据（返回 []interface{}）
func bsAPIGetList(client *http.Client, endpoint, path string) ([]interface{}, error) {
	resp, err := client.Get(endpoint + path)
	if err != nil {
		return nil, fmt.Errorf("GET %s: %w", path, err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	var result struct {
		Data []interface{} `json:"data"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("parse response: %s", string(body))
	}
	return result.Data, nil
}

// bsAPIExists 检查 Casdoor 资源是否存在
func bsAPIExists(client *http.Client, endpoint, path string) (bool, error) {
	resp, err := client.Get(endpoint + path)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	var result struct {
		Data json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return false, err
	}
	// Casdoor returns {"data": null} when not found
	return result.Data != nil && string(result.Data) != "null", nil
}

// bsAPIGetRaw 获取 Casdoor 资源的原始 JSON 数据
func bsAPIGetRaw(client *http.Client, endpoint, path string) (map[string]interface{}, error) {
	resp, err := client.Get(endpoint + path)
	if err != nil {
		return nil, fmt.Errorf("GET %s: %w", path, err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	var result struct {
		Data map[string]interface{} `json:"data"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("parse response: %s", string(body))
	}
	if result.Data == nil {
		return nil, fmt.Errorf("resource not found")
	}
	return result.Data, nil
}

// bsAPIPost 发起 POST 请求到 Casdoor API
func bsAPIPost(client *http.Client, endpoint, path string, payload interface{}) error {
	body, _ := json.Marshal(payload)
	resp, err := client.Post(endpoint+path, "application/json", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("POST %s: %w", path, err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	var result struct {
		Status string `json:"status"`
		Msg    string `json:"msg"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return fmt.Errorf("parse response: %s", string(respBody))
	}
	if result.Status != "ok" {
		return fmt.Errorf("API error: %s", result.Msg)
	}
	return nil
}

// ============================================================
// 清理组织：删除组织下所有资源，然后删除组织本身
// ============================================================

// CleanupOrganization 删除 bootstrap 创建的所有 Casdoor 资源。
// 返回已删除资源的摘要和可能的错误。
func CleanupOrganization() (summary []string, err error) {
	cfg := conf.Get()
	orgName := cfg.OrganizationName
	appName := cfg.ApplicationName

	if orgName == "built-in" {
		return nil, fmt.Errorf("cannot cleanup built-in organization")
	}

	adminClient, err := bootstrapLogin(cfg.Endpoint)
	if err != nil {
		return nil, fmt.Errorf("admin login failed: %w", err)
	}

	ep := cfg.Endpoint

	// 按依赖顺序删除：用户 → 权限 → 角色 → 应用 → 组织

	// 1. 删除组织下所有用户
	users, _ := bsAPIGetList(adminClient, ep,
		fmt.Sprintf("/api/get-users?owner=%s", orgName))
	for _, u := range users {
		uMap, ok := u.(map[string]interface{})
		if !ok {
			continue
		}
		name, _ := uMap["name"].(string)
		if name == "" {
			continue
		}
		err := bsAPIDelete(adminClient, ep,
			fmt.Sprintf("/api/delete-user?id=%s/%s", orgName, name),
			uMap)
		if err != nil {
			global.PRISM_LOG.Warn("Cleanup: failed to delete user",
				zap.String("user", name), zap.Error(err))
		} else {
			summary = append(summary, fmt.Sprintf("deleted user: %s/%s", orgName, name))
		}
	}

	// 2. 删除组织下所有权限
	perms, _ := bsAPIGetList(adminClient, ep,
		fmt.Sprintf("/api/get-permissions?owner=%s", orgName))
	for _, p := range perms {
		pMap, ok := p.(map[string]interface{})
		if !ok {
			continue
		}
		name, _ := pMap["name"].(string)
		if name == "" {
			continue
		}
		err := bsAPIDelete(adminClient, ep,
			fmt.Sprintf("/api/delete-permission?id=%s/%s", orgName, name),
			pMap)
		if err != nil {
			global.PRISM_LOG.Warn("Cleanup: failed to delete permission",
				zap.String("permission", name), zap.Error(err))
		} else {
			summary = append(summary, fmt.Sprintf("deleted permission: %s/%s", orgName, name))
		}
	}

	// 3. 删除组织下所有角色
	roles, _ := bsAPIGetList(adminClient, ep,
		fmt.Sprintf("/api/get-roles?owner=%s", orgName))
	for _, r := range roles {
		rMap, ok := r.(map[string]interface{})
		if !ok {
			continue
		}
		name, _ := rMap["name"].(string)
		if name == "" {
			continue
		}
		err := bsAPIDelete(adminClient, ep,
			fmt.Sprintf("/api/delete-role?id=%s/%s", orgName, name),
			rMap)
		if err != nil {
			global.PRISM_LOG.Warn("Cleanup: failed to delete role",
				zap.String("role", name), zap.Error(err))
		} else {
			summary = append(summary, fmt.Sprintf("deleted role: %s/%s", orgName, name))
		}
	}

	// 4. 删除 bootstrap 创建的 Providers（归属 admin，CleanupOrganization 不会自动删除）
	for _, provName := range []string{captchaProviderName, emailProviderName, storageProviderName} {
		provData, provErr := bsAPIGetRaw(adminClient, ep,
			fmt.Sprintf("/api/get-provider?id=admin/%s", provName))
		if provErr == nil && provData != nil {
			err := bsAPIDelete(adminClient, ep,
				fmt.Sprintf("/api/delete-provider?id=admin/%s", provName),
				provData)
			if err != nil {
				global.PRISM_LOG.Warn("Cleanup: failed to delete provider",
					zap.String("provider", provName), zap.Error(err))
			} else {
				summary = append(summary, fmt.Sprintf("deleted provider: %s", provName))
			}
		}
	}

	// 5. 删除应用
	appData, appErr := bsAPIGetRaw(adminClient, ep,
		fmt.Sprintf("/api/get-application?id=admin/%s", appName))
	if appErr == nil && appData != nil {
		err := bsAPIDelete(adminClient, ep,
			fmt.Sprintf("/api/delete-application?id=admin/%s", appName),
			appData)
		if err != nil {
			global.PRISM_LOG.Warn("Cleanup: failed to delete application",
				zap.String("app", appName), zap.Error(err))
		} else {
			summary = append(summary, fmt.Sprintf("deleted application: %s", appName))
		}
	}

	// 6. 删除组织
	orgData, orgErr := bsAPIGetRaw(adminClient, ep,
		fmt.Sprintf("/api/get-organization?id=admin/%s", orgName))
	if orgErr == nil && orgData != nil {
		err := bsAPIDelete(adminClient, ep,
			fmt.Sprintf("/api/delete-organization?id=admin/%s", orgName),
			orgData)
		if err != nil {
			global.PRISM_LOG.Warn("Cleanup: failed to delete organization",
				zap.String("org", orgName), zap.Error(err))
			return summary, fmt.Errorf("failed to delete organization: %w", err)
		}
		summary = append(summary, fmt.Sprintf("deleted organization: %s", orgName))
	}

	global.PRISM_LOG.Info("Cleanup: organization cleanup complete",
		zap.String("org", orgName), zap.Int("items", len(summary)))
	return summary, nil
}
