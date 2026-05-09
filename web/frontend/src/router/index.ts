import { createRouter, createWebHistory } from 'vue-router'
import { useAuthStore } from '../stores/auth'

const router = createRouter({
  history: createWebHistory(import.meta.env.BASE_URL),
  routes: [
    { path: '/', name: 'home', component: () => import('../views/HomeView.vue') },
    { path: '/login', name: 'login', component: () => import('../views/LoginView.vue') },
    { path: '/register', name: 'register', component: () => import('../views/RegisterView.vue') },
    {
      path: '/console',
      component: () => import('../views/ConsoleLayout.vue'),
      meta: { requiresAuth: true },
      children: [
        { path: '', name: 'console', component: () => import('../views/DashboardView.vue') },
        { path: 'keys', name: 'keys', component: () => import('../views/ApiKeysView.vue') },
        { path: 'redeem', name: 'redeem', component: () => import('../views/RedeemView.vue') },
      ],
    },
    {
      path: '/admin',
      component: () => import('../views/AdminLayout.vue'),
      meta: { requiresAuth: true, requiresAdmin: true },
      children: [
        { path: '', name: 'admin-dash', component: () => import('../views/AdminDashboard.vue') },
        { path: 'users', name: 'admin-users', component: () => import('../views/AdminUsers.vue') },
        { path: 'channels', name: 'admin-channels', component: () => import('../views/AdminChannels.vue') },
        { path: 'logs', name: 'admin-logs', component: () => import('../views/AdminLogs.vue') },
        { path: 'model-market', name: 'admin-model-market', component: () => import('../views/AdminModelMarket.vue') },
      ],
    },
  ],
})

router.beforeEach((to) => {
  const auth = useAuthStore()
  if (to.meta.requiresAuth && !auth.isLoggedIn) {
    return { name: 'login', query: { redirect: to.fullPath } }
  }
  if (to.meta.requiresAdmin && !['admin', 'superadmin'].includes(auth.role)) {
    return { name: 'home' }
  }
  return true
})

export default router
