import service from "@/utils/request";

const CASDOOR_AUTH_BASE = "/api/v1/addons/casdoor-auth";

/** 获取 Casdoor 配置信息（前端用于发起 OAuth 重定向） */
export const getCasdoorConfig = () => {
  return service({
    url: `${CASDOOR_AUTH_BASE}/config`,
    method: "get",
    donNotShowLoading: true
  });
};

/** 获取 Casdoor 登录 URL（OAuth2 重定向模式） */
export const getSigninUrl = (
  state = "casdoor",
  redirectUri = `${window.location.origin}/login`
) => {
  return service({
    url: `${CASDOOR_AUTH_BASE}/signin-url`,
    method: "get",
    params: { state, redirectUri },
    donNotShowLoading: true
  });
};

/** OAuth2 授权码回调：用 code 换 token */
export const signinCallback = (data: {
  code: string;
  state?: string;
  redirectUri: string;
}) => {
  return service({
    url: `${CASDOOR_AUTH_BASE}/signin-callback`,
    method: "post",
    data,
    donNotShowLoading: true
  });
};

/** 刷新 Token */
export const refreshToken = (data: { refreshToken: string }) => {
  return service({
    url: `${CASDOOR_AUTH_BASE}/refresh-token`,
    method: "post",
    data,
    donNotShowLoading: true
  });
};

/** 获取当前用户信息 */
export const getUserInfo = () => {
  return service({
    url: `${CASDOOR_AUTH_BASE}/user-info`,
    method: "get",
    donNotShowLoading: true
  });
};
