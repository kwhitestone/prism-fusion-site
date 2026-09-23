import type { PluginModule } from "prism-fusion-web/plugin";
import routes from "./router";

const plugin: PluginModule = {
  name: "messages",
  description: "消息记录插件，提供消息的增删查功能",
  version: "1.0.0",
  routes,
  permissions: [
    { key: "messages:message:view", name: "查看消息列表" },
    { key: "messages:message:submit", name: "提交消息" },
    { key: "messages:message:delete", name: "删除消息" },
    { key: "messages:message:clear", name: "清空所有消息" }
  ]
};

export default plugin;
