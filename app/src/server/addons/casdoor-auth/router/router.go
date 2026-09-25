package router

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/kwhitestone/prism-fusion/global"
	"top.whitestone/prism-fusion-site/addons/casdoor-auth/conf"
	"top.whitestone/prism-fusion-site/addons/casdoor-auth/service"

	"github.com/danielgtaylor/huma/v2"
	"go.uber.org/zap"
)

var casdoorSvc = &service.CasdoorService{}

// ---- Named response data types ----

// CasdoorSigninURLData Casdoor 登录 URL 数据
type CasdoorSigninURLData struct {
	URL string `json:"url" doc:"Casdoor OAuth2 登录地址"`
}

// CasdoorTokenData Casdoor Token 数据
type CasdoorTokenData struct {
	AccessToken  string            `json:"accessToken" doc:"访问令牌"`
	RefreshToken string            `json:"refreshToken" doc:"刷新令牌"`
	ExpiresIn    int64             `json:"expiresIn" doc:"Access Token 过期时间（秒）"`
	User         *CasdoorUserBrief `json:"user" doc:"用户基本信息"`
}

// CasdoorUserBrief Casdoor 用户简要信息
type CasdoorUserBrief struct {
	Username string `json:"username" doc:"用户名"`
	NickName string `json:"nickName" doc:"昵称"`
	Email    string `json:"email" doc:"邮箱"`
	IsAdmin  bool   `json:"isAdmin" doc:"是否管理员"`
}

// CasdoorUserInfoData 用户详细信息
type CasdoorUserInfoData struct {
	Username string   `json:"username" doc:"用户名"`
	NickName string   `json:"nickName" doc:"昵称"`
	Email    string   `json:"email" doc:"邮箱"`
	Avatar   string   `json:"avatar" doc:"头像"`
	IsAdmin  bool     `json:"isAdmin" doc:"是否管理员"`
	Roles    []string `json:"roles" doc:"角色列表"`
	RoleID   uint     `json:"roleId" doc:"角色ID（兼容）"`
}

// ---- Input types ----

// SigninURLInput 获取登录URL请求
type SigninURLInput struct {
	State       string `query:"state" doc:"OAuth2 state 参数" default:"casdoor"`
	RedirectUri string `query:"redirectUri" doc:"OAuth2 回调地址（前端从地址栏自动获取）" required:"true"`
}

// SigninCallbackInput 回调请求
type SigninCallbackInput struct {
	Body struct {
		Code        string `json:"code" required:"true" doc:"授权码"`
		State       string `json:"state" doc:"OAuth2 state 参数"`
		RedirectUri string `json:"redirectUri" required:"true" doc:"OAuth2 回调地址（必须与发起登录时一致）"`
	}
}

// CasdoorRefreshInput 刷新Token请求
type CasdoorRefreshInput struct {
	Body struct {
		RefreshToken string `json:"refreshToken" required:"true" doc:"刷新令牌"`
	}
}

type CasdoorLogoutInput struct {
	Authorization string `header:"Authorization" required:"true"`
	Body          struct {
		RefreshToken string `json:"refreshToken" maxLength:"16384"`
	}
}
type CasdoorLogoutOutput struct {
	Body struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	}
}

// ---- Output types ----

// SigninURLOutput 登录URL响应
type SigninURLOutput struct {
	Body struct {
		Code    int                   `json:"code" doc:"状态码"`
		Message string                `json:"message" doc:"响应消息"`
		Data    *CasdoorSigninURLData `json:"data" doc:"登录URL"`
	}
}

// CasdoorTokenOutput Token 响应
type CasdoorTokenOutput struct {
	Body struct {
		Code    int               `json:"code" doc:"状态码"`
		Message string            `json:"message" doc:"响应消息"`
		Data    *CasdoorTokenData `json:"data" doc:"Token数据"`
	}
}

// CasdoorUserInfoOutput 用户信息响应
type CasdoorUserInfoOutput struct {
	Body struct {
		Code    int                  `json:"code" doc:"状态码"`
		Message string               `json:"message" doc:"响应消息"`
		Data    *CasdoorUserInfoData `json:"data" doc:"用户信息"`
	}
}

// CasdoorConfigData Casdoor 配置信息（前端需要知道 Casdoor 地址和应用 ID 来发起 OAuth 登录）
type CasdoorConfigData struct {
	Endpoint         string `json:"endpoint" doc:"Casdoor 外部服务地址（浏览器可达）"`
	ClientID         string `json:"clientId" doc:"应用 Client ID"`
	OrganizationName string `json:"organizationName" doc:"组织名称"`
	ApplicationName  string `json:"applicationName" doc:"应用名称"`
	DefaultAvatarURL string `json:"defaultAvatarUrl" doc:"默认头像 URL"`
}

// CasdoorConfigOutput Casdoor 配置响应
type CasdoorConfigOutput struct {
	Body struct {
		Code    int                `json:"code" doc:"状态码"`
		Message string             `json:"message" doc:"响应消息"`
		Data    *CasdoorConfigData `json:"data" doc:"Casdoor 配置"`
	}
}

// RegisterRoutes 注册 Casdoor 认证路由
func RegisterRoutes(api huma.API) {
	// 获取 Casdoor 配置信息（前端用于发起 OAuth 重定向）
	huma.Register(api, huma.Operation{
		OperationID: "casdoorGetConfig",
		Method:      http.MethodGet,
		Path:        "/api/v1/addons/casdoor-auth/config",
		Summary:     "获取 Casdoor 配置",
		Description: "返回前端发起 OAuth2 登录所需的 Casdoor 配置信息",
		Tags:        []string{"Casdoor Auth"},
	}, func(ctx context.Context, input *struct{}) (*CasdoorConfigOutput, error) {
		cfg := conf.Get()
		resp := &CasdoorConfigOutput{}
		resp.Body.Code = 0
		resp.Body.Message = "success"
		resp.Body.Data = &CasdoorConfigData{
			Endpoint:         cfg.ExternalEndpoint,
			ClientID:         cfg.ClientID,
			OrganizationName: cfg.OrganizationName,
			ApplicationName:  cfg.ApplicationName,
			DefaultAvatarURL: service.DefaultAvatarURL(),
		}
		return resp, nil
	})

	// 获取 Casdoor 登录 URL（OAuth2 重定向模式）
	huma.Register(api, huma.Operation{
		OperationID: "casdoorGetSigninURL",
		Method:      http.MethodGet,
		Path:        "/api/v1/addons/casdoor-auth/signin-url",
		Summary:     "获取 Casdoor 登录地址",
		Description: "返回 Casdoor OAuth2 授权页面 URL，前端重定向至此地址进行登录",
		Tags:        []string{"Casdoor Auth"},
	}, func(ctx context.Context, input *SigninURLInput) (*SigninURLOutput, error) {
		state := input.State
		if state == "" {
			state = "casdoor"
		}
		url := casdoorSvc.GetSigninURL(state, input.RedirectUri)

		resp := &SigninURLOutput{}
		resp.Body.Code = 0
		resp.Body.Message = "success"
		resp.Body.Data = &CasdoorSigninURLData{URL: url}
		return resp, nil
	})

	// OAuth2 回调：授权码换 Token
	huma.Register(api, huma.Operation{
		OperationID: "casdoorSigninCallback",
		Method:      http.MethodPost,
		Path:        "/api/v1/addons/casdoor-auth/signin-callback",
		Summary:     "OAuth2 授权码回调",
		Description: "使用 Casdoor 返回的授权码交换 Token",
		Tags:        []string{"Casdoor Auth"},
	}, func(ctx context.Context, input *SigninCallbackInput) (*CasdoorTokenOutput, error) {
		resp := &CasdoorTokenOutput{}

		claims, accessToken, refreshToken, err := casdoorSvc.ExchangeToken(input.Body.Code, input.Body.State, input.Body.RedirectUri)
		if err != nil {
			return nil, huma.NewError(http.StatusUnauthorized, "授权码交换失败: "+err.Error())
		}

		if refreshToken == "" {
			refreshToken = accessToken
		}

		// 用户重新登录，清除旧缓存以确保从 Casdoor API 获取最新权限数据
		// （解决：在 Casdoor 中修改用户角色后重新登录仍读到旧缓存的问题）
		casdoorSvc.InvalidateUserCache(claims.Subject)

		// JWT-Standard 格式下 claims.Name 可能是显示名（含非 ASCII 字符），
		// 需通过 API 补全为登录名，否则前端存到 x-user-id 头会触发 ISO-8859-1 错误
		claims = casdoorSvc.EnrichClaimsFromAPI(claims)

		resp.Body.Code = 0
		resp.Body.Message = "登录成功"
		resp.Body.Data = &CasdoorTokenData{
			AccessToken:  accessToken,
			RefreshToken: refreshToken,
			ExpiresIn:    service.GetTokenExpireSeconds(),
			User: &CasdoorUserBrief{
				Username: claims.Name,
				NickName: claims.DisplayName,
				Email:    claims.Email,
				IsAdmin:  claims.IsAdmin,
			},
		}
		return resp, nil
	})

	huma.Register(api, huma.Operation{OperationID: "casdoorLogout", Method: http.MethodPost,
		Path: "/api/v1/addons/casdoor-auth/logout", Summary: "撤销 Casdoor 会话", Tags: []string{"Casdoor Auth"}},
		func(ctx context.Context, input *CasdoorLogoutInput) (*CasdoorLogoutOutput, error) {
			access := strings.TrimPrefix(input.Authorization, "Bearer ")
			if err := casdoorSvc.Logout(ctx, access, input.Body.RefreshToken); err != nil {
				// Return a generic response; database/IdP errors must not disclose credentials.
				return nil, huma.NewError(http.StatusUnauthorized, "会话注销失败")
			}
			resp := &CasdoorLogoutOutput{}
			resp.Body.Message = "会话已注销"
			return resp, nil
		})

	// 刷新 Token
	huma.Register(api, huma.Operation{
		OperationID: "casdoorRefreshToken",
		Method:      http.MethodPost,
		Path:        "/api/v1/addons/casdoor-auth/refresh-token",
		Summary:     "刷新 Casdoor Token",
		Description: "使用 refresh token 获取新的 access token",
		Tags:        []string{"Casdoor Auth"},
	}, func(ctx context.Context, input *CasdoorRefreshInput) (*CasdoorTokenOutput, error) {
		resp := &CasdoorTokenOutput{}

		global.PRISM_LOG.Info("Refresh token request received",
			zap.Int("refreshTokenLen", len(input.Body.RefreshToken)),
		)

		newAccess, newRefresh, err := casdoorSvc.RefreshToken(input.Body.RefreshToken)
		if err != nil {
			global.PRISM_LOG.Warn("Refresh token failed", zap.Error(err))
			return nil, huma.NewError(http.StatusUnauthorized, "Token 刷新失败: "+err.Error())
		}

		global.PRISM_LOG.Info("Refresh token succeeded",
			zap.Bool("hasNewRefreshToken", newRefresh != ""),
		)

		if newRefresh == "" {
			newRefresh = newAccess
		}

		resp.Body.Code = 0
		resp.Body.Message = "刷新成功"
		resp.Body.Data = &CasdoorTokenData{
			AccessToken:  newAccess,
			RefreshToken: newRefresh,
			ExpiresIn:    service.GetTokenExpireSeconds(),
		}
		return resp, nil
	})

	// 获取当前用户信息
	huma.Register(api, huma.Operation{
		OperationID: "casdoorGetUserInfo",
		Method:      http.MethodGet,
		Path:        "/api/v1/addons/casdoor-auth/user-info",
		Summary:     "获取当前用户信息",
		Description: "根据 Casdoor Token 获取当前登录用户的信息",
		Tags:        []string{"Casdoor Auth"},
		Security: []map[string][]string{
			{"AuthTokenAuth": {}},
		},
	}, func(ctx context.Context, input *struct {
		Authorization string `header:"Authorization" doc:"JWT Token"`
	}) (*CasdoorUserInfoOutput, error) {
		resp := &CasdoorUserInfoOutput{}

		if input.Authorization == "" {
			return nil, huma.NewError(http.StatusUnauthorized, "未提供 Token")
		}

		tokenStr := input.Authorization
		if len(tokenStr) > 7 && tokenStr[:7] == "Bearer " {
			tokenStr = tokenStr[7:]
		}

		claims, err := casdoorSvc.GetUserByToken(tokenStr)
		if err != nil {
			return nil, huma.NewError(http.StatusUnauthorized, "Token 无效或已过期")
		}

		// JWT-Standard 格式下 Token 不含 Roles/IsAdmin 等 Casdoor 扩展字段，
		// 需通过 Casdoor API 补全（带缓存）
		claims = casdoorSvc.EnrichClaimsFromAPI(claims)

		// 提取 Casdoor 角色名列表
		roles := make([]string, 0)
		for _, r := range claims.Roles {
			if r != nil && r.Name != "" {
				roles = append(roles, r.Name)
			}
		}
		if len(roles) == 0 {
			roles = []string{"user"}
		}
		var roleID uint = 1
		if claims.IsAdmin {
			roleID = 999
		}

		resp.Body.Code = 0
		resp.Body.Message = "success"

		avatarURL := claims.Avatar
		if avatarURL == "" {
			avatarURL = service.DefaultAvatarURL()
		}

		resp.Body.Data = &CasdoorUserInfoData{
			Username: claims.Name,
			NickName: claims.DisplayName,
			Email:    claims.Email,
			Avatar:   avatarURL,
			IsAdmin:  claims.IsAdmin,
			Roles:    roles,
			RoleID:   roleID,
		}
		return resp, nil
	})

	// 清理组织：一键删除 bootstrap 创建的所有 Casdoor 资源
	huma.Register(api, huma.Operation{
		OperationID: "casdoorCleanupOrganization",
		Method:      http.MethodDelete,
		Path:        "/api/v1/addons/casdoor-auth/bootstrap",
		Summary:     "重置 Casdoor 组织",
		Description: "一键删除当前组织下的所有资源并重新创建（组织、应用、角色、权限、超管用户）。",
		Tags:        []string{"Casdoor Auth"},
		Security: []map[string][]string{
			{"AuthTokenAuth": {}},
		},
	}, func(ctx context.Context, input *struct {
		Authorization string `header:"Authorization" doc:"JWT Token"`
	}) (*struct {
		Body struct {
			Code    int      `json:"code" doc:"状态码"`
			Message string   `json:"message" doc:"消息"`
			Data    []string `json:"data" doc:"删除明细"`
		}
	}, error) {
		resp := &struct {
			Body struct {
				Code    int      `json:"code" doc:"状态码"`
				Message string   `json:"message" doc:"消息"`
				Data    []string `json:"data" doc:"删除明细"`
			}
		}{}

		summary, err := service.CleanupOrganization()
		if err != nil {
			resp.Body.Code = 7
			resp.Body.Message = "清理过程有错误: " + err.Error()
			resp.Body.Data = summary
			return resp, nil
		}

		if len(summary) == 0 {
			resp.Body.Code = 0
			resp.Body.Message = "组织不存在或已被清理"
			return resp, nil
		}

		// 清理完成后强制重新创建（绕过 sync.Once，并清空旧凭据缓存）
		service.ForceBootstrap()

		// 重新初始化 SDK（重新发现新的 clientId/clientSecret 并更新 casdoorsdk）
		casdoorSvc.InitSDK()

		resp.Body.Code = 0
		resp.Body.Message = fmt.Sprintf("重置完成：删除 %d 项资源并已重新创建。", len(summary))
		resp.Body.Data = summary
		return resp, nil
	})
}
