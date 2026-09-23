<script setup lang="ts">
/**
 * Casdoor OAuth2 登录组件（自治式）
 *
 * 挂载时自动检测 URL 中的 ?code= 参数：
 * - 有 code → 处理 OAuth 回调（换 Token → 获取用户信息 → 跳转主页）
 * - 无 code → 显示 "使用 Casdoor 登录" 按钮
 *
 * 所有 OAuth 逻辑完全封装在此组件内，核心登录页无需了解具体认证方式。
 */
import Motion from "@/views/login/utils/motion";
import { ref, onMounted, onUnmounted } from "vue";
import { useRouter, useRoute } from "vue-router";
import { message } from "@/utils/message";
import { useUserStoreHook } from "@/store/modules/user";
import { initRouter, getTopMenu } from "@/router/utils";
import { setToken, setAuthToken } from "@/utils/auth";
import {
  createAuthRequestID,
  invalidateAuthSession,
  observeAuthSession,
  withAuthSessionLock
} from "@/core/auth-session";
import { triggerPluginRegistryReport } from "@/plugin/loader";
import { getSigninUrl, signinCallback } from "../api";

const router = useRouter();
const route = useRoute();

// UI 状态
const loading = ref(false);
const isProcessingCallback = ref(false);
const statusText = ref("");
let active = true;
onUnmounted(() => {
  active = false;
});

// ========== OAuth 回调处理 ==========
onMounted(async () => {
  // Hash 模式：code 在 window.location.search（# 之前）
  // History 模式：code 在 route.query
  const urlParams = new URLSearchParams(window.location.search);
  const code = urlParams.get("code") || (route.query.code as string) || "";
  if (!code) return;

  const state =
    urlParams.get("state") || (route.query.state as string) || "casdoor";

  isProcessingCallback.value = true;
  statusText.value = "正在验证登录...";

  try {
    const res = await signinCallback({
      code,
      state,
      redirectUri: `${window.location.origin}/login`
    });
    const body = res.data;
    if (!active) return;

    if (body.code !== 0) {
      throw new Error(body.message || "授权码交换失败");
    }

    const { accessToken, refreshToken, expiresIn, user } = body.data;
    const roles = user?.isAdmin ? ["admin"] : ["user"];
    const permissions: string[] = ["*:*:*"];

    // 存储 token 和用户信息
    const expireMs = (expiresIn || 360) * 1000; // 服务端返回秒数
    const tokenData = {
      avatar: "",
      username: user?.username || "",
      nickname: user?.nickName || user?.username || "",
      roles,
      permissions,
      accessToken,
      refreshToken: refreshToken || accessToken,
      sessionId: createAuthRequestID(),
      refreshRequestId: createAuthRequestID(),
      expires: new Date(Date.now() + expireMs)
    };
    const committed = await withAuthSessionLock(async () => {
      if (!active) return false;
      invalidateAuthSession();
      setAuthToken(`Bearer ${accessToken}`);
      setToken(tokenData);
      observeAuthSession(tokenData.sessionId);
      return true;
    });
    if (!committed || !active) return;
    triggerPluginRegistryReport();

    const userStore = useUserStoreHook();

    statusText.value = "登录成功，正在跳转...";

    // 清理 URL 中的 ?code=&state= 参数，避免刷新时重复处理
    const cleanUrl =
      window.location.origin + window.location.pathname + window.location.hash;
    window.history.replaceState({}, "", cleanUrl);

    // 初始化路由 & 同步最新用户信息
    await userStore.fetchUserInfo();
    await initRouter();

    const targetPath = getTopMenu(true)?.path || "/dashboard/index";
    await router.push(targetPath);
    message("登录成功", { type: "success" });
  } catch (error: any) {
    const errMsg = error?.message || "登录失败";
    statusText.value = errMsg;
    message(errMsg, { type: "error" });
    isProcessingCallback.value = false;
  }
});

// ========== Casdoor 登录按钮 ==========
const onCasdoorLogin = async () => {
  loading.value = true;
  try {
    const res = await getSigninUrl("casdoor");
    const url = res.data?.data?.url;
    if (url) {
      window.location.href = url;
    } else {
      message("获取 Casdoor 登录地址失败", { type: "error" });
      loading.value = false;
    }
  } catch (error: any) {
    message(
      error?.response?.data?.message || error?.message || "获取登录地址失败",
      { type: "error" }
    );
    loading.value = false;
  }
};
</script>

<template>
  <!-- OAuth 回调处理中 -->
  <div v-if="isProcessingCallback" class="casdoor-login">
    <Motion>
      <div class="flex flex-col items-center justify-center py-8">
        <el-icon class="is-loading" :size="32" color="#409EFF">
          <i class="ep:loading" />
        </el-icon>
        <p class="mt-4 text-gray-500">{{ statusText }}</p>
      </div>
    </Motion>
  </div>

  <!-- 登录按钮 -->
  <div v-else class="casdoor-login">
    <Motion :delay="100">
      <p class="casdoor-hint">本系统使用 Casdoor 统一认证</p>
    </Motion>

    <Motion :delay="200">
      <el-button
        class="w-full mt-4!"
        size="default"
        type="primary"
        :loading="loading"
        @click="onCasdoorLogin"
      >
        <el-icon v-if="!loading" class="mr-2">
          <svg
            viewBox="0 0 1024 1024"
            xmlns="http://www.w3.org/2000/svg"
            width="16"
            height="16"
          >
            <path
              d="M512 0C229.232 0 0 229.232 0 512c0 282.784 229.232 512 512 512 282.784 0 512-229.216 512-512C1024 229.232 794.784 0 512 0zM256 640a64 64 0 1 1 0-128 64 64 0 0 1 0 128z m256 128a64 64 0 1 1 0-128 64 64 0 0 1 0 128z m0-256a128 128 0 1 1 0-256 128 128 0 0 1 0 256z m256 128a64 64 0 1 1 0-128 64 64 0 0 1 0 128z"
              fill="currentColor"
            />
          </svg>
        </el-icon>
        使用 Casdoor 登录
      </el-button>
    </Motion>
  </div>
</template>

<style scoped>
.casdoor-hint {
  margin-bottom: 8px;
  font-size: 14px;
  color: var(--el-text-color-secondary);
  text-align: center;
}
</style>
