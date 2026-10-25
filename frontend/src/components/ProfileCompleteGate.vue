<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { UserRoundCheck } from 'lucide-vue-next'
import { ElMessage } from 'element-plus'
import { authApi } from '@/api'
import { useAuthStore } from '@/stores/auth'
import { useMetaStore } from '@/stores/meta'

const auth = useAuthStore()
const meta = useMetaStore()

const realName = ref('')
const school = ref('')
const saving = ref(false)

// 是否需要强制完善：已登录 + 后台开关开启 + 微信/QQ 注册或登录的账号 + 真实姓名或学校未填写
const need = computed(() => {
  if (!auth.isLoggedIn) return false
  if (meta.site?.social_profile_required === false) return false
  const u: any = auth.user || {}
  const social = !!u.wechat_bound || !!u.qq_bound
  if (!social) return false
  return !u.real_name_filled || !u.school_filled
})

async function submit() {
  const rn = realName.value.trim()
  const sc = school.value.trim()
  if (rn.length < 2 || rn.length > 32) {
    ElMessage.warning('请填写真实姓名（2-32 字）')
    return
  }
  if (sc.length < 2 || sc.length > 64) {
    ElMessage.warning('请填写就读学校（2-64 字）')
    return
  }
  saving.value = true
  try {
    await authApi.updateProfile({ real_name: rn, school: sc })
    // 刷新用户资料：filled 标志变为 true 后自动放行
    const fresh = await authApi.me()
    auth.setUser(fresh)
    ElMessage.success('资料已完善，感谢配合')
    realName.value = ''
    school.value = ''
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '提交失败，请稍后再试')
  } finally {
    saving.value = false
  }
}

// 进入拦截态时锁定页面滚动，退出时恢复
watch(need, (v) => {
  document.body.style.overflow = v ? 'hidden' : ''
})
</script>

<template>
  <div
    v-if="need"
    class="fixed inset-0 z-[9998] flex items-center justify-center overflow-y-auto bg-black/60 px-4 py-8"
  >
    <div class="paper-card w-full max-w-md p-6 animate-fadeup">
      <div class="flex items-center gap-2">
        <UserRoundCheck :size="22" class="text-[var(--grape)]" />
        <h3 class="font-serif-sc text-lg font-bold">完善账号信息</h3>
      </div>
      <p class="mt-2 text-sm leading-relaxed text-[var(--text-soft)]">
        为保障校园墙社区秩序，通过<b class="text-[var(--text)]">微信 / QQ</b> 注册或登录的账号，
        需补充<b class="text-[var(--text)]">真实姓名</b>与<b class="text-[var(--text)]">就读学校</b>后方可继续使用网页功能。
        信息仅用于身份核验，不对外公开。
      </p>
      <div class="mt-4 space-y-3">
        <div>
          <label class="mb-1 block text-sm font-medium">真实姓名</label>
          <input
            v-model="realName"
            class="w-full rounded-xl border border-[var(--border)] bg-[var(--surface-2)] px-3 py-2.5 text-sm outline-none focus:border-[var(--grape)]"
            placeholder="请输入真实姓名"
            maxlength="32"
          />
        </div>
        <div>
          <label class="mb-1 block text-sm font-medium">就读学校</label>
          <input
            v-model="school"
            class="w-full rounded-xl border border-[var(--border)] bg-[var(--surface-2)] px-3 py-2.5 text-sm outline-none focus:border-[var(--grape)]"
            placeholder="如：江苏省泰兴中学"
            maxlength="64"
          />
        </div>
      </div>
      <button
        class="btn-grape mt-5 w-full rounded-full py-2.5 text-sm font-medium disabled:opacity-60"
        :disabled="saving"
        @click="submit"
      >
        {{ saving ? '提交中…' : '提交并继续' }}
      </button>
      <p class="mt-3 text-center text-xs text-[var(--text-soft)]">如有疑问请联系管理员处理</p>
    </div>
  </div>
</template>
