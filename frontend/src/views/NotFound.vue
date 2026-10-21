<script setup lang="ts">
import { Compass } from 'lucide-vue-next'
</script>

<template>
  <div class="flex min-h-screen flex-col items-center justify-center px-4 text-center">
    <Compass :size="56" class="text-[var(--grape)]" />
    <h1 class="mt-4 font-serif-sc text-2xl font-bold">页面走丢了</h1>
    <p class="mt-2 text-sm text-[var(--text-soft)]">您访问的内容不存在或已被移除。</p>
    <router-link to="/" class="btn-grape mt-6 rounded-full px-6 py-2 text-sm">返回首页</router-link>
  </div>
</template>
