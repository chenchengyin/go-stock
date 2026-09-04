<script setup>
import { onMounted, ref } from 'vue'
import { adminApi } from '../api.js'

const modules = ref([])
const loading = ref(false)
const errorMessage = ref('')
const expandedModuleCode = ref('')
const usersByModule = ref(new Map())
const lookupLoadingCode = ref('')
const lookupError = ref('')
let lookupRequestId = 0

function isPublic(module) {
  return module.accessMode === 'public'
}

function moduleCount(module) {
  return module.authorizedUserCount ?? 0
}

function accountOf(user) {
  return user.phone || user.account || user.username || '—'
}

function nicknameOf(user) {
  return user.nickname || '—'
}

function statusOf(status) {
  if (status === 'active') return '启用'
  if (status === 'disabled') return '禁用'
  return status || '—'
}

function usersOf(moduleCode) {
  return usersByModule.value.get(moduleCode) || []
}

async function loadModules() {
  loading.value = true
  errorMessage.value = ''
  try {
    const response = await adminApi.listModules()
    modules.value = [...(response?.modules || [])].sort((left, right) => {
      return (left.sort || 0) - (right.sort || 0)
    })
  } catch (error) {
    errorMessage.value = error?.message || '加载功能目录失败'
  } finally {
    loading.value = false
  }
}

async function viewModuleUsers(module) {
  if (isPublic(module)) return

  expandedModuleCode.value = module.code
  lookupError.value = ''
  const requestId = ++lookupRequestId
  lookupLoadingCode.value = module.code
  try {
    const response = await adminApi.listModuleUsers(module.code)
    if (requestId !== lookupRequestId) return
    const items = Array.isArray(response?.items)
      ? response.items
      : (Array.isArray(response?.users) ? response.users : [])
    const nextUsers = new Map(usersByModule.value)
    nextUsers.set(module.code, items)
    usersByModule.value = nextUsers
  } catch (error) {
    if (requestId === lookupRequestId) {
      lookupError.value = error?.message || '加载授权用户失败'
    }
  } finally {
    if (requestId === lookupRequestId) lookupLoadingCode.value = ''
  }
}

onMounted(() => { void loadModules() })
</script>

<template>
  <section class="management-view function-management-view">
    <div class="view-heading">
      <div>
        <p class="view-eyebrow">模块目录与授权反查</p>
        <h2>功能管理</h2>
        <p>查看客户端已注册模块和受控模块的授权用户；权限编辑仍在用户管理中完成。</p>
      </div>
      <div class="view-summary">
        <strong>{{ modules.length }}</strong>
        <span>个模块</span>
      </div>
    </div>

    <p v-if="errorMessage" class="page-error" role="alert">{{ errorMessage }}</p>

    <div class="table-card">
      <table class="data-table">
        <thead>
          <tr>
            <th>模块名称</th>
            <th>模块编码</th>
            <th>客户端</th>
            <th>位置</th>
            <th>权限策略</th>
            <th>授权人数</th>
            <th>操作</th>
          </tr>
        </thead>
        <tbody>
          <tr v-if="loading">
            <td colspan="7" class="table-state">正在加载功能目录…</td>
          </tr>
          <tr v-else-if="modules.length === 0">
            <td colspan="7" class="table-state">暂无功能模块</td>
          </tr>
          <template v-for="module in modules" v-else :key="module.code">
            <tr :data-module-code="module.code">
              <td>{{ module.name || '—' }}</td>
              <td><code>{{ module.code }}</code></td>
              <td>{{ module.client || '—' }}</td>
              <td>{{ module.placement || '—' }}</td>
              <td>
                <span v-if="isPublic(module)" class="module-lock" data-role="public-lock">公开模块 · 锁定</span>
                <span v-else class="module-policy" data-role="controlled-policy">用户授权模块</span>
              </td>
              <td>{{ moduleCount(module) }}</td>
              <td>
                <button
                  v-if="!isPublic(module)"
                  type="button"
                  class="table-action"
                  data-action="view-module-users"
                  :disabled="lookupLoadingCode === module.code"
                  @click="viewModuleUsers(module)"
                >
                  {{ lookupLoadingCode === module.code ? '加载中…' : '查看用户' }}
                </button>
                <span v-else>—</span>
              </td>
            </tr>
            <tr v-if="expandedModuleCode === module.code" class="module-users-row">
              <td colspan="7">
                <section class="authorized-users" data-role="authorized-users">
                  <strong>{{ module.name }} · 已授权用户</strong>
                  <p v-if="lookupLoadingCode === module.code" class="table-state">正在加载授权用户…</p>
                  <p v-else-if="lookupError" class="page-error" role="alert">{{ lookupError }}</p>
                  <table v-else-if="usersOf(module.code).length" class="data-table">
                    <thead>
                      <tr>
                        <th>账号</th>
                        <th>昵称</th>
                        <th>状态</th>
                      </tr>
                    </thead>
                    <tbody>
                      <tr v-for="user in usersOf(module.code)" :key="user.id || accountOf(user)">
                        <td>{{ accountOf(user) }}</td>
                        <td>{{ nicknameOf(user) }}</td>
                        <td><span class="status-badge" :class="`status-${user.status}`">{{ statusOf(user.status) }}</span></td>
                      </tr>
                    </tbody>
                  </table>
                  <p v-else class="table-state">暂无已授权用户</p>
                </section>
              </td>
            </tr>
          </template>
        </tbody>
      </table>
    </div>
  </section>
</template>
