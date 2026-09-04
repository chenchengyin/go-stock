import { flushPromises, mount } from '@vue/test-utils'
import { afterEach, expect, test, vi } from 'vitest'
import FunctionManagementView from '../src/views/FunctionManagementView.vue'
import { adminApi } from '../src/api.js'

const publicModule = {
  code: 'radar.monitored',
  name: '监控股票（自选）',
  client: 'flutter_web',
  placement: 'radar_tab',
  accessMode: 'public',
  authorizedUserCount: 0,
  sort: 10,
}

const controlledModule = {
  code: 'radar.main_strategy',
  name: '主板策略',
  client: 'flutter_web',
  placement: 'radar_tab',
  accessMode: 'user_allowlist',
  authorizedUserCount: 2,
  sort: 30,
}

afterEach(() => vi.restoreAllMocks())

test('function management locks public modules and shows controlled counts', async () => {
  vi.spyOn(adminApi, 'listModules').mockResolvedValue({
    modules: [publicModule, controlledModule],
  })
  const wrapper = mount(FunctionManagementView)
  await flushPromises()

  expect(wrapper.text()).toContain('公开模块')
  expect(wrapper.text()).toContain('2')
  expect(wrapper.get(
    '[data-module-code="radar.monitored"] [data-role="public-lock"]',
  ).exists()).toBe(true)
})

test('controlled modules reverse lookup renders account nickname and status', async () => {
  vi.spyOn(adminApi, 'listModules').mockResolvedValue({
    modules: [publicModule, controlledModule],
  })
  const listModuleUsers = vi.spyOn(adminApi, 'listModuleUsers').mockResolvedValue({
    items: [{
      id: 'user-a',
      phone: '13900000000',
      nickname: '主板用户',
      role: 'user',
      status: 'active',
      createdAt: '2026-09-03T08:00:00Z',
      updatedAt: '2026-09-03T08:00:00Z',
    }],
    total: 1,
  })

  const wrapper = mount(FunctionManagementView)
  await flushPromises()
  await wrapper.get('[data-module-code="radar.main_strategy"] [data-action="view-module-users"]').trigger('click')
  await flushPromises()

  expect(listModuleUsers).toHaveBeenCalledWith('radar.main_strategy')
  const authorizedUsers = wrapper.get('[data-role="authorized-users"]')
  expect(authorizedUsers.text()).toContain('13900000000')
  expect(authorizedUsers.text()).toContain('主板用户')
  expect(authorizedUsers.text()).toContain('启用')
  expect(wrapper.find('[data-module-code="radar.monitored"] [data-action="view-module-users"]').exists()).toBe(false)
})

test('module directory errors are rendered as an alert', async () => {
  vi.spyOn(adminApi, 'listModules').mockRejectedValue(new Error('模块目录暂不可用'))

  const wrapper = mount(FunctionManagementView)
  await flushPromises()

  expect(wrapper.get('[role="alert"]').text()).toContain('模块目录暂不可用')
})

test('reverse lookup errors keep the selected controlled module visible', async () => {
  vi.spyOn(adminApi, 'listModules').mockResolvedValue({ modules: [controlledModule] })
  const listModuleUsers = vi.spyOn(adminApi, 'listModuleUsers').mockRejectedValue(new Error('授权用户查询失败'))

  const wrapper = mount(FunctionManagementView)
  await flushPromises()
  await wrapper.get('[data-module-code="radar.main_strategy"] [data-action="view-module-users"]').trigger('click')
  await flushPromises()

  expect(listModuleUsers).toHaveBeenCalledWith('radar.main_strategy')
  expect(wrapper.get('[data-module-code="radar.main_strategy"]').exists()).toBe(true)
  expect(wrapper.get('[data-module-code="radar.main_strategy"] [data-action="view-module-users"]').exists()).toBe(true)
  expect(wrapper.get('[role="alert"]').text()).toContain('授权用户查询失败')
})
