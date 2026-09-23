package service

// s3_bootstrap.go — 启动时自动初始化 S3 头像 Bucket，上传默认头像，
// 并在 Casdoor 创建 Storage Provider 使头像上传/下载走 S3。
//
// 执行时机：doBootstrap() 的最后一步（在 SDK 初始化之前，admin session 已就绪）。
// 幂等设计：bucket 已存在则跳过，provider 已存在则同步凭据。

import (
	"bytes"
	"context"
	_ "embed"
	"fmt"
	"net/http"
	"net/url"
	"path/filepath"
	"strings"
	"time"

	"github.com/kwhitestone/prism-fusion/global"
	"top.whitestone/prism-fusion-site/addons/casdoor-auth/conf"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"go.uber.org/zap"
)

// ============================================================
// Embed 默认头像（编译时打包 logo.svg）
// ============================================================

//go:embed assets/default-avatar.svg
var defaultAvatarData []byte

// ============================================================
// 常量
// ============================================================

const (
	avatarBucketName    = "user-avatar"
	defaultAvatarObject = "default-avatar.svg"
	logoObject          = "logo.svg"
	storageProviderName = "provider-storage-s3"
	storageProviderType = "MinIO"
	s3DefaultRegion     = "us-east-1"
)

// ============================================================
// 对外入口
// ============================================================

// initAvatarBucket 初始化头像 Bucket、上传默认头像、创建并绑定 Casdoor Storage Provider。
// adminClient 为已登录 Casdoor 的 HTTP 客户端，endpoint 为 Casdoor 内部地址。
func initAvatarBucket(adminClient *http.Client, casdoorEndpoint, orgName, appName string) {
	cfg := conf.Get()
	s3Endpoint := cfg.S3Endpoint
	accessKey := cfg.S3AccessKey
	secretKey := cfg.S3SecretKey
	publicURL := cfg.S3PublicURL

	if s3Endpoint == "" || accessKey == "" || secretKey == "" {
		global.PRISM_LOG.Info("S3 Bootstrap: skipped (s3-endpoint/access-key/secret-key not configured)")
		return
	}

	global.PRISM_LOG.Info("S3 Bootstrap: initializing avatar bucket...",
		zap.String("endpoint", s3Endpoint),
		zap.String("bucket", avatarBucketName))

	ctx := context.Background()

	// ---- 1. 连接 S3 ----
	minioClient, err := newS3Client(s3Endpoint, accessKey, secretKey)
	if err != nil {
		global.PRISM_LOG.Warn("S3 Bootstrap: failed to create S3 client", zap.Error(err))
		return
	}

	// ---- 2. 确保 bucket 存在 ----
	if err := ensureBucket(ctx, minioClient); err != nil {
		global.PRISM_LOG.Warn("S3 Bootstrap: failed to ensure bucket", zap.Error(err))
		return
	}

	// ---- 3. 设置公开读策略 ----
	if err := setBucketPublicReadPolicy(ctx, minioClient); err != nil {
		global.PRISM_LOG.Warn("S3 Bootstrap: failed to set bucket policy", zap.Error(err))
		// 不 return，继续后续步骤
	}

	// ---- 4. 上传默认头像 & Logo ----
	if err := uploadDefaultAssets(ctx, minioClient); err != nil {
		global.PRISM_LOG.Warn("S3 Bootstrap: failed to upload default assets", zap.Error(err))
		// 不 return，继续创建 provider
	}

	// ---- 5. 在 Casdoor 创建/同步 Storage Provider ----
	// Casdoor 运行在 Docker 容器中，无法访问 localhost，使用公网 URL 作为 S3 endpoint
	ensureStorageProvider(adminClient, casdoorEndpoint, publicURL, accessKey, secretKey, publicURL)

	// ---- 6. 绑定 Storage Provider 到应用 ----
	attachStorageProviderToApp(adminClient, casdoorEndpoint, appName)

	// ---- 7. 更新组织 defaultAvatar 为 S3 URL ----
	avatarURL := defaultAvatarURL(publicURL)
	updateOrgDefaultAvatar(adminClient, casdoorEndpoint, orgName, avatarURL)

	global.PRISM_LOG.Info("S3 Bootstrap: avatar bucket initialized",
		zap.String("defaultAvatarURL", avatarURL))
}

// defaultAvatarURL 返回默认头像的公开访问 URL
func defaultAvatarURL(publicURL string) string {
	if publicURL == "" {
		publicURL = conf.Get().S3PublicURL
	}
	if publicURL == "" {
		return ""
	}
	return fmt.Sprintf("%s/%s/%s", strings.TrimRight(publicURL, "/"), avatarBucketName, defaultAvatarObject)
}

// DefaultAvatarURL 返回默认头像 URL（供 router 等外部包使用）
func DefaultAvatarURL() string {
	publicURL := conf.Get().S3PublicURL
	if publicURL == "" {
		return "/logo.svg" // 回退到本地
	}
	return defaultAvatarURL(publicURL)
}

// logoURL 返回 logo.svg 在 S3 上的公开访问 URL
func logoURL(publicURL string) string {
	if publicURL == "" {
		publicURL = conf.Get().S3PublicURL
	}
	if publicURL == "" {
		return ""
	}
	return fmt.Sprintf("%s/%s/%s", strings.TrimRight(publicURL, "/"), avatarBucketName, logoObject)
}

// LogoURL 返回 Logo URL（供 bootstrap 等外部包使用）
func LogoURL() string {
	publicURL := conf.Get().S3PublicURL
	if publicURL == "" {
		return "" // 无 S3 则返回空，调用方自行回退
	}
	return logoURL(publicURL)
}

// ============================================================
// S3 操作
// ============================================================

// newS3Client 创建 minio-go S3 客户端
// 显式设置 Region 以跳过 getBucketLocation 自动探测（rclone 不支持该 API，
// 返回错误 XML 会导致 region 值含换行符，触发 net/http 拒绝 Authorization 头）。
// 设置 TrailingHeaders: false 禁用 aws-chunked 传输编码，rclone 不会解码
// chunked 分帧，会把 chunk-signature 元数据一并存储，导致文件内容损坏。
func newS3Client(endpoint, accessKey, secretKey string) (*minio.Client, error) {
	// 解析 endpoint 获取 host 和 scheme
	u, err := url.Parse(endpoint)
	if err != nil {
		return nil, fmt.Errorf("parse S3 endpoint: %w", err)
	}

	host := u.Host
	useSSL := u.Scheme == "https"

	return minio.New(host, &minio.Options{
		Creds:           credentials.NewStaticV4(accessKey, secretKey, ""),
		Secure:          useSSL,
		Region:          s3DefaultRegion,
		TrailingHeaders: false,
	})
}

// ensureBucket 确保 bucket 存在，不存在则创建
func ensureBucket(ctx context.Context, client *minio.Client) error {
	exists, err := client.BucketExists(ctx, avatarBucketName)
	if err != nil {
		return fmt.Errorf("check bucket exists: %w", err)
	}
	if exists {
		global.PRISM_LOG.Info("S3 Bootstrap: bucket already exists", zap.String("bucket", avatarBucketName))
		return nil
	}

	err = client.MakeBucket(ctx, avatarBucketName, minio.MakeBucketOptions{
		Region: s3DefaultRegion,
	})
	if err != nil {
		return fmt.Errorf("create bucket: %w", err)
	}
	global.PRISM_LOG.Info("S3 Bootstrap: bucket created", zap.String("bucket", avatarBucketName))
	return nil
}

// setBucketPublicReadPolicy 设置 bucket 的公开读策略
func setBucketPublicReadPolicy(ctx context.Context, client *minio.Client) error {
	policy := fmt.Sprintf(`{
		"Version": "2012-10-17",
		"Statement": [{
			"Effect": "Allow",
			"Principal": {"AWS": ["*"]},
			"Action": ["s3:GetObject"],
			"Resource": ["arn:aws:s3:::%s/*"]
		}]
	}`, avatarBucketName)

	err := client.SetBucketPolicy(ctx, avatarBucketName, policy)
	if err != nil {
		return fmt.Errorf("set bucket policy: %w", err)
	}
	global.PRISM_LOG.Info("S3 Bootstrap: public read policy set", zap.String("bucket", avatarBucketName))
	return nil
}

// uploadDefaultAssets 上传默认头像和 Logo 到 S3（已存在则覆盖）
func uploadDefaultAssets(ctx context.Context, client *minio.Client) error {
	// 同一份 SVG 同时作为 default-avatar.svg 和 logo.svg 上传
	for _, objName := range []string{defaultAvatarObject, logoObject} {
		reader := bytes.NewReader(defaultAvatarData)
		_, err := client.PutObject(ctx, avatarBucketName, objName, reader, int64(len(defaultAvatarData)),
			minio.PutObjectOptions{
				ContentType:          "image/svg+xml",
				DisableContentSha256: true,
			})
		if err != nil {
			return fmt.Errorf("upload %s: %w", objName, err)
		}
		global.PRISM_LOG.Info("S3 Bootstrap: asset uploaded",
			zap.String("object", objName),
			zap.Int("size", len(defaultAvatarData)))
	}
	return nil
}

// ============================================================
// Casdoor Storage Provider 管理
// ============================================================

// ensureStorageProvider 在 Casdoor 中创建或同步 Storage Provider
func ensureStorageProvider(adminClient *http.Client, casdoorEndpoint, s3Endpoint, accessKey, secretKey, publicURL string) {
	domain := fmt.Sprintf("%s/%s", strings.TrimRight(publicURL, "/"), avatarBucketName)

	exists, _ := bsAPIExists(adminClient, casdoorEndpoint,
		fmt.Sprintf("/api/get-provider?id=admin/%s", storageProviderName))

	if exists {
		// 同步凭据
		provData, err := bsAPIGetRaw(adminClient, casdoorEndpoint,
			fmt.Sprintf("/api/get-provider?id=admin/%s", storageProviderName))
		if err == nil {
			needUpdate := false
			if provData["clientId"] != accessKey {
				provData["clientId"] = accessKey
				needUpdate = true
			}
			if provData["clientSecret"] != secretKey {
				provData["clientSecret"] = secretKey
				needUpdate = true
			}
			if provData["domain"] != domain {
				provData["domain"] = domain
				needUpdate = true
			}
			if provData["endpoint"] != s3Endpoint {
				provData["endpoint"] = s3Endpoint
				needUpdate = true
			}
			if provData["type"] != storageProviderType {
				provData["type"] = storageProviderType
				needUpdate = true
			}
			if needUpdate {
				if err := bsAPIPost(adminClient, casdoorEndpoint,
					fmt.Sprintf("/api/update-provider?id=admin/%s", storageProviderName), provData); err != nil {
					global.PRISM_LOG.Warn("S3 Bootstrap: failed to sync storage provider", zap.Error(err))
				} else {
					global.PRISM_LOG.Info("S3 Bootstrap: storage provider synced")
				}
			} else {
				global.PRISM_LOG.Info("S3 Bootstrap: storage provider already up-to-date")
			}
		}
		return
	}

	// 创建新 provider
	global.PRISM_LOG.Info("S3 Bootstrap: creating storage provider...")

	provider := map[string]interface{}{
		"owner":        "admin",
		"name":         storageProviderName,
		"createdTime":  "",
		"displayName":  "S3 Avatar Storage",
		"category":     "Storage",
		"type":         storageProviderType,
		"clientId":     accessKey,
		"clientSecret": secretKey,
		"endpoint":     s3Endpoint,
		"bucket":       avatarBucketName,
		"regionId":     s3DefaultRegion,
		"domain":       domain,
		"pathPrefix":   "",
	}

	err := bsAPIPost(adminClient, casdoorEndpoint,
		fmt.Sprintf("/api/add-provider?id=admin/%s", storageProviderName), provider)
	if err != nil {
		global.PRISM_LOG.Warn("S3 Bootstrap: failed to create storage provider", zap.Error(err))
		return
	}
	global.PRISM_LOG.Info("S3 Bootstrap: storage provider created")
}

// attachStorageProviderToApp 将 Storage Provider 绑定到 Casdoor 应用
func attachStorageProviderToApp(adminClient *http.Client, casdoorEndpoint, appName string) {
	appData, err := bsAPIGetRaw(adminClient, casdoorEndpoint,
		fmt.Sprintf("/api/get-application?id=admin/%s", appName))
	if err != nil {
		global.PRISM_LOG.Warn("S3 Bootstrap: failed to get application for provider binding", zap.Error(err))
		return
	}

	// 检查 providers 列表中是否已绑定
	providers, _ := appData["providers"].([]interface{})
	for _, p := range providers {
		pMap, ok := p.(map[string]interface{})
		if !ok {
			continue
		}
		pName, _ := pMap["name"].(string)
		if pName == storageProviderName {
			global.PRISM_LOG.Info("S3 Bootstrap: storage provider already attached to application")
			return
		}
	}

	// 追加 provider
	providers = append(providers, map[string]interface{}{
		"owner":       "admin",
		"name":        storageProviderName,
		"canSignUp":   false,
		"canSignIn":   false,
		"canUnlink":   false,
		"prompted":    false,
		"signupGroup": "",
		"provider": map[string]interface{}{
			"owner": "admin",
			"name":  storageProviderName,
		},
	})
	appData["providers"] = providers

	if err := bsAPIPost(adminClient, casdoorEndpoint,
		fmt.Sprintf("/api/update-application?id=admin/%s", appName), appData); err != nil {
		global.PRISM_LOG.Warn("S3 Bootstrap: failed to attach storage provider to application", zap.Error(err))
	} else {
		global.PRISM_LOG.Info("S3 Bootstrap: storage provider attached to application", zap.String("app", appName))
	}
}

// updateOrgDefaultAvatar 更新组织的 defaultAvatar 为 S3 上的默认头像 URL
func updateOrgDefaultAvatar(adminClient *http.Client, casdoorEndpoint, orgName, avatarURL string) {
	if avatarURL == "" {
		return
	}

	orgData, err := bsAPIGetRaw(adminClient, casdoorEndpoint,
		fmt.Sprintf("/api/get-organization?id=admin/%s", orgName))
	if err != nil {
		global.PRISM_LOG.Warn("S3 Bootstrap: failed to get organization for avatar update", zap.Error(err))
		return
	}

	if orgData["defaultAvatar"] == avatarURL {
		return
	}

	orgData["defaultAvatar"] = avatarURL
	if err := bsAPIPost(adminClient, casdoorEndpoint,
		fmt.Sprintf("/api/update-organization?id=admin/%s", orgName), orgData); err != nil {
		global.PRISM_LOG.Warn("S3 Bootstrap: failed to update organization defaultAvatar", zap.Error(err))
	} else {
		global.PRISM_LOG.Info("S3 Bootstrap: organization defaultAvatar updated", zap.String("url", avatarURL))
	}
}

// ============================================================
// 头像直传 S3（供运行时头像上传使用）
// ============================================================

// UploadAvatarToS3 将头像文件直接上传到 S3，返回公开访问 URL。
// objectKey 格式: avatar/{username}/avatar.{ext}
func UploadAvatarToS3(username, filename string, data []byte) (string, error) {
	cfg := conf.Get()
	if cfg.S3Endpoint == "" || cfg.S3AccessKey == "" || cfg.S3SecretKey == "" {
		return "", fmt.Errorf("S3 storage not configured")
	}

	client, err := newS3Client(cfg.S3Endpoint, cfg.S3AccessKey, cfg.S3SecretKey)
	if err != nil {
		return "", fmt.Errorf("create S3 client: %w", err)
	}

	ext := filepath.Ext(filename)
	if ext == "" {
		ext = ".png"
	}
	objectKey := fmt.Sprintf("avatar/%s/avatar%s", username, ext)
	contentType := detectContentType(filename, data)

	_, err = client.PutObject(context.Background(), avatarBucketName, objectKey, bytes.NewReader(data), int64(len(data)),
		minio.PutObjectOptions{
			ContentType:          contentType,
			DisableContentSha256: true,
		})
	if err != nil {
		return "", fmt.Errorf("upload to S3: %w", err)
	}

	publicURL := strings.TrimRight(cfg.S3PublicURL, "/")
	return fmt.Sprintf("%s/%s/%s", publicURL, avatarBucketName, objectKey), nil
}

// ============================================================
// Presigned URL（前端直传 S3）
// ============================================================

// presignExpiry 预签名 URL 有效期（尽量缩短窗口）
const presignExpiry = 60 * time.Second

// GeneratePresignedPutURL 生成一个预签名 PUT URL，前端可直接用它将文件上传到 S3。
// 返回：presignedURL（浏览器 PUT 目标）、avatarPublicURL（最终公开访问地址）。
//
// 实现要点：
//   - 使用 publicURL 初始化 minio client，这样签名中的 Host 与浏览器实际请求的 Host 一致。
//   - 设置 Region 避免 getBucketLocation 远程查询。
func GeneratePresignedPutURL(username, filename string) (presignedURL, avatarPublicURL string, err error) {
	cfg := conf.Get()
	if cfg.S3PublicURL == "" || cfg.S3AccessKey == "" || cfg.S3SecretKey == "" {
		return "", "", fmt.Errorf("S3 storage not configured")
	}

	// 使用 publicURL 创建 presign 专用客户端（签名 Host 必须与浏览器请求一致）
	client, err := newPresignClient(cfg.S3PublicURL, cfg.S3AccessKey, cfg.S3SecretKey)
	if err != nil {
		return "", "", fmt.Errorf("create presign S3 client: %w", err)
	}

	// 每个用户只保留一个头像，固定文件名 avatar + 原始扩展名
	ext := filepath.Ext(filename)
	if ext == "" {
		ext = ".png"
	}
	objectKey := fmt.Sprintf("avatar/%s/avatar%s", username, ext)

	presigned, err := client.PresignedPutObject(context.Background(), avatarBucketName, objectKey, presignExpiry)
	if err != nil {
		return "", "", fmt.Errorf("generate presigned URL: %w", err)
	}

	publicBase := strings.TrimRight(cfg.S3PublicURL, "/")
	avatarPublicURL = fmt.Sprintf("%s/%s/%s", publicBase, avatarBucketName, objectKey)

	return presigned.String(), avatarPublicURL, nil
}

// newPresignClient 创建用于生成预签名 URL 的 minio 客户端。
// 使用 publicURL 作为 endpoint，设置 Region 以跳过 getBucketLocation 网络调用。
func newPresignClient(publicURL, accessKey, secretKey string) (*minio.Client, error) {
	u, err := url.Parse(publicURL)
	if err != nil {
		return nil, fmt.Errorf("parse public URL: %w", err)
	}
	return minio.New(u.Host, &minio.Options{
		Creds:  credentials.NewStaticV4(accessKey, secretKey, ""),
		Secure: u.Scheme == "https",
		Region: s3DefaultRegion,
	})
}

// detectContentType 根据文件扩展名或内容推断 MIME 类型
func detectContentType(filename string, data []byte) string {
	ext := strings.ToLower(filename)
	switch {
	case strings.HasSuffix(ext, ".svg"):
		return "image/svg+xml"
	case strings.HasSuffix(ext, ".png"):
		return "image/png"
	case strings.HasSuffix(ext, ".jpg"), strings.HasSuffix(ext, ".jpeg"):
		return "image/jpeg"
	case strings.HasSuffix(ext, ".gif"):
		return "image/gif"
	case strings.HasSuffix(ext, ".webp"):
		return "image/webp"
	default:
		// 用内容探测
		return http.DetectContentType(data)
	}
}
