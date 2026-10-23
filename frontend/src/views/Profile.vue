<script setup lang="ts">
import { ref, reactive, onMounted } from 'vue'
import { useRoute } from 'vue-router'
import { ElMessage, ElMessageBox } from 'element-plus'
import { useAuthStore } from '@/stores/auth'
import { useMetaStore } from '@/stores/meta'
import { authApi } from '@/api'

const auth = useAuthStore()
const meta = useMetaStore()
const route = useRoute()

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

// 第三方账号绑定
async function refreshUser() {
  try {
    const fresh = await authApi.me()
    auth.setUser(fresh)
  } catch {
    /* 忽略 */
  }
}
async function bindQQ() {
  try {
    const r: any = await authApi.qqBindUrl()
    window.location.href = r.url
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '发起 QQ 绑定失败，请确认已配置 QQ 互联')
  }
}
const bindCodeDialog = ref(false)
const bindCodeData = ref<any>({ code: '', expires_min: 10 })
async function bindWechat() {
  const canWeb = !!meta.site?.features?.social?.wechat_web
  const canMp = !!meta.site?.features?.social?.wechat
  if (canWeb) {
    // 已配置微信网页应用：走网页扫码绑定
    try {
      const r: any = await authApi.wechatWebBindUrl()
      window.location.href = r.url
    } catch (e: any) {
      ElMessage.error(e?.response?.data?.message || '发起微信网页绑定失败')
    }
    return
  }
  if (canMp) {
    // 仅小程序：生成绑定码，在小程序内完成绑定
    try {
      const r: any = await authApi.wechatBindCode()
      bindCodeData.value = r
      bindCodeDialog.value = true
    } catch (e: any) {
      ElMessage.error(e?.response?.data?.message || '生成微信绑定码失败')
    }
    return
  }
  ElMessage.warning('微信登录/绑定暂未开放')
}
async function checkBindStatus() {
  await refreshUser()
  if (auth.user?.wechat_bound) {
    ElMessage.success('微信绑定成功')
    bindCodeDialog.value = false
  } else {
    ElMessage.info('尚未检测到绑定，请在小程序内完成绑定后再点「刷新」')
  }
}
async function unbind(provider: 'wechat' | 'qq') {
  const name = provider === 'wechat' ? '微信' : 'QQ'
  try {
    await ElMessageBox.confirm(`确认解绑${name}？解绑后将无法使用${name}快捷登录。`, '解绑确认', { type: 'warning' })
  } catch {
    return
  }
  try {
    await authApi.unbindProvider({ provider })
    await refreshUser()
    ElMessage.success(`已解绑${name}`)
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '解绑失败（请先设置密码或保留其他登录方式）')
  }
}

onMounted(async () => {
  const q = route.query
  if (q.bind_ok) {
    ElMessage.success((q.bind === 'wechat' ? '微信' : 'QQ') + ' 绑定成功')
  } else if (q.bind_error) {
    ElMessage.error('绑定失败：' + q.bind_error)
  }
  await refreshUser()
})
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
              <div v-else class="flex h-12 w-12 items-center justify-center rounded-full bg-grape-100 text-lg font-semibold text-[var(--grape)]">{{ (auth.user?.nickname || auth.user?.username || '?')[0] }}</div>
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
        <h2 class="mb-1 font-semibold">第三方账号绑定</h2>
        <p class="mb-3 text-sm text-[var(--text-soft)]">绑定后可用微信 / QQ 快捷登录，第三方 OAuth 登录无需邮箱验证。解绑前请确保已设置密码或保留其他登录方式。</p>
        <div class="space-y-3">
          <div class="flex items-center justify-between border-b border-[var(--border)] pb-3">
            <div>
              <span class="font-medium">微信</span>
              <span v-if="auth.user?.wechat_bound" class="ml-2 rounded bg-[var(--jade)]/15 px-2 py-0.5 text-xs text-[var(--jade)]">已绑定</span>
              <span v-else class="ml-2 rounded bg-[var(--surface-2)] px-2 py-0.5 text-xs text-[var(--text-soft)]">未绑定</span>
            </div>
            <button v-if="auth.user?.wechat_bound" class="rounded-full border border-[var(--border)] px-4 py-1.5 text-sm hover:text-[var(--danger)]" @click="unbind('wechat')">解绑</button>
            <button v-else class="btn-grape rounded-full px-4 py-1.5 text-sm" @click="bindWechat">绑定微信</button>
          </div>
          <div class="flex items-center justify-between">
            <div>
              <span class="font-medium">QQ</span>
              <span v-if="auth.user?.qq_bound" class="ml-2 rounded bg-[var(--jade)]/15 px-2 py-0.5 text-xs text-[var(--jade)]">已绑定</span>
              <span v-else class="ml-2 rounded bg-[var(--surface-2)] px-2 py-0.5 text-xs text-[var(--text-soft)]">未绑定</span>
            </div>
            <button v-if="auth.user?.qq_bound" class="rounded-full border border-[var(--border)] px-4 py-1.5 text-sm hover:text-[var(--danger)]" @click="unbind('qq')">解绑</button>
            <button v-else class="btn-grape rounded-full px-4 py-1.5 text-sm" @click="bindQQ">绑定 QQ</button>
          </div>
        </div>
      </div>

      <!-- 微信绑定码（仅配置小程序、未配置网页微信时使用） -->
      <el-dialog v-model="bindCodeDialog" title="绑定微信" width="420px">
        <div class="space-y-3 text-sm">
          <p class="text-[var(--text-soft)]">当前未配置微信网页授权。请在微信小程序中完成绑定：</p>
          <ol class="list-decimal space-y-1 pl-5 text-[var(--text-soft)]">
            <li>打开「泰兴中学校园墙」微信小程序</li>
            <li>进入「我的 → 绑定微信」</li>
            <li>输入下方绑定码完成绑定</li>
          </ol>
          <div class="rounded-lg border border-[var(--border)] bg-[var(--surface-2)] p-4 text-center">
            <div class="font-mono text-2xl font-bold tracking-widest text-[var(--grape)]">{{ bindCodeData.code }}</div>
            <div class="mt-1 text-xs text-[var(--text-soft)]">10 分钟内有效，一次使用</div>
          </div>
        </div>
        <template #footer>
          <div class="flex justify-end gap-2">
            <button class="rounded-full px-4 py-1.5 text-sm" @click="bindCodeDialog = false">关闭</button>
            <button class="btn-grape rounded-full px-4 py-1.5 text-sm text-white" @click="checkBindStatus">我已绑定，刷新</button>
          </div>
        </template>
      </el-dialog>

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
