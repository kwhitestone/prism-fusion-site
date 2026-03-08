package router

import (
	"context"
	"net/http"
	"net/url"
	"strings"

	"whitestone.top/prism-example-site/addons/casbin-rbac/service"

	"github.com/danielgtaylor/huma/v2"
)

// ============================================================
// 管理页面 API — 用户 / 角色 / 权限 CRUD
// ============================================================

// ----- User types -----

type AdminUserItem struct {
	Name        string   `json:"name" doc:"用户名"`
	DisplayName string   `json:"displayName" doc:"显示名"`
	Email       string   `json:"email" doc:"邮箱"`
	Phone       string   `json:"phone" doc:"手机号"`
	Avatar      string   `json:"avatar" doc:"头像"`
	IsAdmin     bool     `json:"isAdmin" doc:"是否管理员"`
	IsForbidden bool     `json:"isForbidden" doc:"是否禁用"`
	CreatedTime string   `json:"createdTime" doc:"创建时间"`
	Roles       []string `json:"roles" doc:"角色列表"`
}

type AdminUsersOutput struct {
	Body struct {
		Code    int             `json:"code" doc:"状态码"`
		Message string          `json:"message" doc:"响应消息"`
		Data    []AdminUserItem `json:"data" doc:"用户列表"`
	}
}

type AdminUpdateUserRolesInput struct {
	Body struct {
		Username string   `json:"username" required:"true" doc:"用户名"`
		Roles    []string `json:"roles" required:"true" doc:"角色名列表"`
	}
}

type AdminAddUserInput struct {
	Body struct {
		Name        string `json:"name" required:"true" doc:"用户名（登录名）"`
		DisplayName string `json:"displayName" doc:"显示名"`
		Email       string `json:"email" doc:"邮箱"`
		Password    string `json:"password" required:"true" doc:"初始密码"`
		Avatar      string `json:"avatar" doc:"头像 URL（为空则自动生成默认头像）"`
	}
}

type AdminDeleteUserInput struct {
	Body struct {
		Name string `json:"name" required:"true" doc:"用户名"`
	}
}

type AdminToggleUserInput struct {
	Body struct {
		Username    string `json:"username" required:"true" doc:"用户名"`
		IsForbidden bool   `json:"isForbidden" doc:"是否禁用"`
	}
}

type AdminUpdateUserProfileInput struct {
	Body struct {
		Name        string `json:"name" required:"true" doc:"用户名"`
		DisplayName string `json:"displayName" doc:"显示名"`
		Email       string `json:"email" doc:"邮箱"`
		Phone       string `json:"phone" doc:"手机号"`
		Avatar      string `json:"avatar" doc:"头像 URL"`
	}
}

type AdminUploadAvatarInput struct {
	Name     string `query:"name" required:"true" doc:"用户名"`
	RawBody  []byte `doc:"头像文件二进制内容"`
	Filename string `header:"X-Filename" doc:"文件名"`
}

type AdminUploadAvatarOutput struct {
	Body struct {
		Code    int    `json:"code" doc:"状态码"`
		Message string `json:"message" doc:"响应消息"`
		Data    string `json:"data" doc:"头像 URL"`
	}
}

type AdminPresignAvatarInput struct {
	Name     string `query:"name" required:"true" doc:"用户名"`
	Filename string `query:"filename" required:"true" doc:"文件名（含扩展名）"`
}

type AdminPresignAvatarOutput struct {
	Body struct {
		Code    int `json:"code" doc:"状态码"`
		Message string `json:"message" doc:"响应消息"`
		Data    struct {
			PresignedURL string `json:"presignedUrl" doc:"预签名 PUT URL（浏览器直传用）"`
			AvatarURL    string `json:"avatarUrl" doc:"最终公开访问地址"`
		} `json:"data"`
	}
}

type AdminConfirmAvatarInput struct {
	Body struct {
		Name      string `json:"name" required:"true" doc:"用户名"`
		AvatarURL string `json:"avatarUrl" required:"true" doc:"头像公开 URL"`
	}
}

type AdminConfirmAvatarOutput struct {
	Body struct {
		Code    int    `json:"code" doc:"状态码"`
		Message string `json:"message" doc:"响应消息"`
	}
}

// ----- Role types -----

type AdminRoleItem struct {
	Name        string   `json:"name" doc:"角色名"`
	DisplayName string   `json:"displayName" doc:"显示名"`
	Description string   `json:"description" doc:"描述"`
	Users       []string `json:"users" doc:"关联用户"`
	IsEnabled   bool     `json:"isEnabled" doc:"是否启用"`
	CreatedTime string   `json:"createdTime" doc:"创建时间"`
}

type AdminRolesOutput struct {
	Body struct {
		Code    int             `json:"code" doc:"状态码"`
		Message string          `json:"message" doc:"响应消息"`
		Data    []AdminRoleItem `json:"data" doc:"角色列表"`
	}
}

type AdminCreateRoleInput struct {
	Body struct {
		Name        string `json:"name" required:"true" doc:"角色名（英文标识）"`
		DisplayName string `json:"displayName" doc:"显示名"`
	}
}

type AdminUpdateRoleInput struct {
	Body struct {
		Name        string   `json:"name" required:"true" doc:"角色名"`
		DisplayName string   `json:"displayName" doc:"显示名"`
		Users       []string `json:"users" doc:"关联用户名列表"`
	}
}

type AdminDeleteRoleInput struct {
	Body struct {
		Name string `json:"name" required:"true" doc:"角色名"`
	}
}

// ----- Permission types -----

type AdminPermissionItem struct {
	Name        string   `json:"name" doc:"权限名"`
	DisplayName string   `json:"displayName" doc:"显示名"`
	Resources   []string `json:"resources" doc:"资源(API路径)列表"`
	Actions     []string `json:"actions" doc:"操作(HTTP方法)列表"`
	Roles       []string `json:"roles" doc:"关联角色"`
	Effect      string   `json:"effect" doc:"效果(Allow/Deny)"`
	IsEnabled   bool     `json:"isEnabled" doc:"是否启用"`
}

type AdminPermissionsOutput struct {
	Body struct {
		Code    int                   `json:"code" doc:"状态码"`
		Message string                `json:"message" doc:"响应消息"`
		Data    []AdminPermissionItem `json:"data" doc:"权限列表"`
	}
}

type AdminCreatePermissionInput struct {
	Body struct {
		Name        string   `json:"name" required:"true" doc:"权限名（英文标识）"`
		DisplayName string   `json:"displayName" doc:"显示名"`
		Resources   []string `json:"resources" required:"true" doc:"API路径列表"`
		Actions     []string `json:"actions" required:"true" doc:"HTTP方法列表(GET/POST/PUT/DELETE/*)"`
		Roles       []string `json:"roles" doc:"关联角色名列表"`
	}
}

type AdminUpdatePermissionInput struct {
	Body struct {
		Name        string   `json:"name" required:"true" doc:"权限名"`
		DisplayName string   `json:"displayName" doc:"显示名"`
		Resources   []string `json:"resources" doc:"API路径列表"`
		Actions     []string `json:"actions" doc:"HTTP方法列表"`
		Roles       []string `json:"roles" doc:"关联角色名列表"`
		Effect      string   `json:"effect" doc:"效果(Allow/Deny)"`
		IsEnabled   *bool    `json:"isEnabled" doc:"是否启用"`
	}
}

type AdminDeletePermissionInput struct {
	Body struct {
		Name string `json:"name" required:"true" doc:"权限名"`
	}
}

// ----- Common output -----

type AdminMutationOutput struct {
	Body struct {
		Code    int    `json:"code" doc:"状态码"`
		Message string `json:"message" doc:"响应消息"`
	}
}

// stripOwner 去掉 "owner/" 前缀
func stripOwner(s string) string {
	if idx := strings.Index(s, "/"); idx >= 0 {
		return s[idx+1:]
	}
	return s
}

// RegisterAdminRoutes 注册管理后台路由
func RegisterAdminRoutes(api huma.API) {
	const basePath = "/api/v1/addons/casbin-rbac/admin"
	const tag = "RBAC Admin"

	// ==================== 用户管理 ====================

	// 获取用户列表
	huma.Register(api, huma.Operation{
		OperationID: "adminGetUsers",
		Method:      http.MethodGet,
		Path:        basePath + "/users",
		Summary:     "获取用户列表",
		Description: "从 Casdoor 获取组织下所有用户（含角色信息）",
		Tags:        []string{tag},
	}, func(ctx context.Context, input *struct{}) (*AdminUsersOutput, error) {
		resp := &AdminUsersOutput{}

		users, err := service.GetUsers()
		if err != nil {
			return nil, huma.NewError(http.StatusInternalServerError, "获取用户失败: "+err.Error())
		}

		// Casdoor 的 GetUsers 列表接口不填充 User.Roles，需从角色侧反查
		roles, _ := service.GetRoles()
		// 构建 userPath → []roleName 映射
		userRoleMap := make(map[string][]string)
		if roles != nil {
			for _, r := range roles {
				for _, u := range r.Users {
					userRoleMap[u] = append(userRoleMap[u], r.Name)
				}
			}
		}
		owner := ""
		if len(users) > 0 {
			owner = users[0].Owner
		}

		items := make([]AdminUserItem, 0, len(users))
		for _, u := range users {
			userPath := owner + "/" + u.Name
			roleNames := userRoleMap[userPath]
			if roleNames == nil {
				roleNames = []string{}
			}
			items = append(items, AdminUserItem{
				Name:        u.Name,
				DisplayName: u.DisplayName,
				Email:       u.Email,
				Phone:       u.Phone,
				Avatar:      u.Avatar,
				IsAdmin:     u.IsAdmin,
				IsForbidden: u.IsForbidden,
				CreatedTime: u.CreatedTime,
				Roles:       roleNames,
			})
		}

		resp.Body.Code = 0
		resp.Body.Message = "success"
		resp.Body.Data = items
		return resp, nil
	})

	// 更新用户角色
	huma.Register(api, huma.Operation{
		OperationID: "adminUpdateUserRoles",
		Method:      http.MethodPut,
		Path:        basePath + "/users/roles",
		Summary:     "更新用户角色",
		Description: "修改指定用户关联的角色列表",
		Tags:        []string{tag},
	}, func(ctx context.Context, input *AdminUpdateUserRolesInput) (*AdminMutationOutput, error) {
		resp := &AdminMutationOutput{}
		if err := service.UpdateUserRoles(input.Body.Username, input.Body.Roles); err != nil {
			return nil, huma.NewError(http.StatusInternalServerError, "更新用户角色失败: "+err.Error())
		}
		resp.Body.Code = 0
		resp.Body.Message = "用户角色更新成功"
		return resp, nil
	})

	// 启用/禁用用户
	huma.Register(api, huma.Operation{
		OperationID: "adminToggleUser",
		Method:      http.MethodPut,
		Path:        basePath + "/users/toggle",
		Summary:     "启停用户",
		Description: "启用或禁用指定用户",
		Tags:        []string{tag},
	}, func(ctx context.Context, input *AdminToggleUserInput) (*AdminMutationOutput, error) {
		resp := &AdminMutationOutput{}
		if err := service.ToggleUserForbidden(input.Body.Username, input.Body.IsForbidden); err != nil {
			return nil, huma.NewError(http.StatusInternalServerError, "操作失败: "+err.Error())
		}
		msg := "用户已启用"
		if input.Body.IsForbidden {
			msg = "用户已禁用"
		}
		resp.Body.Code = 0
		resp.Body.Message = msg
		return resp, nil
	})

	// 新建用户
	huma.Register(api, huma.Operation{
		OperationID: "adminAddUser",
		Method:      http.MethodPost,
		Path:        basePath + "/users",
		Summary:     "新建用户",
		Description: "在 Casdoor 中创建新用户",
		Tags:        []string{tag},
	}, func(ctx context.Context, input *AdminAddUserInput) (*AdminMutationOutput, error) {
		resp := &AdminMutationOutput{}
		ok, err := service.AddUser(input.Body.Name, input.Body.DisplayName, input.Body.Email, input.Body.Password, input.Body.Avatar)
		if err != nil {
			return nil, huma.NewError(http.StatusInternalServerError, "创建用户失败: "+err.Error())
		}
		if !ok {
			return nil, huma.NewError(http.StatusBadRequest, "用户创建失败（可能已存在）")
		}
		resp.Body.Code = 0
		resp.Body.Message = "用户创建成功"
		return resp, nil
	})

	// 删除用户
	huma.Register(api, huma.Operation{
		OperationID: "adminDeleteUser",
		Method:      http.MethodDelete,
		Path:        basePath + "/users",
		Summary:     "删除用户",
		Description: "从 Casdoor 删除用户",
		Tags:        []string{tag},
	}, func(ctx context.Context, input *AdminDeleteUserInput) (*AdminMutationOutput, error) {
		resp := &AdminMutationOutput{}
		ok, err := service.DeleteUser(input.Body.Name)
		if err != nil {
			return nil, huma.NewError(http.StatusInternalServerError, "删除用户失败: "+err.Error())
		}
		if !ok {
			return nil, huma.NewError(http.StatusNotFound, "用户不存在或删除失败")
		}
		resp.Body.Code = 0
		resp.Body.Message = "用户删除成功"
		return resp, nil
	})

	// 更新用户资料
	huma.Register(api, huma.Operation{
		OperationID: "adminUpdateUserProfile",
		Method:      http.MethodPut,
		Path:        basePath + "/users/profile",
		Summary:     "更新用户资料",
		Description: "修改用户显示名、邮箱、手机号、头像等信息",
		Tags:        []string{tag},
	}, func(ctx context.Context, input *AdminUpdateUserProfileInput) (*AdminMutationOutput, error) {
		resp := &AdminMutationOutput{}
		if err := service.UpdateUserProfile(
			input.Body.Name,
			input.Body.DisplayName,
			input.Body.Email,
			input.Body.Phone,
			input.Body.Avatar,
		); err != nil {
			return nil, huma.NewError(http.StatusInternalServerError, "更新用户资料失败: "+err.Error())
		}
		resp.Body.Code = 0
		resp.Body.Message = "用户资料更新成功"
		return resp, nil
	})

	// 上传用户头像
	huma.Register(api, huma.Operation{
		OperationID: "adminUploadAvatar",
		Method:      http.MethodPost,
		Path:        basePath + "/users/avatar",
		Summary:     "上传用户头像",
		Description: "上传头像文件到 Casdoor 并返回 URL",
		Tags:        []string{tag},
	}, func(ctx context.Context, input *AdminUploadAvatarInput) (*AdminUploadAvatarOutput, error) {
		resp := &AdminUploadAvatarOutput{}
		if len(input.RawBody) == 0 {
			return nil, huma.NewError(http.StatusBadRequest, "未提供文件内容")
		}
		filename := input.Filename
		if filename == "" {
			filename = "avatar.png"
		}
		// 前端通过 encodeURIComponent 编码文件名以避免 HTTP header ISO-8859-1 限制
		if decoded, err := url.QueryUnescape(filename); err == nil {
			filename = decoded
		}
		avatarURL, err := service.UploadUserAvatar(input.Name, filename, input.RawBody)
		if err != nil {
			return nil, huma.NewError(http.StatusInternalServerError, "上传头像失败: "+err.Error())
		}
		resp.Body.Code = 0
		resp.Body.Message = "头像上传成功"
		resp.Body.Data = avatarURL
		return resp, nil
	})

	// 获取头像预签名上传 URL
	huma.Register(api, huma.Operation{
		OperationID: "adminPresignAvatar",
		Method:      http.MethodGet,
		Path:        basePath + "/users/avatar/presign",
		Summary:     "获取头像预签名上传 URL",
		Description: "生成一个 5 分钟有效的 S3 预签名 PUT URL，前端拿到后直接将文件 PUT 到该地址",
		Tags:        []string{tag},
	}, func(ctx context.Context, input *AdminPresignAvatarInput) (*AdminPresignAvatarOutput, error) {
		resp := &AdminPresignAvatarOutput{}
		filename := input.Filename
		if decoded, err := url.QueryUnescape(filename); err == nil {
			filename = decoded
		}
		presignedURL, avatarURL, err := service.PresignAvatarUpload(input.Name, filename)
		if err != nil {
			return nil, huma.NewError(http.StatusInternalServerError, "生成预签名 URL 失败: "+err.Error())
		}
		resp.Body.Code = 0
		resp.Body.Message = "预签名 URL 生成成功"
		resp.Body.Data.PresignedURL = presignedURL
		resp.Body.Data.AvatarURL = avatarURL
		return resp, nil
	})

	// 确认头像上传完成
	huma.Register(api, huma.Operation{
		OperationID: "adminConfirmAvatar",
		Method:      http.MethodPost,
		Path:        basePath + "/users/avatar/confirm",
		Summary:     "确认头像上传完成",
		Description: "前端 PUT 完成后调用此接口，将头像 URL 写入 Casdoor 用户记录",
		Tags:        []string{tag},
	}, func(ctx context.Context, input *AdminConfirmAvatarInput) (*AdminConfirmAvatarOutput, error) {
		resp := &AdminConfirmAvatarOutput{}
		if err := service.ConfirmUserAvatar(input.Body.Name, input.Body.AvatarURL); err != nil {
			return nil, huma.NewError(http.StatusInternalServerError, "确认头像失败: "+err.Error())
		}
		resp.Body.Code = 0
		resp.Body.Message = "头像确认成功"
		return resp, nil
	})

	// ==================== 角色管理 ====================

	// 获取角色列表
	huma.Register(api, huma.Operation{
		OperationID: "adminGetRoles",
		Method:      http.MethodGet,
		Path:        basePath + "/roles",
		Summary:     "获取角色列表",
		Description: "从 Casdoor 获取所有角色（含关联用户）",
		Tags:        []string{tag},
	}, func(ctx context.Context, input *struct{}) (*AdminRolesOutput, error) {
		resp := &AdminRolesOutput{}

		roles, err := service.GetRoles()
		if err != nil {
			return nil, huma.NewError(http.StatusInternalServerError, "获取角色失败: "+err.Error())
		}

		items := make([]AdminRoleItem, 0, len(roles))
		for _, r := range roles {
			users := make([]string, 0, len(r.Users))
			for _, u := range r.Users {
				users = append(users, stripOwner(u))
			}
			items = append(items, AdminRoleItem{
				Name:        r.Name,
				DisplayName: r.DisplayName,
				Description: r.Description,
				Users:       users,
				IsEnabled:   r.IsEnabled,
				CreatedTime: r.CreatedTime,
			})
		}

		resp.Body.Code = 0
		resp.Body.Message = "success"
		resp.Body.Data = items
		return resp, nil
	})

	// 创建角色
	huma.Register(api, huma.Operation{
		OperationID: "adminCreateRole",
		Method:      http.MethodPost,
		Path:        basePath + "/roles",
		Summary:     "创建角色",
		Description: "在 Casdoor 中创建新角色",
		Tags:        []string{tag},
	}, func(ctx context.Context, input *AdminCreateRoleInput) (*AdminMutationOutput, error) {
		resp := &AdminMutationOutput{}
		ok, err := service.AddRole(input.Body.Name, input.Body.DisplayName, nil)
		if err != nil {
			return nil, huma.NewError(http.StatusInternalServerError, "创建角色失败: "+err.Error())
		}
		if !ok {
			return nil, huma.NewError(http.StatusBadRequest, "角色创建失败（可能已存在）")
		}
		resp.Body.Code = 0
		resp.Body.Message = "角色创建成功"
		return resp, nil
	})

	// 更新角色
	huma.Register(api, huma.Operation{
		OperationID: "adminUpdateRole",
		Method:      http.MethodPut,
		Path:        basePath + "/roles",
		Summary:     "更新角色",
		Description: "修改角色的显示名和关联用户",
		Tags:        []string{tag},
	}, func(ctx context.Context, input *AdminUpdateRoleInput) (*AdminMutationOutput, error) {
		resp := &AdminMutationOutput{}
		if err := service.UpdateRole(input.Body.Name, input.Body.DisplayName, input.Body.Users); err != nil {
			return nil, huma.NewError(http.StatusInternalServerError, "更新角色失败: "+err.Error())
		}
		resp.Body.Code = 0
		resp.Body.Message = "角色更新成功"
		return resp, nil
	})

	// 删除角色
	huma.Register(api, huma.Operation{
		OperationID: "adminDeleteRole",
		Method:      http.MethodDelete,
		Path:        basePath + "/roles",
		Summary:     "删除角色",
		Description: "从 Casdoor 删除指定角色",
		Tags:        []string{tag},
	}, func(ctx context.Context, input *AdminDeleteRoleInput) (*AdminMutationOutput, error) {
		resp := &AdminMutationOutput{}
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

	// ==================== 权限管理 ====================

	// 获取权限列表
	huma.Register(api, huma.Operation{
		OperationID: "adminGetPermissions",
		Method:      http.MethodGet,
		Path:        basePath + "/permissions",
		Summary:     "获取权限列表",
		Description: "从 Casdoor 获取所有权限策略（含关联角色）",
		Tags:        []string{tag},
	}, func(ctx context.Context, input *struct{}) (*AdminPermissionsOutput, error) {
		resp := &AdminPermissionsOutput{}

		perms, err := service.GetPermissions()
		if err != nil {
			return nil, huma.NewError(http.StatusInternalServerError, "获取权限失败: "+err.Error())
		}

		items := make([]AdminPermissionItem, 0, len(perms))
		for _, p := range perms {
			roles := make([]string, 0, len(p.Roles))
			for _, r := range p.Roles {
				roles = append(roles, stripOwner(r))
			}
			items = append(items, AdminPermissionItem{
				Name:        p.Name,
				DisplayName: p.DisplayName,
				Resources:   p.Resources,
				Actions:     p.Actions,
				Roles:       roles,
				Effect:      p.Effect,
				IsEnabled:   p.IsEnabled,
			})
		}

		resp.Body.Code = 0
		resp.Body.Message = "success"
		resp.Body.Data = items
		return resp, nil
	})

	// 创建权限
	huma.Register(api, huma.Operation{
		OperationID: "adminCreatePermission",
		Method:      http.MethodPost,
		Path:        basePath + "/permissions",
		Summary:     "创建权限",
		Description: "在 Casdoor 中创建权限策略（含角色关联）",
		Tags:        []string{tag},
	}, func(ctx context.Context, input *AdminCreatePermissionInput) (*AdminMutationOutput, error) {
		resp := &AdminMutationOutput{}
		ok, err := service.AddPermissionWithRoles(
			input.Body.Name,
			input.Body.DisplayName,
			input.Body.Resources,
			input.Body.Actions,
			input.Body.Roles,
		)
		if err != nil {
			return nil, huma.NewError(http.StatusInternalServerError, "创建权限失败: "+err.Error())
		}
		if !ok {
			return nil, huma.NewError(http.StatusBadRequest, "权限创建失败（可能已存在）")
		}
		resp.Body.Code = 0
		resp.Body.Message = "权限创建成功"
		return resp, nil
	})

	// 更新权限
	huma.Register(api, huma.Operation{
		OperationID: "adminUpdatePermission",
		Method:      http.MethodPut,
		Path:        basePath + "/permissions",
		Summary:     "更新权限",
		Description: "修改权限的资源、动作、关联角色等",
		Tags:        []string{tag},
	}, func(ctx context.Context, input *AdminUpdatePermissionInput) (*AdminMutationOutput, error) {
		resp := &AdminMutationOutput{}
		if err := service.UpdatePermission(
			input.Body.Name,
			input.Body.DisplayName,
			input.Body.Resources,
			input.Body.Actions,
			input.Body.Roles,
			input.Body.Effect,
			input.Body.IsEnabled,
		); err != nil {
			return nil, huma.NewError(http.StatusInternalServerError, "更新权限失败: "+err.Error())
		}
		resp.Body.Code = 0
		resp.Body.Message = "权限更新成功"
		return resp, nil
	})

	// 删除权限
	huma.Register(api, huma.Operation{
		OperationID: "adminDeletePermission",
		Method:      http.MethodDelete,
		Path:        basePath + "/permissions",
		Summary:     "删除权限",
		Description: "从 Casdoor 删除权限策略",
		Tags:        []string{tag},
	}, func(ctx context.Context, input *AdminDeletePermissionInput) (*AdminMutationOutput, error) {
		resp := &AdminMutationOutput{}
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

	// ==================== 权限测试 ====================

	// 测试权限（快速验证某角色对某路径是否有权限）
	huma.Register(api, huma.Operation{
		OperationID: "adminTestEnforce",
		Method:      http.MethodGet,
		Path:        basePath + "/test-enforce",
		Summary:     "测试权限",
		Description: "快速验证角色对 API 路径的访问权限",
		Tags:        []string{tag},
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
}
