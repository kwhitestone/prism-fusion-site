package conf

import (
	"os"
	"strings"

	"whitestone.top/prism-fusion/global"
)

// CasdoorConfig Casdoor 认证服务配置（由插件自管理，不放入核心 config 包）
type CasdoorConfig struct {
	Endpoint           string  `mapstructure:"endpoint" json:"endpoint" yaml:"endpoint"`                                     // Casdoor 服务地址
	ClientID           string  `mapstructure:"client-id" json:"client-id" yaml:"client-id"`                                  // 应用 Client ID
	ClientSecret       string  `mapstructure:"client-secret" json:"client-secret" yaml:"client-secret"`                      // 应用 Client Secret
	Certificate        string  `mapstructure:"certificate" json:"certificate" yaml:"certificate"`                            // x509 证书 (PEM)
	OrganizationName   string  `mapstructure:"organization-name" json:"organization-name" yaml:"organization-name"`          // 组织名称
	ApplicationName    string  `mapstructure:"application-name" json:"application-name" yaml:"application-name"`             // 应用名称
	ExternalEndpoint   string  `mapstructure:"external-endpoint" json:"external-endpoint" yaml:"external-endpoint"`          // 外部访问的 Casdoor 地址（浏览器跳转用，留空则与 Endpoint 相同）
	TokenExpireHours   float64 `mapstructure:"token-expire-hours" json:"token-expire-hours" yaml:"token-expire-hours"`       // Access Token 过期时间（小时）
	RefreshExpireHours float64 `mapstructure:"refresh-expire-hours" json:"refresh-expire-hours" yaml:"refresh-expire-hours"` // Refresh Token 过期时间（小时）
	S3Endpoint         string  `mapstructure:"s3-endpoint" json:"s3-endpoint" yaml:"s3-endpoint"`                            // S3 API 地址
	S3AccessKey        string  `mapstructure:"s3-access-key" json:"s3-access-key" yaml:"s3-access-key"`                      // S3 Access Key
	S3SecretKey        string  `mapstructure:"s3-secret-key" json:"s3-secret-key" yaml:"s3-secret-key"`                      // S3 Secret Key
	S3PublicURL        string  `mapstructure:"s3-public-url" json:"s3-public-url" yaml:"s3-public-url"`                      // S3 外部公开 URL
}

// cfg 插件内部持有的配置实例
var cfg CasdoorConfig

// Get 返回 Casdoor 配置引用（供本插件 service 和依赖插件读取）
func Get() *CasdoorConfig {
	return &cfg
}

// Init 从 viper 读取 auth.casdoor 配置段并展开环境变量
func Init() {
	sub := global.PRISM_VP.Sub("auth.casdoor")
	if sub != nil {
		_ = sub.Unmarshal(&cfg)
	}

	// 展开环境变量
	cfg.Endpoint = expandEnv(cfg.Endpoint)
	cfg.ClientID = expandEnv(cfg.ClientID)
	cfg.ClientSecret = expandEnv(cfg.ClientSecret)
	cfg.Certificate = expandEnv(cfg.Certificate)
	cfg.OrganizationName = expandEnv(cfg.OrganizationName)
	cfg.ApplicationName = expandEnv(cfg.ApplicationName)
	cfg.ExternalEndpoint = expandEnv(cfg.ExternalEndpoint)
	// 若未设置外部端点，回退到内部端点
	if cfg.ExternalEndpoint == "" {
		cfg.ExternalEndpoint = cfg.Endpoint
	}

	// S3 存储配置
	cfg.S3Endpoint = expandEnv(cfg.S3Endpoint)
	cfg.S3AccessKey = expandEnv(cfg.S3AccessKey)
	cfg.S3SecretKey = expandEnv(cfg.S3SecretKey)
	cfg.S3PublicURL = expandEnv(cfg.S3PublicURL)
}

// expandEnv 展开 ${VAR} 和 ${VAR:-default} 格式的环境变量
func expandEnv(s string) string {
	if !strings.Contains(s, "${") {
		return s
	}
	return os.Expand(s, func(key string) string {
		if idx := strings.Index(key, ":-"); idx >= 0 {
			envKey := key[:idx]
			defaultVal := key[idx+2:]
			if val := os.Getenv(envKey); val != "" {
				return val
			}
			return defaultVal
		}
		return os.Getenv(key)
	})
}
