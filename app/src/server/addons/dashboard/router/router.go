package router

import (
	"context"

	"github.com/danielgtaylor/huma/v2"
	"whitestone.top/prism-example-site/addons/dashboard/service"
)

var dashboardService = service.DashboardService{}

// RegisterRoutes 注册 Dashboard 插件路由
func RegisterRoutes(api huma.API) {
	// Dashboard统计数据接口
	huma.Register(api, huma.Operation{
		OperationID: "getDashboardStats",
		Method:      "GET",
		Path:        "/api/v1/addons/dashboard/stats",
		Summary:     "获取Dashboard统计数据",
		Description: "获取系统Dashboard的各项统计数据",
		Tags:        []string{"Dashboard"},
	}, func(ctx context.Context, input *struct{}) (*struct {
		Body struct {
			Code    int                     `json:"code" example:"0" doc:"状态码"`
			Message string                  `json:"message" example:"success" doc:"响应消息"`
			Data    *service.DashboardStats `json:"data" doc:"统计数据"`
		}
	}, error) {
		stats, err := dashboardService.GetStats()
		if err != nil {
			return nil, err
		}

		resp := &struct {
			Body struct {
				Code    int                     `json:"code" example:"0" doc:"状态码"`
				Message string                  `json:"message" example:"success" doc:"响应消息"`
				Data    *service.DashboardStats `json:"data" doc:"统计数据"`
			}
		}{}
		resp.Body.Code = 0
		resp.Body.Message = "success"
		resp.Body.Data = stats
		return resp, nil
	})
}
