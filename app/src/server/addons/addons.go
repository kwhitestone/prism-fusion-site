// Package addons 业务插件导入入口
// 导入所有业务插件，触发各插件的 init() 函数完成自动注册
// 新增业务插件只需在此添加 import 即可
package addons

import (
	// 认证插件 - Casdoor OAuth2（优先级 10，provider=casdoor）
	_ "top.whitestone/prism-fusion-site/addons/casdoor-auth"
	// 权限管理插件 - Casbin（优先级 20，provider=casbin）
	_ "top.whitestone/prism-fusion-site/addons/casbin-rbac"
	// 示例业务插件
	_ "top.whitestone/prism-fusion-site/addons/site-info"
	// 数据总览插件
	_ "top.whitestone/prism-fusion-site/addons/dashboard"
	// 示例插件
	_ "top.whitestone/prism-fusion-site/addons/example"
	// 消息记录插件
	_ "top.whitestone/prism-fusion-site/addons/messages"
)
