package service

import (
	"whitestone.top/prism-fusion/global"
)

type DashboardService struct{}

// DashboardStats Dashboard统计数据
type DashboardStats struct {
	PluginCount int `json:"pluginCount"`
	UserCount   int `json:"userCount"`
}

// GetStats 获取Dashboard统计数据
func (s *DashboardService) GetStats() (*DashboardStats, error) {
	global.PRISM_LOG.Info("获取Dashboard统计数据")
	// TODO: 从数据库读取真实数据
	stats := &DashboardStats{
		PluginCount: 0,
		UserCount:   0,
	}
	return stats, nil
}
