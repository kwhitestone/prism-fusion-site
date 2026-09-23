package model

import (
	"gorm.io/gorm"
)

// CasbinMenu 菜单表（Casbin RBAC 插件使用，与 builtin rbac 的 Menu 结构相同）
type CasbinMenu struct {
	ID        uint           `json:"id" gorm:"primarykey;comment:主键ID"`
	ParentID  uint           `json:"parentId" gorm:"column:parent_id;comment:父菜单ID"`
	Path      string         `json:"path" gorm:"column:path;comment:路由路径"`
	Name      string         `json:"name" gorm:"column:name;comment:路由名称"`
	Component string         `json:"component" gorm:"column:component;comment:组件路径"`
	Redirect  string         `json:"redirect" gorm:"column:redirect;comment:重定向路径"`
	Title     string         `json:"title" gorm:"column:title;comment:菜单标题"`
	Icon      string         `json:"icon" gorm:"column:icon;comment:菜单图标"`
	Rank      int            `json:"rank" gorm:"column:rank;comment:排序"`
	ShowLink  *bool          `json:"showLink" gorm:"column:show_link;default:true;comment:是否显示"`
	Roles     string         `json:"roles" gorm:"column:roles;type:text;comment:允许的角色(JSON数组)"`
	Auths     string         `json:"auths" gorm:"column:auths;type:text;comment:按钮权限(JSON数组)"`
	DeletedAt gorm.DeletedAt `json:"-" gorm:"index;comment:删除时间"`
}

// TableName 使用与 builtin rbac 相同的菜单表
func (CasbinMenu) TableName() string {
	return "menus"
}
