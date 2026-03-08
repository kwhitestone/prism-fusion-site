<script setup lang="ts">
import { ref, onMounted } from "vue";
import { ElMessage, ElMessageBox } from "element-plus";
import {
  getRoles,
  getUsers,
  createRole,
  updateRole,
  deleteRole,
  type RoleItem,
  type UserItem
} from "../api/admin";

defineOptions({ name: "RbacRoles" });

const loading = ref(false);
const roles = ref<RoleItem[]>([]);
const allUsers = ref<UserItem[]>([]);

// 创建/编辑对话框
const dialogVisible = ref(false);
const isEditing = ref(false);
const saving = ref(false);
const form = ref({
  name: "",
  displayName: "",
  users: [] as string[]
});

async function loadData() {
  loading.value = true;
  try {
    const [rolesRes, usersRes] = await Promise.all([getRoles(), getUsers()]);
    roles.value = rolesRes.data || [];
    allUsers.value = usersRes.data || [];
  } catch {
    ElMessage.error("加载数据失败");
  } finally {
    loading.value = false;
  }
}

function handleAdd() {
  isEditing.value = false;
  form.value = { name: "", displayName: "", users: [] };
  dialogVisible.value = true;
}

function handleEdit(role: RoleItem) {
  isEditing.value = true;
  form.value = {
    name: role.name,
    displayName: role.displayName,
    users: [...(role.users || [])]
  };
  dialogVisible.value = true;
}

async function handleSubmit() {
  if (!form.value.name.trim()) {
    ElMessage.warning("请输入角色名");
    return;
  }
  saving.value = true;
  try {
    if (isEditing.value) {
      await updateRole(
        form.value.name,
        form.value.displayName,
        form.value.users
      );
      ElMessage.success("角色更新成功");
    } else {
      await createRole(form.value.name, form.value.displayName);
      ElMessage.success("角色创建成功");
    }
    dialogVisible.value = false;
    loadData();
  } catch {
    ElMessage.error(isEditing.value ? "更新失败" : "创建失败");
  } finally {
    saving.value = false;
  }
}

async function handleDelete(role: RoleItem) {
  try {
    await ElMessageBox.confirm(
      `确定删除角色「${role.displayName || role.name}」吗？`,
      "提示",
      { type: "warning" }
    );
    await deleteRole(role.name);
    ElMessage.success("删除成功");
    loadData();
  } catch (e) {
    if (e !== "cancel") ElMessage.error("删除失败");
  }
}

// 用户选项列表（过滤用）
function userOptions() {
  return allUsers.value.map(u => ({
    value: u.name,
    label: u.displayName ? `${u.displayName} (${u.name})` : u.name
  }));
}

onMounted(loadData);
</script>

<template>
  <div class="p-4">
    <div class="mb-4 flex items-center justify-between">
      <h2 class="text-lg font-semibold">角色管理</h2>
      <el-button type="primary" @click="handleAdd">新增角色</el-button>
    </div>

    <el-card shadow="never">
      <el-table v-loading="loading" :data="roles" border stripe>
        <el-table-column prop="name" label="角色标识" width="150" />
        <el-table-column prop="displayName" label="显示名" width="150" />
        <el-table-column label="关联用户" min-width="250">
          <template #default="{ row }">
            <el-tag
              v-for="user in row.users"
              :key="user"
              size="small"
              class="mr-1 mb-1"
              >{{ user }}</el-tag
            >
            <span v-if="!row.users?.length" class="text-gray-400"
              >无关联用户</span
            >
          </template>
        </el-table-column>
        <el-table-column label="状态" width="80" align="center">
          <template #default="{ row }">
            <el-tag :type="row.isEnabled ? 'success' : 'info'" size="small">{{
              row.isEnabled ? "启用" : "禁用"
            }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="createdTime" label="创建时间" width="170" />
        <el-table-column label="操作" width="140" fixed="right">
          <template #default="{ row }">
            <el-button type="primary" link @click="handleEdit(row)"
              >编辑</el-button
            >
            <el-button type="danger" link @click="handleDelete(row)"
              >删除</el-button
            >
          </template>
        </el-table-column>
      </el-table>
    </el-card>

    <!-- 新增/编辑对话框 -->
    <el-dialog
      v-model="dialogVisible"
      :title="isEditing ? '编辑角色' : '新增角色'"
      width="520px"
    >
      <el-form :model="form" label-width="90px">
        <el-form-item label="角色标识" required>
          <el-input
            v-model="form.name"
            :disabled="isEditing"
            placeholder="英文标识，如 admin、editor"
          />
        </el-form-item>
        <el-form-item label="显示名">
          <el-input
            v-model="form.displayName"
            placeholder="中文显示名，如 管理员"
          />
        </el-form-item>
        <el-form-item v-if="isEditing" label="关联用户">
          <el-select
            v-model="form.users"
            multiple
            filterable
            placeholder="选择用户"
            style="width: 100%"
          >
            <el-option
              v-for="opt in userOptions()"
              :key="opt.value"
              :label="opt.label"
              :value="opt.value"
            />
          </el-select>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="dialogVisible = false">取消</el-button>
        <el-button type="primary" :loading="saving" @click="handleSubmit"
          >确定</el-button
        >
      </template>
    </el-dialog>
  </div>
</template>
