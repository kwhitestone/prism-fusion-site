import type { PluginModule } from "prism-fusion-admin/plugin";
import routes from "./router";

/**
 * 示例插件 - 展示前端插件开发规范
 */
const plugin: PluginModule = {
  name: "example",
  description: "示例插件，展示前端插件开发规范",
  version: "1.0.0",
  routes,
  permissions: [
    { key: "example:view", name: "查看示例列表" },
    { key: "example:create", name: "创建示例项" },
    { key: "example:delete", name: "删除示例项" },
    { key: "example:export", name: "导出数据", description: "纯前端功能权限" }
  ],
  setup() {
    console.log("[ExamplePlugin] Initialized");
  },
  destroy() {
    console.log("[ExamplePlugin] Destroyed");
  }
};

export default plugin;
