<script setup>
import { onMounted, ref } from 'vue'
import { adminApi } from '../api.js'

const singleDeviceLogin = ref(false)
const loaded = ref(false)
const loading = ref(false)
const saving = ref(false)
const errorMessage = ref('')
const savedMessage = ref('')

function applySettings(settings) {
  if (typeof settings?.singleDeviceLogin !== 'boolean') throw new Error('无法读取开关状态，请重试')
  singleDeviceLogin.value = settings.singleDeviceLogin
}

async function loadSettings() {
  loading.value = true
  loaded.value = false
  errorMessage.value = ''
  try {
    applySettings(await adminApi.getSystemSettings())
    loaded.value = true
  } catch (error) {
    errorMessage.value = error?.message || '加载系统设置失败'
  } finally {
    loading.value = false
  }
}

async function toggleSingleDeviceLogin() {
  if (!loaded.value || saving.value) return
  saving.value = true
  errorMessage.value = ''
  savedMessage.value = ''
  try {
    applySettings(await adminApi.updateSystemSettings(!singleDeviceLogin.value))
    savedMessage.value = '已保存，设置已生效'
  } catch (error) {
    errorMessage.value = error?.message || '保存设置失败，请重试'
  } finally {
    saving.value = false
  }
}

onMounted(loadSettings)
</script>

<template>
  <section class="management-view system-settings-view">
    <div class="view-heading">
      <div>
        <p class="view-eyebrow">系统配置</p>
        <h2>系统设置</h2>
        <p>管理系统功能，修改后自动保存并即时生效。</p>
      </div>
    </div>

    <section class="table-card table-card-primary" aria-labelledby="feature-switches-title">
      <div class="table-toolbar table-toolbar-static">
        <h3 id="feature-switches-title" class="toolbar-title">功能开关</h3>
      </div>
      <div class="settings-row">
        <div class="setting-copy">
          <h4>单点登录</h4>
          <p>开启后，同一账号仅保留最新登录的设备，其他设备会退出登录。</p>
          <p>关闭后，允许同一账号在多个设备同时登录。默认关闭。</p>
        </div>
        <div class="setting-control">
          <span class="setting-state">{{ loading ? '加载中…' : saving ? '保存中…' : !loaded ? '未加载' : singleDeviceLogin ? '已开启' : '已关闭' }}</span>
          <button
            type="button"
            role="switch"
            aria-label="单点登录"
            :aria-checked="singleDeviceLogin"
            :aria-busy="saving"
            :disabled="!loaded || loading || saving"
            class="setting-switch"
            :class="{ 'is-on': singleDeviceLogin }"
            @click="toggleSingleDeviceLogin"
          ><span aria-hidden="true"></span></button>
        </div>
      </div>
      <div v-if="errorMessage || savedMessage" class="settings-feedback">
        <p v-if="errorMessage" class="page-error" role="alert">{{ errorMessage }}</p>
        <p v-else class="settings-success" role="status">{{ savedMessage }}</p>
        <button v-if="!loaded && !loading" type="button" class="table-action" data-action="retry-settings" @click="loadSettings">重新加载</button>
      </div>
    </section>
  </section>
</template>

<style scoped>
.toolbar-title { margin: 0; }
.settings-row { display: flex; align-items: center; justify-content: space-between; gap: 24px; padding: 30px 28px; }
.setting-copy h4 { margin: 0 0 10px; font-size: 16px; font-weight: 600; }
.setting-copy p { margin: 4px 0 0; color: var(--muted); font-size: 13px; line-height: 1.8; }
.setting-control { display: flex; flex-shrink: 0; align-items: center; gap: 14px; }
.setting-state { color: var(--muted); font-size: 13px; }
.setting-switch { width: 48px; height: 28px; border-radius: 999px; padding: 3px; background: #d2d2d7; cursor: pointer; transition: background 160ms ease; }
.setting-switch span { display: block; width: 22px; height: 22px; border-radius: 50%; background: #fff; box-shadow: 0 1px 4px rgba(0,0,0,.16); transition: transform 160ms ease; }
.setting-switch.is-on { background: var(--blue); }
.setting-switch.is-on span { transform: translateX(20px); }
.settings-feedback { display: flex; align-items: center; gap: 16px; padding: 0 28px 24px; }
.settings-success { margin: 0; color: var(--green); font-size: 13px; }
@media (max-width: 640px) {
  .settings-row { align-items: flex-start; flex-direction: column; padding: 24px 20px; }
  .setting-control { align-self: flex-end; }
  .settings-feedback { padding: 0 20px 20px; }
}
</style>
