import service from "@/utils/request";

const CASBIN_RBAC_BASE = "/api/v1/addons/casbin-rbac";

/** 获取动态路由 */
export const getAsyncRoutes = () => {
  return service({
    url: `${CASBIN_RBAC_BASE}/async-routes`,
    method: "get",
    donNotShowLoading: true
  });
};

/** 获取策略列表 */
export const getPolicies = () => {
  return service({
    url: `${CASBIN_RBAC_BASE}/policies`,
    method: "get"
  });
};

/** 添加策略 */
export const addPolicy = (data: {
  role: string;
  path: string;
  method: string;
}) => {
  return service({
    url: `${CASBIN_RBAC_BASE}/policies`,
    method: "post",
    data
  });
};

/** 删除策略 */
export const removePolicy = (data: {
  role: string;
  path: string;
  method: string;
}) => {
  return service({
    url: `${CASBIN_RBAC_BASE}/policies`,
    method: "delete",
    data
  });
};

/** 检查权限 */
export const enforce = (params: {
  role: string;
  path: string;
  method: string;
}) => {
  return service({
    url: `${CASBIN_RBAC_BASE}/enforce`,
    method: "get",
    params
  });
};

/** 分配角色 */
export const assignRole = (data: { user: string; role: string }) => {
  return service({
    url: `${CASBIN_RBAC_BASE}/roles/assign`,
    method: "post",
    data
  });
};

/** 移除角色 */
export const removeRole = (data: { user: string; role: string }) => {
  return service({
    url: `${CASBIN_RBAC_BASE}/roles/assign`,
    method: "delete",
    data
  });
};

/** 获取用户角色 */
export const getUserRoles = (user: string) => {
  return service({
    url: `${CASBIN_RBAC_BASE}/roles/user`,
    method: "get",
    params: { user }
  });
};
