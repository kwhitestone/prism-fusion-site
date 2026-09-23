package casbinrbac

import (
	"github.com/kwhitestone/prism-fusion/global"
	"github.com/kwhitestone/prism-fusion/plugin"
	casbinMiddleware "top.whitestone/prism-fusion-site/addons/casbin-rbac/middleware"
	casbinModel "top.whitestone/prism-fusion-site/addons/casbin-rbac/model"
	casbinRouter "top.whitestone/prism-fusion-site/addons/casbin-rbac/router"
	"top.whitestone/prism-fusion-site/addons/casbin-rbac/service"

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

// Manifest 声明 V2 身份、依赖与路由边界。
//
// 依赖的是本站的 casdoor-auth 而非框架内置 auth：授权中间件消费
// casdoor JWT 写入的 username/roles/casdoor_owner 上下文，且
// enforcer 复用 casdoor-auth/conf 的组织配置，必须排在其后启动。
// admin 子路由同属本前缀，单一作用域即可覆盖。
func (p *CasbinRbacPlugin) Manifest() plugin.Manifest {
	return plugin.Manifest{
		APIVersion:  plugin.APIVersionV2,
		ID:          "casbin-rbac",
		Version:     "2.0.0",
		Kind:        plugin.KindBackendAddon,
		Description: p.Description(),
		Provides:    []string{"authorization.rbac"},
		Requires:    []plugin.Dependency{{ID: "casdoor-auth"}},
		RouteScopes: []string{p.RoutePrefix()},
	}
}

// PluginEnabled 把 provider 选择上移为框架激活契约。
func (p *CasbinRbacPlugin) PluginEnabled() bool {
	return isEnabled()
}

func (p *CasbinRbacPlugin) RegisterRoutes(api huma.API) {
	// 确保 Casdoor SDK 已初始化（远程 Casbin 需要通过 Casdoor API 调用）
	service.InitCasdoorSDK()

	// 初始化菜单种子数据
	service.SeedMenuData()

	// 注册路由
	casbinRouter.RegisterRoutes(api)

	global.PRISM_LOG.Info("Casbin RBAC plugin routes registered (via Casdoor remote Casbin)")
}

func (p *CasbinRbacPlugin) Models() []interface{} {
	return []interface{}{
		&casbinModel.CasbinMenu{},
	}
}

func (p *CasbinRbacPlugin) GlobalMiddlewares() []gin.HandlerFunc {
	return []gin.HandlerFunc{
		casbinMiddleware.CasbinAuthzMiddleware(),
	}
}
