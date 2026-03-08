<script setup lang="ts">
import { ref, onMounted } from "vue";
import { message } from "@/utils/message";
import {
  getMessages,
  postMessage,
  deleteMessage,
  clearMessages,
  type Message
} from "../api";

defineOptions({
  name: "MessagesList"
});

const messages = ref<Message[]>([]);
const loading = ref(false);
const submitting = ref(false);

const form = ref({
  content: "",
  author: ""
});

// 获取消息列表
const fetchMessages = async () => {
  loading.value = true;
  try {
    const res = await getMessages();
    if (res.code === 0) {
      messages.value = res.data || [];
    }
  } catch (error) {
    console.error("获取消息失败:", error);
  } finally {
    loading.value = false;
  }
};

// 提交消息
const submitMessage = async () => {
  if (!form.value.content.trim() || !form.value.author.trim()) {
    message("请填写完整信息", { type: "warning" });
    return;
  }

  submitting.value = true;
  try {
    const res = await postMessage(form.value);
    if (res.code === 0) {
      message("消息提交成功", { type: "success" });
      form.value = { content: "", author: "" };
      await fetchMessages();
    }
  } catch (error) {
    message("提交失败", { type: "error" });
  } finally {
    submitting.value = false;
  }
};

// 删除消息
const handleDelete = async (id: number) => {
  try {
    const res = await deleteMessage(id);
    if (res.code === 0) {
      message("删除成功", { type: "success" });
      await fetchMessages();
    }
  } catch (error) {
    message("删除失败", { type: "error" });
  }
};

// 清空所有消息
const clearAll = async () => {
  try {
    const res = await clearMessages();
    if (res.code === 0) {
      message("已清空所有消息", { type: "success" });
      messages.value = [];
    }
  } catch (error) {
    message("清空失败", { type: "error" });
  }
};

// 格式化时间
const formatTime = (timeStr: string) => {
  return new Date(timeStr).toLocaleString("zh-CN");
};

onMounted(() => {
  fetchMessages();
});
</script>

<template>
  <div class="messages-container">
    <!-- 提交表单 -->
    <el-card class="submit-card" shadow="hover">
      <template #header>
        <div class="card-header">
          <span>📝 提交消息</span>
        </div>
      </template>
      <el-form :model="form" label-width="80px">
        <el-form-item label="作者">
          <el-input
            v-model="form.author"
            placeholder="请输入您的名称"
            maxlength="50"
            show-word-limit
          />
        </el-form-item>
        <el-form-item label="内容">
          <el-input
            v-model="form.content"
            type="textarea"
            :rows="4"
            placeholder="请输入消息内容"
            maxlength="500"
            show-word-limit
          />
        </el-form-item>
        <el-form-item>
          <el-button
            type="primary"
            :loading="submitting"
            @click="submitMessage"
          >
            提交消息
          </el-button>
        </el-form-item>
      </el-form>
    </el-card>

    <!-- 消息列表 -->
    <el-card class="list-card" shadow="hover">
      <template #header>
        <div class="card-header">
          <span>📋 消息列表 ({{ messages.length }})</span>
          <el-button
            v-if="messages.length > 0"
            type="danger"
            size="small"
            @click="clearAll"
          >
            清空全部
          </el-button>
        </div>
      </template>

      <div v-loading="loading">
        <el-empty v-if="messages.length === 0" description="暂无消息" />

        <el-timeline v-else>
          <el-timeline-item
            v-for="msg in messages"
            :key="msg.id"
            :timestamp="formatTime(msg.created_at)"
            placement="top"
          >
            <el-card shadow="never" class="message-item">
              <div class="message-header">
                <el-tag size="small">{{ msg.author }}</el-tag>
                <el-button
                  type="danger"
                  size="small"
                  text
                  @click="handleDelete(msg.id)"
                >
                  删除
                </el-button>
              </div>
              <p class="message-content">{{ msg.content }}</p>
            </el-card>
          </el-timeline-item>
        </el-timeline>
      </div>
    </el-card>
  </div>
</template>

<style scoped>
.messages-container {
  display: flex;
  flex-direction: column;
  gap: 20px;
  padding: 20px;
}

.submit-card {
  max-width: 600px;
}

.card-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  font-weight: 600;
}

.message-item {
  margin-bottom: 0;
}

.message-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 8px;
}

.message-content {
  margin: 0;
  color: #606266;
  word-break: break-word;
  white-space: pre-wrap;
}
</style>
