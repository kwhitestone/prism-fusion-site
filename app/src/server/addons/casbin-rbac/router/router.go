package router

import (
	"context"
	"net/http"

	"top.whitestone/prism-fusion-site/addons/casbin-rbac/service"

	"github.com/danielgtaylor/huma/v2"
)

var menuService = &service.MenuService{}

// ---- Named types ----

// CasbinAsyncRoutesData 动态路由数据
type CasbinAsyncRoutesData struct {
	Success bool               `json:"success" doc:"是否成功"`
	Data    []service.MenuNode `json:"data" doc:"菜单列表"`
}

// CasbinPermissionItem 权限摘要
type CasbinPermissionItem struct {
	Name        string   `json:"name" doc:"权限名"`
	DisplayName string   `json:"displayName" doc:"显示名"`
	Resources   []string `json:"resources" doc:"资源列表"`
	Actions     []string `json:"actions" doc:"操作列表"`
	Effect      string   `json:"effect" doc:"效果(Allow/Deny)"`
	IsEnabled   bool     `json:"isEnabled" doc:"是否启用"`
}

// CasbinPermissionsData 权限列表
type CasbinPermissionsData struct {
	Permissions []CasbinPermissionItem `json:"permissions" doc:"权限列表"`
}

// CasbinEnforceData 权限检查结果
type CasbinEnforceData struct {
	Allowed bool `json:"allowed" doc:"是否允许"`
}

// CasbinRoleItem 角色摘要
type CasbinRoleItem struct {
	Name        string   `json:"name" doc:"角色名"`
	DisplayName string   `json:"displayName" doc:"显示名"`
	Users       []string `json:"users" doc:"用户列表"`
	IsEnabled   bool     `json:"isEnabled" doc:"是否启用"`
}

// CasbinRolesData 角色列表
type CasbinRolesData struct {
	Roles []CasbinRoleItem `json:"roles" doc:"角色列表"`
}

// CasbinUserRolesData 用户角色列表
type CasbinUserRolesData struct {
	Roles []string `json:"roles" doc:"角色列表"`
}

// ---- Output types ----

// CasbinAsyncRoutesOutput 动态路由响应
type CasbinAsyncRoutesOutput struct {
	Body struct {
		Code    int                    `json:"code" doc:"状态码"`
		Message string                 `json:"message" doc:"响应消息"`
		Data    *CasbinAsyncRoutesData `json:"data" doc:"路由数据"`
	}
}

// CasbinPermissionsOutput 权限列表响应
type CasbinPermissionsOutput struct {
	Body struct {
		Code    int                    `json:"code" doc:"状态码"`
		Message string                 `json:"message" doc:"响应消息"`
		Data    *CasbinPermissionsData `json:"data" doc:"权限数据"`
	}
}

// CasbinRolesOutput 角色列表响应
type CasbinRolesOutput struct {
	Body struct {
		Code    int              `json:"code" doc:"状态码"`
		Message string           `json:"message" doc:"响应消息"`
		Data    *CasbinRolesData `json:"data" doc:"角色数据"`
	}
}

// CasbinEnforceOutput 权限检查响应
type CasbinEnforceOutput struct {
	Body struct {
		Code    int                `json:"code" doc:"状态码"`
		Message string             `json:"message" doc:"响应消息"`
		Data    *CasbinEnforceData `json:"data" doc:"检查结果"`
	}
}

// CasbinMutationOutput 通用操作响应
type CasbinMutationOutput struct {
	Body struct {
		Code    int    `json:"code" doc:"状态码"`
		Message string `json:"message" doc:"响应消息"`
	}
}

// CasbinUserRolesOutput 用户角色响应
type CasbinUserRolesOutput struct {
	Body struct {
		Code    int                  `json:"code" doc:"状态码"`
		Message string               `json:"message" doc:"响应消息"`
		Data    *CasbinUserRolesData `json:"data" doc:"角色数据"`
	}
}

// ---- Input types ----

// CasbinAddPermissionInput 添加权限输入
type CasbinAddPermissionInput struct {
	Body struct {
		Name        string   `json:"name" required:"true" doc:"权限名"`
		DisplayName string   `json:"displayName" doc:"显示名"`
		Resources   []string `json:"resources" required:"true" doc:"资源列表"`
		Actions     []string `json:"actions" required:"true" doc:"操作列表 (read/write/admin/*)"`
	}
}

// CasbinDeletePermissionInput 删除权限输入
type CasbinDeletePermissionInput struct {
	Body struct {
		Name string `json:"name" required:"true" doc:"权限名"`
	}
}

// CasbinEnforceInput 权限检查输入
type CasbinEnforceInput struct {
	Role   string `query:"role" required:"true" doc:"角色"`
	Path   string `query:"path" required:"true" doc:"资源路径"`
	Method string `query:"method" required:"true" doc:"HTTP 方法"`
}

// CasbinAddRoleInput 添加角色输入
type CasbinAddRoleInput struct {
	Body struct {
		Name        string   `json:"name" required:"true" doc:"角色名"`
		DisplayName string   `json:"displayName" doc:"显示名"`
		Users       []string `json:"users" doc:"用户列表"`
	}
}

// CasbinDeleteRoleInput 删除角色输入
type CasbinDeleteRoleInput struct {
	Body struct {
		Name string `json:"name" required:"true" doc:"角色名"`
	}
}

// CasbinGetUserRolesInput 获取用户角色输入
type CasbinGetUserRolesInput struct {
	User string `query:"user" required:"true" doc:"用户名"`
}

// RegisterRoutes 注册 Casbin RBAC 路由（通过 Casdoor 远程 API）
func RegisterRoutes(api huma.API) {
	// 注册管理后台路由（用户/角色/权限 CRUD）
	RegisterAdminRoutes(api)

	// 获取动态路由（与 builtin rbac 保持相同接口）
	huma.Register(api, huma.Operation{
		OperationID: "casbinGetAsyncRoutes",
		Method:      http.MethodGet,
		Path:        "/api/v1/addons/casbin-rbac/async-routes",
		Summary:     "获取动态路由",
		Description: "获取当前用户可见的菜单路由（Casbin RBAC）",
		Tags:        []string{"Casbin RBAC"},
	}, func(ctx context.Context, input *struct{}) (*CasbinAsyncRoutesOutput, error) {
		routes, err := menuService.GetAsyncRoutes()
		resp := &CasbinAsyncRoutesOutput{}
		if err != nil {
			return nil, huma.NewError(http.StatusInternalServerError, "获取路由失败")
		}

		resp.Body.Code = 0
		resp.Body.Message = "success"
		resp.Body.Data = &CasbinAsyncRoutesData{
			Success: true,
			Data:    routes,
		}
		return resp, nil
	})

	// 获取权限列表（从 Casdoor）
	huma.Register(api, huma.Operation{
		OperationID: "casbinGetPermissions",
		Method:      http.MethodGet,
		Path:        "/api/v1/addons/casbin-rbac/permissions",
		Summary:     "获取权限列表",
		Description: "从 Casdoor 获取所有权限策略",
		Tags:        []string{"Casbin RBAC"},
	}, func(ctx context.Context, input *struct{}) (*CasbinPermissionsOutput, error) {
		resp := &CasbinPermissionsOutput{}

		perms, err := service.GetPermissions()
		if err != nil {
			return nil, huma.NewError(http.StatusInternalServerError, "获取权限失败: "+err.Error())
		}

		items := make([]CasbinPermissionItem, 0, len(perms))
		for _, p := range perms {
			items = append(items, CasbinPermissionItem{
				Name:        p.Name,
				DisplayName: p.DisplayName,
				Resources:   p.Resources,
				Actions:     p.Actions,
				Effect:      p.Effect,
				IsEnabled:   p.IsEnabled,
			})
		}

		resp.Body.Code = 0
		resp.Body.Message = "success"
		resp.Body.Data = &CasbinPermissionsData{Permissions: items}
		return resp, nil
	})

	// 添加权限（到 Casdoor）
	huma.Register(api, huma.Operation{
		OperationID: "casbinAddPermission",
		Method:      http.MethodPost,
		Path:        "/api/v1/addons/casbin-rbac/permissions",
		Summary:     "添加权限",
		Description: "在 Casdoor 中创建权限策略",
		Tags:        []string{"Casbin RBAC"},
	}, func(ctx context.Context, input *CasbinAddPermissionInput) (*CasbinMutationOutput, error) {
		resp := &CasbinMutationOutput{}

		ok, err := service.AddPermission(
			input.Body.Name,
			input.Body.DisplayName,
			input.Body.Resources,
			input.Body.Actions,
		)
		if err != nil {
			return nil, huma.NewError(http.StatusInternalServerError, "添加权限失败: "+err.Error())
		}
		if !ok {
			return nil, huma.NewError(http.StatusBadRequest, "权限创建失败")
		}

		resp.Body.Code = 0
		resp.Body.Message = "权限创建成功"
		return resp, nil
	})

	// 删除权限（从 Casdoor）
	huma.Register(api, huma.Operation{
		OperationID: "casbinDeletePermission",
		Method:      http.MethodDelete,
		Path:        "/api/v1/addons/casbin-rbac/permissions",
		Summary:     "删除权限",
		Description: "从 Casdoor 删除权限策略",
		Tags:        []string{"Casbin RBAC"},
	}, func(ctx context.Context, input *CasbinDeletePermissionInput) (*CasbinMutationOutput, error) {
		resp := &CasbinMutationOutput{}

		ok, err := service.DeletePermission(input.Body.Name)
		if err != nil {
			return nil, huma.NewError(http.StatusInternalServerError, "删除权限失败: "+err.Error())
		}
		if !ok {
			return nil, huma.NewError(http.StatusNotFound, "权限不存在或删除失败")
		}

		resp.Body.Code = 0
		resp.Body.Message = "权限删除成功"
		return resp, nil
	})

	// 检查权限（本地匹配）
	huma.Register(api, huma.Operation{
		OperationID: "casbinEnforce",
		Method:      http.MethodGet,
		Path:        "/api/v1/addons/casbin-rbac/enforce",
		Summary:     "检查权限",
		Description: "检查指定角色对路径和方法的访问权限",
		Tags:        []string{"Casbin RBAC"},
	}, func(ctx context.Context, input *CasbinEnforceInput) (*CasbinEnforceOutput, error) {
		resp := &CasbinEnforceOutput{}

		allowed, err := service.Enforce("", "", []string{input.Role}, false, input.Path, input.Method)
		if err != nil {
			return nil, huma.NewError(http.StatusInternalServerError, "权限检查失败: "+err.Error())
		}

		resp.Body.Code = 0
		resp.Body.Message = "success"
		resp.Body.Data = &CasbinEnforceData{Allowed: allowed}
		return resp, nil
	})

	// 获取角色列表（从 Casdoor）
	huma.Register(api, huma.Operation{
		OperationID: "casbinGetRoles",
		Method:      http.MethodGet,
		Path:        "/api/v1/addons/casbin-rbac/roles",
		Summary:     "获取角色列表",
		Description: "从 Casdoor 获取所有角色",
		Tags:        []string{"Casbin RBAC"},
	}, func(ctx context.Context, input *struct{}) (*CasbinRolesOutput, error) {
		resp := &CasbinRolesOutput{}

		roles, err := service.GetRoles()
		if err != nil {
			return nil, huma.NewError(http.StatusInternalServerError, "获取角色失败: "+err.Error())
		}

		items := make([]CasbinRoleItem, 0, len(roles))
		for _, r := range roles {
			items = append(items, CasbinRoleItem{
				Name:        r.Name,
				DisplayName: r.DisplayName,
				Users:       r.Users,
				IsEnabled:   r.IsEnabled,
			})
		}

		resp.Body.Code = 0
		resp.Body.Message = "success"
		resp.Body.Data = &CasbinRolesData{Roles: items}
		return resp, nil
	})

	// 添加角色（到 Casdoor）
	huma.Register(api, huma.Operation{
		OperationID: "casbinAddRole",
		Method:      http.MethodPost,
		Path:        "/api/v1/addons/casbin-rbac/roles",
		Summary:     "添加角色",
		Description: "在 Casdoor 中创建角色",
		Tags:        []string{"Casbin RBAC"},
	}, func(ctx context.Context, input *CasbinAddRoleInput) (*CasbinMutationOutput, error) {
		resp := &CasbinMutationOutput{}

		ok, err := service.AddRole(input.Body.Name, input.Body.DisplayName, input.Body.Users)
		if err != nil {
			return nil, huma.NewError(http.StatusInternalServerError, "添加角色失败: "+err.Error())
		}
		if !ok {
			return nil, huma.NewError(http.StatusBadRequest, "角色创建失败")
		}

		resp.Body.Code = 0
		resp.Body.Message = "角色创建成功"
		return resp, nil
	})

	// 删除角色（从 Casdoor）
	huma.Register(api, huma.Operation{
		OperationID: "casbinDeleteRole",
		Method:      http.MethodDelete,
		Path:        "/api/v1/addons/casbin-rbac/roles",
		Summary:     "删除角色",
		Description: "从 Casdoor 删除角色",
		Tags:        []string{"Casbin RBAC"},
	}, func(ctx context.Context, input *CasbinDeleteRoleInput) (*CasbinMutationOutput, error) {
		resp := &CasbinMutationOutput{}

		ok, err := service.DeleteRole(input.Body.Name)
		if err != nil {
			return nil, huma.NewError(http.StatusInternalServerError, "删除角色失败: "+err.Error())
		}
		if !ok {
			return nil, huma.NewError(http.StatusNotFound, "角色不存在或删除失败")
		}

		resp.Body.Code = 0
		resp.Body.Message = "角色删除成功"
		return resp, nil
	})

	// 获取用户角色（从 Casdoor）
	huma.Register(api, huma.Operation{
		OperationID: "casbinGetUserRoles",
		Method:      http.MethodGet,
		Path:        "/api/v1/addons/casbin-rbac/roles/user",
		Summary:     "获取用户角色",
		Description: "从 Casdoor 获取指定用户的所有角色",
		Tags:        []string{"Casbin RBAC"},
	}, func(ctx context.Context, input *CasbinGetUserRolesInput) (*CasbinUserRolesOutput, error) {
		resp := &CasbinUserRolesOutput{}

		roles, err := service.GetUserRoles(input.User)
		if err != nil {
			return nil, huma.NewError(http.StatusInternalServerError, "获取角色失败: "+err.Error())
		}

		resp.Body.Code = 0
		resp.Body.Message = "success"
		resp.Body.Data = &CasbinUserRolesData{Roles: roles}
		return resp, nil
	})
}
