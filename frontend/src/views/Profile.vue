<script setup lang="ts">
import { ref, reactive } from 'vue'
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

// 个人资料编辑
const profile = ref({
  nickname: auth.user?.nickname || '',
  avatar: auth.user?.avatar || '',
  wechat: auth.user?.wechat || '',
  qq: auth.user?.qq || '',
  phone: auth.user?.phone || '',
})
const saving = ref(false)
async function saveProfile() {
  saving.value = true
  try {
    await authApi.updateProfile({ ...profile.value })
    const fresh = await authApi.me()
    auth.setUser(fresh)
    ElMessage.success('资料已保存')
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '保存失败')
  } finally {
    saving.value = false
  }
}

// 公开主页设置
const profilePublic = ref(!!auth.user?.profile_public)
const pubFields = reactive<Record<string, boolean>>({ wechat: false, qq: false, phone: false, school: false, real_name: false })
{
  const cur = (auth.user?.public_fields || '').split(',')
  for (const k of Object.keys(pubFields)) pubFields[k] = cur.includes(k)
}
async function savePublic() {
  saving.value = true
  try {
    const fields = Object.keys(pubFields).filter((k) => pubFields[k]).join(',')
    await authApi.updateProfile({ profile_public: profilePublic.value, public_fields: fields })
    const fresh = await authApi.me()
    auth.setUser(fresh)
    ElMessage.success(profilePublic.value ? '公开主页已开启' : '公开主页已关闭')
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '保存失败')
  } finally {
    saving.value = false
  }
}

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
        <h2 class="mb-3 font-semibold">个人资料</h2>
        <div class="space-y-3">
          <div>
            <label class="mb-1 block text-xs text-[var(--text-soft)]">头像（图片链接）</label>
            <div class="flex items-center gap-3">
              <img v-if="profile.avatar" :src="profile.avatar" class="h-12 w-12 rounded-full object-cover" alt="" />
              <div v-else class="flex h-12 w-12 items-center justify-center rounded-full bg-grape-100 text-lg font-semibold text-[var(--grape)]">{{ (auth.user?.username || '?')[0] }}</div>
              <input v-model="profile.avatar" placeholder="头像图片 URL（可先在发布页上传后复制链接）" class="input-style flex-1" />
            </div>
          </div>
          <div class="grid grid-cols-2 gap-3">
            <div>
              <label class="mb-1 block text-xs text-[var(--text-soft)]">昵称</label>
              <input v-model="profile.nickname" maxlength="64" placeholder="展示昵称" class="input-style" />
            </div>
            <div>
              <label class="mb-1 block text-xs text-[var(--text-soft)]">微信号</label>
              <input v-model="profile.wechat" maxlength="64" placeholder="微信" class="input-style" />
            </div>
            <div>
              <label class="mb-1 block text-xs text-[var(--text-soft)]">QQ</label>
              <input v-model="profile.qq" maxlength="32" placeholder="QQ" class="input-style" />
            </div>
            <div>
              <label class="mb-1 block text-xs text-[var(--text-soft)]">手机号</label>
              <input v-model="profile.phone" maxlength="32" placeholder="手机" class="input-style" />
            </div>
          </div>
          <button class="btn-grape rounded-full px-5 py-2 text-sm" :disabled="saving" @click="saveProfile">保存资料</button>
        </div>
      </div>

      <div class="paper-card p-6">
        <h2 class="mb-1 font-semibold">公开个人主页</h2>
        <p class="mb-3 text-sm text-[var(--text-soft)]">开启后将生成可访问的个人主页（/user/{{ auth.user?.id }}），展示你选择公开的信息与历史发帖。</p>
        <label class="flex items-center gap-2 text-sm"><input type="checkbox" v-model="profilePublic" /> 开启公开主页</label>
        <div class="mt-3 flex flex-wrap gap-4 text-sm">
          <label class="flex items-center gap-1.5"><input type="checkbox" v-model="pubFields.wechat" /> 微信号</label>
          <label class="flex items-center gap-1.5"><input type="checkbox" v-model="pubFields.qq" /> QQ</label>
          <label class="flex items-center gap-1.5"><input type="checkbox" v-model="pubFields.phone" /> 手机号</label>
          <label class="flex items-center gap-1.5"><input type="checkbox" v-model="pubFields.school" /> 学校</label>
          <label class="flex items-center gap-1.5"><input type="checkbox" v-model="pubFields.real_name" /> 真实姓名</label>
        </div>
        <div class="mt-4 flex items-center gap-3">
          <button class="btn-grape rounded-full px-5 py-2 text-sm" :disabled="saving" @click="savePublic">保存公开设置</button>
          <router-link v-if="profilePublic" :to="'/user/' + auth.user?.id" class="text-sm text-[var(--grape)] hover:underline">查看我的主页 →</router-link>
        </div>
      </div>

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
