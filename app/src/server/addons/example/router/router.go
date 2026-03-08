package router

import (
	"context"

	"github.com/danielgtaylor/huma/v2"
	"whitestone.top/prism-example-site/addons/example/service"
)

// exampleService 示例服务实例
var exampleService = service.ExampleService{}

// RegisterRoutes 注册示例插件路由
func RegisterRoutes(api huma.API) {
	// GET - 获取所有示例项
	huma.Register(api, huma.Operation{
		OperationID: "getExampleItems",
		Method:      "GET",
		Path:        "/api/v1/addons/example/items",
		Summary:     "获取示例项列表",
		Description: "获取所有示例项数据",
		Tags:        []string{"Addon-Example"},
	}, func(ctx context.Context, input *struct{}) (*struct {
		Body struct {
			Code    int                           `json:"code" example:"0" doc:"状态码"`
			Message string                        `json:"message" example:"success" doc:"响应消息"`
			Data    []service.ExampleItemResponse `json:"data" doc:"示例项列表"`
		}
	}, error) {
		items, err := exampleService.GetItems()
		if err != nil {
			return nil, err
		}

		resp := &struct {
			Body struct {
				Code    int                           `json:"code" example:"0" doc:"状态码"`
				Message string                        `json:"message" example:"success" doc:"响应消息"`
				Data    []service.ExampleItemResponse `json:"data" doc:"示例项列表"`
			}
		}{}
		resp.Body.Code = 0
		resp.Body.Message = "success"
		resp.Body.Data = items
		return resp, nil
	})

	// POST - 创建示例项
	huma.Register(api, huma.Operation{
		OperationID: "createExampleItem",
		Method:      "POST",
		Path:        "/api/v1/addons/example/items",
		Summary:     "创建示例项",
		Description: "创建新的示例项",
		Tags:        []string{"Addon-Example"},
	}, func(ctx context.Context, input *struct {
		Body struct {
			Name        string `json:"name" required:"true" minLength:"1" doc:"名称"`
			Description string `json:"description" doc:"描述"`
		}
	}) (*struct {
		Body struct {
			Code    int                          `json:"code" example:"0" doc:"状态码"`
			Message string                       `json:"message" example:"success" doc:"响应消息"`
			Data    *service.ExampleItemResponse `json:"data" doc:"创建的示例项"`
		}
	}, error) {
		item, err := exampleService.CreateItem(input.Body.Name, input.Body.Description)
		if err != nil {
			return nil, err
		}

		resp := &struct {
			Body struct {
				Code    int                          `json:"code" example:"0" doc:"状态码"`
				Message string                       `json:"message" example:"success" doc:"响应消息"`
				Data    *service.ExampleItemResponse `json:"data" doc:"创建的示例项"`
			}
		}{}
		resp.Body.Code = 0
		resp.Body.Message = "创建成功"
		resp.Body.Data = item
		return resp, nil
	})

	// GET - 根据ID获取示例项
	huma.Register(api, huma.Operation{
		OperationID: "getExampleItemById",
		Method:      "GET",
		Path:        "/api/v1/addons/example/items/{id}",
		Summary:     "获取示例项详情",
		Description: "根据ID获取示例项详情",
		Tags:        []string{"Addon-Example"},
	}, func(ctx context.Context, input *struct {
		ID uint `path:"id" required:"true" doc:"示例项ID"`
	}) (*struct {
		Body struct {
			Code    int                          `json:"code" example:"0" doc:"状态码"`
			Message string                       `json:"message" example:"success" doc:"响应消息"`
			Data    *service.ExampleItemResponse `json:"data" doc:"示例项详情"`
		}
	}, error) {
		item, err := exampleService.GetItemByID(input.ID)
		if err != nil {
			return nil, err
		}

		resp := &struct {
			Body struct {
				Code    int                          `json:"code" example:"0" doc:"状态码"`
				Message string                       `json:"message" example:"success" doc:"响应消息"`
				Data    *service.ExampleItemResponse `json:"data" doc:"示例项详情"`
			}
		}{}
		resp.Body.Code = 0
		resp.Body.Message = "success"
		resp.Body.Data = item
		return resp, nil
	})

	// DELETE - 删除示例项
	huma.Register(api, huma.Operation{
		OperationID: "deleteExampleItem",
		Method:      "DELETE",
		Path:        "/api/v1/addons/example/items/{id}",
		Summary:     "删除示例项",
		Description: "根据ID删除示例项",
		Tags:        []string{"Addon-Example"},
	}, func(ctx context.Context, input *struct {
		ID uint `path:"id" required:"true" doc:"示例项ID"`
	}) (*struct {
		Body struct {
			Code    int    `json:"code" example:"0" doc:"状态码"`
			Message string `json:"message" example:"success" doc:"响应消息"`
		}
	}, error) {
		if err := exampleService.DeleteItem(input.ID); err != nil {
			return nil, err
		}

		resp := &struct {
			Body struct {
				Code    int    `json:"code" example:"0" doc:"状态码"`
				Message string `json:"message" example:"success" doc:"响应消息"`
			}
		}{}
		resp.Body.Code = 0
		resp.Body.Message = "删除成功"
		return resp, nil
	})
}
