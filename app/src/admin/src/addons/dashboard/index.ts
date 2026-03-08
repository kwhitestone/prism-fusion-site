import type { PluginModule } from "prism-fusion-admin/plugin";
import routes from "./router";

const plugin: PluginModule = {
  name: "dashboard",
  description: "数据总览插件，提供系统运行概况统计",
  version: "1.0.0",
  routes,
  permissions: [
    { key: "dashboard:view", name: "查看数据总览" }
  ]
};

export default plugin;
