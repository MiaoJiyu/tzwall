<script setup lang="ts">
import { Sun, Moon } from 'lucide-vue-next'
import { useUiStore } from '@/stores/ui'

const ui = useUiStore()
</script>

<template>
  <button
    class="flex h-9 w-9 items-center justify-center rounded-full border border-[var(--border)] bg-[var(--surface)] text-[var(--text-soft)] transition hover:text-[var(--grape)]"
    @click="ui.toggle()"
    :title="ui.isDark ? '切换到浅色' : '切换到深色'"
  >
    <component :is="ui.isDark ? Sun : Moon" :size="18" />
  </button>
</template>
