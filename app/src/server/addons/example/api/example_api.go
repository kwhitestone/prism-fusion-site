package api

import (
	"whitestone.top/prism-example-site/addons/example/service"
)

// GetItems 获取示例项列表
func (a *ExampleApi) GetItems() ([]service.ExampleItemResponse, error) {
	return a.service.GetItems()
}

// CreateItem 创建示例项
func (a *ExampleApi) CreateItem(name, description string) (*service.ExampleItemResponse, error) {
	return a.service.CreateItem(name, description)
}

// GetItemByID 根据ID获取示例项
func (a *ExampleApi) GetItemByID(id uint) (*service.ExampleItemResponse, error) {
	return a.service.GetItemByID(id)
}

// DeleteItem 删除示例项
func (a *ExampleApi) DeleteItem(id uint) error {
	return a.service.DeleteItem(id)
}
