import { http } from "@/utils/http";

const BASE = "/api/v1/addons/casbin-rbac/admin";
const CASDOOR_BASE = "/api/v1/addons/casdoor-auth";

// ============================================================
// 类型定义
// ============================================================

export interface UserItem {
  name: string;
  displayName: string;
  email: string;
  phone: string;
  avatar: string;
  isAdmin: boolean;
  isForbidden: boolean;
  createdTime: string;
  roles: string[];
}

export interface RoleItem {
  name: string;
  displayName: string;
  description: string;
  users: string[];
  isEnabled: boolean;
  createdTime: string;
}

export interface PermissionItem {
  name: string;
  displayName: string;
  resources: string[];
  actions: string[];
  roles: string[];
  effect: string;
  isEnabled: boolean;
}

interface ApiResponse<T> {
  code: number;
  message: string;
  data: T;
}

// ============================================================
// 用户管理
// ============================================================

export function getUsers() {
  return http.request<ApiResponse<UserItem[]>>("get", `${BASE}/users`);
}

export function updateUserRoles(username: string, roles: string[]) {
  return http.request<ApiResponse<null>>("put", `${BASE}/users/roles`, {
    data: { username, roles }
  });
}

export function toggleUser(username: string, isForbidden: boolean) {
  return http.request<ApiResponse<null>>("put", `${BASE}/users/toggle`, {
    data: { username, isForbidden }
  });
}

export function createUser(data: {
  name: string;
  displayName: string;
  email: string;
  password: string;
  avatar?: string;
}) {
  return http.request<ApiResponse<null>>("post", `${BASE}/users`, { data });
}

export function deleteUser(name: string) {
  return http.request<ApiResponse<null>>("delete", `${BASE}/users`, {
    data: { name }
  });
}

export function updateUserProfile(data: {
  name: string;
  displayName?: string;
  email?: string;
  phone?: string;
  avatar?: string;
}) {
  return http.request<ApiResponse<null>>("put", `${BASE}/users/profile`, {
    data
  });
}

/** 获取头像预签名 PUT URL */
export function presignAvatarUpload(name: string, filename: string) {
  return http.request<ApiResponse<{ presignedUrl: string; avatarUrl: string }>>(
    "get",
    `${BASE}/users/avatar/presign`,
    {
      params: { name, filename: encodeURIComponent(filename) }
    }
  );
}

/** 前端通过预签名 URL 直传文件到 S3 后，调用此接口将 URL 写入 Casdoor */
export function confirmAvatarUpload(name: string, avatarUrl: string) {
  return http.request<ApiResponse<null>>(
    "post",
    `${BASE}/users/avatar/confirm`,
    {
      data: { name, avatarUrl }
    }
  );
}

/**
 * 上传头像（Presigned URL 流程）
 * 1. 获取预签名 URL
 * 2. 浏览器 PUT 文件直传 S3
 * 3. 确认写入 Casdoor
 */
export async function uploadAvatar(
  name: string,
  file: File
): Promise<ApiResponse<string>> {
  // 1. 获取预签名 URL
  const presign = await presignAvatarUpload(name, file.name);
  if (presign.code !== 0) {
    throw new Error(presign.message || "获取预签名 URL 失败");
  }
  const { presignedUrl, avatarUrl } = presign.data;

  // 2. 浏览器直传 S3 （使用原生 fetch，不走 axios 拦截器）
  const putResp = await fetch(presignedUrl, {
    method: "PUT",
    body: file,
    headers: { "Content-Type": file.type || "application/octet-stream" }
  });
  if (!putResp.ok) {
    throw new Error(`S3 上传失败: ${putResp.status} ${putResp.statusText}`);
  }

  // 3. 确认头像
  const confirm = await confirmAvatarUpload(name, avatarUrl);
  if (confirm.code !== 0) {
    throw new Error(confirm.message || "确认头像失败");
  }

  return { code: 0, message: "头像上传成功", data: avatarUrl };
}

// ============================================================
// 角色管理
// ============================================================

export function getRoles() {
  return http.request<ApiResponse<RoleItem[]>>("get", `${BASE}/roles`);
}

export function createRole(name: string, displayName: string) {
  return http.request<ApiResponse<null>>("post", `${BASE}/roles`, {
    data: { name, displayName }
  });
}

export function updateRole(
  name: string,
  displayName: string,
  users?: string[]
) {
  return http.request<ApiResponse<null>>("put", `${BASE}/roles`, {
    data: { name, displayName, users }
  });
}

export function deleteRole(name: string) {
  return http.request<ApiResponse<null>>("delete", `${BASE}/roles`, {
    data: { name }
  });
}

// ============================================================
// 权限管理
// ============================================================

export function getPermissions() {
  return http.request<ApiResponse<PermissionItem[]>>(
    "get",
    `${BASE}/permissions`
  );
}

export function createPermission(data: {
  name: string;
  displayName: string;
  resources: string[];
  actions: string[];
  roles?: string[];
}) {
  return http.request<ApiResponse<null>>("post", `${BASE}/permissions`, {
    data
  });
}

export function updatePermission(data: {
  name: string;
  displayName?: string;
  resources?: string[];
  actions?: string[];
  roles?: string[];
  effect?: string;
  isEnabled?: boolean;
}) {
  return http.request<ApiResponse<null>>("put", `${BASE}/permissions`, {
    data
  });
}

export function deletePermission(name: string) {
  return http.request<ApiResponse<null>>("delete", `${BASE}/permissions`, {
    data: { name }
  });
}

// ============================================================
// 权限测试
// ============================================================

export function testEnforce(role: string, path: string, method: string) {
  return http.request<ApiResponse<{ allowed: boolean }>>(
    "get",
    `${BASE}/test-enforce`,
    { params: { role, path, method } }
  );
}

// ============================================================
// Casdoor 组织清理（重置 Bootstrap）
// ============================================================

export function cleanupOrganization() {
  return http.request<ApiResponse<string[]>>(
    "delete",
    `${CASDOOR_BASE}/bootstrap`
  );
}

// ============================================================
// Casdoor 配置查询
// ============================================================

export function getDefaultAvatarUrl() {
  return http.request<ApiResponse<{ defaultAvatarUrl: string }>>(
    "get",
    `${CASDOOR_BASE}/config`
  );
}
