import { flushPromises, mount } from '@vue/test-utils'
import { afterEach, expect, test, vi } from 'vitest'
import { createMemoryHistory } from 'vue-router'
import App from '../src/App.vue'
import { createAdminRouter } from '../src/router.js'

const mounted = []
afterEach(() => {
  mounted.splice(0).forEach((wrapper) => wrapper.unmount())
  vi.unstubAllGlobals()
})

function json(body, status = 200) {
  return new Response(JSON.stringify(body), { status, headers: { 'Content-Type': 'application/json' } })
}

async function settingsApp(onSettings) {
  vi.stubGlobal('fetch', vi.fn(async (path, options) => {
    if (path === '/api/admin/me') return json({ user: { id: 'admin-a', nickname: '管理员', role: 'admin' } })
    if (path === '/api/admin/settings') return onSettings(options)
    return json({ items: [], total: 0, modules: [] })
  }))
  const router = createAdminRouter(createMemoryHistory())
  await router.push('/settings')
  const wrapper = mount(App, { global: { plugins: [router] } })
  mounted.push(wrapper)
  await router.isReady()
  await flushPromises()
  return wrapper
}

test('system settings navigation shows the default off switch and saves both directions', async () => {
  let enabled = false
  const updates = []
  const wrapper = await settingsApp(async (options) => {
    if (options.method === 'PATCH') {
      enabled = JSON.parse(options.body).singleDeviceLogin
      updates.push(enabled)
    }
    return json({ singleDeviceLogin: enabled })
  })
  expect(wrapper.text()).toContain('系统设置')
  expect(wrapper.text()).toContain('功能开关')
  const toggle = wrapper.get('[role="switch"][aria-label="单点登录"]')
  expect(toggle.attributes('aria-checked')).toBe('false')
  await toggle.trigger('click')
  await flushPromises()
  expect(toggle.attributes('aria-checked')).toBe('true')
  expect(wrapper.get('[role="status"]').text()).toContain('已保存')
  await toggle.trigger('click')
  await flushPromises()
  expect(toggle.attributes('aria-checked')).toBe('false')
  expect(updates).toEqual([true, false])
})

test('saving disables the switch and a failure keeps the last saved value', async () => {
  let rejectSave
  const wrapper = await settingsApp((options) => options.method === 'PATCH'
    ? new Promise((_, reject) => { rejectSave = reject })
    : json({ singleDeviceLogin: false }))
  const toggle = wrapper.get('[role="switch"]')
  await toggle.trigger('click')
  expect(toggle.element.disabled).toBe(true)
  rejectSave(new Error('保存失败，请重试'))
  await flushPromises()
  expect(toggle.attributes('aria-checked')).toBe('false')
  expect(toggle.element.disabled).toBe(false)
  expect(wrapper.get('[role="alert"]').text()).toContain('保存失败')
})

test('a failed load can be retried without allowing an unknown setting to be overwritten', async () => {
  let attempts = 0
  const wrapper = await settingsApp(() => ++attempts === 1
    ? json({ message: '加载设置失败' }, 500)
    : json({ singleDeviceLogin: true }))
  expect(wrapper.get('[role="switch"]').element.disabled).toBe(true)
  expect(wrapper.get('[role="alert"]').text()).toContain('加载设置失败')
  await wrapper.get('[data-action="retry-settings"]').trigger('click')
  await flushPromises()
  expect(wrapper.get('[role="switch"]').attributes('aria-checked')).toBe('true')
  expect(wrapper.get('[role="switch"]').element.disabled).toBe(false)
})
