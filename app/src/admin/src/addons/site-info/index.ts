import type { PluginModule } from "prism-fusion-admin/plugin";
import routes from "./router";

/**
 * 站点信息插件 - 业务前端示例
 */
const plugin: PluginModule = {
  name: "site-info",
  description: "站点信息插件，展示业务前端插件开发方式",
  version: "1.0.0",
  routes,
  permissions: [
    { key: "site-info:view", name: "查看站点信息" }
  ],
  setup() {
    console.log("[SiteInfoPlugin] Initialized");
  }
};

export default plugin;
