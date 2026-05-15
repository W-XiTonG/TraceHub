import { createRouter, createWebHistory } from 'vue-router'
import { useAuthStore } from '../stores/auth'

const router = createRouter({
  history: createWebHistory(import.meta.env.BASE_URL),
  routes: [
    { path: '/', name: 'home', component: () => import('../views/HomeView.vue') },
    { path: '/docs', name: 'docs', component: () => import('../views/DocsView.vue') },
    { path: '/login', name: 'login', component: () => import('../views/LoginView.vue') },
    { path: '/register', name: 'register', component: () => import('../views/RegisterView.vue') },
    {
      path: '/console',
      component: () => import('../views/ConsoleLayout.vue'),
      meta: { requiresAuth: true },
      children: [
        { path: '', name: 'console', component: () => import('../views/DashboardView.vue') },
        { path: 'models', name: 'console-models', component: () => import('../views/AvailableModelsView.vue') },
        { path: 'playground', name: 'console-playground', component: () => import('../views/PlaygroundView.vue') },
        { path: 'keys', name: 'keys', component: () => import('../views/ApiKeysView.vue') },
        { path: 'logs', name: 'console-logs', component: () => import('../views/UsageLogsView.vue') },
        { path: 'balance-ledger', name: 'console-ledger', component: () => import('../views/BalanceLedgerView.vue') },
        { path: 'group', name: 'console-group', component: () => import('../views/GroupView.vue') },
        { path: 'billing', name: 'console-billing', component: () => import('../views/BillingView.vue') },
        { path: 'settings', name: 'console-settings', component: () => import('../views/SettingsView.vue') },
        { path: 'redeem', name: 'redeem', component: () => import('../views/RedeemView.vue') },
        { path: 'feedback', name: 'console-feedback', component: () => import('../views/FeedbackView.vue') },
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
        { path: 'groups', name: 'admin-groups', component: () => import('../views/AdminGroups.vue') },
        { path: 'logs', name: 'admin-logs', component: () => import('../views/AdminLogs.vue') },
        { path: 'balance-ledger', name: 'admin-balance-ledger', component: () => import('../views/AdminBalanceLedger.vue') },
        { path: 'model-market', name: 'admin-model-market', component: () => import('../views/AdminModelMarket.vue') },
        { path: 'announcements', name: 'admin-announcements', component: () => import('../views/AdminAnnouncements.vue') },
        { path: 'redeem-codes', name: 'admin-redeem-codes', component: () => import('../views/AdminRedeemCodes.vue') },
        { path: 'redeem-ledger', name: 'admin-redeem-ledger', component: () => import('../views/AdminRedeemLedger.vue') },
        { path: 'feedbacks', name: 'admin-feedbacks', component: () => import('../views/AdminFeedbacks.vue') },
        {
          path: 'settings',
          component: () => import('../views/AdminSettingsLayout.vue'),
          redirect: { name: 'admin-settings-brand' },
          children: [
            { path: 'brand', name: 'admin-settings-brand', component: () => import('../views/AdminSettingsBrandTab.vue') },
            { path: 'public', name: 'admin-settings-public', component: () => import('../views/AdminSettingsPublicTab.vue') },
            { path: 'signup', name: 'admin-settings-signup', component: () => import('../views/AdminSettingsSignupTab.vue') },
            { path: 'no-bill', name: 'admin-settings-no-bill', component: () => import('../views/AdminSettingsNoBillTab.vue') },
            { path: 'security', name: 'admin-settings-security', component: () => import('../views/AdminSettingsSecurityTab.vue') },
            { path: 'gateway', name: 'admin-settings-gateway-ops', component: () => import('../views/AdminSettingsGatewayOpsTab.vue') },
          ],
        },
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
    return { path: '/console', query: { denied: 'admin' } }
  }
  return true
})

export default router
