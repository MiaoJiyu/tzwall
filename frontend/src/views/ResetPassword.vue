<script setup lang="ts">
import { ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { Lock } from 'lucide-vue-next'
import { ElMessage } from 'element-plus'
import { authApi } from '@/api'

const route = useRoute()
const router = useRouter()
const password = ref('')
const loading = ref(false)

async function submit() {
  if (!password.value) {
    ElMessage.warning('请输入新密码')
    return
  }
  loading.value = true
  try {
    await authApi.reset({ token: (route.query.token as string) || '', password: password.value })
    ElMessage.success('密码已重置，请登录')
    router.push('/login')
  } finally {
    loading.value = false
  }
}
</script>

<template>
  <div class="mx-auto max-w-md px-4 py-16">
    <div class="paper-card p-6">
      <h1 class="font-serif-sc text-xl font-bold text-[var(--grape)]">重置密码</h1>
      <p class="mt-2 text-sm text-[var(--text-soft)]">设置你的新密码</p>
      <div class="relative mt-4">
        <Lock :size="16" class="absolute left-3 top-1/2 -translate-y-1/2 text-[var(--text-soft)]" />
        <input v-model="password" type="password" placeholder="新密码" class="input-style pl-9" @keyup.enter="submit" />
      </div>
      <button class="btn-grape mt-4 w-full rounded-full py-2.5 text-sm" :disabled="loading" @click="submit">
        {{ loading ? '提交中…' : '确认重置' }}
      </button>
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
