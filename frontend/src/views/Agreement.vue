<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import { useAuthStore } from '@/stores/auth'
import { useMetaStore } from '@/stores/meta'
import { metaApi, authApi } from '@/api'

const auth = useAuthStore()
const meta = useMetaStore()
const router = useRouter()
const list = ref<any[]>([])
const loading = ref(false)
const submitting = ref(false)

onMounted(async () => {
  loading.value = true
  try {
    await meta.fetchAgreements()
    // 逐个加载协议正文，展示全部协议
    const items = await Promise.all(
      meta.agreements.map(async (a: any) => {
        try {
          return await metaApi.agreement(a.key)
        } catch {
          return { key: a.key, title: a.title, version: a.version, content: '' }
        }
      }),
    )
    list.value = items
  } finally {
    loading.value = false
  }
})

async function confirm() {
  submitting.value = true
  try {
    await authApi.confirmAgreement()
    const latestVersion = meta.agreements[0]?.version || list.value[0]?.version
    auth.setUser({ ...auth.user, agreement_version: latestVersion })
    ElMessage.success('已确认最新协议')
    router.back()
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || e?.message || '确认失败，请重试')
  } finally {
    submitting.value = false
  }
}
</script>

<template>
  <div class="mx-auto max-w-3xl px-4 py-6">
    <h1 class="mb-1 font-serif-sc text-2xl font-bold text-[var(--grape)]">社区协议</h1>
    <p class="mb-4 text-sm text-[var(--text-soft)]">请阅读以下全部协议，确认后方可继续使用校园墙。</p>

    <div v-if="list.length" class="space-y-4">
      <div v-for="(a, i) in list" :key="a.key || i" class="paper-card p-6">
        <div class="mb-3 flex items-center justify-between">
          <h2 class="font-semibold text-[var(--grape)]">{{ a.title }}</h2>
          <span class="text-xs text-[var(--text-soft)]">版本 {{ a.version }}</span>
        </div>
        <div class="prose-sm max-h-[42vh] overflow-auto whitespace-pre-wrap text-sm leading-relaxed text-[var(--text-soft)]">
          {{ a.content || '（暂无正文内容）' }}
        </div>
      </div>

      <div class="sticky bottom-0 rounded-xl border border-[var(--border)] bg-[var(--surface)] p-3">
        <button class="btn-grape w-full rounded-full py-2.5 text-sm" :disabled="submitting" @click="confirm">
          {{ submitting ? '提交中…' : '我已阅读并同意以上全部协议' }}
        </button>
      </div>
    </div>
    <div v-else class="paper-card p-10 text-center text-[var(--text-soft)]">
      {{ loading ? '加载中…' : '暂无协议内容' }}
    </div>
  </div>
</template>
