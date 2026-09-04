import { flushPromises, mount } from '@vue/test-utils'
import { afterEach, expect, test, vi } from 'vitest'
import ModulePermissionDialog from '../src/components/ModulePermissionDialog.vue'
import { adminApi } from '../src/api.js'

const modules = [
  { code: 'radar.monitored', name: '监控股票（自选）', sort: 10, accessMode: 'public' },
  { code: 'radar.purple_strategy', name: '紫策', sort: 20, accessMode: 'user_allowlist' },
  { code: 'radar.main_strategy', name: '主板策略', sort: 30, accessMode: 'user_allowlist' },
  { code: 'radar.blue_strategy', name: '蓝策', sort: 40, accessMode: 'user_allowlist' },
]

afterEach(() => vi.restoreAllMocks())

function mountDialog(access = [
  { userId: 'a', moduleCodes: ['radar.main_strategy'] },
  { userId: 'b', moduleCodes: [] },
]) {
  vi.spyOn(adminApi, 'listModules').mockResolvedValue({ modules })
  vi.spyOn(adminApi, 'getAccess').mockResolvedValue({ users: access })
  return mount(ModulePermissionDialog, {
    props: { open: true, userIds: ['a', 'b'] },
  })
}

test('loads sorted modules, locks public modules, and renders partial state', async () => {
  const wrapper = mountDialog()
  await flushPromises()

  expect(adminApi.getAccess).toHaveBeenCalledWith(['a', 'b'])
  expect(wrapper.findAll('.module-permission-row').map((row) => row.attributes('data-module-code'))).toEqual([
    'radar.monitored',
    'radar.purple_strategy',
    'radar.main_strategy',
    'radar.blue_strategy',
  ])
  const publicInput = wrapper.get('[data-module-code="radar.monitored"] input')
  expect(publicInput.element.disabled).toBe(true)
  expect(wrapper.get('[data-module-code="radar.monitored"] [data-role="public-lock"]').exists()).toBe(true)
  expect(wrapper.get('[data-module-code="radar.main_strategy"] input').attributes('data-state')).toBe('indeterminate')
})

test('a partial selection click explicitly checks the independent module', async () => {
  const wrapper = mountDialog()
  await flushPromises()

  await wrapper.get('[data-module-code="radar.main_strategy"] input').trigger('click')
  expect(wrapper.get('[data-module-code="radar.main_strategy"] input').attributes('data-state')).toBe('checked')
  expect(wrapper.get('[data-module-code="radar.purple_strategy"] input').attributes('data-state')).toBe('unchecked')
})

test('saves selected users and controlled modules, then closes', async () => {
  const replace = vi.spyOn(adminApi, 'replaceAccess').mockResolvedValue({ status: 'ok' })
  const wrapper = mountDialog([
    { userId: 'a', moduleCodes: [] },
    { userId: 'b', moduleCodes: [] },
  ])
  await flushPromises()

  await wrapper.get('[data-module-code="radar.blue_strategy"] input').trigger('click')
  await wrapper.get('[data-action="save-permissions"]').trigger('click')
  await flushPromises()

  expect(replace).toHaveBeenCalledWith(['a', 'b'], ['radar.blue_strategy'])
  expect(wrapper.emitted('saved')?.[0]?.[0]).toEqual({
    userIds: ['a', 'b'],
    moduleCodes: ['radar.blue_strategy'],
  })
  expect(wrapper.emitted('close')).toHaveLength(1)
})

test('saving an empty controlled set revokes every controlled grant', async () => {
  const replace = vi.spyOn(adminApi, 'replaceAccess').mockResolvedValue({ status: 'ok' })
  const wrapper = mountDialog([
    { userId: 'a', moduleCodes: ['radar.purple_strategy', 'radar.main_strategy'] },
    { userId: 'b', moduleCodes: ['radar.purple_strategy', 'radar.main_strategy'] },
  ])
  await flushPromises()

  await wrapper.get('[data-module-code="radar.purple_strategy"] input').trigger('click')
  await wrapper.get('[data-module-code="radar.main_strategy"] input').trigger('click')
  await wrapper.get('[data-action="save-permissions"]').trigger('click')
  await flushPromises()

  expect(replace).toHaveBeenCalledWith(['a', 'b'], [])
})

test('save errors keep the dialog open and the current selection', async () => {
  vi.spyOn(adminApi, 'replaceAccess').mockRejectedValue(new Error('保存失败'))
  const wrapper = mountDialog([
    { userId: 'a', moduleCodes: [] },
    { userId: 'b', moduleCodes: [] },
  ])
  await flushPromises()

  await wrapper.get('[data-module-code="radar.purple_strategy"] input').trigger('click')
  await wrapper.get('[data-action="save-permissions"]').trigger('click')
  await flushPromises()

  expect(wrapper.find('[role="dialog"]').exists()).toBe(true)
  expect(wrapper.get('[role="alert"]').text()).toContain('保存失败')
  expect(wrapper.get('[data-module-code="radar.purple_strategy"] input').attributes('data-state')).toBe('checked')
})
