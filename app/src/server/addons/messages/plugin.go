// Package messages 消息记录插件
package messages

import (
	"whitestone.top/prism-example-site/addons/messages/router"
	"whitestone.top/prism-fusion/plugin"

	"github.com/danielgtaylor/huma/v2"
)

func init() {
	plugin.Register(&MessagesPlugin{})
}

// MessagesPlugin 消息记录插件
type MessagesPlugin struct {
	plugin.BasePlugin
}

func (p *MessagesPlugin) Name() string {
	return "messages"
}

func (p *MessagesPlugin) Description() string {
	return "消息记录插件，提供消息的增删查功能"
}

func (p *MessagesPlugin) RegisterRoutes(humaApi huma.API) {
	router.RegisterRoutes(humaApi)
}
