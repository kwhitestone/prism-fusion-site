package model

import "whitestone.top/prism-fusion/model"

// ExampleItem 示例数据模型
type ExampleItem struct {
	model.MODEL
	Name        string `json:"name" gorm:"size:100;comment:名称"`
	Description string `json:"description" gorm:"size:500;comment:描述"`
	Status      int    `json:"status" gorm:"default:1;comment:状态 1-启用 0-禁用"`
}

// TableName 指定表名
func (ExampleItem) TableName() string {
	return "addon_example_items"
}
