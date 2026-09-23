import type { PluginModule } from "prism-fusion-web/plugin";
import { setAsyncRoutesProvider } from "@/api/routes";
import { getAsyncRoutes } from "./api";
import { getConfig } from "@/config";
import routes from "./router";

let disposeSetup: (() => void) | undefined;

/**
 * Casbin RBAC 前端插件
 *
 * - 替换 builtin RBAC 插件的动态路由获取策略
 * - 提供用户/角色/权限管理页面（对接 Casdoor）
 */
const casbinRbacPlugin: PluginModule = {
  name: "casbin-rbac",
  description: "Casbin RBAC 插件 - 提供基于 Casbin 的动态路由和权限管理",
  version: "2.0.0",
  manifest: {
    apiVersion: "prism-fusion/v2",
    kind: "frontend-addon",
    id: "casbin-rbac",
    version: "2.0.0",
    requires: [{ id: "casdoor-auth" }],
    routeScopes: ["/rbac"]
  },
  priority: 20,
  enabled: () => getConfig()?.RBACProvider === "casbin",
  routes, // 注册管理页面路由（用户/角色/权限）

  setup() {
    let active = true;
    // 注入 Casbin RBAC 动态路由获取策略
    const dispose = setAsyncRoutesProvider(async () => {
      if (!active) return { success: false, data: [] };
      try {
        const res = await getAsyncRoutes();
        if (!active) return { success: false, data: [] };
        // axios 响应拦截器会将 { success, data } 包装为 { code, data: { success, data }, msg }
        const body = res.data;
        const inner = body?.data;
        return {
          success: inner?.success ?? body?.success ?? false,
          data: Array.isArray(inner?.data)
            ? inner.data
            : Array.isArray(inner)
              ? inner
              : Array.isArray(body?.data)
                ? body.data
                : []
        };
      } catch {
        console.warn(
          "[Casbin RBAC Plugin] Failed to fetch async routes from backend"
        );
        return { success: false, data: [] };
      }
    });
    disposeSetup = () => {
      active = false;
      dispose();
    };

    console.log(
      "[Plugin] Casbin RBAC plugin setup complete - using Casbin backend"
    );
  },
  destroy() {
    const dispose = disposeSetup;
    disposeSetup = undefined;
    dispose?.();
  }
};

export default casbinRbacPlugin;
