<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import { Plus } from 'lucide-vue-next'
import { ticketApi } from '@/api'

const router = useRouter()
const items = ref<any[]>([])
const tab = ref('all')
const tabs = [
  { k: 'all', l: '全部' },
  { k: 'pending', l: '待处理' },
  { k: 'processing', l: '处理中' },
  { k: 'resolved', l: '已解决' },
  { k: 'rejected', l: '已驳回' },
  { k: 'closed', l: '已关闭' },
]
const statusCls: Record<string, string> = {
  pending: 'bg-warn/15 text-warn',
  processing: 'bg-grape-50 text-[var(--grape)]',
  resolved: 'bg-[var(--jade)]/15 text-[var(--jade)]',
  rejected: 'bg-[var(--danger)]/15 text-[var(--danger)]',
  closed: 'bg-[var(--surface-2)] text-[var(--text-soft)]',
}
const statusText: Record<string, string> = { pending: '待处理', processing: '处理中', resolved: '已解决', rejected: '已驳回', closed: '已关闭' }
const typeText: Record<string, string> = { bug: '功能故障', feedback: '意见反馈', report: '违规举报', appeal: '处罚申诉', account: '账号/安全', other: '其他' }

async function load() {
  const p = await ticketApi.mine()
  items.value = p.items || []
}
onMounted(load)
const filtered = computed(() => (tab.value === 'all' ? items.value : items.value.filter((i) => i.status === tab.value)))
</script>

<template>
  <div class="mx-auto max-w-3xl px-4 py-6">
    <div class="mb-5 flex items-center justify-between">
      <h1 class="font-serif-sc text-2xl font-bold text-[var(--grape)]">我的工单</h1>
      <button class="btn-grape inline-flex items-center gap-1 rounded-full px-4 py-2 text-sm font-medium" @click="router.push('/tickets/new')">
        <Plus :size="16" /> 新建工单
      </button>
    </div>

    <div class="mb-5 flex flex-wrap gap-2">
      <button
        v-for="t in tabs"
        :key="t.k"
        @click="tab = t.k"
        :class="tab === t.k ? 'bg-grape-600 text-white' : 'bg-[var(--surface-2)] text-[var(--text-soft)]'"
        class="rounded-full px-4 py-1.5 text-sm"
      >
        {{ t.l }}
      </button>
    </div>

    <div v-if="filtered.length" class="space-y-3">
      <div v-for="it in filtered" :key="it.id" class="paper-card cursor-pointer p-4 transition hover:shadow" @click="router.push('/tickets/' + it.id)">
        <div class="flex items-center justify-between gap-2">
          <span class="font-medium">{{ it.title }}</span>
          <span class="shrink-0 rounded-full px-2 py-0.5 text-xs" :class="statusCls[it.status] || 'bg-[var(--surface-2)]'">{{ statusText[it.status] || it.status }}</span>
        </div>
        <div class="mt-1 flex flex-wrap items-center gap-3 text-xs text-[var(--text-soft)]">
          <span class="rounded bg-[var(--surface-2)] px-1.5 py-0.5">{{ typeText[it.type] || it.type }}</span>
          <span>#{{ it.id }}</span>
          <span>{{ it.created_at }}</span>
          <span v-if="it.attachments?.length">附件 {{ it.attachments.length }}</span>
        </div>
      </div>
    </div>
    <div v-else class="paper-card p-10 text-center text-[var(--text-soft)]">暂无工单</div>
  </div>
</template>
