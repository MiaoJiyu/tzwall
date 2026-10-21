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

  return { site, agreements, routes, allowedPages, loading, fetchSite, fetchRoutes, fetchAgreements, ensureAgreement, can }
})
