<script setup lang="ts">
import { onMounted, onBeforeUnmount, ref, nextTick } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { Send } from 'lucide-vue-next'
import { ElMessage } from 'element-plus'
import { chatApi } from '@/api'

const route = useRoute()
const router = useRouter()
const convs = ref<any[]>([])
const activeId = ref<number>(Number(route.params.id) || 0)
const messages = ref<any[]>([])
const input = ref('')
const sending = ref(false)
const scroller = ref<HTMLElement | null>(null)
let timer: any = null

async function loadConvs() {
  const r = await chatApi.list()
  convs.value = r.items || []
}
async function loadMessages() {
  if (!activeId.value) {
    messages.value = []
    return
  }
  const r = await chatApi.messages(activeId.value)
  messages.value = r.items || []
  await chatApi.read(activeId.value)
  await loadConvs()
  await nextTick()
  scrollBottom()
}
function scrollBottom() {
  if (scroller.value) scroller.value.scrollTop = scroller.value.scrollHeight
}
async function openConv(id: number) {
  router.push('/messages/' + id)
  activeId.value = id
  await loadMessages()
}
async function send() {
  if (!input.value.trim() || !activeId.value) return
  sending.value = true
  try {
    const m = await chatApi.send(activeId.value, input.value.trim())
    messages.value.push(m)
    input.value = ''
    await loadConvs()
    await nextTick()
    scrollBottom()
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '发送失败')
  } finally {
    sending.value = false
  }
}
function otherName(c: any) {
  return c.other?.nickname || c.other?.username || '用户'
}
function fmt(t: string) {
  return (t || '').replace('T', ' ').slice(0, 16)
}

onMounted(async () => {
  await loadConvs()
  if (route.params.id) {
    activeId.value = Number(route.params.id)
    await loadMessages()
  }
  timer = setInterval(async () => {
    if (activeId.value) {
      const r = await chatApi.messages(activeId.value)
      messages.value = r.items || []
      await nextTick()
      scrollBottom()
    }
    await loadConvs()
  }, 5000)
})
onBeforeUnmount(() => {
  if (timer) clearInterval(timer)
})
</script>

<template>
  <div class="mx-auto max-w-5xl px-4 py-6">
    <h1 class="mb-5 font-serif-sc text-2xl font-bold text-[var(--grape)]">私聊</h1>
    <div class="paper-card grid min-h-[520px] grid-cols-1 overflow-hidden md:grid-cols-[280px_1fr]">
      <!-- 会话列表 -->
      <div class="border-b border-[var(--border)] md:border-b-0 md:border-r">
        <div class="max-h-[300px] overflow-y-auto md:max-h-[520px]">
          <button
            v-for="c in convs"
            :key="c.id"
            class="flex w-full items-center gap-2 border-b border-[var(--border)] px-4 py-3 text-left transition hover:bg-[var(--surface-2)]"
            :class="activeId === c.id ? 'bg-grape-50' : ''"
            @click="openConv(c.id)"
          >
            <div class="flex h-9 w-9 shrink-0 items-center justify-center rounded-full bg-grape-100 text-sm font-semibold text-[var(--grape)]">{{ otherName(c)[0] }}</div>
            <div class="min-w-0 flex-1">
              <div class="flex items-center justify-between gap-2">
                <span class="truncate text-sm font-medium">{{ otherName(c) }}</span>
                <span v-if="c.unread" class="flex h-4 min-w-4 items-center justify-center rounded-full bg-[var(--danger)] px-1 text-[10px] text-white">{{ c.unread > 99 ? '99+' : c.unread }}</span>
              </div>
              <p class="truncate text-xs text-[var(--text-soft)]">{{ c.last_message || '暂无消息' }}</p>
            </div>
          </button>
          <div v-if="!convs.length" class="p-6 text-center text-sm text-[var(--text-soft)]">暂无会话。去投稿详情页点击作者“私聊”发起吧。</div>
        </div>
      </div>

      <!-- 聊天区 -->
      <div class="flex flex-col">
        <div v-if="activeId" ref="scroller" class="flex-1 space-y-3 overflow-y-auto p-4">
          <div v-for="(m, i) in messages" :key="i" class="flex" :class="m.mine ? 'justify-end' : 'justify-start'">
            <div class="max-w-[75%] rounded-2xl px-3 py-2 text-sm" :class="m.mine ? 'bg-grape-500 text-white' : 'bg-[var(--surface-2)] text-[var(--text)]'">
              <p class="whitespace-pre-wrap break-words">{{ m.content }}</p>
              <p class="mt-1 text-[10px] opacity-70">{{ fmt(m.created_at) }}</p>
            </div>
          </div>
          <div v-if="!messages.length" class="py-10 text-center text-sm text-[var(--text-soft)]">开始你们的第一条消息吧～</div>
        </div>
        <div v-else class="flex flex-1 items-center justify-center text-sm text-[var(--text-soft)]">选择左侧会话开始聊天</div>

        <div v-if="activeId" class="flex items-center gap-2 border-t border-[var(--border)] p-3">
          <input v-model="input" @keyup.enter="send" placeholder="输入消息…" class="flex-1 rounded-full border border-[var(--border)] bg-[var(--surface-2)] px-4 py-2 text-sm outline-none focus:border-[var(--grape)]" />
          <button class="btn-grape flex h-9 w-9 items-center justify-center rounded-full text-white disabled:opacity-60" :disabled="sending" @click="send"><Send :size="16" /></button>
        </div>
      </div>
    </div>
  </div>
</template>
