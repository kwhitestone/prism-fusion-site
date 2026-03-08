import type { PluginModule } from "@/plugin/types";
import { setAsyncRoutesProvider } from "@/api/routes";
import { getAsyncRoutes } from "./api";
import { getConfig } from "@/config";
import routes from "./router";

/**
 * Casbin RBAC 前端插件
 *
 * - 替换 builtin RBAC 插件的动态路由获取策略
 * - 提供用户/角色/权限管理页面（对接 Casdoor）
 */
const casbinRbacPlugin: PluginModule = {
  name: "casbin-rbac",
  description: "Casbin RBAC 插件 - 提供基于 Casbin 的动态路由和权限管理",
  version: "1.0.0",
  routes, // 注册管理页面路由（用户/角色/权限）

  setup() {
    const provider = getConfig()?.RBACProvider;
    if (provider !== "casbin") {
      console.log(
        "[Plugin] Casbin RBAC plugin skipped - provider is",
        provider || "builtin"
      );
      return;
    }
    // 注入 Casbin RBAC 动态路由获取策略
    setAsyncRoutesProvider(async () => {
      try {
        const res = await getAsyncRoutes();
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

    console.log(
      "[Plugin] Casbin RBAC plugin setup complete - using Casbin backend"
    );
  }
};

export default casbinRbacPlugin;
