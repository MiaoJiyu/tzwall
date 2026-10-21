<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { submissionApi } from '@/api'
import SubmissionCard from '@/components/SubmissionCard.vue'

const items = ref<any[]>([])
const tab = ref('all')
const tabs = [
  { k: 'all', l: '全部' },
  { k: 'approved', l: '通过' },
  { k: 'pending', l: '审核中' },
  { k: 'need_edit', l: '需修改' },
  { k: 'rejected', l: '未通过' },
]

async function load() {
  const p = await submissionApi.myList()
  items.value = p.items || []
}
onMounted(load)

const filtered = computed(() => (tab.value === 'all' ? items.value : items.value.filter((i) => i.status === tab.value)))
</script>

<template>
  <div class="mx-auto max-w-3xl px-4 py-6">
    <h1 class="mb-5 font-serif-sc text-2xl font-bold text-[var(--grape)]">我的投稿</h1>

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

    <div v-if="filtered.length" class="space-y-4">
      <div v-for="it in filtered" :key="it.id">
        <SubmissionCard :item="it" />
        <p v-if="(it.status === 'need_edit' || it.status === 'rejected') && it.admin_note" class="mt-2 rounded-lg border border-warn/30 bg-warn/10 p-3 text-sm text-warn">
          审核意见：{{ it.admin_note }}
        </p>
      </div>
    </div>
    <div v-else class="paper-card p-10 text-center text-[var(--text-soft)]">暂无该状态投稿</div>
  </div>
</template>
