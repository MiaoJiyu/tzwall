<script setup lang="ts">
import { Megaphone } from 'lucide-vue-next'
defineProps<{ items?: any[] }>()
</script>

<template>
  <div v-if="items && items.length" class="paper-card p-4">
    <div class="mb-3 flex items-center gap-2 text-sm font-semibold text-[var(--grape)]">
      <Megaphone :size="16" /> 校园公告
    </div>
    <ul class="space-y-3">
      <li v-for="a in items" :key="a.id" class="border-b border-[var(--border)] pb-3 last:border-0 last:pb-0">
        <div class="flex flex-wrap items-center gap-1.5">
          <span v-if="a.pinned" class="rounded bg-[var(--grape)]/10 px-1.5 py-0.5 text-[11px] font-medium text-[var(--grape)]">置顶</span>
          <span class="font-medium text-[var(--text)]">{{ a.title }}</span>
          <span v-if="a.require_confirm && !a.confirmed" class="rounded bg-warn/15 px-1.5 py-0.5 text-[11px] text-warn">待确认</span>
        </div>
        <p class="mt-1 whitespace-pre-wrap text-sm leading-relaxed text-[var(--text-soft)]">{{ a.content }}</p>
      </li>
    </ul>
  </div>
</template>
