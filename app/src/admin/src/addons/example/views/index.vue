<script setup lang="ts">
import { ref, onMounted } from "vue";
import { ElMessage, ElMessageBox } from "element-plus";
import {
  getExampleItems,
  createExampleItem,
  deleteExampleItem,
  type ExampleItem
} from "../api";

defineOptions({
  name: "AddonExampleIndex"
});

const loading = ref(false);
const items = ref<ExampleItem[]>([]);
const dialogVisible = ref(false);
const form = ref({
  name: "",
  description: ""
});

/** 加载数据 */
async function loadData() {
  loading.value = true;
  try {
    const res = await getExampleItems();
    items.value = res.data || [];
  } catch (error) {
    console.error("加载数据失败:", error);
    ElMessage.error("加载数据失败");
  } finally {
    loading.value = false;
  }
}

/** 新增 */
function handleAdd() {
  form.value = { name: "", description: "" };
  dialogVisible.value = true;
}

/** 提交表单 */
async function handleSubmit() {
  if (!form.value.name.trim()) {
    ElMessage.warning("请输入名称");
    return;
  }
  try {
    await createExampleItem(form.value);
    ElMessage.success("创建成功");
    dialogVisible.value = false;
    loadData();
  } catch (error) {
    console.error("创建失败:", error);
    ElMessage.error("创建失败");
  }
}

/** 删除 */
async function handleDelete(row: ExampleItem) {
  try {
    await ElMessageBox.confirm(`确定删除「${row.name}」吗？`, "提示", {
      type: "warning"
    });
    await deleteExampleItem(row.id);
    ElMessage.success("删除成功");
    loadData();
  } catch (error) {
    if (error !== "cancel") {
      console.error("删除失败:", error);
      ElMessage.error("删除失败");
    }
  }
}

onMounted(() => {
  loadData();
});
</script>

<template>
  <div class="p-4">
    <!-- 页面头部 -->
    <div class="mb-4 flex items-center justify-between">
      <h2 class="text-lg font-semibold">示例插件页面</h2>
      <el-button type="primary" @click="handleAdd">
        <template #icon>
          <IconifyIconOffline icon="ep:plus" />
        </template>
        新增
      </el-button>
    </div>

    <!-- 数据表格 -->
    <el-card shadow="never">
      <el-table v-loading="loading" :data="items" border stripe>
        <el-table-column prop="id" label="ID" width="80" />
        <el-table-column prop="name" label="名称" min-width="150" />
        <el-table-column prop="description" label="描述" min-width="200" />
        <el-table-column prop="status" label="状态" width="100">
          <template #default="{ row }">
            <el-tag :type="row.status === 1 ? 'success' : 'danger'">
              {{ row.status === 1 ? "启用" : "禁用" }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="created_at" label="创建时间" width="180" />
        <el-table-column label="操作" width="120" fixed="right">
          <template #default="{ row }">
            <el-button type="danger" link @click="handleDelete(row)">
              删除
            </el-button>
          </template>
        </el-table-column>
      </el-table>

      <!-- 空状态 -->
      <el-empty v-if="!loading && items.length === 0" description="暂无数据" />
    </el-card>

    <!-- 新增对话框 -->
    <el-dialog v-model="dialogVisible" title="新增示例项" width="500px">
      <el-form :model="form" label-width="80px">
        <el-form-item label="名称" required>
          <el-input v-model="form.name" placeholder="请输入名称" />
        </el-form-item>
        <el-form-item label="描述">
          <el-input
            v-model="form.description"
            type="textarea"
            :rows="3"
            placeholder="请输入描述"
          />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="dialogVisible = false">取消</el-button>
        <el-button type="primary" @click="handleSubmit">确定</el-button>
      </template>
    </el-dialog>
  </div>
</template>
