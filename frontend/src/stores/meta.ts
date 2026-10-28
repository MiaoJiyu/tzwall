import { defineStore } from 'pinia'
import { ref } from 'vue'
import http from '@/api/http'
import { useAuthStore } from './auth'

export const useMetaStore = defineStore('meta', () => {
  const site = ref<any>({})
  const agreements = ref<any[]>([])
  const routes = ref<any[]>([])
  const allowedPages = ref<string[]>([])
  const loading = ref(false)
  // 记录最近一次「页面权限清单」所对应的 token（空串表示游客）。
  // 用于在登录/登出后避免用旧（游客）清单误判页面权限。
  const loadedToken = ref<string | null>(null)

  async function fetchSite() {
    try {
      site.value = await http.get('/meta/site')
    } catch {
      site.value = {}
    }
  }

  // 页面权限清单由后端维护下发，前端守卫据此判定
  async function fetchRoutes() {
    try {
      const d = await http.get('/meta/routes')
      routes.value = d.pages || []
      allowedPages.value = d.allowed || []
    } catch {
      routes.value = []
      allowedPages.value = []
    }
    // 记录本次清单对应的登录态（token 变化即视为不同用户/游客）
    const auth = useAuthStore()
    loadedToken.value = auth.token || ''
  }

  // ensureRoutes 确保当前登录态对应的页面权限清单已加载：
  // 登录/登出后 token 变化，守卫判定前必须先按新 token 重新拉取，
  // 否则会用「游客态」清单误判已登录页面为无权限（跳 /403）。
  // 仅 token 变化时才真正请求，正常导航不重复拉取。
  async function ensureRoutes() {
    const auth = useAuthStore()
    const t = auth.token || ''
    if (loadedToken.value === t) return
    await fetchRoutes()
  }

  async function fetchAgreements() {
    const data = await http.get('/meta/agreements')
    agreements.value = data.items || []
  }

  async function ensureAgreement() {
    const auth = useAuthStore()
    if (!auth.isLoggedIn) return
    // 已在协议确认页则不再跳转，避免整页刷新造成无限重复跳转
    if (location.pathname.startsWith('/agreement')) return
    await fetchAgreements()
    const latest = agreements.value[0]
    if (latest && auth.user?.agreement_version && auth.user.agreement_version !== latest.version) {
      location.assign('/agreement')
    }
  }

  function can(pageKey: string) {
    if (!allowedPages.value.length) return true // 清单未加载时不误拦
    return allowedPages.value.includes(pageKey)
  }

  return { site, agreements, routes, allowedPages, loading, fetchSite, fetchRoutes, ensureRoutes, fetchAgreements, ensureAgreement, can }
})
