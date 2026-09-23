import type { PluginModule } from "prism-fusion-web/plugin";
import routes from "./router";

/**
 * 示例插件 - 展示前端插件开发规范
 */
const plugin: PluginModule = {
  name: "example",
  description: "示例插件，展示前端插件开发规范",
  version: "2.0.0",
  manifest: {
    apiVersion: "prism-fusion/v2",
    kind: "frontend-addon",
    id: "example",
    version: "2.0.0",
    requires: [{ id: "casdoor-auth" }, { id: "casbin-rbac" }],
    routeScopes: ["/addon-example"]
  },
  routes,
  permissions: [
    { key: "example:item:view", name: "查看示例列表" },
    { key: "example:item:create", name: "创建示例项" },
    { key: "example:item:delete", name: "删除示例项" },
    {
      key: "example:item:export",
      name: "导出数据",
      description: "纯前端功能权限"
    }
  ],
  setup() {
    console.log("[ExamplePlugin] Initialized");
  },
  destroy() {
    console.log("[ExamplePlugin] Destroyed");
  }
};

export default plugin;
