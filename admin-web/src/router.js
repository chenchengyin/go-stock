import { h } from 'vue'
import { createRouter, createWebHistory } from 'vue-router'
import LoginView from './views/LoginView.vue'

function placeholderView(title, description) {
  return {
    name: `${title}Placeholder`,
    setup() {
      return () => h('section', { class: 'view-placeholder' }, [
        h('p', { class: 'view-eyebrow' }, '后台模块'),
        h('h1', title),
        h('p', description),
      ])
    },
  }
}

export const routes = [
  { path: '/', redirect: '/users' },
  { path: '/login', name: 'login', component: LoginView },
  {
    path: '/users',
    name: 'users',
    component: placeholderView('用户管理', '用户管理功能将在后续任务中提供。'),
    meta: { requiresAuth: true },
  },
  {
    path: '/functions',
    name: 'functions',
    component: placeholderView('功能管理', '功能目录将在后续任务中提供。'),
    meta: { requiresAuth: true },
  },
]

export function createAdminRouter(history = createWebHistory()) {
  return createRouter({ history, routes })
}

export default createAdminRouter()
