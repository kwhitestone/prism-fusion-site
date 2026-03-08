<template>
  <div class="site-info-container p-6">
    <el-card shadow="hover">
      <template #header>
        <div class="flex items-center justify-between">
          <span class="text-lg font-bold">站点信息</span>
          <el-tag type="success">业务插件</el-tag>
        </div>
      </template>

      <el-descriptions :column="2" border>
        <el-descriptions-item label="站点名称">
          {{ siteInfo.siteName }}
        </el-descriptions-item>
        <el-descriptions-item label="版本号">
          {{ siteInfo.version }}
        </el-descriptions-item>
        <el-descriptions-item label="描述" :span="2">
          {{ siteInfo.description }}
        </el-descriptions-item>
        <el-descriptions-item label="架构模式" :span="2">
          <el-tag>Mono-Repo</el-tag>
          <el-tag class="ml-2" type="info">prism-fusion (submodule)</el-tag>
        </el-descriptions-item>
      </el-descriptions>

      <el-divider />

      <el-alert
        title="此页面由业务插件 site-info 提供，代码位于 app/src/admin/src/addons/site-info/"
        type="info"
        :closable="false"
        show-icon
      />
    </el-card>
  </div>
</template>

<script setup lang="ts">
import { reactive, onMounted } from "vue";
import axios from "axios";

const siteInfo = reactive({
  siteName: "Prism Example Site",
  version: "1.0.0",
  description: "基于 Prism Fusion 框架的示例站点"
});

onMounted(async () => {
  try {
    const { data } = await axios.get("/api/v1/addons/site-info/info");
    Object.assign(siteInfo, {
      siteName: data.site_name,
      version: data.version,
      description: data.description
    });
  } catch {
    // 后端未启动时使用默认值
    console.log("[SiteInfo] Using default values (backend not available)");
  }
});
</script>

<style scoped>
.site-info-container {
  max-width: 800px;
}
</style>
