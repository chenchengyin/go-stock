<script setup>
import { computed, onMounted, ref } from 'vue'
import { adminApi } from '../api.js'
import ModulePermissionDialog from '../components/ModulePermissionDialog.vue'

const users = ref([])
const total = ref(0)
const keyword = ref('')
const loading = ref(false)
const selectedIds = ref(new Set())
const permissionDialogOpen = ref(false)
const statusPendingId = ref('')
const errorMessage = ref('')

const selectedCount = computed(() => selectedIds.value.size)
const allSelected = computed(() => users.value.length > 0 && selectedCount.value === users.value.length)
const activeUserCount = computed(() => users.value.filter((user) => user.status === 'active').length)

async function loadUsers() {
  loading.value = true
  errorMessage.value = ''
  try {
    const response = await adminApi.listUsers(keyword.value)
    users.value = response?.items || []
    total.value = response?.total ?? users.value.length
    const available = new Set(users.value.map((user) => user.id))
    selectedIds.value = new Set([...selectedIds.value].filter((id) => available.has(id)))
  } catch (error) {
    errorMessage.value = error?.message || '加载用户列表失败'
  } finally {
    loading.value = false
  }
}

onMounted(() => { void loadUsers() })

function toggleUser(userId, checked) {
  const next = new Set(selectedIds.value)
  if (checked) next.add(userId)
  else next.delete(userId)
  selectedIds.value = next
}

function toggleAll(checked) {
  selectedIds.value = checked ? new Set(users.value.map((user) => user.id)) : new Set()
}

async function submitSearch() {
  await loadUsers()
}

async function toggleStatus(user) {
  if (statusPendingId.value) return
  const nextStatus = user.status === 'active' ? 'disabled' : 'active'
  statusPendingId.value = user.id
  errorMessage.value = ''
  try {
    await adminApi.updateUserStatus(user.id, nextStatus)
    await loadUsers()
  } catch (error) {
    errorMessage.value = error?.message || '更新用户状态失败'
  } finally {
    statusPendingId.value = ''
  }
}

function openPermissionDialog() {
  if (selectedCount.value > 0) permissionDialogOpen.value = true
}

async function handlePermissionsSaved() {
  await loadUsers()
}
</script>

<template>
  <section class="management-view user-management-view">
    <div class="view-heading">
      <div>
        <p class="view-eyebrow">账号与访问控制</p>
        <h2>用户管理</h2>
        <p>选择一个或多个普通用户，再一次性覆盖其受控模块权限。</p>
      </div>
      <div class="view-heading-note">
        <span class="status-dot" aria-hidden="true"></span>
        权限变更即时同步
      </div>
    </div>

    <div class="metrics-grid" data-role="user-stats">
      <article class="metric-card">
        <span>用户总数</span>
        <strong>{{ total }}</strong>
        <small>当前查询范围</small>
      </article>
      <article class="metric-card metric-card-accent">
        <span>已选择</span>
        <strong>{{ selectedCount }}</strong>
        <small>用于批量配置权限</small>
      </article>
      <article class="metric-card">
        <span>当前启用</span>
        <strong>{{ activeUserCount }}</strong>
        <small>本页已加载用户</small>
      </article>
    </div>

    <p v-if="errorMessage" class="page-error" role="alert">{{ errorMessage }}</p>

    <div class="table-card table-card-primary">
      <div class="table-toolbar" data-role="user-toolbar">
        <form class="user-search" @submit.prevent="submitSearch">
          <label class="search-field">
            <span class="search-icon" aria-hidden="true"></span>
            <span class="sr-only">搜索用户</span>
            <input v-model="keyword" type="search" placeholder="搜索账号或昵称" aria-label="搜索账号或昵称">
          </label>
          <button type="submit" class="button-secondary" :disabled="loading">搜索</button>
        </form>
        <div class="toolbar-selection">
          <span v-if="selectedCount" class="selection-count">已选择 {{ selectedCount }} 位用户</span>
          <span v-else class="selection-count is-muted">选择用户后配置模块权限</span>
          <button
            type="button"
            class="button-primary"
            data-action="configure-permissions"
            :disabled="selectedCount === 0"
            @click="openPermissionDialog"
          >
            配置模块权限<span v-if="selectedCount">（{{ selectedCount }}）</span>
          </button>
        </div>
      </div>

      <table class="data-table">
        <thead>
          <tr>
            <th><input type="checkbox" aria-label="选择全部用户" :checked="allSelected" @change="toggleAll($event.target.checked)"></th>
            <th>账号</th>
            <th>昵称</th>
            <th>角色</th>
            <th>状态</th>
            <th>注册时间</th>
            <th>操作</th>
          </tr>
        </thead>
        <tbody>
          <tr v-if="loading">
            <td colspan="7" class="table-state">正在加载用户…</td>
          </tr>
          <tr v-else-if="users.length === 0">
            <td colspan="7" class="table-state">暂无用户</td>
          </tr>
          <tr v-for="user in users" v-else :key="user.id">
            <td>
              <input
                type="checkbox"
                :aria-label="`选择 ${user.nickname || user.phone}`"
                :checked="selectedIds.has(user.id)"
                :data-user-id="user.id"
                @change="toggleUser(user.id, $event.target.checked)"
              >
            </td>
            <td>{{ user.phone }}</td>
            <td>{{ user.nickname || '—' }}</td>
            <td>{{ user.role === 'admin' ? '管理员' : '普通用户' }}</td>
            <td><span class="status-badge" :class="`status-${user.status}`">{{ user.status === 'active' ? '启用' : '禁用' }}</span></td>
            <td>{{ new Date(user.createdAt).toLocaleString('zh-CN') }}</td>
            <td>
              <button
                type="button"
                class="table-action"
                :disabled="statusPendingId === user.id"
                @click="toggleStatus(user)"
              >
                {{ statusPendingId === user.id ? '处理中…' : (user.status === 'active' ? '禁用' : '启用') }}
              </button>
            </td>
          </tr>
        </tbody>
      </table>
    </div>

    <ModulePermissionDialog
      :open="permissionDialogOpen"
      :user-ids="[...selectedIds]"
      @close="permissionDialogOpen = false"
      @saved="handlePermissionsSaved"
    />
  </section>
</template>
