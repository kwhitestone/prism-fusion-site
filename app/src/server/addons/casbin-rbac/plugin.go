package casbinrbac

import (
	casbinMiddleware "whitestone.top/prism-example-site/addons/casbin-rbac/middleware"
	casbinModel "whitestone.top/prism-example-site/addons/casbin-rbac/model"
	casbinRouter "whitestone.top/prism-example-site/addons/casbin-rbac/router"
	"whitestone.top/prism-example-site/addons/casbin-rbac/service"
	"whitestone.top/prism-fusion/global"
	"whitestone.top/prism-fusion/plugin"

	"github.com/danielgtaylor/huma/v2"
	"github.com/gin-gonic/gin"
)

// CasbinRbacPlugin Casbin RBAC 插件
type CasbinRbacPlugin struct {
	plugin.BasePlugin
}

func init() {
	plugin.Register(&CasbinRbacPlugin{
		BasePlugin: plugin.BasePlugin{
			PluginName:        "casbin-rbac",
			PluginDescription: "Casbin RBAC 插件 - 提供基于 Casbin 的 API 访问控制和动态路由管理",
		},
	})
}

// isEnabled 检查当前是否启用 Casbin RBAC
func isEnabled() bool {
	return global.PRISM_CONFIG.RBAC.Provider == "casbin"
}

func (p *CasbinRbacPlugin) Priority() int {
	// 与 rbac 同优先级，通过 provider 互斥
	return 20
}

func (p *CasbinRbacPlugin) RoutePrefix() string {
	return "/api/v1/addons/casbin-rbac"
}

func (p *CasbinRbacPlugin) RegisterRoutes(api huma.API) {
	if !isEnabled() {
		return
	}

	// 确保 Casdoor SDK 已初始化（远程 Casbin 需要通过 Casdoor API 调用）
	service.InitCasdoorSDK()

	// 初始化菜单种子数据
	service.SeedMenuData()

	// 注册路由
	casbinRouter.RegisterRoutes(api)

	global.PRISM_LOG.Info("Casbin RBAC plugin routes registered (via Casdoor remote Casbin)")
}

func (p *CasbinRbacPlugin) Models() []interface{} {
	if !isEnabled() {
		return nil
	}
	return []interface{}{
		&casbinModel.CasbinMenu{},
	}
}

func (p *CasbinRbacPlugin) GlobalMiddlewares() []gin.HandlerFunc {
	if !isEnabled() {
		return nil
	}
	return []gin.HandlerFunc{
		casbinMiddleware.CasbinAuthzMiddleware(),
	}
}
