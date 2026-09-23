import type { PluginModule } from "prism-fusion-web/plugin";
import routes from "./router";
import { setFooterComponent } from "prism-fusion-web";
import LegalFooter from "./components/LegalFooter";

let disposeFooter: (() => void) | undefined;

/**
 * 站点信息插件 - 业务前端示例
 */
const plugin: PluginModule = {
  name: "site-info",
  description: "站点信息插件，展示业务前端插件开发方式",
  version: "1.0.0",
  routes,
  permissions: [{ key: "site-info:site:view", name: "查看站点信息" }],
  setup() {
    disposeFooter = setFooterComponent(LegalFooter);
    console.log("[SiteInfoPlugin] Initialized");
  },
  destroy() {
    disposeFooter?.();
    disposeFooter = undefined;
  }
};

export default plugin;
