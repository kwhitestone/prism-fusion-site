// Package dashboard 数据总览插件
package dashboard

import (
	"whitestone.top/prism-example-site/addons/dashboard/router"
	"whitestone.top/prism-fusion/plugin"

	"github.com/danielgtaylor/huma/v2"
)

func init() {
	plugin.Register(&DashboardPlugin{})
}

// DashboardPlugin 数据总览插件
type DashboardPlugin struct {
	plugin.BasePlugin
}

func (p *DashboardPlugin) Name() string {
	return "dashboard"
}

func (p *DashboardPlugin) Description() string {
	return "数据总览插件，提供系统运行概况统计"
}

func (p *DashboardPlugin) RegisterRoutes(humaApi huma.API) {
	router.RegisterRoutes(humaApi)
}
