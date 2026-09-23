// Package dashboard 数据总览插件
package dashboard

import (
	"github.com/kwhitestone/prism-fusion/plugin"
	"top.whitestone/prism-fusion-site/addons/dashboard/router"

	"github.com/danielgtaylor/huma/v2"
)

func init() {
	plugin.Register(newDashboardPlugin())
}

// DashboardPlugin 数据总览插件
type DashboardPlugin struct {
	plugin.BasePlugin
}

func newDashboardPlugin() *DashboardPlugin {
	return &DashboardPlugin{BasePlugin: plugin.BasePlugin{
		PluginName:        "dashboard",
		PluginDescription: "数据总览插件，提供系统运行概况统计",
	}}
}

func (p *DashboardPlugin) Name() string {
	return "dashboard"
}

func (p *DashboardPlugin) Description() string {
	return "数据总览插件，提供系统运行概况统计"
}

// 保留 V1 合同：本站的授权在 casbin-rbac 全局中间件里按路径统一裁决，
// 不使用框架内置 rbac 的 RequirePermission（其依赖 builtin auth 写入的
// user_id/permissions 上下文，casdoor JWT 并不提供）。
func (p *DashboardPlugin) RegisterRoutes(humaApi huma.API) {
	router.RegisterRoutes(humaApi)
}
