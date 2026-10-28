<script setup lang="ts">
import { onMounted, onBeforeUnmount, ref, watch } from 'vue'
import { useUiStore } from '@/stores/ui'
import { useAuthStore } from '@/stores/auth'
import { useMetaStore } from '@/stores/meta'
import { authApi } from '@/api'
import WatermarkOverlay from '@/components/WatermarkOverlay.vue'
import PunishmentGate from '@/components/PunishmentGate.vue'
import ProfileCompleteGate from '@/components/ProfileCompleteGate.vue'

const ui = useUiStore()
const auth = useAuthStore()
const meta = useMetaStore()
const ready = ref(false)

async function refreshMe() {
  if (!auth.isLoggedIn) return
  try {
    const u = await authApi.me()
    auth.setUser(u)
    meta.ensureAgreement()
  } catch {
    /* 错误已在拦截器处理 */
  }
}

// 站点配置（含「启用点歌」等模块开关）定时刷新，确保后台改动实时生效
let siteTimer: any = null

onMounted(async () => {
  ui.apply()
  window.matchMedia('(prefers-color-scheme: dark)').addEventListener('change', ui.apply)
  await meta.fetchSite()
  await refreshMe()
  // 页面权限清单先于路由渲染加载，避免误拦
  await meta.fetchRoutes()
  ready.value = true
  // 每 60s 同步一次站点开关（meta/site 带 ETag，未变更时为 304，开销极小）
  siteTimer = setInterval(() => meta.fetchSite(), 60000)
})

onBeforeUnmount(() => {
  if (siteTimer) clearInterval(siteTimer)
})

watch(
  () => auth.token,
  async () => {
    await refreshMe()
    await meta.fetchRoutes()
  },
)
</script>

<template>
  <router-view v-if="ready" />
  <PunishmentGate />
  <ProfileCompleteGate />
  <WatermarkOverlay />
</template>
