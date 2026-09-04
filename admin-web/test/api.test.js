import { afterEach, expect, test, vi } from 'vitest'
import { adminApi, apiRequest } from '../src/api.js'

afterEach(() => vi.restoreAllMocks())

test('api client sends cookie credentials and never stores a token', async () => {
  global.fetch = vi.fn().mockResolvedValue(new Response(
    JSON.stringify({ user: { id: 'admin-a' } }),
    { status: 200, headers: { 'Content-Type': 'application/json' } },
  ))

  await adminApi.me()

  expect(fetch).toHaveBeenCalledWith('/api/admin/me', expect.objectContaining({
    credentials: 'include',
  }))
  expect(localStorage.getItem('adminAccessToken')).toBeNull()
})

test('api client converts an expired session into a structured error', async () => {
  global.fetch = vi.fn().mockResolvedValue(new Response(
    JSON.stringify({
      code: 'ADMIN_UNAUTHENTICATED',
      message: '管理员未登录',
    }),
    { status: 401, headers: { 'Content-Type': 'application/json' } },
  ))

  await expect(apiRequest('/api/admin/me')).rejects.toMatchObject({
    code: 'ADMIN_UNAUTHENTICATED',
    status: 401,
  })
})
