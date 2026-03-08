package middleware

import (
	"net/http"
	"strings"

	"whitestone.top/prism-example-site/addons/casbin-rbac/service"
	"whitestone.top/prism-example-site/addons/casdoor-auth/conf"
	"whitestone.top/prism-fusion/global"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// 不需要授权检查的路径（仍需通过认证中间件验证 JWT）
var skipPaths = []string{
	// 认证相关（公开）
	"/api/v1/addons/casdoor-auth/login",
	"/api/v1/addons/casdoor-auth/signin-url",
	"/api/v1/addons/casdoor-auth/signin-callback",
	"/api/v1/addons/casdoor-auth/refresh-token",
	"/api/v1/addons/casdoor-auth/config",
	"/api/v1/addons/auth/login",
	"/api/v1/addons/auth/register",
	"/api/v1/addons/auth/refresh-token",
	// 已认证用户均可访问（无需角色权限）
	"/api/v1/addons/casdoor-auth/user-info",
	"/api/v1/addons/auth/user-info",
	"/api/v1/addons/casbin-rbac/async-routes",
	"/api/v1/addons/rbac/async-routes",
	// 基础设施
	"/health",
	"/openapi",
	"/docs",
	"/scalar",
}

// CasbinAuthzMiddleware Casbin 授权中间件
// 在认证中间件之后执行，检查用户角色对 API 路径的访问权限
func CasbinAuthzMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		path := c.Request.URL.Path
		method := c.Request.Method

		// 非 API 路径直接放行
		if !strings.HasPrefix(path, "/api/") {
			c.Next()
			return
		}

		// 跳过白名单路径
		for _, p := range skipPaths {
			if path == p || strings.HasPrefix(path, p) {
				c.Next()
				return
			}
		}

		// 获取用户信息（由认证中间件设置）
		username := ""
		if v, ok := c.Get("username"); ok {
			if s, ok := v.(string); ok {
				username = s
			}
		}

		owner := conf.Get().OrganizationName
		if v, ok := c.Get("casdoor_owner"); ok {
			if s, ok := v.(string); ok && s != "" {
				owner = s
			}
		}

		// 获取 IsAdmin 标志
		isAdmin := false
		if roleID, exists := c.Get("role_id"); exists {
			if rid, ok := roleID.(uint); ok && rid == 999 {
				isAdmin = true
			}
		}

		// 收集用户角色列表
		var roles []string
		if rolesVal, ok := c.Get("roles"); ok {
			if r, ok := rolesVal.([]string); ok {
				roles = r
			}
		}

		// 使用本地权限匹配检查
		allowed, err := service.Enforce(username, owner, roles, isAdmin, path, method)
		if err != nil {
			global.PRISM_LOG.Error("Casbin enforce error",
				zap.String("username", username),
				zap.String("path", path),
				zap.String("method", method),
				zap.Error(err),
			)
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
				"code":    500,
				"message": "权限检查异常",
			})
			return
		}

		if !allowed {
			global.PRISM_LOG.Debug("Casbin access denied",
				zap.String("username", username),
				zap.Bool("isAdmin", isAdmin),
				zap.Strings("roles", roles),
				zap.String("path", path),
				zap.String("method", method),
			)
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
				"code":    403,
				"message": "没有权限访问此资源",
			})
			return
		}

		c.Next()
	}
}
