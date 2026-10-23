<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { useRoute } from 'vue-router'
import { MessageSquare } from 'lucide-vue-next'
import { ElMessage } from 'element-plus'
import { userApi, chatApi } from '@/api'
import { useAuthStore } from '@/stores/auth'
import { useRouter } from 'vue-router'
import SubmissionCard from '@/components/SubmissionCard.vue'

const route = useRoute()
const router = useRouter()
const auth = useAuthStore()
const id = Number(route.params.id)
const profile = ref<any>(null)
const notPublic = ref(false)
const fieldLabel: Record<string, string> = { wechat: '微信号', qq: 'QQ', phone: '手机号', school: '学校', real_name: '真实姓名' }

async function load() {
  try {
    profile.value = await userApi.public(id)
  } catch (e: any) {
    notPublic.value = true
  }
}
async function startChat() {
  if (!auth.isLoggedIn) {
    router.push('/login')
    return
  }
  try {
    const r = await chatApi.open(id)
    router.push('/messages/' + r.id)
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '无法发起会话')
  }
}
onMounted(load)
</script>

<template>
  <div class="mx-auto max-w-3xl px-4 py-6">
    <div v-if="notPublic" class="paper-card p-10 text-center text-[var(--text-soft)]">该用户未开启公开主页或主页不存在。</div>

    <template v-else-if="profile">
      <div class="paper-card mb-5 flex items-center gap-4 p-6">
        <img v-if="profile.avatar" :src="profile.avatar" class="h-16 w-16 rounded-full object-cover" alt="" />
        <div v-else class="flex h-16 w-16 items-center justify-center rounded-full bg-grape-100 text-2xl font-semibold text-[var(--grape)]">{{ (profile.username || '?')[0] }}</div>
        <div class="flex-1">
          <div class="flex items-center gap-2">
            <h1 class="font-serif-sc text-xl font-bold">{{ profile.nickname || profile.username }}</h1>
            <span v-if="profile.title" class="inline-flex items-center rounded-full border border-[var(--grape)]/30 bg-[var(--grape)]/10 px-1.5 py-0.5 text-[10px] font-medium text-[var(--grape)]">{{ profile.title }}</span>
          </div>
          <p class="text-sm text-[var(--text-soft)]">@{{ profile.username }}</p>
          <div v-if="Object.keys(profile.fields || {}).length" class="mt-2 flex flex-wrap gap-x-4 gap-y-1 text-xs text-[var(--text-soft)]">
            <span v-for="(v, k) in profile.fields" :key="k">{{ fieldLabel[k] || k }}：{{ v }}</span>
          </div>
        </div>
        <button v-if="auth.isLoggedIn && auth.user?.id !== profile.id" class="btn-grape inline-flex items-center gap-1 self-start rounded-full px-4 py-2 text-sm" @click="startChat">
          <MessageSquare :size="15" /> 私聊
        </button>
      </div>

      <h2 class="mb-3 font-serif-sc text-lg font-bold">历史发帖（{{ profile.posts?.length || 0 }}）</h2>
      <div v-if="profile.posts?.length" class="space-y-4">
        <SubmissionCard v-for="p in profile.posts" :key="p.id" :item="p" />
      </div>
      <div v-else class="paper-card p-10 text-center text-[var(--text-soft)]">暂无公开发帖（匿名帖不在此展示）。</div>
    </template>
  </div>
</template>
