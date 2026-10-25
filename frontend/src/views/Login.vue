<script setup lang="ts">
import { ref, onMounted, computed } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import { Mail, KeyRound } from 'lucide-vue-next'
import { ElMessage } from 'element-plus'
import { authApi } from '@/api'
import { useAuthStore } from '@/stores/auth'
import { useMetaStore } from '@/stores/meta'
import MiniProgramQrDialog from '@/components/MiniProgramQrDialog.vue'
import WebLoginDialog from '@/components/WebLoginDialog.vue'

const auth = useAuthStore()
const meta = useMetaStore()
const router = useRouter()
const route = useRoute()
const identifier = ref('')
const password = ref('')
const loading = ref(false)
const social = ref({ wechat: false, wechat_web: false, qq: false })
const mp = computed(() => meta.site?.miniprogram || {})
const mpLoginDialog = ref(false)
const webLoginDialog = ref(false)
function onWebLoginOk() {
  meta.ensureAgreement()
  router.push((route.query.redirect as string) || '/')
}

onMounted(async () => {
  // QQ/微信 授权回跳：用一次性 code 兑换登录态
  const sc = route.query.social_code as string | undefined
  if (sc) {
    try {
      const r: any = await authApi.socialExchange(sc)
      auth.set(r.token, r.user)
      meta.ensureAgreement()
      router.replace((route.query.redirect as string) || '/')
    } catch (e: any) {
      ElMessage.error(e?.response?.data?.message || '社交登录失败，请重试')
    }
    return
  }
  if (route.query.social_error) {
    ElMessage.error((route.query.social_error as string) || '社交登录失败')
  }
  social.value = {
    wechat: !!meta.site?.features?.social?.wechat,
    wechat_web: !!meta.site?.features?.social?.wechat_web,
    qq: !!meta.site?.features?.social?.qq,
  }
})

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

async function qqLogin() {
  try {
    const r: any = await authApi.qqAuthorize()
    window.location.href = r.url
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '发起 QQ 登录失败')
  }
}

const wechatChoose = ref(false)
// 微信登录：按后台实际配置的能力选择流程，避免"仅小程序配置"时误请求网页接口报错
function wechatLogin() {
  const canMp = social.value.wechat
  const canWeb = social.value.wechat_web
  if (canMp && canWeb) {
    wechatChoose.value = true // 两种方式都可用，让用户选择
    return
  }
  if (canWeb) {
    wechatWeb()
    return
  }
  if (canMp) {
    wechatMiniapp()
    return
  }
  ElMessage.warning('微信登录暂未开放')
}
function wechatMiniapp() {
  wechatChoose.value = false
  webLoginDialog.value = true // 登录场景：弹「网页扫码即登」小程序码（带 scene）
}
async function wechatWeb() {
  wechatChoose.value = false
  try {
    const r: any = await authApi.wechatWebUrl()
    window.location.href = r.url
  } catch {
    ElMessage.info('微信登录请在「泰兴中学校园墙」微信小程序中使用')
  }
}
</script>

<template>
  <div class="grid min-h-screen md:grid-cols-2">
    <div class="relative hidden flex-col justify-between overflow-hidden bg-gradient-to-br from-grape-700 to-grape-600 p-10 text-white md:flex">
      <div class="absolute -right-16 -top-16 h-64 w-64 rounded-full bg-white/10 blur-2xl"></div>
      <div class="absolute -bottom-20 -left-10 h-72 w-72 rounded-full bg-white/10 blur-3xl"></div>
      <div class="flex items-center gap-2">
        <img src="/favicon.png" class="h-10 w-10" alt="校徽" />
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

          <div v-if="social.wechat || social.wechat_web || social.qq" class="mt-4">
            <div class="flex items-center gap-3 text-xs text-[var(--text-soft)]">
              <span class="h-px flex-1 bg-[var(--border)]"></span>
              第三方账号登录
              <span class="h-px flex-1 bg-[var(--border)]"></span>
            </div>
            <div class="mt-3 flex gap-3">
              <button v-if="social.qq" @click="qqLogin"
                class="flex-1 inline-flex items-center justify-center gap-2 rounded-full bg-[#12B7F5] py-2.5 text-sm font-medium text-white transition hover:bg-[#0F9ED8]">
                <img src="/qq_bt_white_24X24.png" alt="QQ" class="h-5 w-5 shrink-0" draggable="false" />
                QQ 登录
              </button>
              <button v-if="social.wechat || social.wechat_web" @click="wechatLogin"
                class="flex-1 inline-flex items-center justify-center gap-2 rounded-full bg-[#07C160] py-2.5 text-sm font-medium text-white transition hover:bg-[#06AD56]">
                <img src="/wechat_logo_icon_147207.png" alt="微信" class="h-5 w-5 shrink-0" draggable="false" />
                微信登录
              </button>
            </div>
          </div>
        </div>

        <div class="mt-4 flex justify-between text-sm">
          <router-link to="/register" class="text-[var(--grape)]">注册账号</router-link>
          <router-link v-if="meta.site.features?.mailer" to="/forgot" class="text-[var(--text-soft)] hover:text-[var(--grape)]">忘记密码</router-link>
        </div>
      </div>
    </div>
  </div>

  <!-- 微信登录方式选择（仅当小程序与网页两种方式都可用时弹出） -->
  <el-dialog v-model="wechatChoose" title="微信登录" width="380px">
    <div class="space-y-3">
      <button v-if="social.wechat" class="flex w-full items-center gap-3 rounded-xl border border-[var(--border)] p-3 text-left hover:border-[var(--grape)]" @click="wechatMiniapp">
        <span class="text-lg">📱</span>
        <span>
          <span class="block font-medium">微信小程序登录</span>
          <span class="text-xs text-[var(--text-soft)]">在「泰兴中学校园墙」小程序内登录</span>
        </span>
      </button>
      <button v-if="social.wechat_web" class="flex w-full items-center gap-3 rounded-xl border border-[var(--border)] p-3 text-left hover:border-[var(--grape)]" @click="wechatWeb">
        <span class="text-lg">💻</span>
        <span>
          <span class="block font-medium">网页扫码登录</span>
          <span class="text-xs text-[var(--text-soft)]">使用微信扫码授权登录</span>
        </span>
      </button>
    </div>
  </el-dialog>

  <!-- 登录场景小程序码（与绑定场景独立） -->
  <MiniProgramQrDialog v-model="mpLoginDialog" title="微信登录 / 注册" :name="mp.name" :qrcode="mp.login_qrcode" :desc="mp.login_desc" />

  <!-- 网页扫码即登（方案 B：动态小程序码携带 scene，小程序确认后自动登录） -->
  <WebLoginDialog v-model="webLoginDialog" :name="mp.name" :desc="mp.login_desc" :fallback-qrcode="mp.login_qrcode" @success="onWebLoginOk" />
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
