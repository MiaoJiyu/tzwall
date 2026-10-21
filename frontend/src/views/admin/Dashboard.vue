<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { Users, FileText, FileCheck, Tags, MessageCircle, Flame } from 'lucide-vue-next'
import { adminApi } from '@/api'

const stat = ref<any>({})

onMounted(async () => {
  stat.value = await adminApi.dashboard()
})

const cards = computed(() => [
  { label: '用户数', value: stat.value.users || 0, icon: Users },
  { label: '投稿总数', value: stat.value.submissions || 0, icon: FileText },
  { label: '待审核投稿', value: stat.value.pending_submissions || 0, icon: FileCheck },
  { label: '待审核标签', value: stat.value.pending_tags || 0, icon: Tags },
  { label: '评论数', value: stat.value.comments || 0, icon: MessageCircle },
  { label: '今日投稿', value: stat.value.today_submissions || 0, icon: Flame },
])
</script>

<template>
  <div>
    <h1 class="mb-5 font-serif-sc text-2xl font-bold text-[var(--grape)]">数据概览</h1>
    <div class="grid grid-cols-2 gap-4 md:grid-cols-3">
      <div v-for="c in cards" :key="c.label" class="paper-card flex items-center gap-4 p-5 animate-fadeup">
        <div class="flex h-12 w-12 items-center justify-center rounded-xl bg-grape-50 text-[var(--grape)]">
          <component :is="c.icon" :size="22" />
        </div>
        <div>
          <div class="text-2xl font-bold">{{ c.value }}</div>
          <div class="text-xs text-[var(--text-soft)]">{{ c.label }}</div>
        </div>
      </div>
    </div>
  </div>
</template>
