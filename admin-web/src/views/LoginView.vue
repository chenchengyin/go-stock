<script setup>
import { ref } from 'vue'

defineProps({
  loading: { type: Boolean, default: false },
  error: { type: String, default: '' },
})

const emit = defineEmits(['submit'])
const username = ref('')
const password = ref('')

function submitLogin() {
  emit('submit', { username: username.value, password: password.value })
}
</script>

<template>
  <main class="login-shell">
    <section class="login-card" aria-labelledby="login-title">
      <div class="login-brand">
        <span class="login-brand-mark">盘</span>
        <span>盘达权限管理</span>
      </div>
      <p class="login-kicker">ADMIN CONSOLE</p>
      <h1 id="login-title">后台管理</h1>
      <p class="login-description">请使用管理员账号登录管理工作台</p>

      <form class="login-form" @submit.prevent="submitLogin">
        <label class="login-field">
          <span>账号</span>
          <input
            v-model="username"
            name="username"
            type="text"
            autocomplete="username"
            placeholder="请输入管理员账号"
          >
        </label>
        <label class="login-field">
          <span>密码</span>
          <input
            v-model="password"
            name="password"
            type="password"
            autocomplete="current-password"
            placeholder="请输入管理员密码"
          >
        </label>
        <p v-if="error" class="login-error" role="alert">{{ error }}</p>
        <button type="submit" class="login-submit" :disabled="loading">
          {{ loading ? '登录中…' : '登录' }}
        </button>
      </form>
    </section>
  </main>
</template>
