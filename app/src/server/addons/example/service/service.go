package service

import (
	"whitestone.top/prism-example-site/addons/example/model"
	"whitestone.top/prism-fusion/global"
)

// ExampleService 示例服务
type ExampleService struct{}

// ExampleItemResponse 示例项响应结构
type ExampleItemResponse struct {
	ID          uint   `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Status      int    `json:"status"`
	CreatedAt   string `json:"created_at"`
}

// GetItems 获取所有示例项
func (s *ExampleService) GetItems() ([]ExampleItemResponse, error) {
	var items []model.ExampleItem
	if err := global.PRISM_DB.Find(&items).Error; err != nil {
		return nil, err
	}

	var result []ExampleItemResponse
	for _, item := range items {
		result = append(result, ExampleItemResponse{
			ID:          item.ID,
			Name:        item.Name,
			Description: item.Description,
			Status:      item.Status,
			CreatedAt:   item.CreatedAt.Format("2006-01-02 15:04:05"),
		})
	}
	return result, nil
}

// CreateItem 创建示例项
func (s *ExampleService) CreateItem(name, description string) (*ExampleItemResponse, error) {
	item := model.ExampleItem{
		Name:        name,
		Description: description,
		Status:      1,
	}

	if err := global.PRISM_DB.Create(&item).Error; err != nil {
		return nil, err
	}

	return &ExampleItemResponse{
		ID:          item.ID,
		Name:        item.Name,
		Description: item.Description,
		Status:      item.Status,
		CreatedAt:   item.CreatedAt.Format("2006-01-02 15:04:05"),
	}, nil
}

// GetItemByID 根据ID获取示例项
func (s *ExampleService) GetItemByID(id uint) (*ExampleItemResponse, error) {
	var item model.ExampleItem
	if err := global.PRISM_DB.First(&item, id).Error; err != nil {
		return nil, err
	}

	return &ExampleItemResponse{
		ID:          item.ID,
		Name:        item.Name,
		Description: item.Description,
		Status:      item.Status,
		CreatedAt:   item.CreatedAt.Format("2006-01-02 15:04:05"),
	}, nil
}

// DeleteItem 删除示例项
func (s *ExampleService) DeleteItem(id uint) error {
	return global.PRISM_DB.Delete(&model.ExampleItem{}, id).Error
}
