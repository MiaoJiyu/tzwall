<script setup lang="ts">
import { ref, watch, onBeforeUnmount } from 'vue'
import { ElMessage } from 'element-plus'
import { RefreshCw } from 'lucide-vue-next'
import { authApi } from '@/api'
import { useAuthStore } from '@/stores/auth'

const props = defineProps<{
  modelValue: boolean
  name?: string
  desc?: string
  fallbackQrcode?: string
}>()
const emit = defineEmits(['update:modelValue', 'success'])

const auth = useAuthStore()
const ticket = ref('')
const qrBase64 = ref('')
const useFallback = ref(false)
const status = ref<'loading' | 'pending' | 'expired' | 'error'>('loading')
const errMsg = ref('')

let pollTimer: any = null
let expireTimer: any = null
const EXPIRE_MS = 5 * 60 * 1000
const POLL_MS = 2000

function clearTimers() {
  if (pollTimer) {
    clearInterval(pollTimer)
    pollTimer = null
  }
  if (expireTimer) {
    clearTimeout(expireTimer)
    expireTimer = null
  }
}

async function start() {
  clearTimers()
  status.value = 'loading'
  qrBase64.value = ''
  useFallback.value = false
  errMsg.value = ''
  try {
    const s: any = await authApi.webLoginSession()
    ticket.value = s.ticket || ''
    try {
      const q: any = await authApi.webLoginQrcode(ticket.value)
      qrBase64.value = q.image_base64 || ''
      if (!qrBase64.value) throw new Error('empty qrcode')
      status.value = 'pending'
    } catch (e: any) {
      // 动态小程序码生成失败（如小程序未配置/页面未发布）：降级为静态码 + 手动输入票号
      if (props.fallbackQrcode) {
        useFallback.value = true
        status.value = 'pending'
      } else {
        status.value = 'error'
        errMsg.value = e?.response?.data?.message || '小程序码生成失败，请稍后重试'
      }
    }
    if (status.value === 'pending') {
      pollTimer = setInterval(poll, POLL_MS)
      expireTimer = setTimeout(() => {
        if (status.value === 'pending' || status.value === 'loading') {
          status.value = 'expired'
          clearTimers()
        }
      }, EXPIRE_MS)
    }
  } catch (e: any) {
    status.value = 'error'
    errMsg.value = e?.response?.data?.message || '创建登录会话失败，请稍后重试'
  }
}

async function poll() {
  if (!ticket.value) return
  try {
    const r: any = await authApi.webLoginPoll(ticket.value)
    if (r.status === 'expired') {
      status.value = 'expired'
      clearTimers()
      return
    }
    if (r.status === 'confirmed' && r.exchange_code) {
      clearTimers()
      status.value = 'pending'
      const ex: any = await authApi.socialExchange(r.exchange_code)
      if (ex?.token) {
        auth.set(ex.token, ex.user)
        emit('success')
        emit('update:modelValue', false)
        ElMessage.success('登录成功')
      }
    }
  } catch {
    /* 单次轮询失败不打断，下一轮继续 */
  }
}

function refresh() {
  start()
}

watch(
  () => props.modelValue,
  (v) => {
    if (v) start()
    else clearTimers()
  },
)
onBeforeUnmount(clearTimers)
</script>

<template>
  <el-dialog
    :model-value="modelValue"
    title="微信扫码登录"
    width="380px"
    @update:model-value="(v: boolean) => emit('update:modelValue', v)"
  >
    <div class="space-y-3 text-center">
      <p v-if="name" class="text-sm font-medium text-[var(--text)]">{{ name }}</p>
      <div class="mx-auto flex h-56 w-56 items-center justify-center rounded-xl border border-[var(--border)] bg-[var(--surface-2)] p-2">
        <img v-if="qrBase64" :src="'data:image/png;base64,' + qrBase64" class="h-full w-full object-contain" alt="微信登录小程序码" />
        <img v-else-if="useFallback && fallbackQrcode" :src="fallbackQrcode" class="h-full w-full object-contain" alt="小程序码" />
        <span v-else-if="status === 'loading'" class="text-xs text-[var(--text-soft)]">正在生成小程序码…</span>
        <span v-else-if="status === 'expired'" class="text-xs leading-relaxed text-[var(--text-soft)]">小程序码已失效<br />请点击下方「刷新二维码」</span>
        <span v-else class="px-3 text-xs leading-relaxed text-[var(--text-soft)]">{{ errMsg || '小程序码生成失败，请稍后重试' }}</span>
      </div>
      <p class="text-xs leading-relaxed text-[var(--text-soft)]">
        {{ desc || '请用微信扫描上方小程序码，在小程序内确认后即可自动登录（5 分钟内有效）' }}
      </p>
      <p v-if="status === 'pending' && ticket" class="text-xs text-[var(--text-soft)]">
        {{ useFallback ? '请在小程序「网页登录」页输入票号：' : '等待小程序确认… 票号：' }}
        <span class="font-mono font-bold tracking-widest text-[var(--grape)]">{{ ticket }}</span>
      </p>
    </div>
    <template #footer>
      <div class="flex justify-end gap-2">
        <button class="inline-flex items-center gap-1 rounded-full px-4 py-1.5 text-sm" @click="refresh">
          <RefreshCw :size="14" /> 刷新二维码
        </button>
        <button class="rounded-full px-4 py-1.5 text-sm" @click="emit('update:modelValue', false)">关闭</button>
      </div>
    </template>
  </el-dialog>
</template>
