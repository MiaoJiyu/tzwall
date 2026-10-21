import { createRouter, createWebHistory } from 'vue-router'
import { useAuthStore } from '@/stores/auth'
import { useMetaStore } from '@/stores/meta'

const routes = [
  {
    path: '/',
    component: () => import('@/layouts/PublicLayout.vue'),
    children: [
      { path: '', name: 'home', component: () => import('@/views/Home.vue'), meta: { page: 'page:home' } },
      { path: 'submission/:id', name: 'submission', component: () => import('@/views/SubmissionDetail.vue'), meta: { page: 'page:submission' } },
      { path: 'login', name: 'login', component: () => import('@/views/Login.vue'), meta: { page: 'page:login' } },
      { path: 'register', name: 'register', component: () => import('@/views/Register.vue'), meta: { page: 'page:register' } },
      { path: 'verify', name: 'verify', component: () => import('@/views/VerifyEmail.vue'), meta: { page: 'page:verify' } },
      { path: 'forgot', name: 'forgot', component: () => import('@/views/ForgotPassword.vue'), meta: { page: 'page:forgot' } },
      { path: 'reset', name: 'reset', component: () => import('@/views/ResetPassword.vue'), meta: { page: 'page:reset' } },
      { path: 'publish', name: 'publish', component: () => import('@/views/Publish.vue'), meta: { requireAuth: true, page: 'page:publish' } },
      { path: 'my', name: 'my', component: () => import('@/views/MySubmissions.vue'), meta: { requireAuth: true, page: 'page:my' } },
      { path: 'profile', name: 'profile', component: () => import('@/views/Profile.vue'), meta: { requireAuth: true, page: 'page:profile' } },
      { path: 'agreement', name: 'agreement', component: () => import('@/views/Agreement.vue'), meta: { requireAuth: true, page: 'page:agreement' } },
      { path: 'my/punishments', name: 'my-punishments', component: () => import('@/views/MyPunishments.vue'), meta: { requireAuth: true, page: 'page:my.punishments' } },
    ],
  },
  {
    path: '/admin',
    component: () => import('@/layouts/AdminLayout.vue'),
    meta: { requireAuth: true, admin: true },
    children: [
      { path: '', redirect: '/admin/dashboard' },
      { path: 'dashboard', name: 'admin-dashboard', component: () => import('@/views/admin/Dashboard.vue'), meta: { page: 'page:admin.dashboard' } },
      { path: 'users', name: 'admin-users', component: () => import('@/views/admin/Users.vue'), meta: { page: 'page:admin.users' } },
      { path: 'submissions', name: 'admin-submissions', component: () => import('@/views/admin/Submissions.vue'), meta: { page: 'page:admin.submissions' } },
      { path: 'all-submissions', name: 'admin-all-submissions', component: () => import('@/views/admin/AllSubmissions.vue'), meta: { page: 'page:admin.allsubmissions' } },
      { path: 'tags', name: 'admin-tags', component: () => import('@/views/admin/Tags.vue'), meta: { page: 'page:admin.tags' } },
      { path: 'reports', name: 'admin-reports', component: () => import('@/views/admin/Reports.vue'), meta: { page: 'page:admin.reports' } },
      { path: 'punishments', name: 'admin-punishments', component: () => import('@/views/admin/Punishments.vue'), meta: { page: 'page:admin.punishments' } },
      { path: 'permissions', name: 'admin-permissions', component: () => import('@/views/admin/Permissions.vue'), meta: { page: 'page:admin.permissions' } },
      { path: 'configs', name: 'admin-configs', component: () => import('@/views/admin/Configs.vue'), meta: { page: 'page:admin.configs' } },
      { path: 'logs/access', name: 'admin-logs-access', component: () => import('@/views/admin/LogsAccess.vue'), meta: { page: 'page:admin.logs.access' } },
      { path: 'logs/audit', name: 'admin-logs-audit', component: () => import('@/views/admin/LogsAudit.vue'), meta: { page: 'page:admin.logs.audit' } },
      { path: 'ops', name: 'admin-ops', component: () => import('@/views/admin/Ops.vue'), meta: { page: 'page:admin.ops' } },
    ],
  },
  { path: '/403', name: 'forbidden', component: () => import('@/views/Forbidden.vue') },
  { path: '/:pathMatch(.*)*', name: 'notfound', component: () => import('@/views/NotFound.vue') },
]

const router = createRouter({ history: createWebHistory(), routes })

router.beforeEach((to) => {
  const auth = useAuthStore()
  const meta = useMetaStore()
  const needAuth = to.matched.some((r) => r.meta.requireAuth)
  if (needAuth && !auth.isLoggedIn) {
    return { path: '/login', query: { redirect: to.fullPath } }
  }
  const needAdmin = to.matched.some((r) => r.meta.admin)
  if (needAdmin && !auth.isAdmin) {
    return { path: '/403' }
  }
  // 页面权限：后端下发的可访问页面清单
  const page = to.meta.page as string | undefined
  if (page && !meta.can(page)) {
    return { path: '/403' }
  }
  return true
})

export default router
