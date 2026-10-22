<script setup lang="ts">
// 用户称号徽章：展示于用户名后。无称号时不渲染。
defineProps<{ title?: string }>()
</script>

<template>
  <span
    v-if="title"
    class="inline-flex shrink-0 items-center rounded-full border border-[var(--grape)]/30 bg-[var(--grape)]/10 px-1.5 py-0.5 text-[10px] font-medium leading-none text-[var(--grape)]"
    :title="'称号：' + title"
  >{{ title }}</span>
</template>
