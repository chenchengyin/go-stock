import { createRouter, createWebHistory } from 'vue-router'
import LoginView from './views/LoginView.vue'
import UserManagementView from './views/UserManagementView.vue'
import FunctionManagementView from './views/FunctionManagementView.vue'

export const routes = [
  { path: '/', name: 'root', component: LoginView },
  { path: '/login', name: 'login', component: LoginView },
  {
    path: '/users',
    name: 'users',
    component: UserManagementView,
    meta: { requiresAuth: true },
  },
  {
    path: '/functions',
    name: 'functions',
    component: FunctionManagementView,
    meta: { requiresAuth: true },
  },
]

export function createAdminRouter(history = createWebHistory()) {
  return createRouter({ history, routes })
}

export default createAdminRouter()
