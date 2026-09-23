package service

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/kwhitestone/prism-fusion/global"
	"top.whitestone/prism-fusion-site/addons/casdoor-auth/conf"

	"github.com/casdoor/casdoor-go-sdk/casdoorsdk"
	"go.uber.org/zap"
)

var sdkOnce sync.Once

// InitCasdoorSDK 确保 Casdoor SDK 已初始化
// 如果 casdoor-auth 插件已初始化则此处为幂等操作
func InitCasdoorSDK() {
	sdkOnce.Do(func() {
		cfg := conf.Get()
		if cfg.Endpoint == "" {
			global.PRISM_LOG.Warn("Casbin RBAC: Casdoor endpoint not configured, " +
				"ensure casdoor-auth plugin is enabled or auth.casdoor is configured")
			return
		}
		casdoorsdk.InitConfig(
			cfg.Endpoint,
			cfg.ClientID,
			cfg.ClientSecret,
			cfg.Certificate,
			cfg.OrganizationName,
			cfg.ApplicationName,
		)
		global.PRISM_LOG.Info("Casbin RBAC: Casdoor SDK initialized for remote Casbin")
	})
}

// ======================== 本地权限匹配 ========================
// Casdoor 的默认 Casbin 模型使用精确匹配 (r.obj == p.obj)，不支持路径通配符（如 /api/v1/*）。
// 因此我们从 Casdoor 拉取权限列表在本地进行通配符匹配，而非调用远程 Enforce API。

var permCache struct {
	sync.RWMutex
	perms    []*casdoorsdk.Permission
	loadedAt time.Time
}

const permCacheTTL = 2 * time.Minute

// loadPermissions 从 Casdoor 获取权限列表（带缓存）
func loadPermissions() []*casdoorsdk.Permission {
	permCache.RLock()
	if time.Since(permCache.loadedAt) < permCacheTTL && permCache.perms != nil {
		defer permCache.RUnlock()
		return permCache.perms
	}
	permCache.RUnlock()

	permCache.Lock()
	defer permCache.Unlock()

	// double-check
	if time.Since(permCache.loadedAt) < permCacheTTL && permCache.perms != nil {
		return permCache.perms
	}

	perms, err := casdoorsdk.GetPermissions()
	if err != nil {
		global.PRISM_LOG.Error("Failed to fetch permissions from Casdoor", zap.Error(err))
		return permCache.perms // 返回过期数据
	}

	permCache.perms = perms
	permCache.loadedAt = time.Now()
	global.PRISM_LOG.Debug("Refreshed permission cache from Casdoor", zap.Int("count", len(perms)))
	return perms
}

// InvalidatePermCache 使权限缓存失效（增删改权限后调用）
func InvalidatePermCache() {
	permCache.Lock()
	defer permCache.Unlock()
	permCache.loadedAt = time.Time{} // zero time → 下次访问时刷新
}

// Enforce 本地权限检查：判断用户是否有权限访问指定路径
// username: Casdoor 用户名（如 "admin"）
// owner:    Casdoor 组织名（如 "built-in"）
// roles:    用户角色列表（如 ["Admin"]）
// isAdmin:  是否为管理员
// path:     请求路径
// method:   请求方法
func Enforce(username, owner string, roles []string, isAdmin bool, path, method string) (bool, error) {
	perms := loadPermissions()
	if perms == nil {
		return false, fmt.Errorf("no permissions loaded from Casdoor")
	}

	method = strings.ToUpper(method)
	userID := owner + "/" + username

	allowCount := 0
	denyCount := 0

	for _, perm := range perms {
		if !perm.IsEnabled {
			continue
		}
		// State 检查：Casdoor 内置权限要求 Approved，但用户自建可能为空
		if perm.State != "" && perm.State != "Approved" {
			continue
		}

		// 1) 检查用户/角色是否匹配此权限
		if !matchUserOrRole(userID, owner, roles, isAdmin, perm) {
			continue
		}

		// 2) 检查资源路径是否匹配
		if !matchResources(path, perm.Resources) {
			continue
		}

		// 3) 检查操作方法是否匹配
		if !matchActions(method, perm.Actions) {
			continue
		}

		// 匹配成功，统计 Allow/Deny
		if perm.Effect == "Deny" {
			denyCount++
		} else {
			allowCount++
		}
	}

	// deny-override: 有任何 Deny 匹配就拒绝
	if denyCount > 0 {
		return false, nil
	}
	return allowCount > 0, nil
}

// matchUserOrRole 检查用户或其角色是否匹配权限的 Users/Roles
func matchUserOrRole(userID, owner string, userRoles []string, isAdmin bool, perm *casdoorsdk.Permission) bool {
	// 检查 Users 字段
	for _, u := range perm.Users {
		if u == userID || u == owner+"/*" {
			return true
		}
	}

	// 检查 Roles 字段
	for _, permRole := range perm.Roles {
		// Casdoor 存储角色引用格式: "built-in/Admin"
		_, permRoleName := splitOwnerName(permRole)

		// 管理员特判：IsAdmin 标志匹配名为 Admin 的角色
		if isAdmin && strings.EqualFold(permRoleName, "admin") {
			return true
		}

		// 逐个比对用户角色
		for _, ur := range userRoles {
			// 支持完整 ID 或纯名称匹配（大小写不敏感）
			if ur == permRole || owner+"/"+ur == permRole || strings.EqualFold(ur, permRoleName) {
				return true
			}
		}
	}

	return false
}

// matchResources 检查路径是否匹配任一资源模式
func matchResources(path string, resources []string) bool {
	for _, pattern := range resources {
		if keyMatch(path, pattern) {
			return true
		}
	}
	return false
}

// matchActions 检查方法是否匹配任一操作
func matchActions(method string, actions []string) bool {
	for _, act := range actions {
		if act == "*" || strings.EqualFold(act, method) {
			return true
		}
	}
	return false
}

// keyMatch 路径通配符匹配，兼容 Casbin keyMatch 语义
// 支持: /api/v1/* 匹配 /api/v1/foo/bar
func keyMatch(requestPath, pattern string) bool {
	if pattern == "*" || pattern == "/*" {
		return true
	}
	// /api/v1/* → 前缀匹配
	if i := strings.Index(pattern, "*"); i >= 0 {
		prefix := pattern[:i]
		return strings.HasPrefix(requestPath, prefix)
	}
	// 精确匹配
	return requestPath == pattern
}

// splitOwnerName 将 "built-in/Admin" 拆为 ("built-in", "Admin")
func splitOwnerName(id string) (string, string) {
	if i := strings.Index(id, "/"); i >= 0 {
		return id[:i], id[i+1:]
	}
	return "", id
}

// ----- Permission CRUD (代理 Casdoor Permission API) -----

// PermissionInfo 权限摘要信息
type PermissionInfo struct {
	Name        string   `json:"name"`
	DisplayName string   `json:"displayName"`
	Resources   []string `json:"resources"`
	Actions     []string `json:"actions"`
	Effect      string   `json:"effect"`
	IsEnabled   bool     `json:"isEnabled"`
}

// GetPermissions 获取所有权限
func GetPermissions() ([]*casdoorsdk.Permission, error) {
	perms, err := casdoorsdk.GetPermissions()
	if err != nil {
		return nil, fmt.Errorf("get permissions from casdoor failed: %w", err)
	}
	return perms, nil
}

// AddPermission 添加权限
func AddPermission(name, displayName string, resources, actions []string) (bool, error) {
	owner := conf.Get().OrganizationName
	perm := &casdoorsdk.Permission{
		Owner:       owner,
		Name:        name,
		DisplayName: displayName,
		Resources:   resources,
		Actions:     actions,
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

// DeletePermission 删除权限
func DeletePermission(name string) (bool, error) {
	owner := conf.Get().OrganizationName
	perm := &casdoorsdk.Permission{
		Owner: owner,
		Name:  name,
	}
	ok, err := casdoorsdk.DeletePermission(perm)
	if err != nil {
		return false, fmt.Errorf("delete permission failed: %w", err)
	}
	if ok {
		InvalidatePermCache()
	}
	return ok, nil
}

// ----- Role CRUD (代理 Casdoor Role API) -----

// GetRoles 获取所有角色
func GetRoles() ([]*casdoorsdk.Role, error) {
	roles, err := casdoorsdk.GetRoles()
	if err != nil {
		return nil, fmt.Errorf("get roles from casdoor failed: %w", err)
	}
	return roles, nil
}

// AddRole 添加角色
func AddRole(name, displayName string, users []string) (bool, error) {
	owner := conf.Get().OrganizationName
	role := &casdoorsdk.Role{
		Owner:       owner,
		Name:        name,
		DisplayName: displayName,
		Users:       users,
		IsEnabled:   true,
	}
	ok, err := casdoorsdk.AddRole(role)
	if err != nil {
		return false, fmt.Errorf("add role failed: %w", err)
	}
	if ok {
		InvalidatePermCache()
	}
	return ok, nil
}

// DeleteRole 删除角色
func DeleteRole(name string) (bool, error) {
	owner := conf.Get().OrganizationName
	role := &casdoorsdk.Role{
		Owner: owner,
		Name:  name,
	}
	ok, err := casdoorsdk.DeleteRole(role)
	if err != nil {
		return false, fmt.Errorf("delete role failed: %w", err)
	}
	if ok {
		InvalidatePermCache()
	}
	return ok, nil
}

// GetUserRoles 从 Casdoor 获取用户角色（通过解析用户信息）
func GetUserRoles(username string) ([]string, error) {
	user, err := casdoorsdk.GetUser(username)
	if err != nil {
		return nil, fmt.Errorf("get user from casdoor failed: %w", err)
	}
	if user == nil {
		return nil, fmt.Errorf("user %s not found", username)
	}

	// 从 Casdoor 用户的 Roles 字段提取角色名
	var roleNames []string
	for _, r := range user.Roles {
		roleNames = append(roleNames, r.Name)
	}
	return roleNames, nil
}
