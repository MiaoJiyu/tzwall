<script setup lang="ts">
import { reactive, ref } from 'vue'
import { useRouter } from 'vue-router'
import { User, Mail, Lock, School, Phone } from 'lucide-vue-next'
import { ElMessage } from 'element-plus'
import { authApi } from '@/api'
import { useMetaStore } from '@/stores/meta'

const meta = useMetaStore()
const router = useRouter()
const loading = ref(false)
const agree = reactive({ user: false, privacy: false, data: false })

const form = reactive({
  username: '',
  password: '',
  email: '',
  real_name: '',
  school: '',
  wechat: '',
  qq: '',
  phone: '',
})

async function submit() {
  if (!form.username || !form.password || !form.email || !form.real_name || !form.school) {
    ElMessage.warning('请填写必填项（用户名/密码/邮箱/真实姓名/就读学校）')
    return
  }
  if (!agree.user || !agree.privacy || !agree.data) {
    ElMessage.warning('请阅读并勾选全部协议')
    return
  }
  loading.value = true
  try {
    await authApi.register({ ...form, agree: true })
    ElMessage.success(meta.site.features?.mailer ? '注册成功，请前往邮箱激活' : '注册成功，可直接登录')
    router.push('/login')
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || e?.message || '注册失败，请稍后再试')
  } finally {
    loading.value = false
  }
}
</script>

<template>
  <div class="mx-auto max-w-md px-4 py-10">
    <h1 class="mb-1 text-center font-serif-sc text-2xl font-bold text-[var(--grape)]">创建账号</h1>
    <p class="mb-6 text-center text-sm text-[var(--text-soft)]">加入泰兴中学校园墙</p>

    <div class="paper-card space-y-4 p-6">
      <div class="grid grid-cols-2 gap-3">
        <div class="relative">
          <User :size="16" class="absolute left-3 top-1/2 -translate-y-1/2 text-[var(--text-soft)]" />
          <input v-model="form.username" placeholder="用户名(6-12位)" class="input-style" />
        </div>
        <div class="relative">
          <Lock :size="16" class="absolute left-3 top-1/2 -translate-y-1/2 text-[var(--text-soft)]" />
          <input v-model="form.password" type="password" placeholder="密码" class="input-style" />
        </div>
      </div>
      <div class="relative">
        <Mail :size="16" class="absolute left-3 top-1/2 -translate-y-1/2 text-[var(--text-soft)]" />
        <input v-model="form.email" placeholder="邮箱（必填且唯一）" class="input-style" />
      </div>
      <div class="grid grid-cols-2 gap-3">
        <div class="relative">
          <School :size="16" class="absolute left-3 top-1/2 -translate-y-1/2 text-[var(--text-soft)]" />
          <input v-model="form.real_name" placeholder="真实姓名(仅后台可见)" class="input-style" />
        </div>
        <input v-model="form.school" placeholder="就读学校(仅后台可见)" class="input-style no-icon" />
      </div>
      <div class="grid grid-cols-3 gap-3">
        <input v-model="form.wechat" placeholder="微信(选填)" class="input-style no-icon" />
        <input v-model="form.qq" placeholder="QQ(选填)" class="input-style no-icon" />
        <div class="relative">
          <Phone :size="16" class="absolute left-3 top-1/2 -translate-y-1/2 text-[var(--text-soft)]" />
          <input v-model="form.phone" placeholder="手机(选填)" class="input-style" />
        </div>
      </div>

      <div class="space-y-1.5 rounded-lg bg-[var(--surface-2)] p-3 text-sm">
        <label class="flex items-center gap-2"><input type="checkbox" v-model="agree.user" /> 我已阅读并同意《用户协议》</label>
        <label class="flex items-center gap-2"><input type="checkbox" v-model="agree.privacy" /> 我已阅读并同意《隐私协议》</label>
        <label class="flex items-center gap-2"><input type="checkbox" v-model="agree.data" /> 我已阅读并同意《数据保护协议》</label>
      </div>

      <button class="btn-grape w-full rounded-full py-2.5 font-medium" :disabled="loading" @click="submit">
        {{ loading ? '提交中…' : '注册' }}
      </button>
    </div>
    <p class="mt-4 text-center text-sm text-[var(--text-soft)]">
      已有账号？<router-link to="/login" class="text-[var(--grape)]">去登录</router-link>
    </p>
  </div>
</template>

<style scoped>
.input-style {
  width: 100%;
  box-sizing: border-box;
  height: 44px;
  border-radius: 12px;
  border: 1px solid var(--border);
  background: var(--surface-2);
  padding: 0 14px 0 2.5rem;
  outline: none;
  font-size: 14px;
}
.input-style.no-icon {
  padding-left: 14px;
}
.input-style:focus {
  border-color: var(--grape);
}
</style>
