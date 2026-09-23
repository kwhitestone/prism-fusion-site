package service

import (
	"encoding/json"

	"github.com/kwhitestone/prism-fusion/global"
	"top.whitestone/prism-fusion-site/addons/casbin-rbac/model"
)

// MenuService 菜单服务（复用 builtin rbac 的菜单树构建逻辑）
type MenuService struct{}

// MenuNode 菜单树节点（返回给前端的格式）
type MenuNode struct {
	Path      string                 `json:"path"`
	Name      string                 `json:"name,omitempty"`
	Component string                 `json:"component,omitempty"`
	Redirect  string                 `json:"redirect,omitempty"`
	Meta      map[string]interface{} `json:"meta"`
	Children  []MenuNode             `json:"children,omitempty"`
}

// GetAsyncRoutes 获取动态路由
func (s *MenuService) GetAsyncRoutes() ([]MenuNode, error) {
	var menus []model.CasbinMenu
	if err := global.PRISM_DB.Order("rank asc, id asc").Find(&menus).Error; err != nil {
		return nil, err
	}
	return buildMenuTree(menus, 0), nil
}

// buildMenuTree 递归构建菜单树
func buildMenuTree(menus []model.CasbinMenu, parentID uint) []MenuNode {
	var nodes []MenuNode
	for _, m := range menus {
		if m.ParentID != parentID {
			continue
		}

		node := MenuNode{
			Path:      m.Path,
			Name:      m.Name,
			Component: m.Component,
			Redirect:  m.Redirect,
			Meta:      make(map[string]interface{}),
		}

		node.Meta["title"] = m.Title
		if m.Icon != "" {
			node.Meta["icon"] = m.Icon
		}
		if m.Rank > 0 {
			node.Meta["rank"] = m.Rank
		}
		if m.ShowLink != nil && !*m.ShowLink {
			node.Meta["showLink"] = false
		}

		// 解析 roles JSON
		if m.Roles != "" {
			var roles []string
			if err := json.Unmarshal([]byte(m.Roles), &roles); err == nil && len(roles) > 0 {
				node.Meta["roles"] = roles
			}
		}

		// 解析 auths JSON
		if m.Auths != "" {
			var auths []string
			if err := json.Unmarshal([]byte(m.Auths), &auths); err == nil && len(auths) > 0 {
				node.Meta["auths"] = auths
			}
		}

		children := buildMenuTree(menus, m.ID)
		if len(children) > 0 {
			node.Children = children
		}

		nodes = append(nodes, node)
	}
	return nodes
}

// SeedMenuData 初始化菜单种子数据（如果 menus 表为空）
func SeedMenuData() {
	db := global.PRISM_DB
	var count int64
	db.Model(&model.CasbinMenu{}).Count(&count)
	if count > 0 {
		return
	}

	// 与 builtin rbac 共用相同的 menus 表，不重复 seed
	global.PRISM_LOG.Info("Casbin RBAC: menus table already managed by builtin RBAC or is empty")
}
