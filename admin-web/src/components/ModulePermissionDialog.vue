<script setup>
import { computed, ref, watch } from 'vue'
import { adminApi } from '../api.js'
import { moduleCheckState, selectedControlledCodes } from '../permission-selection.js'

const props = defineProps({
  open: { type: Boolean, default: false },
  userIds: { type: Array, default: () => [] },
})

const emit = defineEmits(['close', 'saved'])

const modules = ref([])
const accessByUser = ref([])
const selectedCodes = ref(new Set())
const selectionOverrides = ref(new Map())
const loading = ref(false)
const saving = ref(false)
const loadError = ref('')
const saveError = ref('')
const requestNumber = ref(0)

const orderedUserIds = computed(() => [...new Set(props.userIds || [])])

function moduleState(code) {
  if (selectionOverrides.value.has(code)) {
    return selectionOverrides.value.get(code) ? 'checked' : 'unchecked'
  }
  return moduleCheckState(accessByUser.value, code)
}

function isPublic(module) {
  return module.accessMode === 'public'
}

function syncCheckboxState(element, code) {
  if (element) element.indeterminate = moduleState(code) === 'indeterminate'
}

async function loadPermissionState() {
  if (!props.open || orderedUserIds.value.length === 0) return

  const currentRequest = requestNumber.value + 1
  requestNumber.value = currentRequest
  loading.value = true
  loadError.value = ''
  saveError.value = ''
  try {
    const [moduleResponse, accessResponse] = await Promise.all([
      adminApi.listModules(),
      adminApi.getAccess(orderedUserIds.value),
    ])
    if (currentRequest !== requestNumber.value) return

    modules.value = [...(moduleResponse?.modules || [])].sort((left, right) => {
      return (left.sort || 0) - (right.sort || 0)
    })
    accessByUser.value = accessResponse?.users || []
    selectionOverrides.value = new Map()
    selectedCodes.value = new Set(
      modules.value
        .filter((module) => module.accessMode === 'user_allowlist')
        .filter((module) => moduleState(module.code) === 'checked')
        .map((module) => module.code),
    )
  } catch (error) {
    if (currentRequest === requestNumber.value) {
      loadError.value = error?.message || '加载模块权限失败'
    }
  } finally {
    if (currentRequest === requestNumber.value) loading.value = false
  }
}

watch(
  [() => props.open, orderedUserIds],
  () => { void loadPermissionState() },
  { immediate: true },
)

function toggleModule(event, module) {
  event.preventDefault()
  if (isPublic(module) || loading.value || saving.value) return

  const next = new Set(selectedCodes.value)
  const nextChecked = moduleState(module.code) !== 'checked'
  const overrides = new Map(selectionOverrides.value)
  overrides.set(module.code, nextChecked)
  selectionOverrides.value = overrides
  if (!nextChecked) {
    next.delete(module.code)
  } else {
    next.add(module.code)
  }
  selectedCodes.value = next
}

async function save() {
  if (saving.value || loading.value || orderedUserIds.value.length === 0) return

  saving.value = true
  saveError.value = ''
  const moduleCodes = selectedControlledCodes(modules.value, selectedCodes.value)
  try {
    await adminApi.replaceAccess(orderedUserIds.value, moduleCodes)
    emit('saved', { userIds: orderedUserIds.value, moduleCodes })
    emit('close')
  } catch (error) {
    saveError.value = error?.message || '保存模块权限失败'
  } finally {
    saving.value = false
  }
}

function close() {
  if (!saving.value) emit('close')
}
</script>

<template>
  <div v-if="open" class="permission-dialog-backdrop" role="presentation" @click.self="close">
    <section class="permission-dialog" data-role="permission-sheet" role="dialog" aria-modal="true" aria-labelledby="permission-dialog-title">
      <header class="permission-dialog-header">
        <div>
          <p class="view-eyebrow">批量权限配置</p>
          <h2 id="permission-dialog-title">配置模块权限</h2>
          <p>已选择 {{ orderedUserIds.length }} 位用户；只覆盖受控模块权限。</p>
        </div>
        <button type="button" class="dialog-close" aria-label="关闭" @click="close">×</button>
      </header>

      <div v-if="loading" class="dialog-state" role="status">正在加载模块权限…</div>
      <div v-if="loadError" class="dialog-error" role="alert">{{ loadError }}</div>

      <div v-if="!loading && !loadError" class="module-permission-list">
        <div
          v-for="module in modules"
          :key="module.code"
          class="module-permission-row"
          :class="{ 'is-checked': moduleState(module.code) === 'checked' }"
          :data-module-code="module.code"
        >
          <div class="module-permission-copy">
            <strong>{{ module.name }}</strong>
            <code>{{ module.code }}</code>
          </div>
          <span v-if="isPublic(module)" class="module-lock" data-role="public-lock">公开模块 · 锁定</span>
          <span v-else class="module-policy">用户授权</span>
          <input
            type="checkbox"
            :checked="moduleState(module.code) === 'checked'"
            :disabled="isPublic(module) || saving"
            :aria-label="'授权 ' + module.name"
            :data-state="moduleState(module.code)"
            :data-role="isPublic(module) ? 'public-lock' : 'module-checkbox'"
            :ref="(element) => syncCheckboxState(element, module.code)"
            @click="toggleModule($event, module)"
          >
        </div>
      </div>

      <div v-if="saveError" class="dialog-error" role="alert">{{ saveError }}</div>

      <footer class="permission-dialog-footer" data-role="permission-actions">
        <span class="permission-hint">公开模块始终对已登录用户可见</span>
        <div class="dialog-actions">
          <button type="button" class="button-secondary" :disabled="saving" @click="close">取消</button>
          <button
            type="button"
            class="button-primary"
            data-action="save-permissions"
            :disabled="loading || saving || orderedUserIds.length === 0"
            @click="save"
          >
            {{ saving ? '保存中…' : '完成' }}
          </button>
        </div>
      </footer>
    </section>
  </div>
</template>
