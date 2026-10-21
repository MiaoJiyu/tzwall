<script setup lang="ts">
import { onMounted, onUnmounted, ref, watch } from 'vue'
import { useAuthStore } from '@/stores/auth'

const auth = useAuthStore()
const root = ref<HTMLElement | null>(null)
let observer: MutationObserver | null = null
let timer: number | null = null

function tileText(): string {
  const name = auth.isLoggedIn ? auth.user?.username || '游客' : '游客'
  const t = new Date().toLocaleString('zh-CN', { hour12: false })
  return `${name} | ${t}`
}

// 用原生 DOM 填充，绕过 Vue 管理，便于防篡改重建
function build() {
  const el = root.value
  if (!el) return
  el.innerHTML = ''
  const inner = document.createElement('div')
  inner.className = 'wm-inner'
  const text = tileText()
  for (let i = 0; i < 220; i++) {
    const s = document.createElement('span')
    s.textContent = text
    inner.appendChild(s)
  }
  el.appendChild(inner)
}

onMounted(() => {
  build()
  observer = new MutationObserver(() => {
    if (root.value && root.value.childElementCount === 0) build()
  })
  if (root.value) observer.observe(root.value, { childList: true })
  // 每分钟更新时间水印
  timer = window.setInterval(build, 60_000)
})

// 登录/登出或用户信息变化时立即重建水印，无需刷新页面
watch(
  () => [auth.isLoggedIn, auth.user?.username],
  () => build(),
)

onUnmounted(() => {
  observer?.disconnect()
  if (timer) clearInterval(timer)
})
</script>

<template>
  <div class="watermark-layer" ref="root"></div>
</template>

<style scoped>
.watermark-layer :deep(.wm-inner) {
  position: absolute;
  top: -20%;
  left: -20%;
  width: 140%;
  height: 140%;
  transform: rotate(-28deg);
  display: grid;
  grid-template-columns: repeat(11, 1fr);
  gap: 28px 64px;
  pointer-events: none;
}
.watermark-layer :deep(.wm-inner span) {
  font-size: 14px;
  font-weight: 600;
  color: var(--text);
  white-space: nowrap;
  user-select: none;
}
</style>
