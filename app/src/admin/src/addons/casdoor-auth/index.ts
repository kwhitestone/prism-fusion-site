import type { PluginModule } from "prism-fusion-web/plugin";
import {
  setRefreshHandler,
  setUserInfoHandler,
  useUserStoreHook
} from "@/store/modules/user";
import { setLoginComponent } from "@/store/modules/loginUI";
import { getConfig } from "@/config";
import { refreshToken as refreshTokenApi, getUserInfo } from "./api";
import {
  setToken,
  setAuthToken,
  getToken,
  userKey,
  endAuthSessionIfCurrent
} from "@/utils/auth";
import {
  createAuthRequestID,
  currentAuthSessionEpoch,
  crossTabSessionAction,
  getObservedAuthSession,
  isAuthSessionEpoch,
  observeAuthSession
} from "@/core/auth-session";
import CasdoorLogin from "./components/CasdoorLogin.vue";

let disposeSetup: (() => void) | undefined;

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
  manifest: {
    apiVersion: "prism-fusion/v2",
    kind: "frontend-addon",
    id: "casdoor-auth",
    version: "2.0.0"
  },
  priority: 10,
  enabled: () => getConfig()?.AuthProvider === "casdoor",

  setup() {
    let active = true;
    const disposers: (() => void)[] = [];
    disposeSetup = () => {
      active = false;
      for (const dispose of disposers.reverse()) dispose();
    };
    const initialSession = getToken();
    if (
      initialSession &&
      (!initialSession.sessionId || !initialSession.refreshRequestId)
    ) {
      observeAuthSession("invalid-legacy-session");
      void endAuthSessionIfCurrent(initialSession.sessionId);
    } else {
      observeAuthSession(initialSession?.sessionId);
    }
    const onStorage = (event: StorageEvent) => {
      if (event.storageArea !== localStorage || event.key !== userKey) return;
      const action = crossTabSessionAction(
        getObservedAuthSession().sessionId,
        getToken()?.sessionId
      );
      if (action === "reload") window.location.reload();
      if (action === "end") {
        observeAuthSession(undefined);
        useUserStoreHook().endSession();
      }
    };
    window.addEventListener("storage", onStorage);
    disposers.push(() => window.removeEventListener("storage", onStorage));

    // 注册登录界面组件："使用 Casdoor 登录" 按钮
    disposers.push(setLoginComponent(CasdoorLogin));

    // 注入 Token 刷新策略
    disposers.push(
      setRefreshHandler(async data => {
        const epoch = currentAuthSessionEpoch();
        const sessionChanged = () =>
          !active ||
          !isAuthSessionEpoch(epoch) ||
          getToken()?.sessionId !== data.sessionId ||
          getToken()?.refreshToken !== data.refreshToken;
        if (sessionChanged()) return { success: false, sessionChanged: true };
        try {
          const res = await refreshTokenApi({
            refreshToken: data.refreshToken
          });
          const body = res.data;

          if (body.code !== 0) {
            return { success: false, sessionChanged: sessionChanged() };
          }
          if (sessionChanged()) return { success: false, sessionChanged: true };

          const { accessToken, refreshToken: rToken, expiresIn } = body.data;
          setAuthToken(`Bearer ${accessToken}`);

          const expireMs = (expiresIn || 360) * 1000;
          const tokenData = {
            accessToken,
            refreshToken: rToken || accessToken,
            sessionId: data.sessionId,
            refreshRequestId: createAuthRequestID(),
            expires: new Date(Date.now() + expireMs)
          };

          setToken(tokenData);
          observeAuthSession(data.sessionId);
          return { success: true, data: tokenData };
        } catch {
          return { success: false, sessionChanged: sessionChanged() };
        }
      })
    );

    // 注入用户信息获取策略（页面刷新时同步头像、角色等）
    disposers.push(
      setUserInfoHandler(async () => {
        if (!active) return { success: false };
        try {
          const res = await getUserInfo();
          if (!active) return { success: false };
          if (res.data?.code === 0 && res.data?.data) {
            return {
              success: true,
              data: {
                avatar: res.data.data.avatar || "",
                roles: res.data.data.roles || [],
                nickname: res.data.data.nickName || "",
                username: res.data.data.username || ""
              }
            };
          }
          return { success: false };
        } catch {
          return { success: false };
        }
      })
    );

    console.log(
      "[Plugin] Casdoor Auth plugin setup complete - using OAuth2 redirect flow"
    );
  },
  destroy() {
    const dispose = disposeSetup;
    disposeSetup = undefined;
    dispose?.();
  }
};

export default casdoorAuthPlugin;
