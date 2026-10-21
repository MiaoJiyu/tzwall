<script setup lang="ts">
import { onMounted, ref, watch } from 'vue'
import { useRouter } from 'vue-router'
import { AlertTriangle, MessageSquareOff } from 'lucide-vue-next'
import { ElMessage } from 'element-plus'
import { punishmentApi } from '@/api'
import { useAuthStore } from '@/stores/auth'

const auth = useAuthStore()
const router = useRouter()
const snap = ref<any>(null)
const showWarn = ref(false)

async function load() {
  if (!auth.isLoggedIn) {
    snap.value = null
    return
  }
  try {
    snap.value = await punishmentApi.status()
  } catch {
    snap.value = null
    return
  }
  showWarn.value = !!snap.value?.warn
  // 强制修改密码：引导到账号安全页
  if (snap.value?.force_reset && router.currentRoute.value.path !== '/profile') {
    ElMessage.warning('请先修改密码后再继续操作')
    router.push('/profile')
  }
}

async function ack() {
  if (!snap.value?.warn) return
  await punishmentApi.ack(snap.value.warn.id)
  showWarn.value = false
  ElMessage.success('已确认警告')
  await load()
}

onMounted(load)
watch(
  () => auth.token,
  () => load(),
)
</script>

<template>
  <template v-if="auth.isLoggedIn && snap">
    <!-- 禁言横幅 -->
    <div
      v-if="snap.muted"
      class="fixed inset-x-0 top-0 z-[9990] flex flex-wrap items-center justify-center gap-1 bg-warn px-4 py-2 text-center text-sm text-white shadow-soft"
    >
      <MessageSquareOff :size="14" class="inline" />
      <span>您已被禁言，理由：{{ snap.mute_reason }}</span>
      <span v-if="snap.mute_permanent">（这是永久禁言）</span>
      <span v-else-if="snap.mute_expires">（禁言到 {{ snap.mute_expires }}）</span>
      <span class="opacity-90">— 投稿、评论、点赞、分享、举报已禁用</span>
    </div>

    <!-- 警告确认弹窗（确认后不再出现） -->
    <div v-if="showWarn && snap.warn" class="fixed inset-0 z-[9995] flex items-center justify-center bg-black/55 px-4">
      <div class="paper-card w-full max-w-md p-6 text-center animate-fadeup">
        <AlertTriangle :size="40" class="mx-auto text-[var(--warn)]" />
        <h3 class="mt-3 font-serif-sc text-lg font-bold">您收到一条警告</h3>
        <p class="mt-2 text-sm text-[var(--text-soft)]">警告时间：{{ snap.warn.created_at }}</p>
        <p class="mt-1 text-sm leading-relaxed">理由：{{ snap.warn.reason }}</p>
        <button class="btn-grape mt-5 w-full rounded-full py-2.5 text-sm font-medium" @click="ack">我已阅读并确认</button>
        <router-link to="/my/punishments" class="mt-3 block text-xs text-[var(--text-soft)] hover:text-[var(--grape)]" @click="showWarn = false">
          查看我的权限变更记录
        </router-link>
      </div>
    </div>
  </template>
</template>
