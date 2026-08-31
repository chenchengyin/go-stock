<script setup>
import {computed, h, onMounted, ref} from 'vue'
import {NButton, NTag, useDialog, useMessage} from 'naive-ui'

const apiBaseUrl = 'http://localhost:8080'
const adminTokenKey = 'adminAccessToken'

const message = useMessage()
const dialog = useDialog()
const username = ref('admin')
const password = ref('admin')
const keyword = ref('')
const adminToken = ref(localStorage.getItem(adminTokenKey) || '')
const users = ref([])
const total = ref(0)
const loginLoading = ref(false)
const usersLoading = ref(false)

const loggedIn = computed(() => adminToken.value !== '')

function clearAdminSession() {
  adminToken.value = ''
  users.value = []
  total.value = 0
  localStorage.removeItem(adminTokenKey)
}

async function request(path, options = {}) {
  const headers = new Headers(options.headers || {})
  headers.set('Content-Type', 'application/json')
  if (adminToken.value) {
    headers.set('Authorization', `Bearer ${adminToken.value}`)
  }

  const response = await fetch(`${apiBaseUrl}${path}`, {...options, headers})
  let body = {}
  try {
    body = await response.json()
  } catch (_) {
    body = {}
  }

  if (response.status === 401 && path !== '/api/admin/login') {
    clearAdminSession()
  }
  if (!response.ok) {
    throw new Error(body.message || '请求失败')
  }
  return body
}

async function login() {
  if (!username.value.trim() || !password.value) {
    message.warning('请输入管理员账号和密码')
    return
  }

  loginLoading.value = true
  try {
    const result = await request('/api/admin/login', {
      method: 'POST',
      body: JSON.stringify({username: username.value, password: password.value}),
    })
    adminToken.value = result.accessToken || ''
    if (!adminToken.value) {
      throw new Error('管理员登录响应缺少 Token')
    }
    localStorage.setItem(adminTokenKey, adminToken.value)
    await loadUsers()
    message.success('登录成功')
  } catch (error) {
    clearAdminSession()
    message.error(error.message || '管理员登录失败')
  } finally {
    loginLoading.value = false
  }
}

async function loadUsers() {
  if (!loggedIn.value) {
    return
  }

  usersLoading.value = true
  try {
    const params = new URLSearchParams()
    if (keyword.value.trim()) {
      params.set('keyword', keyword.value.trim())
    }
    const query = params.toString()
    const result = await request(`/api/admin/users${query ? `?${query}` : ''}`)
    users.value = Array.isArray(result.items) ? result.items : []
    total.value = Number(result.total || 0)
  } catch (error) {
    if (loggedIn.value) {
      message.error(error.message || '用户列表加载失败')
    }
  } finally {
    usersLoading.value = false
  }
}

function searchUsers() {
  loadUsers()
}

function logout() {
  clearAdminSession()
  message.info('已退出后台')
}

function formatDate(value) {
  if (!value) {
    return '-'
  }
  const date = new Date(value)
  return Number.isNaN(date.getTime()) ? value : date.toLocaleString('zh-CN', {hour12: false})
}

function toggleStatus(row) {
  const nextStatus = row.status === 'active' ? 'disabled' : 'active'
  const actionText = nextStatus === 'disabled' ? '禁用' : '启用'
  dialog.warning({
    title: `确认${actionText}用户`,
    content: `确定要${actionText}用户“${row.nickname || row.phone}”吗？`,
    positiveText: '确定',
    negativeText: '取消',
    onPositiveClick: async () => {
      try {
        await request(`/api/admin/users/${encodeURIComponent(row.id)}/status`, {
          method: 'PATCH',
          body: JSON.stringify({status: nextStatus}),
        })
        message.success(`${actionText}成功`)
        await loadUsers()
      } catch (error) {
        message.error(error.message || `${actionText}失败`)
      }
    },
  })
}

const columns = [
  {title: '账号', key: 'phone', minWidth: 150},
  {title: '昵称', key: 'nickname', minWidth: 150},
  {title: '角色', key: 'role', width: 100},
  {
    title: '状态',
    key: 'status',
    width: 100,
    render: (row) => h(
        NTag,
        {type: row.status === 'active' ? 'success' : 'warning', bordered: false},
        {default: () => row.status === 'active' ? '启用' : '禁用'},
    ),
  },
  {
    title: '注册时间',
    key: 'createdAt',
    minWidth: 180,
    render: (row) => formatDate(row.createdAt),
  },
  {
    title: '操作',
    key: 'actions',
    width: 100,
    render: (row) => h(
        NButton,
        {
          size: 'small',
          type: row.status === 'active' ? 'warning' : 'success',
          secondary: true,
          onClick: () => toggleStatus(row),
        },
        {default: () => row.status === 'active' ? '禁用' : '启用'},
    ),
  },
]

onMounted(() => {
  if (loggedIn.value) {
    loadUsers()
  }
})
</script>

<template>
  <div class="admin-page">
    <n-card v-if="!loggedIn" class="admin-login-card" title="后台管理" size="large">
      <n-alert type="info" :show-icon="false" class="admin-login-tip">
        管理员账号：admin，密码：admin
      </n-alert>
      <n-form @submit.prevent="login">
        <n-form-item label="账号">
          <n-input v-model:value="username" placeholder="请输入管理员账号" @keyup.enter="login" />
        </n-form-item>
        <n-form-item label="密码">
          <n-input v-model:value="password" type="password" show-password-on="click" placeholder="请输入管理员密码" @keyup.enter="login" />
        </n-form-item>
        <n-button type="primary" block :loading="loginLoading" @click="login">登录</n-button>
      </n-form>
    </n-card>

    <n-card v-else title="用户管理" size="large">
      <template #header-extra>
        <n-space align="center">
          <n-tag type="info" :bordered="false">管理员</n-tag>
          <n-button quaternary size="small" @click="logout">退出</n-button>
        </n-space>
      </template>

      <n-space align="center" :wrap="true" class="admin-toolbar">
        <n-input
            v-model:value="keyword"
            clearable
            placeholder="搜索账号或昵称"
            style="width: 260px"
            @keyup.enter="searchUsers"
        />
        <n-button type="primary" @click="searchUsers">查询</n-button>
        <n-button @click="loadUsers">刷新</n-button>
      </n-space>

      <n-data-table
          :columns="columns"
          :data="users"
          :loading="usersLoading"
          :bordered="false"
          :single-line="false"
          :row-key="(row) => row.id"
      />
      <div class="admin-total">共 {{ total }} 个用户</div>
    </n-card>
  </div>
</template>

<style scoped>
.admin-page {
  min-height: calc(100vh - 180px);
  padding: 40px 5vw 100px;
}

.admin-login-card {
  max-width: 460px;
  margin: 40px auto;
}

.admin-login-tip {
  margin-bottom: 24px;
}

.admin-toolbar {
  margin-bottom: 20px;
}

.admin-total {
  margin-top: 16px;
  color: #888;
  text-align: right;
}
</style>
