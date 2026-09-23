// Package example 示例插件 - 展示插件开发规范
package example

import (
	"github.com/kwhitestone/prism-fusion/plugin"
	"top.whitestone/prism-fusion-site/addons/example/middleware"
	"top.whitestone/prism-fusion-site/addons/example/model"
	"top.whitestone/prism-fusion-site/addons/example/router"

	"github.com/danielgtaylor/huma/v2"
	"github.com/gin-gonic/gin"
)

func init() {
	// 在 init() 中自动注册插件到框架
	plugin.Register(newExamplePlugin())
}

// ExamplePlugin 示例插件实现
type ExamplePlugin struct {
	plugin.BasePlugin
}

func newExamplePlugin() *ExamplePlugin {
	return &ExamplePlugin{BasePlugin: plugin.BasePlugin{
		PluginName:        "example",
		PluginDescription: "示例插件，展示插件开发规范",
	}}
}

func (p *ExamplePlugin) Name() string {
	return "example"
}

func (p *ExamplePlugin) Description() string {
	return "示例插件，展示插件开发规范"
}

// Manifest 声明 V2 身份、依赖与路由作用域。
//
// 依赖本站实际启用的认证/授权插件（casdoor-auth + casbin-rbac），
// 而非框架内置 auth/rbac —— 后者在本站 provider 配置下不激活，
// 而未激活的依赖等同缺失，会导致启动失败。
func (p *ExamplePlugin) Manifest() plugin.Manifest {
	return plugin.Manifest{
		APIVersion:  plugin.APIVersionV2,
		ID:          p.Name(),
		Version:     "2.0.0",
		Kind:        plugin.KindBackendAddon,
		Description: p.Description(),
		Provides:    []string{"example.items"},
		Requires: []plugin.Dependency{
			{ID: "casbin-rbac"},
			{ID: "casdoor-auth"},
		},
		RouteScopes: []string{p.RoutePrefix()},
	}
}

func (p *ExamplePlugin) RegisterRoutes(humaApi huma.API) {
	// 直接调用 router 包注册路由
	router.RegisterRoutes(humaApi)
}

func (p *ExamplePlugin) Models() []interface{} {
	// 返回需要自动迁移的模型
	return []interface{}{
		&model.ExampleItem{},
	}
}

func (p *ExamplePlugin) Middlewares() []gin.HandlerFunc {
	// 返回插件作用域中间件（仅对本插件路由生效）
	return []gin.HandlerFunc{
		middleware.ExampleMiddleware(),
	}
}

func (p *ExamplePlugin) GlobalMiddlewares() []gin.HandlerFunc {
	// 返回全局中间件（对所有路由生效）
	return []gin.HandlerFunc{
		middleware.ExampleGlobalMiddleware(),
	}
}
