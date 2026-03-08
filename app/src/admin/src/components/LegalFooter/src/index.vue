<script setup lang="ts">
import { computed } from "vue";
import { getConfig } from "@/config";

// ── 版权与法规页脚 ──────────────────────────────────────────────────────
// 如需修改备案信息，仅在此文件中更改以下常量即可。

const siteTitle = getConfig("Title") as string;

// 版权年份
const copyrightYear = "2026";

// 工信部 ICP 备案
const icpLicense = "闽ICP备2026001798号-1";
const icpLicenseUrl = "https://beian.miit.gov.cn/";

// 公安联网备案
const policeRegistration = "闽公网安备35011102351136号";
const policeRegistrationUrl =
  "https://beian.mps.gov.cn/#/query/webSearch?code=35011102351136";
const policeRegistrationIcon = "/static/beian.png";

const displayIcp = computed(() => icpLicense.trim() !== "");
const displayPolice = computed(() => policeRegistration.trim() !== "");
</script>

<template>
  <div class="legal-footer">
    <span>Copyright &copy; {{ copyrightYear }}&nbsp;{{ siteTitle }}</span>
    <template v-if="displayIcp">
      <span class="legal-footer__sep">|</span>
      <a
        :href="icpLicenseUrl"
        target="_blank"
        rel="noreferrer"
        class="legal-footer__link hover:text-primary!"
      >
        {{ icpLicense }}
      </a>
    </template>
    <template v-if="displayPolice">
      <span class="legal-footer__sep">|</span>
      <a
        :href="policeRegistrationUrl"
        target="_blank"
        rel="noreferrer"
        class="legal-footer__link hover:text-primary!"
      >
        <img
          v-if="policeRegistrationIcon"
          :src="policeRegistrationIcon"
          alt=""
          class="legal-footer__icon"
        />
        {{ policeRegistration }}
      </a>
    </template>
  </div>
</template>

<style lang="scss" scoped>
.legal-footer {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
  align-items: center;
  justify-content: center;
  font-size: 12px;
  color: rgb(0 0 0 / 45%);

  .dark & {
    color: rgb(220 220 242 / 55%);
  }
}

.legal-footer__link {
  display: inline-flex;
  gap: 4px;
  align-items: center;
  color: inherit;
  text-decoration: none;
  transition: color 0.2s;
}

.legal-footer__sep {
  opacity: 0.35;
}

.legal-footer__icon {
  width: 13px;
  height: 13px;
  opacity: 0.7;
}
</style>
