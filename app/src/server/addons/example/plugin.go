// Package example 示例插件 - 展示插件开发规范
package example

import (
	"whitestone.top/prism-example-site/addons/example/middleware"
	"whitestone.top/prism-example-site/addons/example/model"
	"whitestone.top/prism-example-site/addons/example/router"
	"whitestone.top/prism-fusion/plugin"

	"github.com/danielgtaylor/huma/v2"
	"github.com/gin-gonic/gin"
)

func init() {
	// 在 init() 中自动注册插件到框架
	plugin.Register(&ExamplePlugin{})
}

// ExamplePlugin 示例插件实现
type ExamplePlugin struct {
	plugin.BasePlugin
}

func (p *ExamplePlugin) Name() string {
	return "example"
}

func (p *ExamplePlugin) Description() string {
	return "示例插件，展示插件开发规范"
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
