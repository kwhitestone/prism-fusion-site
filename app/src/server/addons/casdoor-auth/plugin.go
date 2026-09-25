package casdoorauth

import (
	"time"

	"github.com/kwhitestone/prism-fusion/global"
	"github.com/kwhitestone/prism-fusion/plugin"
	"gorm.io/gorm"
	"top.whitestone/prism-fusion-site/addons/casdoor-auth/conf"
	casdoorMiddleware "top.whitestone/prism-fusion-site/addons/casdoor-auth/middleware"
	"top.whitestone/prism-fusion-site/addons/casdoor-auth/model"
	casdoorRouter "top.whitestone/prism-fusion-site/addons/casdoor-auth/router"
	"top.whitestone/prism-fusion-site/addons/casdoor-auth/service"

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

// Manifest 声明 V2 身份与路由边界。
//
// 本站以 casdoor 取代框架内置 auth，两者通过 auth.provider 互斥：
// 内置 auth 在 provider=casdoor 时不激活，而未激活的依赖等同缺失，
// 因此这里不能声明 Requires{auth}，否则启动会因缺依赖而失败。
func (p *CasdoorAuthPlugin) Manifest() plugin.Manifest {
	return plugin.Manifest{
		APIVersion:  plugin.APIVersionV2,
		ID:          "casdoor-auth",
		Version:     "2.0.0",
		Kind:        plugin.KindBackendAddon,
		Description: p.Description(),
		Provides:    []string{"auth.identity"},
		RouteScopes: []string{p.RoutePrefix()},
	}
}

// PluginEnabled 把 provider 选择上移为框架激活契约。
// 冻结后该决定不可变，插件方法内不再各自切换行为。
func (p *CasdoorAuthPlugin) PluginEnabled() bool {
	return isEnabled()
}

func (p *CasdoorAuthPlugin) RegisterRoutes(api huma.API) {
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
	return []interface{}{&model.CasdoorSession{}, &model.CasdoorTokenBlacklist{}}
}

func (p *CasdoorAuthPlugin) GlobalMiddlewares() []gin.HandlerFunc {
	return []gin.HandlerFunc{
		casdoorMiddleware.CasdoorJwtMiddleware(),
	}
}

func (p *CasdoorAuthPlugin) AfterMigrate(db *gorm.DB) error {
	return service.CleanupSessions(db, time.Now())
}
