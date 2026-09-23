package service

import (
	"fmt"

	"top.whitestone/prism-fusion-site/addons/casdoor-auth/conf"
	casdoorService "top.whitestone/prism-fusion-site/addons/casdoor-auth/service"

	"github.com/casdoor/casdoor-go-sdk/casdoorsdk"
)

// ============================================================
// 用户管理（代理 Casdoor User API）
// ============================================================

// UserInfo 用户摘要（前端展示用，避免暴露敏感字段）
type UserInfo struct {
	Name        string   `json:"name"`
	DisplayName string   `json:"displayName"`
	Email       string   `json:"email"`
	Phone       string   `json:"phone"`
	Avatar      string   `json:"avatar"`
	IsAdmin     bool     `json:"isAdmin"`
	IsForbidden bool     `json:"isForbidden"`
	CreatedTime string   `json:"createdTime"`
	Roles       []string `json:"roles"`
}

// GetUsers 获取组织下所有用户
func GetUsers() ([]*casdoorsdk.User, error) {
	users, err := casdoorsdk.GetUsers()
	if err != nil {
		return nil, fmt.Errorf("get users from casdoor failed: %w", err)
	}
	return users, nil
}

// GetUserDetail 获取单个用户详情
func GetUserDetail(name string) (*casdoorsdk.User, error) {
	user, err := casdoorsdk.GetUser(name)
	if err != nil {
		return nil, fmt.Errorf("get user from casdoor failed: %w", err)
	}
	if user == nil {
		return nil, fmt.Errorf("user %s not found", name)
	}
	return user, nil
}

// UpdateUserRoles 更新用户的角色列表
// Casdoor 的角色-用户关联由 Role.Users 管理，不能通过 User.Roles 写入。
// 因此需要遍历所有角色，把用户加入/移出各角色的 Users 列表。
func UpdateUserRoles(username string, roleNames []string) error {
	owner := conf.Get().OrganizationName
	userPath := owner + "/" + username

	// 验证用户存在
	user, err := casdoorsdk.GetUser(username)
	if err != nil {
		return fmt.Errorf("get user failed: %w", err)
	}
	if user == nil {
		return fmt.Errorf("user %s not found", username)
	}

	// 目标角色集合
	wantRoles := make(map[string]bool, len(roleNames))
	for _, rn := range roleNames {
		wantRoles[rn] = true
	}

	// 获取所有角色，逐个同步 Users 列表
	allRoles, err := casdoorsdk.GetRoles()
	if err != nil {
		return fmt.Errorf("get roles failed: %w", err)
	}

	for _, role := range allRoles {
		hasUser := false
		userIdx := -1
		for i, u := range role.Users {
			if u == userPath {
				hasUser = true
				userIdx = i
				break
			}
		}

		shouldHave := wantRoles[role.Name]

		if shouldHave && !hasUser {
			// 需要加入
			role.Users = append(role.Users, userPath)
			if _, err := casdoorsdk.UpdateRole(role); err != nil {
				return fmt.Errorf("add user to role %s failed: %w", role.Name, err)
			}
		} else if !shouldHave && hasUser {
			// 需要移除
			role.Users = append(role.Users[:userIdx], role.Users[userIdx+1:]...)
			if _, err := casdoorsdk.UpdateRole(role); err != nil {
				return fmt.Errorf("remove user from role %s failed: %w", role.Name, err)
			}
		}
	}

	InvalidatePermCache()
	return nil
}

// defaultAvatarURL 返回默认头像 URL（优先 S3，回退本地）
func defaultAvatarURL(_, _ string) string {
	return casdoorService.DefaultAvatarURL()
}

// AddUser 新建用户
func AddUser(name, displayName, email, password, avatar string) (bool, error) {
	owner := conf.Get().OrganizationName
	if avatar == "" {
		avatar = defaultAvatarURL(name, displayName)
	}
	user := &casdoorsdk.User{
		Owner:       owner,
		Name:        name,
		DisplayName: displayName,
		Email:       email,
		Password:    password,
		Avatar:      avatar,
	}
	ok, err := casdoorsdk.AddUser(user)
	if err != nil {
		return false, fmt.Errorf("add user to casdoor failed: %w", err)
	}
	return ok, nil
}

// UpdateUserProfile 更新用户资料（显示名、邮箱、手机、头像）
func UpdateUserProfile(name, displayName, email, phone, avatar string) error {
	user, err := casdoorsdk.GetUser(name)
	if err != nil {
		return fmt.Errorf("get user failed: %w", err)
	}
	if user == nil {
		return fmt.Errorf("user %s not found", name)
	}

	if displayName != "" {
		user.DisplayName = displayName
	}
	if email != "" {
		user.Email = email
	}
	// phone 和 avatar 允许设置为空字符串（清空）
	user.Phone = phone
	user.Avatar = avatar

	ok, err := casdoorsdk.UpdateUser(user)
	if err != nil {
		return fmt.Errorf("update user profile failed: %w", err)
	}
	if !ok {
		return fmt.Errorf("update user profile returned false")
	}
	return nil
}

// UploadUserAvatar 直传头像到 S3，若用户已存在则同步更新 Casdoor avatar 字段。
// 新建用户场景：用户尚不存在，仅返回 URL，由 createUser 时写入 avatar。
func UploadUserAvatar(username string, filename string, fileBytes []byte) (string, error) {
	// 1. 直传 S3
	avatarURL, err := casdoorService.UploadAvatarToS3(username, filename, fileBytes)
	if err != nil {
		return "", fmt.Errorf("upload avatar to S3 failed: %w", err)
	}

	// 2. 尝试更新已有用户的 avatar 字段（新建用户场景下用户不存在，跳过即可）
	user, err := casdoorsdk.GetUser(username)
	if err == nil && user != nil {
		user.Avatar = avatarURL
		_, _ = casdoorsdk.UpdateUser(user)
	}

	return avatarURL, nil
}

// PresignAvatarUpload 生成预签名 PUT URL，前端拿到后直接 PUT 文件到 S3。
// 返回 presignedURL（浏览器上传用）和 avatarURL（最终公开访问地址）。
func PresignAvatarUpload(username, filename string) (presignedURL, avatarURL string, err error) {
	return casdoorService.GeneratePresignedPutURL(username, filename)
}

// ConfirmUserAvatar 在前端完成 S3 直传后，将头像 URL 写入 Casdoor 用户记录。
// 新建用户场景：用户尚不存在时静默跳过 Casdoor 更新，仅返回成功。
func ConfirmUserAvatar(username, avatarURL string) error {
	user, err := casdoorsdk.GetUser(username)
	if err == nil && user != nil {
		user.Avatar = avatarURL
		if _, err := casdoorsdk.UpdateUser(user); err != nil {
			return fmt.Errorf("update casdoor user avatar: %w", err)
		}
	}
	return nil
}

// DeleteUser 删除用户
func DeleteUser(name string) (bool, error) {
	owner := conf.Get().OrganizationName
	user := &casdoorsdk.User{
		Owner: owner,
		Name:  name,
	}
	ok, err := casdoorsdk.DeleteUser(user)
	if err != nil {
		return false, fmt.Errorf("delete user from casdoor failed: %w", err)
	}
	InvalidatePermCache()
	return ok, nil
}

// ToggleUserForbidden 启用/禁用用户
func ToggleUserForbidden(username string, forbidden bool) error {
	user, err := casdoorsdk.GetUser(username)
	if err != nil {
		return fmt.Errorf("get user failed: %w", err)
	}
	if user == nil {
		return fmt.Errorf("user %s not found", username)
	}
	user.IsForbidden = forbidden
	ok, err := casdoorsdk.UpdateUser(user)
	if err != nil {
		return fmt.Errorf("toggle user forbidden failed: %w", err)
	}
	if !ok {
		return fmt.Errorf("toggle user forbidden returned false")
	}
	return nil
}

// ============================================================
// 角色管理增强（更新角色、角色详情）
// ============================================================

// GetRoleDetail 获取角色详情
func GetRoleDetail(name string) (*casdoorsdk.Role, error) {
	role, err := casdoorsdk.GetRole(name)
	if err != nil {
		return nil, fmt.Errorf("get role from casdoor failed: %w", err)
	}
	if role == nil {
		return nil, fmt.Errorf("role %s not found", name)
	}
	return role, nil
}

// UpdateRole 更新角色（显示名、用户列表）
func UpdateRole(name, displayName string, users []string) error {
	owner := conf.Get().OrganizationName

	role, err := casdoorsdk.GetRole(name)
	if err != nil {
		return fmt.Errorf("get role failed: %w", err)
	}
	if role == nil {
		return fmt.Errorf("role %s not found", name)
	}

	if displayName != "" {
		role.DisplayName = displayName
	}
	if users != nil {
		// Casdoor Role.Users 格式为 "owner/username"
		formatted := make([]string, 0, len(users))
		for _, u := range users {
			formatted = append(formatted, owner+"/"+u)
		}
		role.Users = formatted
	}

	ok, err := casdoorsdk.UpdateRole(role)
	if err != nil {
		return fmt.Errorf("update role failed: %w", err)
	}
	if !ok {
		return fmt.Errorf("update role returned false")
	}
	InvalidatePermCache()
	return nil
}

// ============================================================
// 权限管理增强（更新权限、权限详情、角色关联）
// ============================================================

// GetPermissionDetail 获取权限详情
func GetPermissionDetail(name string) (*casdoorsdk.Permission, error) {
	perm, err := casdoorsdk.GetPermission(name)
	if err != nil {
		return nil, fmt.Errorf("get permission from casdoor failed: %w", err)
	}
	if perm == nil {
		return nil, fmt.Errorf("permission %s not found", name)
	}
	return perm, nil
}

// UpdatePermission 更新权限（资源、动作、角色、效果）
func UpdatePermission(name string, displayName string, resources, actions, roles []string, effect string, isEnabled *bool) error {
	owner := conf.Get().OrganizationName

	perm, err := casdoorsdk.GetPermission(name)
	if err != nil {
		return fmt.Errorf("get permission failed: %w", err)
	}
	if perm == nil {
		return fmt.Errorf("permission %s not found", name)
	}

	if displayName != "" {
		perm.DisplayName = displayName
	}
	if resources != nil {
		perm.Resources = resources
	}
	if actions != nil {
		perm.Actions = actions
	}
	if roles != nil {
		// 格式化为 Casdoor 期望的 "owner/roleName"
		formatted := make([]string, 0, len(roles))
		for _, r := range roles {
			formatted = append(formatted, owner+"/"+r)
		}
		perm.Roles = formatted
	}
	if effect != "" {
		perm.Effect = effect
	}
	if isEnabled != nil {
		perm.IsEnabled = *isEnabled
	}

	ok, err := casdoorsdk.UpdatePermission(perm)
	if err != nil {
		return fmt.Errorf("update permission failed: %w", err)
	}
	if !ok {
		return fmt.Errorf("update permission returned false")
	}
	InvalidatePermCache()
	return nil
}

// AddPermissionWithRoles 添加权限并关联角色
func AddPermissionWithRoles(name, displayName string, resources, actions, roles []string) (bool, error) {
	owner := conf.Get().OrganizationName

	// 格式化角色为 "owner/roleName"
	formattedRoles := make([]string, 0, len(roles))
	for _, r := range roles {
		formattedRoles = append(formattedRoles, owner+"/"+r)
	}

	perm := &casdoorsdk.Permission{
		Owner:       owner,
		Name:        name,
		DisplayName: displayName,
		Resources:   resources,
		Actions:     actions,
		Roles:       formattedRoles,
		Effect:      "Allow",
		IsEnabled:   true,
	}
	ok, err := casdoorsdk.AddPermission(perm)
	if err != nil {
		return false, fmt.Errorf("add permission failed: %w", err)
	}
	if ok {
		InvalidatePermCache()
	}
	return ok, nil
}
