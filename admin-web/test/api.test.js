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

test('api client encodes JSON bodies and maps batch access requests', async () => {
  global.fetch = vi.fn().mockResolvedValue(new Response(
    JSON.stringify({ status: 'ok' }),
    { status: 200, headers: { 'Content-Type': 'application/json' } },
  ))

  await adminApi.replaceAccess(['user-a'], ['radar.main_strategy'])

  const [path, request] = fetch.mock.calls[0]
  expect(path).toBe('/api/admin/access')
  expect(request.method).toBe('PUT')
  expect(request.credentials).toBe('include')
  expect(request.headers.get('Content-Type')).toBe('application/json')
  expect(request.body).toBe(JSON.stringify({
    userIds: ['user-a'],
    moduleCodes: ['radar.main_strategy'],
  }))
})
