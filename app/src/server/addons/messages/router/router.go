package router

import (
	"context"
	"net/http"

	"github.com/danielgtaylor/huma/v2"
	"whitestone.top/prism-example-site/addons/messages/service"
)

var messagesService = service.MessagesService{}

// RegisterRoutes 注册 Messages 插件路由
func RegisterRoutes(api huma.API) {
	// POST - 提交消息
	huma.Register(api, huma.Operation{
		OperationID: "postMessage",
		Method:      "POST",
		Path:        "/api/v1/addons/messages",
		Summary:     "提交消息",
		Description: "将用户消息保存到内存中",
		Tags:        []string{"Messages"},
	}, func(ctx context.Context, input *struct {
		Body struct {
			Content string `json:"content" required:"true" minLength:"1" doc:"消息内容"`
			Author  string `json:"author" required:"true" minLength:"1" doc:"作者名称"`
		}
	}) (*struct {
		Body struct {
			Code    int              `json:"code" example:"0" doc:"状态码"`
			Message string           `json:"message" example:"success" doc:"响应消息"`
			Data    *service.Message `json:"data" doc:"创建的消息"`
		}
	}, error) {
		msg := messagesService.AddMessage(input.Body.Content, input.Body.Author)

		resp := &struct {
			Body struct {
				Code    int              `json:"code" example:"0" doc:"状态码"`
				Message string           `json:"message" example:"success" doc:"响应消息"`
				Data    *service.Message `json:"data" doc:"创建的消息"`
			}
		}{}
		resp.Body.Code = 0
		resp.Body.Message = "消息提交成功"
		resp.Body.Data = msg
		return resp, nil
	})

	// GET - 获取所有消息
	huma.Register(api, huma.Operation{
		OperationID: "getMessages",
		Method:      "GET",
		Path:        "/api/v1/addons/messages",
		Summary:     "获取所有消息",
		Description: "获取内存中保存的所有消息",
		Tags:        []string{"Messages"},
	}, func(ctx context.Context, input *struct{}) (*struct {
		Body struct {
			Code    int               `json:"code" example:"0" doc:"状态码"`
			Message string            `json:"message" example:"success" doc:"响应消息"`
			Data    []service.Message `json:"data" doc:"消息列表"`
		}
	}, error) {
		msgs := messagesService.GetMessages()

		resp := &struct {
			Body struct {
				Code    int               `json:"code" example:"0" doc:"状态码"`
				Message string            `json:"message" example:"success" doc:"响应消息"`
				Data    []service.Message `json:"data" doc:"消息列表"`
			}
		}{}
		resp.Body.Code = 0
		resp.Body.Message = "success"
		resp.Body.Data = msgs
		return resp, nil
	})

	// DELETE - 删除指定消息
	huma.Register(api, huma.Operation{
		OperationID: "deleteMessage",
		Method:      "DELETE",
		Path:        "/api/v1/addons/messages/{id}",
		Summary:     "删除消息",
		Description: "根据ID删除指定消息",
		Tags:        []string{"Messages"},
	}, func(ctx context.Context, input *struct {
		ID int `path:"id" doc:"消息ID"`
	}) (*struct {
		Body struct {
			Code    int    `json:"code" example:"0" doc:"状态码"`
			Message string `json:"message" example:"success" doc:"响应消息"`
		}
	}, error) {
		deleted := messagesService.DeleteMessage(input.ID)

		resp := &struct {
			Body struct {
				Code    int    `json:"code" example:"0" doc:"状态码"`
				Message string `json:"message" example:"success" doc:"响应消息"`
			}
		}{}

		if deleted {
			resp.Body.Code = 0
			resp.Body.Message = "删除成功"
		} else {
			return nil, huma.NewError(http.StatusNotFound, "消息不存在")
		}
		return resp, nil
	})

	// DELETE - 清空所有消息
	huma.Register(api, huma.Operation{
		OperationID: "clearMessages",
		Method:      "DELETE",
		Path:        "/api/v1/addons/messages",
		Summary:     "清空所有消息",
		Description: "清空内存中保存的所有消息",
		Tags:        []string{"Messages"},
	}, func(ctx context.Context, input *struct{}) (*struct {
		Body struct {
			Code    int    `json:"code" example:"0" doc:"状态码"`
			Message string `json:"message" example:"success" doc:"响应消息"`
		}
	}, error) {
		messagesService.ClearMessages()

		resp := &struct {
			Body struct {
				Code    int    `json:"code" example:"0" doc:"状态码"`
				Message string `json:"message" example:"success" doc:"响应消息"`
			}
		}{}
		resp.Body.Code = 0
		resp.Body.Message = "已清空所有消息"
		return resp, nil
	})
}
