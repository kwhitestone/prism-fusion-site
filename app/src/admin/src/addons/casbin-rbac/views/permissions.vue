<script setup lang="ts">
import { ref, onMounted } from "vue";
import { ElMessage, ElMessageBox } from "element-plus";
import {
  getPermissions,
  getRoles,
  createPermission,
  updatePermission,
  deletePermission,
  testEnforce,
  type PermissionItem,
  type RoleItem
} from "../api/admin";

defineOptions({ name: "RbacPermissions" });

const loading = ref(false);
const permissions = ref<PermissionItem[]>([]);
const allRoles = ref<RoleItem[]>([]);

// 创建/编辑对话框
const dialogVisible = ref(false);
const isEditing = ref(false);
const saving = ref(false);
const form = ref({
  name: "",
  displayName: "",
  resources: [] as string[],
  actions: [] as string[],
  roles: [] as string[],
  effect: "Allow",
  isEnabled: true
});

// 资源输入
const newResource = ref("");

// 测试对话框
const testDialogVisible = ref(false);
const testForm = ref({ role: "", path: "", method: "GET" });
const testResult = ref<boolean | null>(null);
const testing = ref(false);

const httpMethods = ["GET", "POST", "PUT", "DELETE", "*"];

async function loadData() {
  loading.value = true;
  try {
    const [permsRes, rolesRes] = await Promise.all([
      getPermissions(),
      getRoles()
    ]);
    permissions.value = permsRes.data || [];
    allRoles.value = rolesRes.data || [];
  } catch {
    ElMessage.error("加载数据失败");
  } finally {
    loading.value = false;
  }
}

function handleAdd() {
  isEditing.value = false;
  form.value = {
    name: "",
    displayName: "",
    resources: [],
    actions: ["*"],
    roles: [],
    effect: "Allow",
    isEnabled: true
  };
  dialogVisible.value = true;
}

function handleEdit(perm: PermissionItem) {
  isEditing.value = true;
  form.value = {
    name: perm.name,
    displayName: perm.displayName,
    resources: [...(perm.resources || [])],
    actions: [...(perm.actions || [])],
    roles: [...(perm.roles || [])],
    effect: perm.effect || "Allow",
    isEnabled: perm.isEnabled
  };
  dialogVisible.value = true;
}

function addResource() {
  const val = newResource.value.trim();
  if (val && !form.value.resources.includes(val)) {
    form.value.resources.push(val);
  }
  newResource.value = "";
}

function removeResource(idx: number) {
  form.value.resources.splice(idx, 1);
}

async function handleSubmit() {
  if (!form.value.name.trim()) {
    ElMessage.warning("请输入权限名");
    return;
  }
  if (form.value.resources.length === 0) {
    ElMessage.warning("请至少添加一个资源路径");
    return;
  }
  if (form.value.actions.length === 0) {
    ElMessage.warning("请至少选择一个操作");
    return;
  }

  saving.value = true;
  try {
    if (isEditing.value) {
      await updatePermission({
        name: form.value.name,
        displayName: form.value.displayName,
        resources: form.value.resources,
        actions: form.value.actions,
        roles: form.value.roles,
        effect: form.value.effect,
        isEnabled: form.value.isEnabled
      });
      ElMessage.success("权限更新成功");
    } else {
      await createPermission({
        name: form.value.name,
        displayName: form.value.displayName,
        resources: form.value.resources,
        actions: form.value.actions,
        roles: form.value.roles
      });
      ElMessage.success("权限创建成功");
    }
    dialogVisible.value = false;
    loadData();
  } catch {
    ElMessage.error(isEditing.value ? "更新失败" : "创建失败");
  } finally {
    saving.value = false;
  }
}

async function handleDelete(perm: PermissionItem) {
  try {
    await ElMessageBox.confirm(
      `确定删除权限「${perm.displayName || perm.name}」吗？`,
      "提示",
      { type: "warning" }
    );
    await deletePermission(perm.name);
    ElMessage.success("删除成功");
    loadData();
  } catch (e) {
    if (e !== "cancel") ElMessage.error("删除失败");
  }
}

function openTest() {
  testResult.value = null;
  testDialogVisible.value = true;
}

async function handleTest() {
  if (!testForm.value.role || !testForm.value.path) {
    ElMessage.warning("请填写角色和路径");
    return;
  }
  testing.value = true;
  try {
    const res = await testEnforce(
      testForm.value.role,
      testForm.value.path,
      testForm.value.method
    );
    testResult.value = res.data?.allowed ?? false;
  } catch {
    ElMessage.error("测试失败");
  } finally {
    testing.value = false;
  }
}

onMounted(loadData);
</script>

<template>
  <div class="p-4">
    <div class="mb-4 flex items-center justify-between">
      <h2 class="text-lg font-semibold">权限管理</h2>
      <div>
        <el-button @click="openTest">权限测试</el-button>
        <el-button type="primary" @click="handleAdd">新增权限</el-button>
      </div>
    </div>

    <el-card shadow="never">
      <el-table v-loading="loading" :data="permissions" border stripe>
        <el-table-column prop="name" label="权限标识" width="180" />
        <el-table-column prop="displayName" label="显示名" width="140" />
        <el-table-column label="资源 (API路径)" min-width="250">
          <template #default="{ row }">
            <el-tag
              v-for="r in row.resources"
              :key="r"
              size="small"
              type="info"
              class="mr-1 mb-1"
              >{{ r }}</el-tag
            >
          </template>
        </el-table-column>
        <el-table-column label="操作方法" width="200">
          <template #default="{ row }">
            <el-tag
              v-for="a in row.actions"
              :key="a"
              size="small"
              class="mr-1"
              >{{ a }}</el-tag
            >
          </template>
        </el-table-column>
        <el-table-column label="关联角色" width="180">
          <template #default="{ row }">
            <el-tag
              v-for="role in row.roles"
              :key="role"
              size="small"
              type="warning"
              class="mr-1"
              >{{ role }}</el-tag
            >
            <span v-if="!row.roles?.length" class="text-gray-400">未关联</span>
          </template>
        </el-table-column>
        <el-table-column label="效果" width="80" align="center">
          <template #default="{ row }">
            <el-tag
              :type="row.effect === 'Allow' ? 'success' : 'danger'"
              size="small"
              >{{ row.effect }}</el-tag
            >
          </template>
        </el-table-column>
        <el-table-column label="状态" width="70" align="center">
          <template #default="{ row }">
            <el-tag :type="row.isEnabled ? 'success' : 'info'" size="small">{{
              row.isEnabled ? "启用" : "禁用"
            }}</el-tag>
          </template>
        </el-table-column>
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

    <!-- 新增/编辑权限对话框 -->
    <el-dialog
      v-model="dialogVisible"
      :title="isEditing ? '编辑权限' : '新增权限'"
      width="600px"
    >
      <el-form :model="form" label-width="100px">
        <el-form-item label="权限标识" required>
          <el-input
            v-model="form.name"
            :disabled="isEditing"
            placeholder="英文标识，如 api-all-access"
          />
        </el-form-item>
        <el-form-item label="显示名">
          <el-input
            v-model="form.displayName"
            placeholder="中文显示名，如 全部API访问权限"
          />
        </el-form-item>
        <el-form-item label="资源路径" required>
          <div class="w-full">
            <div class="flex gap-2 mb-2">
              <el-input
                v-model="newResource"
                placeholder="输入 API 路径，如 /api/v1/*"
                @keyup.enter="addResource"
              />
              <el-button @click="addResource">添加</el-button>
            </div>
            <div>
              <el-tag
                v-for="(r, idx) in form.resources"
                :key="r"
                closable
                class="mr-1 mb-1"
                @close="removeResource(idx)"
                >{{ r }}</el-tag
              >
            </div>
          </div>
        </el-form-item>
        <el-form-item label="操作方法" required>
          <el-checkbox-group v-model="form.actions">
            <el-checkbox
              v-for="m in httpMethods"
              :key="m"
              :label="m"
              :value="m"
              >{{ m }}</el-checkbox
            >
          </el-checkbox-group>
        </el-form-item>
        <el-form-item label="关联角色">
          <el-select
            v-model="form.roles"
            multiple
            filterable
            placeholder="选择角色"
            style="width: 100%"
          >
            <el-option
              v-for="role in allRoles"
              :key="role.name"
              :label="role.displayName || role.name"
              :value="role.name"
            />
          </el-select>
        </el-form-item>
        <el-form-item v-if="isEditing" label="效果">
          <el-radio-group v-model="form.effect">
            <el-radio value="Allow">Allow</el-radio>
            <el-radio value="Deny">Deny</el-radio>
          </el-radio-group>
        </el-form-item>
        <el-form-item v-if="isEditing" label="启用">
          <el-switch v-model="form.isEnabled" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="dialogVisible = false">取消</el-button>
        <el-button type="primary" :loading="saving" @click="handleSubmit"
          >确定</el-button
        >
      </template>
    </el-dialog>

    <!-- 权限测试对话框 -->
    <el-dialog v-model="testDialogVisible" title="权限测试" width="480px">
      <el-form :model="testForm" label-width="80px">
        <el-form-item label="角色">
          <el-select
            v-model="testForm.role"
            filterable
            placeholder="选择角色"
            style="width: 100%"
          >
            <el-option
              v-for="role in allRoles"
              :key="role.name"
              :label="role.displayName || role.name"
              :value="role.name"
            />
          </el-select>
        </el-form-item>
        <el-form-item label="API 路径">
          <el-input
            v-model="testForm.path"
            placeholder="/api/v1/addons/example/items"
          />
        </el-form-item>
        <el-form-item label="方法">
          <el-select v-model="testForm.method" style="width: 100%">
            <el-option
              v-for="m in httpMethods.filter(x => x !== '*')"
              :key="m"
              :label="m"
              :value="m"
            />
          </el-select>
        </el-form-item>
        <el-form-item v-if="testResult !== null" label="结果">
          <el-tag :type="testResult ? 'success' : 'danger'" size="large">
            {{ testResult ? "✅ 允许访问" : "❌ 拒绝访问" }}
          </el-tag>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="testDialogVisible = false">关闭</el-button>
        <el-button type="primary" :loading="testing" @click="handleTest"
          >测试</el-button
        >
      </template>
    </el-dialog>
  </div>
</template>
