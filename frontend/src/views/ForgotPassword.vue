<script setup lang="ts">
import { ref } from 'vue'
import { Mail } from 'lucide-vue-next'
import { ElMessage } from 'element-plus'
import { authApi } from '@/api'
import { useMetaStore } from '@/stores/meta'

const meta = useMetaStore()
const email = ref('')
const loading = ref(false)
const sent = ref(false)

async function submit() {
  if (!email.value) {
    ElMessage.warning('请输入邮箱')
    return
  }
  loading.value = true
  try {
    await authApi.forgot({ email: email.value })
    sent.value = true
  } finally {
    loading.value = false
  }
}
</script>

<template>
  <div class="mx-auto max-w-md px-4 py-16">
    <div class="paper-card p-6">
      <h1 class="font-serif-sc text-xl font-bold text-[var(--grape)]">找回密码</h1>
      <p v-if="!meta.site.features?.mailer" class="mt-2 rounded-md bg-warn/15 p-2 text-sm text-warn">
        当前未配置邮件服务，找回密码功能不可用，请联系管理员。
      </p>
      <template v-else-if="!sent">
        <p class="mt-2 text-sm text-[var(--text-soft)]">输入注册邮箱，我们将发送重置链接。</p>
        <div class="relative mt-4">
          <Mail :size="16" class="absolute left-3 top-1/2 -translate-y-1/2 text-[var(--text-soft)]" />
          <input v-model="email" placeholder="邮箱" class="input-style pl-9" @keyup.enter="submit" />
        </div>
        <button class="btn-grape mt-4 w-full rounded-full py-2.5 text-sm" :disabled="loading || !meta.site.features?.mailer" @click="submit">
          {{ loading ? '发送中…' : '发送重置邮件' }}
        </button>
      </template>
      <p v-else class="mt-4 text-sm text-[var(--jade)]">若邮箱存在，重置链接已发送，请查收。</p>
    </div>
    <div class="mt-4 text-center text-sm">
      <router-link to="/login" class="text-[var(--grape)]">返回登录</router-link>
    </div>
  </div>
</template>

<style scoped>
.input-style {
  height: 44px;
  width: 100%;
  border-radius: 12px;
  border: 1px solid var(--border);
  background: var(--surface-2);
  padding: 0 12px;
  outline: none;
  font-size: 14px;
}
.input-style:focus {
  border-color: var(--grape);
}
</style>
