import { flushPromises, mount } from '@vue/test-utils'
import { afterEach, expect, test, vi } from 'vitest'
import { createMemoryHistory } from 'vue-router'
import App from '../src/App.vue'
import { adminApi } from '../src/api.js'
import { createAdminRouter } from '../src/router.js'

const mountedApps = []

afterEach(() => {
  mountedApps.splice(0).forEach((wrapper) => wrapper.unmount())
  vi.restoreAllMocks()
})

function mountApp() {
  const router = createAdminRouter(createMemoryHistory())
  router.push('/')
  const wrapper = mount(App, { global: { plugins: [router] } })
  mountedApps.push(wrapper)
  return { router, wrapper }
}

test('unauthenticated app renders login without default credentials', async () => {
  vi.spyOn(adminApi, 'me').mockRejectedValue({
    status: 401,
    code: 'ADMIN_UNAUTHENTICATED',
  })

  const { router, wrapper } = mountApp()
  await router.isReady()
  await flushPromises()

  expect(router.currentRoute.value.path).toBe('/login')
  expect(wrapper.text()).toContain('后台管理')
  expect(wrapper.text()).toContain('登录')
  expect(wrapper.text()).not.toContain('admin/admin')
  expect(wrapper.get('input[name="username"]').element.value).toBe('')
  expect(wrapper.get('input[name="password"]').element.value).toBe('')
  expect(localStorage.getItem('adminAccessToken')).toBeNull()
})

test('login establishes the admin from me after login and navigates to users', async () => {
  const me = vi.spyOn(adminApi, 'me')
    .mockRejectedValueOnce({ status: 401, code: 'ADMIN_UNAUTHENTICATED' })
    .mockResolvedValueOnce({
      user: {
        id: 'admin-a',
        phone: '13900000000',
        nickname: '值班管理员',
        role: 'admin',
        status: 'active',
      },
    })
  const login = vi.spyOn(adminApi, 'login').mockResolvedValue({ status: 'ok' })

  const { router, wrapper } = mountApp()
  await router.isReady()
  await flushPromises()

  await wrapper.get('input[name="username"]').setValue('13900000000')
  await wrapper.get('input[name="password"]').setValue('secret123')
  await wrapper.get('form').trigger('submit')
  await flushPromises()

  expect(login).toHaveBeenCalledWith('13900000000', 'secret123')
  expect(me).toHaveBeenCalledTimes(2)
  expect(login.mock.invocationCallOrder[0]).toBeLessThan(me.mock.invocationCallOrder[1])
  expect(router.currentRoute.value.path).toBe('/users')
  expect(wrapper.text()).toContain('用户管理')
  expect(wrapper.text()).toContain('功能管理')
  expect(wrapper.text()).toContain('值班管理员')
  expect(wrapper.text()).toContain('13900000000')
  expect(localStorage.getItem('adminAccessToken')).toBeNull()
})

test('protected routes redirect to login when the session is missing', async () => {
  vi.spyOn(adminApi, 'me').mockRejectedValue({
    status: 401,
    code: 'ADMIN_UNAUTHENTICATED',
  })

  const { router, wrapper } = mountApp()
  await router.isReady()
  await flushPromises()
  await router.push('/functions')
  await flushPromises()

  expect(router.currentRoute.value.path).toBe('/login')
  expect(wrapper.text()).toContain('登录')
})

test('logout clears the in-memory admin and navigates even when the request fails', async () => {
  vi.spyOn(adminApi, 'me').mockResolvedValue({
    user: {
      id: 'admin-a',
      phone: '13900000000',
      nickname: '值班管理员',
      role: 'admin',
      status: 'active',
    },
  })
  const logout = vi.spyOn(adminApi, 'logout').mockRejectedValue(new Error('network down'))

  const { router, wrapper } = mountApp()
  await router.isReady()
  await flushPromises()
  expect(router.currentRoute.value.path).toBe('/users')

  await wrapper.get('[data-action="logout"]').trigger('click')
  await flushPromises()

  expect(logout).toHaveBeenCalledTimes(1)
  expect(router.currentRoute.value.path).toBe('/login')
  expect(wrapper.text()).toContain('登录')
  expect(wrapper.text()).not.toContain('值班管理员')
})
