<script setup lang="ts">
import { onMounted, ref, watch } from 'vue'
import { useUiStore } from '@/stores/ui'
import { useAuthStore } from '@/stores/auth'
import { useMetaStore } from '@/stores/meta'
import { authApi } from '@/api'
import WatermarkOverlay from '@/components/WatermarkOverlay.vue'
import PunishmentGate from '@/components/PunishmentGate.vue'

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

onMounted(async () => {
  ui.apply()
  window.matchMedia('(prefers-color-scheme: dark)').addEventListener('change', ui.apply)
  await meta.fetchSite()
  await refreshMe()
  // 页面权限清单先于路由渲染加载，避免误拦
  await meta.fetchRoutes()
  ready.value = true
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
  <WatermarkOverlay />
</template>
