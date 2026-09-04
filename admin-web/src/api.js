export class AdminApiError extends Error {
  constructor({ code, message, status }) {
    super(message || '管理请求失败')
    this.name = 'AdminApiError'
    this.code = code || 'HTTP_ERROR'
    this.message = message || '管理请求失败'
    this.status = status || 0
  }
}

async function decodeResponse(response) {
  const text = await response.text()
  if (!text) return null
  try {
    return JSON.parse(text)
  } catch {
    return { message: text }
  }
}

export async function apiRequest(path, options = {}) {
  const { body, headers, ...rest } = options
  const requestHeaders = new Headers(headers)
  const request = {
    ...rest,
    credentials: 'include',
    headers: requestHeaders,
  }

  if (body !== undefined && body !== null) {
    requestHeaders.set('Content-Type', 'application/json')
    request.body = typeof body === 'string' ? body : JSON.stringify(body)
  }

  const response = await fetch(path, request)
  const payload = await decodeResponse(response)
  if (!response.ok) {
    throw new AdminApiError({
      code: payload?.code || `HTTP_${response.status}`,
      message: payload?.message || `管理请求失败（${response.status}）`,
      status: response.status,
    })
  }
  return payload
}

const query = (params) => {
  const search = new URLSearchParams(params)
  const value = search.toString()
  return value ? `?${value}` : ''
}

export const adminApi = {
  login(username, password) {
    return apiRequest('/api/admin/login', {
      method: 'POST',
      body: { username, password },
    })
  },
  logout() {
    return apiRequest('/api/admin/logout', { method: 'POST' })
  },
  me() {
    return apiRequest('/api/admin/me')
  },
  listUsers(keyword = '') {
    return apiRequest(`/api/admin/users${query(keyword.trim() ? { keyword: keyword.trim() } : {})}`)
  },
  updateUserStatus(userId, status) {
    return apiRequest(`/api/admin/users/${encodeURIComponent(userId)}/status`, {
      method: 'PATCH',
      body: { status },
    })
  },
  listModules() {
    return apiRequest('/api/admin/modules')
  },
  getAccess(userIds) {
    return apiRequest(`/api/admin/access${query({ user_ids: userIds.join(',') })}`)
  },
  replaceAccess(userIds, moduleCodes) {
    return apiRequest('/api/admin/access', {
      method: 'PUT',
      body: { userIds, moduleCodes },
    })
  },
  listModuleUsers(moduleCode) {
    return apiRequest(`/api/admin/modules/${encodeURIComponent(moduleCode)}/users`)
  },
}
