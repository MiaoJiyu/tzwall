<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { useRoute } from 'vue-router'
import { CheckCircle2, XCircle } from 'lucide-vue-next'
import { authApi } from '@/api'

const route = useRoute()
const status = ref<'loading' | 'ok' | 'fail'>('loading')

onMounted(async () => {
  try {
    await authApi.verifyEmail((route.query.token as string) || '')
    status.value = 'ok'
  } catch {
    status.value = 'fail'
  }
})
</script>

<template>
  <div class="mx-auto max-w-md px-4 py-20 text-center">
    <template v-if="status === 'loading'"><p class="text-[var(--text-soft)]">正在验证…</p></template>
    <template v-else-if="status === 'ok'">
      <CheckCircle2 :size="48" class="mx-auto text-[var(--jade)]" />
      <h1 class="mt-4 font-serif-sc text-xl font-bold">邮箱已激活</h1>
      <p class="mt-2 text-sm text-[var(--text-soft)]">现在可以使用账号登录了。</p>
      <router-link to="/login" class="btn-grape mt-6 inline-block rounded-full px-6 py-2 text-sm">去登录</router-link>
    </template>
    <template v-else>
      <XCircle :size="48" class="mx-auto text-[var(--danger)]" />
      <h1 class="mt-4 font-serif-sc text-xl font-bold">验证失败</h1>
      <p class="mt-2 text-sm text-[var(--text-soft)]">链接无效或已过期，请重新获取激活邮件。</p>
      <router-link to="/login" class="mt-6 inline-block text-sm text-[var(--grape)]">返回登录</router-link>
    </template>
  </div>
</template>
