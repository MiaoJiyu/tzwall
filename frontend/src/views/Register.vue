<script setup lang="ts">
import { reactive, ref, onBeforeUnmount } from 'vue'
import { useRouter } from 'vue-router'
import { User, Mail, Lock, School, Phone, Loader2, RotateCcw } from 'lucide-vue-next'
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

// 注册成功后的邮箱激活状态
const registered = ref(false)
const registeredEmail = ref('')
const qqModal = ref(false)
const resendLoading = ref(false)
const resendMsg = ref('')
const resendOk = ref(false)
const cooldown = ref(0)
let cooldownTimer: ReturnType<typeof setInterval> | null = null

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
    const r: any = await authApi.register({ ...form, agree: true })
    // QQ 邮箱且需验证：立即弹出查收提示（由后端判定，前端仅展示）
    if (r?.qq_pending) qqModal.value = true
    // 以后端返回的激活状态为准：未激活→停留激活面板；已激活（免验证/未配邮件）→去登录
    const verified = r?.user ? !!r.user.email_verified : !meta.site.features?.mailer
    if (verified) {
      ElMessage.success('注册成功，可直接登录')
      router.push('/login')
    } else {
      registeredEmail.value = form.email
      registered.value = true
      ElMessage.success('注册成功，激活邮件已发送，请查收')
    }
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || e?.message || '注册失败，请稍后再试')
  } finally {
    loading.value = false
  }
}

// 再次发送激活邮件：含发送中/成功/失败反馈与冷却，避免重复操作
async function resend() {
  if (resendLoading.value || cooldown.value > 0) return
  resendLoading.value = true
  resendMsg.value = ''
  try {
    await authApi.resendVerify({ email: registeredEmail.value })
    resendOk.value = true
    resendMsg.value = '已重新发送，请查收邮箱（记得查看垃圾箱）'
    startCooldown(60)
  } catch (e: any) {
    resendOk.value = false
    resendMsg.value = e?.response?.data?.message || e?.message || '发送失败，请稍后再试'
  } finally {
    resendLoading.value = false
  }
}

function startCooldown(sec: number) {
  cooldown.value = sec
  if (cooldownTimer) clearInterval(cooldownTimer)
  cooldownTimer = setInterval(() => {
    cooldown.value -= 1
    if (cooldown.value <= 0 && cooldownTimer) {
      clearInterval(cooldownTimer)
      cooldownTimer = null
    }
  }, 1000)
}

onBeforeUnmount(() => {
  if (cooldownTimer) clearInterval(cooldownTimer)
})
</script>

<template>
  <div class="mx-auto max-w-md px-4 py-10">
    <h1 class="mb-1 text-center font-serif-sc text-2xl font-bold text-[var(--grape)]">创建账号</h1>
    <p class="mb-6 text-center text-sm text-[var(--text-soft)]">加入泰兴中学校园墙</p>

    <!-- 激活提示面板：注册成功且需邮箱激活时展示 -->
    <div v-if="registered" class="paper-card space-y-4 p-6 text-center">
      <div class="mx-auto flex h-12 w-12 items-center justify-center rounded-full bg-[var(--grape)]/10">
        <Mail :size="24" class="text-[var(--grape)]" />
      </div>
      <h2 class="font-serif-sc text-lg font-bold">注册成功，请激活邮箱</h2>
      <p class="text-sm text-[var(--text-soft)]">
        激活邮件已发送至 <span class="font-medium text-[var(--text)]">{{ registeredEmail }}</span>，请查收并点击邮件中的链接完成激活。
      </p>
      <div class="rounded-lg border border-warn/30 bg-warn/10 p-3 text-left text-xs leading-relaxed text-[var(--text-soft)]">
        如果收不到邮件，请检查邮件是否被系统拦截或识别为垃圾邮件（QQ邮箱有这个问题）。
      </div>
      <button
        class="btn-grape inline-flex w-full items-center justify-center gap-1.5 rounded-full py-2.5 text-sm font-medium disabled:opacity-60"
        :disabled="resendLoading || cooldown > 0"
        @click="resend"
      >
        <Loader2 v-if="resendLoading" :size="16" class="animate-spin" />
        <RotateCcw v-else :size="16" />
        <span v-if="resendLoading">发送中…</span>
        <span v-else-if="cooldown > 0">{{ cooldown }}s 后可重发</span>
        <span v-else>再次发送激活邮件</span>
      </button>
      <p v-if="resendMsg" class="text-xs" :class="resendOk ? 'text-[var(--jade)]' : 'text-[var(--danger)]'">{{ resendMsg }}</p>
      <p class="text-sm text-[var(--text-soft)]">
        已完成激活？<router-link to="/login" class="text-[var(--grape)]">去登录</router-link>
      </p>
    </div>

    <template v-else>
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
    </template>

    <!-- QQ 邮箱注册提示（仅 QQ 邮箱且需验证时由后端触发） -->
    <el-dialog v-model="qqModal" title="QQ 邮箱注册提示" width="420px" :close-on-click-modal="false" :show-close="true">
      <p class="text-sm leading-relaxed text-[var(--text)]">
        您使用了QQ邮箱注册。如果没有收到激活邮件，请检查“设置-自助查询-收信查询”或者垃圾邮件箱。
      </p>
      <template #footer>
        <div class="flex justify-end">
          <button class="btn-grape rounded-full px-5 py-2 text-sm font-medium text-white" @click="qqModal = false">我知道了</button>
        </div>
      </template>
    </el-dialog>
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
