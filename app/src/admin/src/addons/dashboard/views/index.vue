<template>
  <div class="dashboard-container">
    <!-- 页面标题 -->
    <div class="page-header">
      <h1 class="page-title">数据总览</h1>
      <p class="page-subtitle">Prism Fusion 系统运行概况</p>
    </div>

    <!-- 统计卡片网格 -->
    <div class="stats-grid">
      <div
        v-for="card in statCards"
        :key="card.key"
        class="stat-card"
        :style="{ '--card-color': card.color }"
      >
        <div class="card-icon">
          <component :is="card.icon" />
        </div>
        <div class="card-content">
          <div class="card-value">
            <span v-if="loading" class="loading-placeholder" />
            <span v-else class="value-number">{{ card.value }}</span>
          </div>
          <div class="card-label">{{ card.label }}</div>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from "vue";
import {
  Files,
  Goods,
  Location,
  Monitor,
  CopyDocument,
  Connection,
  Download
} from "@element-plus/icons-vue";
import { getDashboardStats, type DashboardStats } from "../api";

defineOptions({
  name: "Dashboard"
});

const loading = ref(true);

// 统计数据
const stats = ref<DashboardStats>({
  resourceCount: 0,
  productCount: 0,
  regionCount: 0,
  deviceCount: 0,
  distributionCount: 0,
  syncCount: 0,
  downloadCount: 0
});

// 统计卡片配置
const statCards = computed(() => [
  {
    key: "resource",
    label: "资源总数",
    value: stats.value.resourceCount,
    icon: Files,
    color: "#3b82f6"
  },
  {
    key: "product",
    label: "产品总数",
    value: stats.value.productCount,
    icon: Goods,
    color: "#8b5cf6"
  },
  {
    key: "region",
    label: "区域总数",
    value: stats.value.regionCount,
    icon: Location,
    color: "#10b981"
  },
  {
    key: "device",
    label: "设备总数",
    value: stats.value.deviceCount,
    icon: Monitor,
    color: "#f59e0b"
  },
  {
    key: "distribution",
    label: "资源分布总数",
    value: stats.value.distributionCount,
    icon: CopyDocument,
    color: "#ec4899"
  },
  {
    key: "sync",
    label: "资源同步总数",
    value: stats.value.syncCount,
    icon: Connection,
    color: "#06b6d4"
  },
  {
    key: "download",
    label: "下载总数",
    value: stats.value.downloadCount,
    icon: Download,
    color: "#ef4444"
  }
]);

// 获取统计数据
const fetchStats = async () => {
  loading.value = true;
  try {
    const response = await getDashboardStats();
    if (response.code === 0 && response.data) {
      stats.value = response.data;
    }
  } catch (error) {
    console.error("获取统计数据失败:", error);
  } finally {
    loading.value = false;
  }
};

onMounted(() => {
  fetchStats();
});
</script>

<style scoped>
@keyframes loading {
  0% {
    background-position: 200% 0;
  }

  100% {
    background-position: -200% 0;
  }
}

/* 响应式设计 */
@media (width <= 768px) {
  .page-title {
    font-size: 24px;
  }

  .stats-grid {
    grid-template-columns: 1fr;
    gap: 16px;
  }

  .stat-card {
    padding: 20px;
  }

  .card-icon {
    width: 48px;
    height: 48px;
  }

  .card-icon svg {
    width: 24px;
    height: 24px;
  }

  .value-number {
    font-size: 28px;
  }
}

.dashboard-container {
  min-height: 100%;
}

/* 页面标题 */
.page-header {
  margin-bottom: 32px;
}

.page-title {
  margin: 0 0 8px;
  font-size: 28px;
  font-weight: 700;
  color: #1e293b;
}

.page-subtitle {
  margin: 0;
  font-size: 15px;
  color: #64748b;
}

/* 统计卡片网格 */
.stats-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(280px, 1fr));
  gap: 20px;
}

/* 统计卡片 */
.stat-card {
  display: flex;
  gap: 16px;
  align-items: center;
  padding: 24px;
  background: white;
  border: 1px solid #e2e8f0;
  border-radius: 12px;
  box-shadow: 0 1px 3px rgb(0 0 0 / 5%);
  transition: all 0.2s ease;
}

.stat-card:hover {
  border-color: var(--card-color);
  box-shadow: 0 4px 12px rgb(0 0 0 / 10%);
  transform: translateY(-2px);
}

/* 卡片图标 */
.card-icon {
  display: flex;
  flex-shrink: 0;
  align-items: center;
  justify-content: center;
  width: 56px;
  height: 56px;
  color: white;
  background: linear-gradient(
    135deg,
    var(--card-color) 0%,
    color-mix(in srgb, var(--card-color) 80%, white) 100%
  );
  border-radius: 12px;
}

.card-icon svg {
  width: 28px;
  height: 28px;
}

/* 卡片内容 */
.card-content {
  flex: 1;
  min-width: 0;
}

.card-value {
  margin-bottom: 4px;
}

.value-number {
  font-size: 32px;
  font-weight: 700;
  line-height: 1.2;
  color: #1e293b;
}

.card-label {
  font-size: 14px;
  font-weight: 500;
  color: #64748b;
}

/* 加载占位符 */
.loading-placeholder {
  display: inline-block;
  width: 60px;
  height: 32px;
  background: linear-gradient(90deg, #f1f5f9 25%, #e2e8f0 50%, #f1f5f9 75%);
  background-size: 200% 100%;
  border-radius: 6px;
  animation: loading 1.5s infinite;
}
</style>
