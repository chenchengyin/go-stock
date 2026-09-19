import { flushPromises, mount } from '@vue/test-utils'
import { afterEach, expect, test, vi } from 'vitest'
import UserManagementView from '../src/views/UserManagementView.vue'
import { adminApi } from '../src/api.js'

afterEach(() => vi.restoreAllMocks())

test('user management shows last used time and a placeholder when unavailable', async () => {
  vi.spyOn(adminApi, 'listUsers').mockResolvedValue({
    items: [
      {
        id: 'user-a',
        phone: '13800000000',
        nickname: 'Alice',
        role: 'user',
        status: 'active',
        createdAt: '2026-09-08T08:00:00Z',
        lastUsedAt: '2026-09-19T08:00:00Z',
      },
      {
        id: 'user-b',
        phone: '13600000000',
        nickname: 'Bob',
        role: 'user',
        status: 'disabled',
        createdAt: '2026-09-07T08:00:00Z',
        lastUsedAt: null,
      },
    ],
    total: 2,
  })

  const wrapper = mount(UserManagementView)
  await flushPromises()

  expect(wrapper.text()).toContain('上次使用')
  const rows = wrapper.findAll('tbody tr')
  expect(rows).toHaveLength(2)
  expect(rows[0].findAll('td')[6].text()).toContain('2026/9/19')
  expect(rows[1].findAll('td')[6].text()).toBe('—')
})
