<script setup lang="ts">
import { ref, reactive, onMounted } from "vue";
import { ElMessage, ElMessageBox } from "element-plus";
import { useUserStoreHook } from "@/store/modules/user";
import {
  getUsers,
  getRoles,
  updateUserRoles,
  toggleUser,
  createUser,
  deleteUser,
  updateUserProfile,
  uploadAvatar,
  cleanupOrganization,
  getDefaultAvatarUrl,
  type UserItem,
  type RoleItem
} from "../api/admin";

defineOptions({ name: "RbacUsers" });

const loading = ref(false);
const users = ref<UserItem[]>([]);
const allRoles = ref<RoleItem[]>([]);
const search = ref("");

// 角色分配对话框
const roleDialogVisible = ref(false);
const editingUser = ref<UserItem | null>(null);
const selectedRoles = ref<string[]>([]);
const saving = ref(false);

// 新建用户对话框
const addDialogVisible = ref(false);
const addLoading = ref(false);
const defaultAvatar = ref("/logo.svg");
const addForm = reactive({
  name: "",
  displayName: "",
  email: "",
  password: "",
  avatar: ""
});
const addAvatarUploading = ref(false);
const addAvatarInputRef = ref<HTMLInputElement>();

function triggerAddAvatarInput() {
  if (!addForm.name) {
    ElMessage.warning("请先填写用户名");
    return;
  }
  addAvatarInputRef.value?.click();
}

function resetAddForm() {
  addForm.name = "";
  addForm.displayName = "";
  addForm.email = "";
  addForm.password = "";
  addForm.avatar = "";
}

// 编辑用户对话框
const editDialogVisible = ref(false);
const editLoading = ref(false);
const editForm = reactive({
  name: "",
  displayName: "",
  email: "",
  phone: "",
  avatar: ""
});
const avatarUploading = ref(false);
const editAvatarInputRef = ref<HTMLInputElement>();

function triggerEditAvatarInput() {
  editAvatarInputRef.value?.click();
}

function handleEdit(user: UserItem) {
  editForm.name = user.name;
  editForm.displayName = user.displayName;
  editForm.email = user.email || "";
  editForm.phone = user.phone || "";
  editForm.avatar = user.avatar || "";
  editDialogVisible.value = true;
}

async function handleSaveProfile() {
  if (!editForm.name) return;
  editLoading.value = true;
  try {
    const res = await updateUserProfile({ ...editForm });
    if (res.code !== 0) throw new Error(res.message);
    ElMessage.success("用户资料更新成功");
    editDialogVisible.value = false;
    loadData();
  } catch (e: any) {
    ElMessage.error(e?.message || "更新用户资料失败");
  } finally {
    editLoading.value = false;
  }
}

async function handleAvatarUpload(event: Event) {
  const input = event.target as HTMLInputElement;
  const file = input.files?.[0];
  if (!file) return;
  if (file.size > 2 * 1024 * 1024) {
    ElMessage.warning("头像文件不能超过 2MB");
    return;
  }
  avatarUploading.value = true;
  try {
    const res = await uploadAvatar(editForm.name, file);
    if (res.code !== 0) throw new Error(res.message);
    editForm.avatar = res.data;
    ElMessage.success("头像上传成功");
  } catch (e: any) {
    ElMessage.error(e?.message || "头像上传失败");
  } finally {
    avatarUploading.value = false;
    input.value = "";
  }
}

async function handleAddAvatarUpload(event: Event) {
  const input = event.target as HTMLInputElement;
  const file = input.files?.[0];
  if (!file) return;
  if (!addForm.name) {
    ElMessage.warning("请先填写用户名");
    input.value = "";
    return;
  }
  if (file.size > 2 * 1024 * 1024) {
    ElMessage.warning("头像文件不能超过 2MB");
    input.value = "";
    return;
  }
  addAvatarUploading.value = true;
  try {
    const res = await uploadAvatar(addForm.name, file);
    if (res.code !== 0) throw new Error(res.message);
    addForm.avatar = res.data;
    ElMessage.success("头像上传成功");
  } catch (e: any) {
    ElMessage.error(e?.message || "头像上传失败");
  } finally {
    addAvatarUploading.value = false;
    input.value = "";
  }
}

async function loadData() {
  loading.value = true;
  try {
    const [usersRes, rolesRes] = await Promise.all([getUsers(), getRoles()]);
    users.value = usersRes.data || [];
    allRoles.value = rolesRes.data || [];
  } catch {
    ElMessage.error("加载数据失败");
  } finally {
    loading.value = false;
  }
}

function filteredUsers() {
  if (!search.value) return users.value;
  const q = search.value.toLowerCase();
  return users.value.filter(
    u =>
      u.name.toLowerCase().includes(q) ||
      u.displayName.toLowerCase().includes(q) ||
      u.email?.toLowerCase().includes(q)
  );
}

function handleEditRoles(user: UserItem) {
  editingUser.value = user;
  selectedRoles.value = [...(user.roles || [])];
  roleDialogVisible.value = true;
}

async function handleSaveRoles() {
  if (!editingUser.value) return;
  saving.value = true;
  try {
    const res = await updateUserRoles(
      editingUser.value.name,
      selectedRoles.value
    );
    if (res.code !== 0) throw new Error(res.message);
    ElMessage.success("角色更新成功");
    roleDialogVisible.value = false;
    loadData();
  } catch (e: any) {
    ElMessage.error(e?.message || "更新角色失败");
  } finally {
    saving.value = false;
  }
}

async function handleToggle(user: UserItem) {
  const action = user.isForbidden ? "启用" : "禁用";
  try {
    await ElMessageBox.confirm(
      `确定${action}用户「${user.name}」吗？`,
      "提示",
      {
        type: "warning"
      }
    );
    const res = await toggleUser(user.name, !user.isForbidden);
    if (res.code !== 0) throw new Error(res.message);
    ElMessage.success(`${action}成功`);
    loadData();
  } catch (e: any) {
    if (e !== "cancel") ElMessage.error(e?.message || `${action}失败`);
  }
}

async function handleAddUser() {
  if (!addForm.name || !addForm.password) {
    ElMessage.warning("用户名和密码为必填项");
    return;
  }
  addLoading.value = true;
  try {
    const res = await createUser({ ...addForm });
    if (res.code !== 0) throw new Error(res.message);
    ElMessage.success("用户创建成功");
    addDialogVisible.value = false;
    resetAddForm();
    loadData();
  } catch (e: any) {
    ElMessage.error(e?.message || "创建用户失败");
  } finally {
    addLoading.value = false;
  }
}

async function handleDelete(user: UserItem) {
  try {
    await ElMessageBox.confirm(
      `确定删除用户「${user.name}」吗？此操作不可恢复。`,
      "警告",
      {
        type: "error",
        confirmButtonText: "删除",
        confirmButtonClass: "el-button--danger"
      }
    );
    const res = await deleteUser(user.name);
    if (res.code !== 0) throw new Error(res.message);
    ElMessage.success("用户已删除");
    loadData();
  } catch (e: any) {
    if (e !== "cancel") ElMessage.error(e?.message || "删除用户失败");
  }
}

const cleanupLoading = ref(false);

async function handleCleanup() {
  try {
    await ElMessageBox.confirm(
      "此操作将删除当前组织下的所有用户、角色、权限、应用和组织，然后自动重新创建。\n当前登录会话将失效，需要重新登录。",
      "重置 Casdoor 组织",
      {
        type: "error",
        confirmButtonText: "确认重置",
        confirmButtonClass: "el-button--danger",
        dangerouslyUseHTMLString: false
      }
    );
    cleanupLoading.value = true;
    const res = await cleanupOrganization();
    if (res.code !== 0) throw new Error(res.message);
    ElMessage.success(res.message || "组织已重置");
    // 清除登录态并跳转到登录页
    setTimeout(() => {
      useUserStoreHook().logOut();
    }, 1500);
  } catch (e: any) {
    if (e !== "cancel") ElMessage.error(e?.message || "清理失败");
  } finally {
    cleanupLoading.value = false;
  }
}

onMounted(async () => {
  loadData();
  // 获取默认头像 URL
  try {
    const res = await getDefaultAvatarUrl();
    if (res.data?.defaultAvatarUrl) {
      defaultAvatar.value = res.data.defaultAvatarUrl;
    }
  } catch {
    // 忽略，保留 /logo.svg 回退
  }
});
</script>

<template>
  <div class="p-4">
    <div class="mb-4 flex items-center justify-between">
      <h2 class="text-lg font-semibold">用户管理</h2>
      <div class="flex items-center gap-3">
        <el-input
          v-model="search"
          placeholder="搜索用户名/显示名/邮箱"
          clearable
          style="width: 280px"
          prefix-icon="Search"
        />
        <el-button type="primary" @click="addDialogVisible = true"
          >新建用户</el-button
        >
        <el-button
          type="danger"
          plain
          :loading="cleanupLoading"
          @click="handleCleanup"
          >重置组织</el-button
        >
      </div>
    </div>

    <el-card shadow="never">
      <el-table v-loading="loading" :data="filteredUsers()" border stripe>
        <el-table-column label="" width="50" align="center">
          <template #default="{ row }">
            <el-avatar
              :size="32"
              :src="row.avatar"
              style="vertical-align: middle"
            >
              {{ (row.displayName || row.name)?.charAt(0) }}
            </el-avatar>
          </template>
        </el-table-column>
        <el-table-column prop="name" label="用户名" min-width="120" />
        <el-table-column prop="displayName" label="显示名" min-width="120" />
        <el-table-column prop="email" label="邮箱" min-width="180" />
        <el-table-column prop="phone" label="手机号" width="130" />
        <el-table-column label="角色" min-width="200">
          <template #default="{ row }">
            <el-tag
              v-for="role in row.roles"
              :key="role"
              size="small"
              class="mr-1"
              >{{ role }}</el-tag
            >
            <el-tag v-if="!row.roles?.length" type="info" size="small"
              >无角色</el-tag
            >
          </template>
        </el-table-column>
        <el-table-column label="状态" width="80" align="center">
          <template #default="{ row }">
            <el-tag :type="row.isForbidden ? 'danger' : 'success'" size="small">
              {{ row.isForbidden ? "禁用" : "正常" }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="createdTime" label="创建时间" width="170" />
        <el-table-column label="操作" width="270" fixed="right">
          <template #default="{ row }">
            <el-button type="primary" link @click="handleEdit(row)"
              >编辑</el-button
            >
            <el-button type="primary" link @click="handleEditRoles(row)"
              >分配角色</el-button
            >
            <el-button
              :type="row.isForbidden ? 'success' : 'danger'"
              link
              @click="handleToggle(row)"
            >
              {{ row.isForbidden ? "启用" : "禁用" }}
            </el-button>
            <el-button type="danger" link @click="handleDelete(row)"
              >删除</el-button
            >
          </template>
        </el-table-column>
      </el-table>
    </el-card>

    <!-- 角色分配对话框 -->
    <el-dialog
      v-model="roleDialogVisible"
      :title="`分配角色 - ${editingUser?.name}`"
      width="480px"
    >
      <el-checkbox-group v-model="selectedRoles">
        <el-checkbox
          v-for="role in allRoles"
          :key="role.name"
          :label="role.name"
          :value="role.name"
          border
          class="mb-2! mr-2!"
        >
          {{ role.displayName || role.name }}
        </el-checkbox>
      </el-checkbox-group>
      <el-empty
        v-if="allRoles.length === 0"
        description="暂无角色，请先创建"
        :image-size="60"
      />
      <template #footer>
        <el-button @click="roleDialogVisible = false">取消</el-button>
        <el-button type="primary" :loading="saving" @click="handleSaveRoles"
          >确定</el-button
        >
      </template>
    </el-dialog>

    <!-- 编辑用户对话框 -->
    <el-dialog
      v-model="editDialogVisible"
      :title="`编辑用户 - ${editForm.name}`"
      width="520px"
    >
      <el-form :model="editForm" label-width="80px">
        <el-form-item label="头像">
          <div class="flex items-center gap-4">
            <el-avatar :size="64" :src="editForm.avatar || defaultAvatar">
              {{ (editForm.displayName || editForm.name)?.charAt(0) }}
            </el-avatar>
            <div class="flex flex-col gap-2">
              <el-button
                type="primary"
                plain
                size="small"
                :loading="avatarUploading"
                :disabled="avatarUploading"
                @click="triggerEditAvatarInput"
              >
                {{ avatarUploading ? "上传中..." : "上传头像" }}
              </el-button>
              <input
                ref="editAvatarInputRef"
                type="file"
                accept="image/*"
                style="display: none"
                :disabled="avatarUploading"
                @change="handleAvatarUpload"
              />
              <el-input
                v-model="editForm.avatar"
                :placeholder="defaultAvatar"
                size="small"
                clearable
              />
            </div>
          </div>
        </el-form-item>
        <el-form-item label="显示名">
          <el-input v-model="editForm.displayName" placeholder="请输入显示名" />
        </el-form-item>
        <el-form-item label="邮箱">
          <el-input v-model="editForm.email" placeholder="请输入邮箱" />
        </el-form-item>
        <el-form-item label="手机号">
          <el-input v-model="editForm.phone" placeholder="请输入手机号" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="editDialogVisible = false">取消</el-button>
        <el-button
          type="primary"
          :loading="editLoading"
          @click="handleSaveProfile"
          >保存</el-button
        >
      </template>
    </el-dialog>

    <!-- 新建用户对话框 -->
    <el-dialog
      v-model="addDialogVisible"
      title="新建用户"
      width="480px"
      @closed="resetAddForm"
    >
      <el-form :model="addForm" label-width="80px">
        <el-form-item label="用户名" required>
          <el-input v-model="addForm.name" placeholder="请输入用户名" />
        </el-form-item>
        <el-form-item label="显示名">
          <el-input v-model="addForm.displayName" placeholder="请输入显示名" />
        </el-form-item>
        <el-form-item label="邮箱">
          <el-input v-model="addForm.email" placeholder="请输入邮箱" />
        </el-form-item>
        <el-form-item label="密码" required>
          <el-input
            v-model="addForm.password"
            type="password"
            show-password
            placeholder="请输入密码"
          />
        </el-form-item>
        <el-form-item label="头像">
          <div class="flex items-center gap-4">
            <el-avatar :size="64" :src="addForm.avatar || defaultAvatar">
              {{ (addForm.displayName || addForm.name || "?")?.charAt(0) }}
            </el-avatar>
            <div class="flex flex-col gap-2">
              <el-button
                type="primary"
                plain
                size="small"
                :loading="addAvatarUploading"
                :disabled="addAvatarUploading || !addForm.name"
                @click="triggerAddAvatarInput"
              >
                {{ addAvatarUploading ? "上传中..." : "上传头像" }}
              </el-button>
              <input
                ref="addAvatarInputRef"
                type="file"
                accept="image/*"
                style="display: none"
                :disabled="addAvatarUploading || !addForm.name"
                @change="handleAddAvatarUpload"
              />
              <el-input
                v-model="addForm.avatar"
                :placeholder="defaultAvatar"
                size="small"
                clearable
              />
            </div>
          </div>
          <div
            v-if="!addForm.name"
            class="mt-1 text-xs"
            style="color: var(--el-text-color-placeholder)"
          >
            请先填写用户名再上传头像
          </div>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="addDialogVisible = false">取消</el-button>
        <el-button type="primary" :loading="addLoading" @click="handleAddUser"
          >确定</el-button
        >
      </template>
    </el-dialog>
  </div>
</template>
