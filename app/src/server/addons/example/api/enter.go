package api

import "whitestone.top/prism-example-site/addons/example/service"

// ExampleApi 示例插件 API 处理器
type ExampleApi struct {
	service service.ExampleService
}

var exampleApi = ExampleApi{}
