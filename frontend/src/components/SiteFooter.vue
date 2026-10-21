<script setup lang="ts">
import { useMetaStore } from '@/stores/meta'
const meta = useMetaStore()
</script>

<template>
  <footer class="mt-12 border-t border-[var(--border)] bg-[var(--surface)]">
    <div class="mx-auto max-w-[1200px] px-4 py-8 text-sm text-[var(--text-soft)]">
      <div class="flex flex-wrap items-center gap-x-4 gap-y-2">
        <span class="font-serif-sc font-semibold text-[var(--grape)]">{{ meta.site.site_name || '泰兴中学校园墙' }}</span>
        <span class="text-[var(--border)]">·</span>
        <span>爱国 · 严谨 · 勤奋 · 奉献</span>
      </div>
      <div class="mt-3 flex flex-wrap items-center gap-x-4 gap-y-1">
        <a v-if="meta.site.footer_icp" :href="meta.site.footer_icp_link || '#'" target="_blank" rel="noopener" class="hover:text-[var(--grape)]">
          {{ meta.site.footer_icp }}
        </a>
        <a v-if="meta.site.footer_psba" :href="meta.site.footer_psba_link || '#'" target="_blank" rel="noopener" class="hover:text-[var(--grape)]">
          {{ meta.site.footer_psba }}
        </a>
      </div>
      <p v-if="meta.site.footer_custom" class="mt-2 leading-relaxed">{{ meta.site.footer_custom }}</p>
      <p class="mt-3 text-xs opacity-70">© 泰兴中学校园墙 · 仅用于校园社区交流</p>
    </div>
  </footer>
</template>
