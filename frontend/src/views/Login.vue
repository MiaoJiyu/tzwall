<script setup lang="ts">
import { ref } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import { Mail, KeyRound } from 'lucide-vue-next'
import { ElMessage } from 'element-plus'
import { authApi } from '@/api'
import { useAuthStore } from '@/stores/auth'
import { useMetaStore } from '@/stores/meta'

const auth = useAuthStore()
const meta = useMetaStore()
const router = useRouter()
const route = useRoute()
const identifier = ref('')
const password = ref('')
const loading = ref(false)

async function submit() {
  if (!identifier.value || !password.value) {
    ElMessage.warning('请输入账号和密码')
    return
  }
  loading.value = true
  try {
    const r = await authApi.login({ identifier: identifier.value, password: password.value })
    auth.set(r.token, r.user)
    meta.ensureAgreement()
    router.push((route.query.redirect as string) || '/')
  } finally {
    loading.value = false
  }
}
</script>

<template>
  <div class="grid min-h-screen md:grid-cols-2">
    <div class="relative hidden flex-col justify-between overflow-hidden bg-gradient-to-br from-grape-700 to-grape-600 p-10 text-white md:flex">
      <div class="absolute -right-16 -top-16 h-64 w-64 rounded-full bg-white/10 blur-2xl"></div>
      <div class="absolute -bottom-20 -left-10 h-72 w-72 rounded-full bg-white/10 blur-3xl"></div>
      <div class="flex items-center gap-2">
        <img src="/favicon.svg" class="h-10 w-10" alt="校徽" />
        <span class="font-serif-sc text-xl font-bold">泰兴中学校园墙</span>
      </div>
      <div>
        <h2 class="font-serif-sc text-4xl font-bold leading-snug">爱国 <br />严谨</h2>
        <p class="mt-4 max-w-sm text-white/85">勤奋 奉献，记录校园里每一份真诚的分享。</p>
      </div>
      <p class="text-xs text-white/60">© 泰兴中学校园墙</p>
    </div>

    <div class="flex items-center justify-center p-6">
      <div class="w-full max-w-sm">
        <h1 class="mb-1 font-serif-sc text-2xl font-bold text-[var(--grape)]">欢迎回来</h1>
        <p class="mb-6 text-sm text-[var(--text-soft)]">登录以发布、评论与互动</p>

        <div class="space-y-4">
          <div class="relative">
            <Mail :size="16" class="absolute left-3 top-1/2 -translate-y-1/2 text-[var(--text-soft)]" />
            <input v-model="identifier" placeholder="用户名或邮箱" class="input-style pl-9" @keyup.enter="submit" />
          </div>
          <div class="relative">
            <KeyRound :size="16" class="absolute left-3 top-1/2 -translate-y-1/2 text-[var(--text-soft)]" />
            <input v-model="password" type="password" placeholder="密码" class="input-style pl-9" @keyup.enter="submit" />
          </div>
          <button class="btn-grape w-full rounded-full py-2.5 font-medium" :disabled="loading" @click="submit">
            {{ loading ? '登录中…' : '登录' }}
          </button>
        </div>

        <div class="mt-4 flex justify-between text-sm">
          <router-link to="/register" class="text-[var(--grape)]">注册账号</router-link>
          <router-link v-if="meta.site.features?.mailer" to="/forgot" class="text-[var(--text-soft)] hover:text-[var(--grape)]">忘记密码</router-link>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.input-style {
  width: 100%;
  height: 44px;
  border-radius: 12px;
  border: 1px solid var(--border);
  background: var(--surface-2);
  padding: 0 12px;
  padding-left: 2.5rem;
  outline: none;
  font-size: 14px;
}
.input-style:focus {
  border-color: var(--grape);
}
</style>
