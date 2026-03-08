import type { PluginModule } from "@/plugin/types";
import { setRefreshHandler, setUserInfoHandler } from "@/store/modules/user";
import { setLoginComponent } from "@/store/modules/loginUI";
import { getConfig } from "@/config";
import { refreshToken as refreshTokenApi, getUserInfo } from "./api";
import { setToken, setAuthToken } from "@/utils/auth";
import CasdoorLogin from "./components/CasdoorLogin.vue";

/**
 * Casdoor Auth 前端插件
 *
 * 标准 OAuth2 流程：
 * 1. 用户访问登录页 → 显示"使用 Casdoor 登录"按钮
 * 2. 点击按钮 → 浏览器跳转到 Casdoor 登录页
 * 3. 用户在 Casdoor 认证 → Casdoor 重定向回 /login/callback?code=xxx
 * 4. callback 页面拿 code 发送给后端 /signin-callback
 * 5. 后端用 code 换取 JWT，返回给前端
 */
const casdoorAuthPlugin: PluginModule = {
  name: "casdoor-auth",
  description: "Casdoor 认证插件 - OAuth2 标准流程登录",
  version: "2.0.0",

  setup() {
    const provider = getConfig()?.AuthProvider;
    if (provider !== "casdoor") {
      console.log(
        "[Plugin] Casdoor Auth plugin skipped - provider is",
        provider || "builtin"
      );
      return;
    }

    // 注册登录界面组件："使用 Casdoor 登录" 按钮
    setLoginComponent(CasdoorLogin);

    // 注入 Token 刷新策略
    setRefreshHandler(async data => {
      try {
        const res = await refreshTokenApi(data);
        const body = res.data;

        if (body.code !== 0) {
          return { success: false };
        }

        const { accessToken, refreshToken: rToken, expiresIn } = body.data;
        setAuthToken(`Bearer ${accessToken}`);

        const expireMs = (expiresIn || 360) * 1000;
        const tokenData = {
          accessToken,
          refreshToken: rToken || accessToken,
          expires: new Date(Date.now() + expireMs)
        };

        setToken(tokenData as any);
        return { success: true, data: tokenData };
      } catch {
        return { success: false };
      }
    });

    // 注入用户信息获取策略（页面刷新时同步头像、角色等）
    setUserInfoHandler(async () => {
      try {
        const res = await getUserInfo();
        if (res.data?.code === 0 && res.data?.data) {
          return {
            success: true,
            data: {
              avatar: res.data.data.avatar || "",
              roles: res.data.data.roles || [],
              nickname: res.data.data.nickname || "",
              username: res.data.data.username || ""
            }
          };
        }
        return { success: false };
      } catch {
        return { success: false };
      }
    });

    console.log(
      "[Plugin] Casdoor Auth plugin setup complete - using OAuth2 redirect flow"
    );
  }
};

export default casdoorAuthPlugin;
