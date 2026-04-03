<template>
  <Transition name="fade">
    <div
      v-if="showButton"
      :class="['edge-trigger', { 'edge-trigger-busy': hasBackgroundTask }]"
      @click="togglePanel"
      :title="hasBackgroundTask ? 'go-stock AI Agent 助手正在后台分析...' : 'go-stock AI Agent 助手'"
    >
      <div class="edge-trigger-inner">
        <NIcon :component="SparklesOutline" size="22" />
        <div v-if="hasBackgroundTask" class="edge-trigger-badge" />
      </div>
    </div>
  </Transition>

  <Transition name="drawer-slide">
    <div v-if="panelVisible" class="drawer-wrap">
      <div class="drawer-mask" @click="closePanel" />
      <div class="drawer-panel" @click.stop>
        <NCard
          size="small"
          class="panel-card"
          :bordered="false"
          content-style="padding: 0; display: flex; flex-direction: column; height: 100%;"
        >
          <template #header>
            <div class="panel-header">
              <span class="panel-title">go-stock AI Agent 助手</span>
              <div class="panel-actions">
                <NButton size="small" quaternary @click="startNewChat" title="开始新对话">
                  新对话
                </NButton>
                <NButton quaternary circle size="small" title="分享到社区" :loading="shareLoading" @click="shareAiToCommunity">
                  <template #icon>
                    <NIcon :component="ShareSocialOutline" />
                  </template>
                </NButton>
                <NButton quaternary circle size="small" title="关闭" @click="closePanel">
                  <template #icon>
                    <NIcon :component="CloseOutline" />
                  </template>
                </NButton>
              </div>
            </div>
          </template>

          <div class="chat-body">
            <div v-if="shareTipVisible" class="share-tip">
              <div class="share-tip-text">{{ shareTipText }}</div>
              <NButton size="tiny" quaternary class="share-tip-close" @click="shareTipVisible = false">关闭</NButton>
            </div>
            <NScrollbar ref="scrollbarRef" class="chat-scroll">
              <div class="message-list">
                <div
                  v-for="(group, groupIndex) in messageGroups"
                  :key="group.id"
                  class="message-group"
                >
                  <div class="message-group-header" @click="toggleGroup(groupIndex)">
                    <div class="message-group-summary">
                      <NIcon :component="isGroupExpanded(groupIndex) ? ChevronDownOutline : ChevronForwardOutline" size="16" />
                      <span class="message-group-title">{{ group.userMsg.content.slice(0, 50) }}{{ group.userMsg.content.length > 50 ? '...' : '' }}</span>
                      <span class="message-group-time">{{ group.userMsg.time }}</span>
                    </div>
                  </div>
                  <div v-show="isGroupExpanded(groupIndex)" class="message-group-content">
                    <div
                      :class="['message-item', group.userMsg.role]"
                    >
                      <div class="msg-bubble">
                        <div class="msg-content">
                          <div v-if="group.userMsg.time" class="msg-meta msg-meta-user-inner">
                            <span class="msg-time">{{ group.userMsg.time }}</span>
                          </div>
                          <MdPreview
                            :theme="theme"
                            :style="{ textAlign: 'right' }"
                            v-if="group.userMsg.content"
                            :model-value="group.userMsg.content"
                            :editor-id="'agent-msg-' + group.userIndex"
                            class="msg-markdown"
                          />
                        </div>
                      </div>
                      <div class="msg-avatar user-avatar">
                        <NIcon :component="PersonCircleOutline" size="20" />
                      </div>
                    </div>
                    <div
                      v-if="group.assistantMsg"
                      :class="['message-item', 'assistant']"
                    >
                      <div class="msg-avatar assistant-avatar">
                        <NIcon :component="SparklesOutline" size="20" />
                      </div>
                      <div class="msg-bubble">
                        <div class="msg-content">
                          <MdPreview
                            :theme="theme"
                            :style="{ textAlign: 'left' }"
                            :model-value="group.assistantMsg.content || '...'"
                            :editor-id="'agent-msg-' + group.assistantIndex"
                            class="msg-markdown"
                          />
                          <div v-if="isStreamLoad && groupIndex === messageGroups.length - 1 && !group.assistantMsg.content" class="msg-loading">
                            <NSpin size="small" />
                            <span>思考中...</span>
                          </div>
                          <div class="msg-bubble-actions">
                            <div v-if="group.assistantMsg.modelName || group.assistantMsg.time" class="msg-meta-row-assistant">
                              <span v-if="group.assistantMsg.modelName" class="msg-model-name" :title="group.assistantMsg.modelName">{{ group.assistantMsg.modelName }}</span>
                              <span v-if="group.assistantMsg.time" class="msg-time">{{ group.assistantMsg.time }}</span>
                            </div>
                            <NButton quaternary size="tiny" class="msg-toggle-btn" @click="toggleGroup(groupIndex)">
                              <template #icon>
                                <NIcon :component="isGroupExpanded(groupIndex) ? ChevronUpOutline : ChevronDownOutline" />
                              </template>
                              {{ isGroupExpanded(groupIndex) ? '收起' : '展开' }}
                            </NButton>
                            <NButton quaternary size="tiny" class="msg-copy-btn" @click="copyAiContent(group.assistantMsg)">
                              <template #icon>
                                <NIcon :component="CopyOutline" />
                              </template>
                              复制
                            </NButton>
                            <NButton
                              quaternary
                              size="tiny"
                              class="msg-export-img-btn"
                              :loading="exportImageKey === String(group.assistantIndex)"
                              title="导出为图片"
                              @click="exportAiReplyImage(group.assistantIndex, $event)"
                            >
                              <template #icon>
                                <NIcon :component="ImageOutline" />
                              </template>
                              导出图
                            </NButton>
                            <NButton quaternary size="tiny" class="msg-share-btn" :loading="shareLoading" @click="shareAiContent(group.assistantMsg)">
                              <template #icon>
                                <NIcon :component="ShareSocialOutline" />
                              </template>
                              分享
                            </NButton>
                          </div>
                        </div>
                      </div>
                    </div>
                  </div>
                </div>
              </div>
            </NScrollbar>

            <div class="chat-footer">
              <div class="chat-footer-row">
                <NSelect
                  v-model:value="aiConfigId"
                  :options="aiConfigOptions"
                  size="small"
                  filterable
                  to="body"
                  placement="top-start"
                  placeholder="选择模型"
                  :consistent-menu-width="false"
                  :menu-props="{ style: { zIndex: 10002 } }"
                  class="chat-footer-select"
                />
                <NSelect
                  v-model:value="sysPromptId"
                  :options="sysPromptOptions"
                  size="small"
                  clearable
                  to="body"
                  placement="top-start"
                  placeholder="系统提示词"
                  :consistent-menu-width="false"
                  :menu-props="{ style: { zIndex: 10002 } }"
                  class="chat-footer-prompt"
                />
                <NSelect
                  v-model:value="userPromptId"
                  :options="userPromptOptions"
                  size="small"
                  clearable
                  to="body"
                  placement="top-start"
                  placeholder="用户提示词"
                  :consistent-menu-width="false"
                  :menu-props="{ style: { zIndex: 10002 } }"
                  class="chat-footer-prompt"
                  @update:value="onUserPromptChange"
                />
                <div class="chat-footer-thinking">
                  <span class="chat-footer-thinking-label">思考模式</span>
                  <NSwitch v-model:value="thinkingMode" size="small" />
                </div>
                <div class="chat-footer-memory">
                  <span class="chat-footer-thinking-label">记忆模式</span>
                  <NSwitch v-model:value="memoryMode" size="small" />
                  <NSelect
                    v-if="memoryMode"
                    v-model:value="memoryCount"
                    :options="memoryCountOptions"
                    size="small"
                    :consistent-menu-width="false"
                    to="body"
                    placement="top-start"
                    :menu-props="{ style: { zIndex: 10002 } }"
                    class="chat-footer-memory-count"
                  />
                </div>
              </div>
              <div class="chat-footer-input">
                <NInput
                  v-model:value="inputValue"
                  type="textarea"
                  placeholder="输入消息，回车发送..."
                  :autosize="{ minRows: 2, maxRows: 4 }"
                  :disabled="isStreamLoad"
                  @keydown.enter.exact.prevent="sendMessage"
                />
                <NButton
                  v-if="isStreamLoad"
                  type="warning"
                  quaternary
                  class="chat-footer-abort"
                  @click="abortStream(true)"
                >
                  中断
                </NButton>
                <NButton
                  type="primary"
                  :loading="isStreamLoad"
                  :disabled="isStreamLoad || !canSend"
                  @click="sendMessage"
                >
                  发送
                </NButton>
              </div>
            </div>
          </div>
        </NCard>
      </div>
    </div>
  </Transition>
</template>

<script setup>
import { ref, computed, watch, nextTick, onMounted, onBeforeUnmount, onBeforeMount } from 'vue'
import { useRoute } from 'vue-router'
import { NButton, NCard, NIcon, NInput, NScrollbar, NSelect, NSpin, NSwitch, useMessage } from 'naive-ui'
import {
  CloseOutline,
  SparklesOutline,
  PersonCircleOutline,
  CopyOutline,
  ShareSocialOutline,
  ImageOutline,
  ChevronDownOutline,
  ChevronForwardOutline,
  ChevronUpOutline
} from '@vicons/ionicons5'
import {
  ChatWithAgent,
  GetAiConfigs,
  GetConfig,
  GetPromptTemplates,
  GetSponsorInfo,
  SaveAiAssistantSession,
  GetAiAssistantSession,
  ShareText,
  AbortChatWithAgent,
  SaveAIResponseResult
} from '../../wailsjs/go/main/App'
import { EventsOff, EventsOn } from '../../wailsjs/runtime'
import { MdPreview } from 'md-editor-v3'
import 'md-editor-v3/lib/preview.css'
import html2canvas from 'html2canvas'

const STORAGE_KEY_MODEL_ID = 'go-stock-agent-last-model-id'

const route = useRoute()
const message = useMessage()

const showButton = computed(() => route.name !== 'agent')

const panelVisible = ref(false)
const inputValue = ref('')
const isStreamLoad = ref(false)
const sentFromFloating = ref(false)
const messages = ref([])
const sessionId = ref('')
const aiConfigOptions = ref([])
const aiConfigId = ref(null)

function modelLabelForConfig(configId) {
  const opts = aiConfigOptions.value
  if (!opts?.length) return ''
  const id = configId != null ? Number(configId) : Number(opts[0].value)
  const found = opts.find(o => Number(o.value) === id)
  return found?.label != null ? String(found.label) : ''
}

const sysPromptTemplates = ref([])
const sysPromptOptions = computed(() =>
  sysPromptTemplates.value.map(t => ({ label: t.name ?? '', value: t.ID ?? t.id }))
)
const sysPromptId = ref(null)

const userPromptTemplates = ref([])
const userPromptOptions = computed(() =>
  userPromptTemplates.value.map(t => ({ label: t.name ?? '', value: t.ID ?? t.id }))
)
const userPromptId = ref(null)
const thinkingMode = ref(false)
const memoryMode = ref(true)
const memoryCount = ref(3)
const memoryCountOptions = [
  { label: '1 条', value: 1 },
  { label: '2 条', value: 2 },
  { label: '3 条', value: 3 },
  { label: '4 条', value: 4 },
  { label: '5 条', value: 5 },
  { label: '10 条', value: 10 },
]

function onUserPromptChange(id) {
  if (!id) return
  const t = userPromptTemplates.value.find(x => (x.ID ?? x.id) === id)
  if (t?.content) inputValue.value = t.content
}

const canSend = computed(() => !!inputValue.value.trim())
const scrollbarRef = ref(null)
const darkTheme = ref(false)
const shareLoading = ref(false)
const exportImageKey = ref('')
const shareTipVisible = ref(false)
const shareTipText = ref('')
const vipLevel = ref(0)
const vipLoaded = ref(false)
const vipLoading = ref(false)
const isAborted = ref(false)
const expandedGroups = ref(new Set())

const hasBackgroundTask = computed(() => isStreamLoad.value && sentFromFloating.value && !panelVisible.value)
const AGENT_EVENT = 'agent-message'

const messageGroups = computed(() => {
  const groups = []
  let currentGroup = null
  
  for (let i = 0; i < messages.value.length; i++) {
    const msg = messages.value[i]
    if (msg.role === 'user') {
      if (currentGroup) {
        groups.push(currentGroup)
      }
      currentGroup = {
        id: i,
        userMsg: msg,
        userIndex: i,
        assistantMsg: null,
        assistantIndex: -1
      }
    } else if (msg.role === 'assistant' && currentGroup) {
      currentGroup.assistantMsg = msg
      currentGroup.assistantIndex = i
    }
  }
  if (currentGroup) {
    groups.push(currentGroup)
  }
  return groups
})

function isGroupExpanded(groupIndex) {
  return expandedGroups.value.has(groupIndex)
}

function toggleGroup(groupIndex) {
  const newSet = new Set(expandedGroups.value)
  if (newSet.has(groupIndex)) {
    newSet.delete(groupIndex)
  } else {
    newSet.add(groupIndex)
  }
  expandedGroups.value = newSet
}

function initDefaultExpanded() {
  if (messageGroups.value.length > 0 && expandedGroups.value.size === 0) {
    expandedGroups.value = new Set([messageGroups.value.length - 1])
  }
}

function ensureLatestGroupExpanded() {
  if (messageGroups.value.length > 0) {
    const lastIndex = messageGroups.value.length - 1
    const newSet = new Set(expandedGroups.value)
    newSet.add(lastIndex)
    expandedGroups.value = newSet
  }
}

async function copyAiContent(msg) {
  const text = (msg?.content ?? '').trim()
  if (!text) {
    message.warning('暂无可复制的 AI 正文内容')
    return
  }
  try {
    if (navigator && navigator.clipboard && navigator.clipboard.writeText) {
      await navigator.clipboard.writeText(text)
      message.success('已复制 AI 回答内容')
    } else {
      const textarea = document.createElement('textarea')
      textarea.value = text
      textarea.style.position = 'fixed'
      textarea.style.opacity = '0'
      document.body.appendChild(textarea)
      textarea.select()
      document.execCommand('copy')
      document.body.removeChild(textarea)
      message.success('已复制 AI 回答内容')
    }
  } catch (e) {
    message.error('复制失败，请手动选择文本')
  }
}

function shareTextToCommunity(text, title) {
  if (shareLoading.value) return
  shareLoading.value = true
  shareTipText.value = '正在分享到社区...'
  shareTipVisible.value = true
  ShareText(text, title)
    .then((msg) => {
      shareTipText.value = msg
      shareTipVisible.value = true
    })
    .catch((err) => {
      shareTipText.value = '分享失败: ' + (err?.message ?? err)
      shareTipVisible.value = true
    })
    .finally(() => {
      shareLoading.value = false
    })
}

function shareAiContent(msg) {
  const text = (msg?.content ?? '').trim()
  if (!text) {
    shareTipText.value = '暂无可分享的 AI 正文内容'
    shareTipVisible.value = true
    return
  }
  shareTextToCommunity(text, 'go-stock AI Agent助手')
}

function getLastAssistantContent() {
  for (let i = messages.value.length - 1; i >= 0; i--) {
    const m = messages.value[i]
    if (m?.role === 'assistant') {
      const text = (m?.content ?? '').trim()
      if (text) return text
    }
  }
  return ''
}

function shareAiToCommunity() {
  const text = getLastAssistantContent()
  if (!text) {
    shareTipText.value = '暂无可分享的 AI 回复内容'
    shareTipVisible.value = true
    return
  }
  shareTextToCommunity(text, 'go-stock AI Agent助手')
}

async function exportAiReplyImage(assistantIndex, evt) {
  const msg = messages.value[assistantIndex]
  if (msg?.role !== 'assistant') return
  if (!(msg.content ?? '').trim()) {
    shareTipText.value = '暂无可导出的 AI 回答内容'
    shareTipVisible.value = true
    return
  }
  const editorId = 'agent-msg-' + assistantIndex
  const bubble = evt?.currentTarget?.closest?.('.msg-bubble')
  const key = String(assistantIndex)
  if (exportImageKey.value) return
  exportImageKey.value = key
  await nextTick()
  try {
    const target = document.getElementById(`${editorId}-preview-wrapper`) ||
      document.getElementById(`${editorId}-preview`) ||
      bubble?.querySelector('.md-editor-preview') ||
      null
    if (!target) {
      shareTipText.value = '未找到预览区域，请展开回答后重试'
      shareTipVisible.value = true
      return
    }
    const canvas = await html2canvas(target, {
      useCORS: true,
      scale: 2,
      allowTaint: true,
      logging: false,
      backgroundColor: darkTheme.value ? '#1e1e1e' : '#ffffff'
    })
    const link = document.createElement('a')
    const safeTime = new Date().toISOString().slice(0, 19).replace(/[:.]/g, '-')
    link.href = canvas.toDataURL('image/png')
    link.download = `go-stock-agent-${safeTime}.png`
    link.click()
    shareTipText.value = '已导出为 PNG 图片'
    shareTipVisible.value = true
  } catch (e) {
    shareTipText.value = '导出图片失败: ' + (e?.message ?? e)
    shareTipVisible.value = true
  } finally {
    exportImageKey.value = ''
  }
}

function abortStream(showTip = true) {
  if (!isStreamLoad.value) return
  isAborted.value = true
  isStreamLoad.value = false
  if (showTip) {
    shareTipText.value = '已中断本次 AI 回答'
    shareTipVisible.value = true
  }
  AbortChatWithAgent()
}

const theme = computed(() => (darkTheme.value ? 'dark' : 'light'))

async function loadHistory() {
  try {
    const resp = await GetAiAssistantSession('')
    if (resp?.sessionId) {
      sessionId.value = resp.sessionId
    }
    const list = resp?.messages
    if (Array.isArray(list) && list.length > 0) {
      messages.value = list.map(m => ({
        role: m.role ?? '',
        content: m.content ?? '',
        time: m.time ?? '',
        modelName: m.modelName ?? ''
      }))
      nextTick(() => {
        initDefaultExpanded()
      })
    }
  } catch (_) {
  }
}

function saveHistory() {
  if (messages.value.length === 0) return
  const list = messages.value.map(m => ({
    role: m.role,
    content: m.content,
    time: m.time ?? '',
    modelName: m.modelName ?? ''
  }))
  SaveAiAssistantSession(sessionId.value, list).catch(() => {})
}

function openPanel() {
  panelVisible.value = true
  if (!sessionId.value) {
    sessionId.value = Date.now().toString()
  }
  if (messages.value.length === 0) {
    messages.value = [
      {
        role: 'assistant',
        content: '我是 go-stock AI Agent 助手，可以帮您分析股票、查询市场数据、获取研究报告等。请问有什么可以帮您的？',
        time: new Date().toLocaleString(),
        modelName: ''
      }
    ]
  }
  nextTick(() => {
    initDefaultExpanded()
    scrollToBottom()
  })
}

function closePanel() {
  panelVisible.value = false
}

async function ensureVipInfo() {
  if (vipLoaded.value || vipLoading.value) return
  vipLoading.value = true
  try {
    const res = await GetSponsorInfo()
    const lvl = Number(res?.vipLevel ?? 0)
    vipLevel.value = Number.isNaN(lvl) ? 0 : lvl
  } catch (_) {
    vipLevel.value = 0
  } finally {
    vipLoaded.value = true
    vipLoading.value = false
  }
}

async function togglePanel() {
  if (!panelVisible.value) {
    await ensureVipInfo()
    if ((vipLevel.value ?? 0) < 2) {
      message.warning('go-stock AI Agent 助手功能仅对 VIP2 及以上赞助用户开放，请前往关于页面查看赞助方式。')
      return
    }
    openPanel()
  } else {
    closePanel()
  }
}

function scrollToBottom() {
  nextTick(() => {
    scrollbarRef.value?.scrollTo({ top: 99999, behavior: 'smooth' })
  })
}

function sendMessage() {
  if (isStreamLoad.value) {
    abortStream(false)
  }
  const text = inputValue.value.trim()
  if (!text) {
    message.warning('请输入你的问题')
    return
  }

  messages.value.push({
    role: 'user',
    content: text,
    time: new Date().toLocaleString(),
    modelName: ''
  })
  const configId = aiConfigId.value ?? aiConfigOptions.value[0]?.value ?? 0
  const modelName = modelLabelForConfig(configId)
  messages.value.push({
    role: 'assistant',
    content: '',
    time: new Date().toLocaleString(),
    modelName
  })
  inputValue.value = ''
  isStreamLoad.value = true
  isAborted.value = false
  sentFromFloating.value = true
  saveHistory()
  nextTick(() => {
    ensureLatestGroupExpanded()
    scrollToBottom()
  })
  ChatWithAgent(text, configId, sysPromptId.value, memoryMode.value, memoryCount.value, thinkingMode.value)
}

function startNewChat() {
  if (isStreamLoad.value) {
    message.warning('当前有回答正在生成，请先中断或等待完成')
    return
  }
  messages.value = []
  sessionId.value = Date.now().toString()
}

function onAgentMessage(msg) {
  if (isAborted.value) return

  if (msg.content === 'agent-DONE' || (msg?.response_meta?.finish_reason === 'stop')) {
    isStreamLoad.value = false
    sentFromFloating.value = false
    isAborted.value = false
    saveHistory()
    nextTick(scrollToBottom)
    const last = messages.value[messages.value.length - 1]
    if (msg.content === 'agent-DONE' && last && last.role === 'assistant' && last.content) {
      const user = messages.value[messages.value.length - 2]
      SaveAIResponseResult("agent","市场分析", last.content, sessionId.value,user.content, aiConfigId.value)
    }
    return
  }

  const roleLower = String(msg?.role || '').toLowerCase()
  if (roleLower !== 'assistant') {
    return
  }

  const last = messages.value[messages.value.length - 1]
  if (last && last.role === 'assistant' && msg?.content) {
    last.content = (last.content || '') + msg.content
    nextTick(scrollToBottom)
  }
}

function loadPromptTemplates() {
  GetPromptTemplates('', '').then(res => {
    const list = Array.isArray(res) ? res : []
    sysPromptTemplates.value = list.filter(t => t.type === '模型系统Prompt')
    userPromptTemplates.value = list.filter(t => t.type === '模型用户Prompt')
  })
}

watch(panelVisible, (v) => {
  if (v) {
    loadPromptTemplates()
    nextTick(scrollToBottom)
  }
})

onBeforeMount(() => {
  GetConfig().then(result => {
    darkTheme.value = result.darkTheme
  })
})

onMounted(() => {
  EventsOn(AGENT_EVENT, onAgentMessage)
  loadHistory()
  GetAiConfigs().then(res => {
    const list = Array.isArray(res) ? res : []
    aiConfigOptions.value = list.map((c, index) => {
      const id = c.ID != null ? Number(c.ID) : (c.id != null ? Number(c.id) : index)
      const name = c.name ?? c.Name ?? ''
      const modelName = c.modelName ?? c.ModelName ?? ''
      return {
        label: name + (modelName ? ' [' + modelName + ']' : ''),
        value: id
      }
    })
    if (aiConfigOptions.value.length) {
      const lastModelId = localStorage.getItem(STORAGE_KEY_MODEL_ID)
      if (lastModelId) {
        const foundId = Number(lastModelId)
        const isValid = aiConfigOptions.value.some(opt => opt.value === foundId)
        aiConfigId.value = isValid ? foundId : aiConfigOptions.value[0].value
      } else {
        aiConfigId.value = aiConfigOptions.value[0].value
      }
    }
  })
  loadPromptTemplates()
})

watch(aiConfigId, (newId) => {
  if (newId != null) {
    localStorage.setItem(STORAGE_KEY_MODEL_ID, String(newId))
  }
})

onBeforeUnmount(() => {
  EventsOff(AGENT_EVENT)
})
</script>

<style scoped>
.edge-trigger {
  position: fixed;
  top: 50%;
  right: 0;
  z-index: 9998;
  transform: translateY(-50%);
  width: 32px;
  height: 120px;
  border-radius: 12px 0 0 12px;
  background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
  color: #fff;
  cursor: pointer;
  display: flex;
  align-items: center;
  justify-content: center;
  box-shadow: -2px 0 12px rgba(102, 126, 234, 0.4);
  transition: width 0.2s ease, box-shadow 0.2s ease;
}
.edge-trigger-busy {
  box-shadow: -4px 0 18px rgba(248, 113, 113, 0.8);
}
.edge-trigger:hover {
  width: 40px;
  box-shadow: -4px 0 16px rgba(102, 126, 234, 0.5);
}
.edge-trigger-inner {
  position: relative;
  display: flex;
  align-items: center;
  justify-content: center;
}
.edge-trigger-badge {
  position: absolute;
  top: 6px;
  left: 6px;
  width: 8px;
  height: 8px;
  border-radius: 50%;
  background: #f97316;
  box-shadow: 0 0 6px rgba(248, 113, 113, 0.9);
  animation: pulse 1.5s infinite;
}

@keyframes pulse {
  0%, 100% { opacity: 1; }
  50% { opacity: 0.5; }
}

.drawer-wrap {
  position: fixed;
  inset: 0;
  z-index: 9999;
  pointer-events: none;
}
.drawer-wrap > * {
  pointer-events: auto;
}
.drawer-mask {
  position: absolute;
  inset: 0;
  background: rgba(0, 0, 0, 0.35);
  cursor: pointer;
}
.drawer-panel {
  position: absolute;
  top: 0;
  right: 0;
  bottom: 0;
  width: 60vw;
  min-width: 320px;
  max-width: calc(100vw - 48px);
  background: var(--n-color-modal);
  box-shadow: -8px 0 24px rgba(0, 0, 0, 0.15);
  overflow: hidden;
  display: flex;
  flex-direction: column;
}

.panel-card {
  height: 100%;
  border-radius: 0;
  box-shadow: none;
  display: flex;
  flex-direction: column;
  min-height: 0;
}
.panel-card :deep(.n-card-header) {
  padding: 12px 16px;
  flex-shrink: 0;
}
.panel-card :deep(.n-card__content) {
  flex: 1;
  min-height: 0;
  display: flex;
  flex-direction: column;
  overflow: hidden;
}
.panel-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
}
.panel-actions {
  display: flex;
  align-items: center;
  gap: 6px;
}
.panel-title {
  font-weight: 600;
  font-size: 16px;
}

.chat-body {
  display: flex;
  flex-direction: column;
  flex: 1;
  min-height: 0;
  overflow: hidden;
}
.share-tip {
  flex-shrink: 0;
  margin: 10px 16px 0;
  padding: 10px 12px;
  border-radius: 10px;
  background: rgba(0, 0, 0, 0.04);
  border: 1px solid var(--n-border-color);
  display: flex;
  gap: 10px;
  align-items: flex-start;
}
.share-tip-text {
  flex: 1;
  min-width: 0;
  font-size: 13px;
  line-height: 1.5;
  white-space: pre-wrap;
  word-break: break-word;
  text-align: left;
}
.share-tip-close {
  flex-shrink: 0;
}
.chat-scroll {
  flex: 1;
  min-height: 0;
  overflow: hidden;
  display: flex;
  flex-direction: column;
}
.chat-scroll :deep(.n-scrollbar-content) {
  flex: 1;
  min-height: 0;
}
.message-list {
  padding: 12px 16px 16px;
  display: flex;
  flex-direction: column;
  gap: 16px;
}
.message-group {
  border: 1px solid var(--n-border-color);
  border-radius: 12px;
  overflow: hidden;
  background: var(--n-color-modal);
}
.message-group-header {
  padding: 10px 14px;
  cursor: pointer;
  background: rgba(0, 0, 0, 0.02);
  border-bottom: 1px solid var(--n-border-color);
  transition: background 0.2s;
}
.message-group-header:hover {
  background: rgba(0, 0, 0, 0.04);
}
.message-group-summary {
  display: flex;
  align-items: center;
  gap: 8px;
}
.message-group-title {
  flex: 1;
  font-size: 13px;
  font-weight: 500;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.message-group-time {
  font-size: 11px;
  color: var(--n-text-color-3);
  flex-shrink: 0;
}
.message-group-content {
  padding: 12px 0;
  display: flex;
  flex-direction: column;
  gap: 12px;
}
.message-group-content .message-item {
  padding: 0 14px;
}
.message-item {
  display: flex;
  gap: 10px;
  align-items: flex-start;
}
.message-item.user {
  justify-content: flex-end;
}
.msg-avatar {
  width: 32px;
  height: 32px;
  border-radius: 50%;
  display: flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
}
.assistant-avatar {
  background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
  color: #fff;
}
.user-avatar {
  background: linear-gradient(135deg, #34d399 0%, #22c55e 35%, #06b6d4 100%);
  color: #fff;
  box-shadow: 0 6px 14px rgba(34, 197, 94, 0.22);
  border: 1px solid rgba(255, 255, 255, 0.45);
}
.msg-bubble {
  max-width: 100%;
  flex: 1;
  min-width: 0;
  width: 100%;
  box-sizing: border-box;
  padding: 10px 14px;
  border-radius: 12px;
  font-size: 14px;
  line-height: 1.5;
  word-break: break-word;
  display: flex;
  flex-direction: column;
}
.message-item.assistant .msg-bubble {
  background: var(--n-color-modal);
  border: 1px solid var(--n-border-color);
}
.message-item.user .msg-bubble {
  background: var(--n-color-primary);
  color: #fff;
  text-align: right;
}
.message-item.user .msg-content,
.message-item.user .msg-content :deep(.md-editor-preview),
.message-item.user .msg-content :deep(.md-editor-preview-wrapper) {
  text-align: right;
}
.msg-content {
  white-space: normal;
  width: 100%;
  min-width: 0;
  flex: 1;
}
.msg-bubble-actions {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  justify-content: flex-end;
  align-items: center;
  margin-top: 8px;
}
.msg-meta-row-assistant {
  flex: 1 1 100%;
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 10px;
  font-size: 11px;
  color: var(--n-text-color-3);
}
.msg-meta-row-assistant .msg-time {
  flex-shrink: 0;
}
.msg-model-name {
  flex: 1;
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  text-align: left;
}
.msg-share-btn,
.msg-copy-btn,
.msg-export-img-btn,
.msg-toggle-btn {
  padding: 2px 10px;
  font-size: 12px;
  border-radius: 12px;
  color: var(--n-primary-color);
  background-color: var(--n-primary-color-suppl);
  border: 1px solid var(--n-primary-color);
  transition: color 0.2s, border-color 0.2s, background-color 0.2s;
}
.msg-share-btn:hover,
.msg-copy-btn:hover,
.msg-export-img-btn:hover,
.msg-toggle-btn:hover {
  border-color: var(--n-primary-color);
  background-color: var(--n-primary-color);
  color: #fff;
}
.message-item.user .msg-bubble .msg-share-btn,
.message-item.user .msg-bubble .msg-copy-btn,
.message-item.user .msg-bubble .msg-export-img-btn,
.message-item.user .msg-bubble .msg-toggle-btn {
  color: rgba(255, 255, 255, 0.92);
  background-color: rgba(255, 255, 255, 0.22);
  border-color: rgba(255, 255, 255, 0.65);
}
.message-item.user .msg-bubble .msg-share-btn:hover,
.message-item.user .msg-bubble .msg-copy-btn:hover,
.message-item.user .msg-bubble .msg-export-img-btn:hover,
.message-item.user .msg-bubble .msg-toggle-btn:hover {
  color: #fff;
  border-color: rgba(255, 255, 255, 0.95);
  background-color: rgba(255, 255, 255, 0.32);
}
.msg-content .msg-markdown {
  width: 100%;
  min-width: 0;
  box-sizing: border-box;
}
.msg-content .msg-markdown :deep(.md-editor-preview) {
  font-size: 13px;
  line-height: 1.6;
}
.message-item.user .msg-content :deep(.md-editor-preview),
.message-item.user .msg-content :deep(.md-editor-preview-wrapper) {
  color: inherit;
}
.msg-loading {
  display: flex;
  align-items: center;
  gap: 6px;
  margin-top: 6px;
  font-size: 12px;
  color: var(--n-text-color-3);
}

.msg-meta {
  margin-top: 4px;
  font-size: 11px;
  color: var(--n-text-color-3);
  display: flex;
}
.msg-meta-user-inner {
  justify-content: flex-end;
  margin-top: 6px;
  margin-bottom: 0;
}
.message-item.user .msg-meta-user-inner {
  color: rgba(255, 255, 255, 0.78);
}

.chat-footer {
  flex-shrink: 0;
  padding: 12px 16px 16px;
  border-top: 1px solid var(--n-border-color);
  display: flex;
  flex-direction: column;
  gap: 8px;
  background: var(--n-color-modal);
}
.chat-footer-row {
  display: flex;
  align-items: center;
  gap: 12px;
}
.chat-footer-select {
  flex: 1;
  min-width: 0;
}
.chat-footer-select .n-select {
  width: 100%;
}
.chat-footer-prompt {
  flex: 0 0 120px;
  min-width: 0;
}
.chat-footer-prompt .n-select {
  width: 100%;
}
.chat-footer-thinking {
  display: flex;
  align-items: center;
  gap: 6px;
  flex-shrink: 0;
}
.chat-footer-thinking-label {
  font-size: 12px;
  color: var(--n-text-color-2);
}
.chat-footer-memory {
  display: flex;
  align-items: center;
  gap: 6px;
  flex-shrink: 0;
}
.chat-footer-memory-count {
  width: 80px;
}
.chat-footer-memory-count .n-select {
  width: 100%;
}
.chat-footer-input {
  display: flex;
  gap: 8px;
  align-items: flex-end;
}
.chat-footer-input .n-input {
  flex: 1;
}
.chat-footer-input .n-input :deep(textarea) {
  text-align: left;
}
.chat-footer-input .n-button {
  flex-shrink: 0;
}
.chat-footer-abort {
  color: #f97316;
}

.fade-enter-active,
.fade-leave-active {
  transition: opacity 0.2s;
}
.fade-enter-from,
.fade-leave-to {
  opacity: 0;
}

.drawer-slide-enter-active .drawer-mask,
.drawer-slide-leave-active .drawer-mask {
  transition: opacity 0.25s ease;
}
.drawer-slide-enter-active .drawer-panel,
.drawer-slide-leave-active .drawer-panel {
  transition: transform 0.25s ease;
}
.drawer-slide-enter-from .drawer-mask,
.drawer-slide-leave-to .drawer-mask {
  opacity: 0;
}
.drawer-slide-enter-from .drawer-panel,
.drawer-slide-leave-to .drawer-panel {
  transform: translateX(100%);
}
.drawer-slide-enter-to .drawer-mask,
.drawer-slide-leave-from .drawer-mask {
  opacity: 1;
}
.drawer-slide-enter-to .drawer-panel,
.drawer-slide-leave-from .drawer-panel {
  transform: translateX(0);
}
</style>

<style>
body > div:has(.n-select-menu) {
  z-index: 10002 !important;
}
</style>
