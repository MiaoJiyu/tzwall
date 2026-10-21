<script setup lang="ts">
import { ShieldAlert } from 'lucide-vue-next'
</script>

<template>
  <div class="flex min-h-screen flex-col items-center justify-center px-4 text-center">
    <ShieldAlert :size="56" class="text-[var(--danger)]" />
    <h1 class="mt-4 font-serif-sc text-2xl font-bold">403</h1>
    <p class="mt-2 text-sm text-[var(--text-soft)]">您没有访问该页面的权限，请联系系统管理员。</p>
    <router-link to="/" class="btn-grape mt-6 rounded-full px-6 py-2 text-sm">返回首页</router-link>
  </div>
</template>
