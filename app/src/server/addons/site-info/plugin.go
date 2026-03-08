// Package siteinfo 站点信息插件 - 业务示例
package siteinfo

import (
	"context"
	"net/http"

	"whitestone.top/prism-fusion/plugin"

	"github.com/danielgtaylor/huma/v2"
)

func init() {
	plugin.Register(&SiteInfoPlugin{})
}

// SiteInfoPlugin 站点信息插件
type SiteInfoPlugin struct {
	plugin.BasePlugin
}

func (p *SiteInfoPlugin) Name() string {
	return "site-info"
}

func (p *SiteInfoPlugin) Description() string {
	return "站点信息插件，展示业务插件开发方式"
}

// SiteInfoOutput 站点信息响应
type SiteInfoOutput struct {
	Body struct {
		SiteName    string `json:"site_name" doc:"站点名称"`
		Version     string `json:"version" doc:"版本号"`
		Description string `json:"description" doc:"站点描述"`
	}
}

func (p *SiteInfoPlugin) RegisterRoutes(api huma.API) {
	huma.Register(api, huma.Operation{
		OperationID: "getSiteInfo",
		Method:      http.MethodGet,
		Path:        "/api/v1/addons/site-info/info",
		Summary:     "获取站点信息",
		Description: "返回当前站点的基本信息",
		Tags:        []string{"SiteInfo"},
	}, func(ctx context.Context, input *struct{}) (*SiteInfoOutput, error) {
		resp := &SiteInfoOutput{}
		resp.Body.SiteName = "Prism Example Site"
		resp.Body.Version = "1.0.0"
		resp.Body.Description = "基于 Prism Fusion 框架的示例站点"
		return resp, nil
	})
}
