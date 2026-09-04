<script setup>
import { computed, onMounted, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { adminApi } from './api.js'

const route = useRoute()
const router = useRouter()

const currentAdmin = ref(null)
const sessionChecked = ref(false)
const loginLoading = ref(false)
const loginError = ref('')

const adminName = computed(() => currentAdmin.value?.nickname || currentAdmin.value?.phone || '管理员')
const adminAccount = computed(() => currentAdmin.value?.phone || '')
const pageTitle = computed(() => route.path === '/functions' ? '功能管理' : '用户管理')

async function syncRoute() {
  if (!sessionChecked.value) return

  if (currentAdmin.value && (route.path === '/' || route.path === '/login')) {
    await router.replace('/users')
    return
  }

  if (!currentAdmin.value && route.meta.requiresAuth) {
    await router.replace('/login')
  }
}

watch(
  [() => route.path, () => route.meta.requiresAuth, sessionChecked, currentAdmin],
  () => { void syncRoute() },
  { immediate: true },
)

onMounted(async () => {
  try {
    const response = await adminApi.me()
    currentAdmin.value = response?.user || null
  } catch {
    currentAdmin.value = null
  } finally {
    sessionChecked.value = true
  }
})

async function handleLogin(credentials) {
  if (loginLoading.value) return

  const username = credentials?.username?.trim() || ''
  const password = credentials?.password || ''
  if (!username || !password) {
    loginError.value = '请输入管理员账号和密码'
    return
  }

  loginLoading.value = true
  loginError.value = ''
  try {
    await adminApi.login(username, password)
    const response = await adminApi.me()
    currentAdmin.value = response?.user || null
    if (!currentAdmin.value) throw new Error('管理员会话无效')
    sessionChecked.value = true
    await router.replace('/users')
  } catch (error) {
    currentAdmin.value = null
    sessionChecked.value = true
    loginError.value = error?.message || '管理员登录失败'
  } finally {
    loginLoading.value = false
  }
}

async function handleLogout() {
  try {
    await adminApi.logout()
  } catch {
    // Logout is best effort; local state must be cleared regardless.
  } finally {
    currentAdmin.value = null
    sessionChecked.value = true
    loginError.value = ''
    await router.replace('/login')
  }
}
</script>

<template>
  <div v-if="!sessionChecked" class="session-loading" role="status">
    正在检查管理员会话…
  </div>

  <RouterView
    v-else-if="!currentAdmin"
    v-slot="{ Component }"
  >
    <component
      :is="Component"
      :loading="loginLoading"
      :error="loginError"
      @submit="handleLogin"
    />
  </RouterView>

  <div v-else class="admin-shell">
    <aside class="admin-sidebar">
      <div class="admin-brand">
        <span class="admin-brand-mark">盘</span>
        <span>
          <strong>盘达</strong>
          <small>权限管理</small>
        </span>
      </div>

      <nav class="admin-nav" aria-label="后台导航">
        <RouterLink to="/users" class="admin-nav-link" active-class="is-active">
          <span class="admin-nav-icon">U</span>
          用户管理
        </RouterLink>
        <RouterLink to="/functions" class="admin-nav-link" active-class="is-active">
          <span class="admin-nav-icon">F</span>
          功能管理
        </RouterLink>
      </nav>

      <div class="admin-sidebar-footer">管理员工作台</div>
    </aside>

    <div class="admin-main">
      <header class="admin-topbar">
        <div>
          <p class="admin-topbar-kicker">盘达权限管理</p>
          <h1>{{ pageTitle }}</h1>
        </div>
        <div class="admin-identity">
          <div class="admin-identity-copy">
            <strong>{{ adminName }}</strong>
            <span v-if="adminAccount">{{ adminAccount }}</span>
          </div>
          <button type="button" data-action="logout" class="admin-logout" @click="handleLogout">
            退出
          </button>
        </div>
      </header>

      <main class="admin-content">
        <RouterView />
      </main>
    </div>
  </div>
</template>
