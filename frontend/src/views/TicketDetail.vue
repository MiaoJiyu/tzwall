<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ElMessage, ElMessageBox } from 'element-plus'
import { ticketApi } from '@/api'

const route = useRoute()
const router = useRouter()
const id = Number(route.params.id)
const t = ref<any>(null)

const statusText: Record<string, string> = { pending: '待处理', processing: '处理中', resolved: '已解决', rejected: '已驳回', closed: '已关闭' }
const statusCls: Record<string, string> = {
  pending: 'bg-warn/15 text-warn',
  processing: 'bg-grape-50 text-[var(--grape)]',
  resolved: 'bg-[var(--jade)]/15 text-[var(--jade)]',
  rejected: 'bg-[var(--danger)]/15 text-[var(--danger)]',
  closed: 'bg-[var(--surface-2)] text-[var(--text-soft)]',
}
const typeText: Record<string, string> = { bug: '功能故障', feedback: '意见反馈', report: '违规举报', appeal: '处罚申诉', account: '账号/安全', other: '其他' }
const closable = computed(() => ['pending', 'processing', 'resolved'].includes(t.value?.status))

async function load() {
  try {
    t.value = await ticketApi.get(id)
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '加载失败')
    router.replace('/tickets')
  }
}
onMounted(load)

async function close() {
  try {
    await ElMessageBox.confirm('确定关闭该工单？关闭后将不再处理。', '关闭工单', { type: 'warning' })
  } catch {
    return
  }
  try {
    await ticketApi.close(id)
    ElMessage.success('已关闭')
    load()
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '关闭失败')
  }
}
</script>

<template>
  <div class="mx-auto max-w-2xl px-4 py-6" v-if="t">
    <button class="mb-3 text-sm text-[var(--text-soft)]" @click="router.push('/tickets')">← 返回我的工单</button>

    <div class="paper-card p-6">
      <div class="flex items-start justify-between gap-3">
        <h1 class="font-serif-sc text-xl font-bold">{{ t.title }}</h1>
        <span class="shrink-0 rounded-full px-2.5 py-1 text-xs" :class="statusCls[t.status] || 'bg-[var(--surface-2)]'">{{ statusText[t.status] || t.status }}</span>
      </div>
      <div class="mt-2 flex flex-wrap items-center gap-3 text-xs text-[var(--text-soft)]">
        <span class="rounded bg-[var(--surface-2)] px-1.5 py-0.5">{{ typeText[t.type] || t.type }}</span>
        <span>#{{ t.id }}</span>
        <span>提交于 {{ t.created_at }}</span>
      </div>

      <p class="mt-4 whitespace-pre-wrap text-sm leading-relaxed">{{ t.content }}</p>

      <div v-if="t.attachments?.length" class="mt-4 grid grid-cols-3 gap-2">
        <img v-for="a in t.attachments" :key="a.id" :src="a.url" class="h-28 w-full rounded-lg object-cover" alt="" />
      </div>

      <div v-if="closable" class="mt-5 border-t border-[var(--border)] pt-4">
        <button class="rounded-full border border-[var(--border)] px-4 py-1.5 text-sm hover:text-[var(--danger)]" @click="close">关闭工单</button>
      </div>
    </div>

    <h2 class="mb-3 mt-6 font-serif-sc text-lg font-bold">处理记录</h2>
    <div v-if="t.replies?.length" class="space-y-3">
      <div v-for="(r, idx) in t.replies" :key="idx" class="rounded-lg bg-[var(--surface-2)] p-3 text-sm">
        <div class="mb-1 flex items-center gap-2 text-xs text-[var(--text-soft)]">
          <span class="font-medium text-[var(--text)]">{{ r.status_to ? '系统' : r.is_admin ? '管理员' : '我' }}{{ r.username ? ' · ' + r.username : '' }}</span>
          <span>{{ r.created_at }}</span>
        </div>
        <p class="leading-relaxed">{{ r.content }}</p>
      </div>
    </div>
    <p v-else class="text-sm text-[var(--text-soft)]">暂无处理记录，请耐心等待管理员处理。</p>
  </div>
</template>
