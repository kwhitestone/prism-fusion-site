// Package messages 消息记录插件
package messages

import (
	"github.com/kwhitestone/prism-fusion/plugin"
	"top.whitestone/prism-fusion-site/addons/messages/router"

	"github.com/danielgtaylor/huma/v2"
)

func init() {
	plugin.Register(newMessagesPlugin())
}

// MessagesPlugin 消息记录插件
type MessagesPlugin struct {
	plugin.BasePlugin
}

func newMessagesPlugin() *MessagesPlugin {
	return &MessagesPlugin{BasePlugin: plugin.BasePlugin{
		PluginName:        "messages",
		PluginDescription: "消息记录插件，提供消息的增删查功能",
	}}
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
