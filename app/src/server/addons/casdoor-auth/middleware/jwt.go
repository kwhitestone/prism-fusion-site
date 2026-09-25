package middleware

import (
	"net/http"
	"strings"

	"github.com/kwhitestone/prism-fusion/global"
	casdoorService "top.whitestone/prism-fusion-site/addons/casdoor-auth/service"

	"github.com/casdoor/casdoor-go-sdk/casdoorsdk"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// 不需要认证的路径白名单
var publicPaths = []string{
	"/api/v1/addons/casdoor-auth/config",
	"/api/v1/addons/casdoor-auth/signin-url",
	"/api/v1/addons/casdoor-auth/signin-callback",
	"/api/v1/addons/casdoor-auth/refresh-token",
	"/health",
	"/openapi",
	"/docs",
	"/scalar",
}

// CasdoorJwtMiddleware Casdoor JWT 认证全局中间件
// 使用站点验签器验证 Token，设置用户信息到 Context
func CasdoorJwtMiddleware() gin.HandlerFunc {
	svc := &casdoorService.CasdoorService{}

	return func(c *gin.Context) {
		path := c.Request.URL.Path

		// 非 API 路径直接放行
		if !strings.HasPrefix(path, "/api/") {
			c.Next()
			return
		}

		// Logout validates signed credentials itself, including expired/revoked ones.
		if path == "/api/v1/addons/casdoor-auth/logout" {
			c.Next()
			return
		}

		// 白名单路径放行
		for _, p := range publicPaths {
			if path == p || strings.HasPrefix(path, p) {
				c.Next()
				return
			}
		}

		// 提取 Token
		tokenStr := c.GetHeader("Authorization")
		if tokenStr == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"code":    401,
				"message": "未提供认证令牌",
			})
			return
		}

		// 去除 Bearer 前缀
		if strings.HasPrefix(tokenStr, "Bearer ") {
			tokenStr = tokenStr[7:]
		}

		// 使用站点验签器解析验证 Token
		claims, err := svc.ParseToken(tokenStr)
		if err != nil {
			global.PRISM_LOG.Debug("Casdoor JWT 验证失败",
				zap.String("path", path),
				zap.Error(err),
			)
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"code":    401,
				"message": "认证令牌无效或已过期",
			})
			return
		}

		// JWT-Standard 格式下 Token 不含 Roles/IsAdmin 等 Casdoor 扩展字段，
		// 需通过 Casdoor API 补全（带缓存）
		claims = svc.EnrichClaimsFromAPI(claims)

		// 将 Casdoor 用户信息写入 Context
		c.Set("username", claims.Name)
		c.Set("casdoor_owner", claims.Owner)
		c.Set("casdoor_email", claims.Email)

		// 提取角色信息：从 Casdoor Claims 中获取
		roles := extractRoles(claims)
		c.Set("roles", roles)

		// 如果是管理员，设置 role_id=999 以兼容原有逻辑
		if claims.IsAdmin {
			c.Set("role_id", uint(999))
		} else {
			c.Set("role_id", uint(1))
		}

		c.Next()
	}
}

// extractRoles 从 Casdoor Claims 中提取角色列表
// casdoorsdk.Claims 内嵌 User，User.Roles 为 []*Role（含 Name 字段）
func extractRoles(claims *casdoorsdk.Claims) []string {
	if claims == nil {
		return nil
	}
	roles := make([]string, 0, len(claims.Roles))
	for _, r := range claims.Roles {
		if r != nil && r.Name != "" {
			roles = append(roles, r.Name)
		}
	}
	return roles
}
