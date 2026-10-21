<script setup lang="ts">
import { ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { useAuthStore } from '@/stores/auth'
import { useMetaStore } from '@/stores/meta'
import { authApi } from '@/api'

const auth = useAuthStore()
const meta = useMetaStore()

const oldPwd = ref('')
const newPwd = ref('')
const email = ref(auth.user?.email || '')
const loading = ref(false)

async function changePwd() {
  if (!oldPwd.value || !newPwd.value) {
    ElMessage.warning('请填写新旧密码')
    return
  }
  loading.value = true
  try {
    await authApi.changePassword({ old: oldPwd.value, new: newPwd.value })
    ElMessage.success('密码已修改，其他设备已退出')
    oldPwd.value = newPwd.value = ''
  } finally {
    loading.value = false
  }
}
async function changeEmail() {
  if (!email.value) return
  loading.value = true
  try {
    await authApi.changeEmail({ email: email.value })
    auth.setUser({ ...auth.user, email: email.value })
    ElMessage.success('邮箱已更新')
  } finally {
    loading.value = false
  }
}
async function deleteAccount() {
  try {
    await ElMessageBox.confirm('注销后账号将匿名化，投稿与记录保留，邮箱不可再次注册。确认注销？', '注销账号', { type: 'warning' })
  } catch {
    return
  }
  await authApi.deleteAccount()
  auth.logout()
  ElMessage.success('账号已注销')
  location.href = '/'
}
</script>

<template>
  <div class="mx-auto max-w-2xl px-4 py-6">
    <h1 class="mb-5 font-serif-sc text-2xl font-bold text-[var(--grape)]">账号安全</h1>

    <div class="space-y-5">
      <div class="paper-card p-6">
        <h2 class="mb-3 font-semibold">修改密码</h2>
        <div class="space-y-3">
          <input v-model="oldPwd" type="password" placeholder="当前密码" class="input-style" />
          <input v-model="newPwd" type="password" placeholder="新密码" class="input-style" />
          <button class="btn-grape rounded-full px-5 py-2 text-sm" :disabled="loading" @click="changePwd">确认修改</button>
        </div>
      </div>

      <div class="paper-card p-6">
        <h2 class="mb-3 font-semibold">修改邮箱</h2>
        <div class="flex gap-2">
          <input v-model="email" placeholder="新邮箱" class="input-style flex-1" />
          <button class="btn-grape rounded-full px-5 py-2 text-sm" :disabled="loading" @click="changeEmail">保存</button>
        </div>
      </div>

      <div class="paper-card p-6">
        <h2 class="mb-1 font-semibold">协议版本</h2>
        <p class="text-sm text-[var(--text-soft)]">当前已同意版本：{{ auth.user?.agreement_version || '—' }}</p>
        <router-link to="/agreement" class="mt-2 inline-block text-sm text-[var(--grape)]">查看并确认最新协议</router-link>
      </div>

      <div class="paper-card border border-[var(--danger)]/30 p-6">
        <h2 class="mb-1 font-semibold text-[var(--danger)]">注销账号</h2>
        <p class="mb-3 text-sm text-[var(--text-soft)]">注销不可恢复，请谨慎操作。</p>
        <button class="rounded-full border border-[var(--danger)] px-5 py-2 text-sm text-[var(--danger)]" @click="deleteAccount">注销账号</button>
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
  outline: none;
  font-size: 14px;
}
.input-style:focus {
  border-color: var(--grape);
}
</style>
