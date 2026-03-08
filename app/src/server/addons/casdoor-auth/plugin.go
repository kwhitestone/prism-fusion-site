package casdoorauth

import (
	"whitestone.top/prism-example-site/addons/casdoor-auth/conf"
	casdoorMiddleware "whitestone.top/prism-example-site/addons/casdoor-auth/middleware"
	casdoorRouter "whitestone.top/prism-example-site/addons/casdoor-auth/router"
	"whitestone.top/prism-example-site/addons/casdoor-auth/service"
	"whitestone.top/prism-fusion/global"
	"whitestone.top/prism-fusion/plugin"

	"github.com/danielgtaylor/huma/v2"
	"github.com/gin-gonic/gin"
)

// CasdoorAuthPlugin Casdoor 认证插件
type CasdoorAuthPlugin struct {
	plugin.BasePlugin
}

func init() {
	plugin.Register(&CasdoorAuthPlugin{
		BasePlugin: plugin.BasePlugin{
			PluginName:        "casdoor-auth",
			PluginDescription: "Casdoor 认证插件 - 提供 OAuth2/代理登录、Token 管理",
		},
	})
}

// isEnabled 检查当前是否启用 Casdoor 认证
func isEnabled() bool {
	return global.PRISM_CONFIG.Auth.Provider == "casdoor"
}

func (p *CasdoorAuthPlugin) Priority() int {
	// 与 auth 同优先级，但通过 provider 互斥
	return 10
}

func (p *CasdoorAuthPlugin) RoutePrefix() string {
	return "/api/v1/addons/casdoor-auth"
}

func (p *CasdoorAuthPlugin) RegisterRoutes(api huma.API) {
	if !isEnabled() {
		return
	}

	// 初始化插件自有配置（从 viper 读取 auth.casdoor 段）
	conf.Init()

	// 自动引导：创建独立组织、应用、超管角色/权限/用户（在 SDK 初始化之前执行）
	service.BootstrapOrganization()

	// 初始化 Casdoor SDK
	svc := &service.CasdoorService{}
	svc.InitSDK()

	// 注册路由
	casdoorRouter.RegisterRoutes(api)

	global.PRISM_LOG.Info("Casdoor Auth plugin routes registered")
}

func (p *CasdoorAuthPlugin) Models() []interface{} {
	// Casdoor 管理用户，本地不需要 User 模型
	return nil
}

func (p *CasdoorAuthPlugin) GlobalMiddlewares() []gin.HandlerFunc {
	if !isEnabled() {
		return nil
	}
	return []gin.HandlerFunc{
		casdoorMiddleware.CasdoorJwtMiddleware(),
	}
}
