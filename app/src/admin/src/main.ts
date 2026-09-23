/**
 * Prism Example Site - 前端入口
 *
 * 组装框架核心 + 业务插件，启动应用
 */

// ========== 框架核心 ==========
import {
  App,
  router,
  setupStore,
  getPlatformConfig,
  installPlugins,
  registerExternalPlugins,
  configurePluginHost,
  injectResponsiveStorage
} from "prism-fusion-web";

// ========== 框架样式 ==========
import "@/style/reset.scss";
import "@/style/index.scss";
import "@/style/tailwind.css";
import "element-plus/dist/index.css";
import "@/assets/iconfont/iconfont.js";
import "@/assets/iconfont/iconfont.css";

// ========== 框架插件 ==========
import { MotionPlugin } from "@vueuse/motion";
import { useElementPlus } from "@/plugins/elementPlus";
import Table from "@pureadmin/table";

import { createApp, type Directive } from "vue";

// ========== 框架指令 ==========
import * as directives from "@/directives";

// ========== 框架图标 ==========
import {
  IconifyIconOffline,
  IconifyIconOnline,
  FontIcon
} from "@/components/ReIcon";

// ========== vue-tippy ==========
import "tippy.js/dist/tippy.css";
import "tippy.js/themes/light.css";
import VueTippy from "vue-tippy";

// ========== ★ 业务插件（显式导入） ==========
import casdoorAuthPlugin from "./addons/casdoor-auth";
import casbinRbacPlugin from "./addons/casbin-rbac";
import siteInfoPlugin from "./addons/site-info";
import dashboardPlugin from "./addons/dashboard";
import examplePlugin from "./addons/example";
import messagesPlugin from "./addons/messages";

// 注册业务插件到框架
registerExternalPlugins([
  casdoorAuthPlugin,
  casbinRbacPlugin,
  siteInfoPlugin,
  dashboardPlugin,
  examplePlugin,
  messagesPlugin
]);

configurePluginHost({ homePath: "/dashboard/index" });

// ========== 启动应用 ==========
const app = createApp(App);

// 注册指令
Object.keys(directives).forEach(key => {
  app.directive(key, (directives as { [key: string]: Directive })[key]);
});

// 注册全局图标组件
app.component("IconifyIconOffline", IconifyIconOffline);
app.component("IconifyIconOnline", IconifyIconOnline);
app.component("FontIcon", FontIcon);

// 注册 vue-tippy
app.use(VueTippy);

getPlatformConfig(app)
  .then(async config => {
    setupStore(app);
    // 安装插件（框架内置 + 业务插件）
    await installPlugins(app, router);

    app.use(router);
    await router.isReady();
    injectResponsiveStorage(app, config);
    app.use(MotionPlugin).use(useElementPlus).use(Table);

    app.mount("#app");
  })
  .catch(error => {
    console.error("[Startup] Application initialization failed", error);
    const root = document.querySelector("#app");
    if (root) root.textContent = "应用初始化失败，请联系管理员并检查插件配置。";
  });
