<script setup lang="ts">
import { onMounted, ref, computed, watch } from 'vue'
import { Music, Send, Clock, User as UserIcon, Play, Ban } from 'lucide-vue-next'
import { ElMessage } from 'element-plus'
import { songApi } from '@/api'
import { useAuthStore } from '@/stores/auth'
import { useMetaStore } from '@/stores/meta'

const auth = useAuthStore()
const meta = useMetaStore()
// 后台「启用点歌」开关关闭时，本页兜底显示为已关闭（路由守卫已阻止进入，此处防止开关中途变化的已开页面）
const songEnabled = computed(() => meta.site?.song_enabled !== false)
const items = ref<any[]>([])
const total = ref(0)
const page = ref(1)
const pageSize = 20
const loading = ref(false)
const submitting = ref(false)
const quota = ref<any>(null)
const form = ref({ title: '', artist: '', note: '' })

const statusMap: Record<string, { text: string; cls: string }> = {
  pending: { text: '待播放', cls: 'bg-[var(--surface-2)] text-[var(--text-soft)]' },
  played: { text: '已播放', cls: 'bg-[var(--jade)]/15 text-[var(--jade)]' },
  canceled: { text: '已取消', cls: 'bg-[var(--danger)]/15 text-[var(--danger)]' },
}

async function load() {
  loading.value = true
  try {
    const p = await songApi.list({ page: page.value, page_size: pageSize })
    items.value = p.items || []
    total.value = p.total
  } finally {
    loading.value = false
  }
}

async function loadQuota() {
  if (!auth.isLoggedIn) return
  try {
    quota.value = await songApi.quota()
  } catch {
    quota.value = null
  }
}

async function submit() {
  if (!form.value.title.trim()) {
    ElMessage.warning('请填写歌名')
    return
  }
  submitting.value = true
  try {
    await songApi.create({
      title: form.value.title.trim(),
      artist: form.value.artist.trim(),
      note: form.value.note.trim(),
    })
    ElMessage.success('点歌成功，等待播放～')
    form.value = { title: '', artist: '', note: '' }
    page.value = 1
    await load()
    await loadQuota()
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || e?.message || '点歌失败')
  } finally {
    submitting.value = false
  }
}

function remainText() {
  if (!quota.value) return ''
  if (quota.value.limit === 0) return '本周不限次数'
  return `本周剩余 ${quota.value.remaining} / ${quota.value.limit} 首`
}

function canSubmit() {
  if (!quota.value) return true
  if (quota.value.limit === 0) return true
  return quota.value.remaining > 0
}

onMounted(() => {
  if (songEnabled.value) {
    load()
    loadQuota()
  }
})

// 后台中途切换「启用点歌」时，实时同步本页内容
watch(songEnabled, (on) => {
  if (on) {
    load()
    loadQuota()
  } else {
    items.value = []
    total.value = 0
  }
})
</script>

<template>
  <div class="mx-auto max-w-[900px] px-4 py-6">
    <div class="mb-5 flex items-end justify-between">
      <div>
        <h1 class="flex items-center gap-2 font-serif-sc text-2xl font-bold text-[var(--grape)]">
          <Music :size="24" /> 校园点歌台
        </h1>
        <p class="mt-1 text-sm text-[var(--text-soft)]">点一首喜欢的歌，让它响彻校园的午后。</p>
      </div>
      <span v-if="auth.isLoggedIn && quota && songEnabled" class="rounded-full bg-grape-50 px-3 py-1 text-xs text-[var(--grape)]">{{ remainText() }}</span>
    </div>

    <!-- 后台已关闭点歌：兜底提示（路由守卫已阻止进入，此处防开关中途变化） -->
    <div v-if="!songEnabled" class="paper-card p-10 text-center text-[var(--text-soft)]">
      <Ban :size="28" class="mx-auto mb-2 opacity-40" /> 点歌功能已关闭
    </div>

    <!-- 点歌表单 -->
    <div v-if="auth.isLoggedIn && songEnabled" class="paper-card mb-6 p-5">
      <div class="grid gap-3 sm:grid-cols-2">
        <div class="relative">
          <Music :size="16" class="absolute left-3 top-1/2 -translate-y-1/2 text-[var(--text-soft)]" />
          <input v-model="form.title" placeholder="歌名（必填）" class="input-box input-box--with-icon" @keyup.enter="submit" />
        </div>
        <div class="relative">
          <UserIcon :size="16" class="absolute left-3 top-1/2 -translate-y-1/2 text-[var(--text-soft)]" />
          <input v-model="form.artist" placeholder="歌手（选填）" class="input-box input-box--with-icon" />
        </div>
      </div>
      <textarea v-model="form.note" rows="2" placeholder="想说的话（选填，200 字内）" class="input-box mt-3 w-full"></textarea>
      <div class="mt-3 flex items-center justify-between">
        <span class="text-xs text-[var(--text-soft)]">{{ quota?.enabled === false ? '点歌功能未开启' : '提交后即进入播放列表' }}</span>
        <button
          class="btn-grape flex items-center gap-1 rounded-full px-5 py-2 text-sm disabled:cursor-not-allowed disabled:opacity-50"
          :disabled="submitting || !canSubmit() || quota?.enabled === false"
          @click="submit"
        >
          <Send :size="14" /> {{ submitting ? '提交中…' : '点歌' }}
        </button>
      </div>
      <p v-if="quota && !canSubmit()" class="mt-2 text-xs text-warn">本周点歌次数已用完，下周再来吧～</p>
    </div>
    <div v-else-if="!songEnabled" class="paper-card mb-6 p-4 text-center text-sm text-[var(--text-soft)]">
      点歌功能已关闭
    </div>
    <div v-else class="paper-card mb-6 p-4 text-center text-sm text-[var(--text-soft)]">
      请<router-link to="/login" class="mx-1 text-[var(--grape)]">登录</router-link>后点歌
    </div>

    <!-- 点歌列表 -->
    <div v-if="items.length && songEnabled" class="space-y-3">
      <div v-for="s in items" :key="s.id" class="paper-card flex items-start gap-3 p-4">
        <div class="flex h-10 w-10 shrink-0 items-center justify-center rounded-full bg-grape-50 text-[var(--grape)]">
          <Music :size="18" />
        </div>
        <div class="min-w-0 flex-1">
          <div class="flex flex-wrap items-center gap-2">
            <h3 class="truncate font-medium">{{ s.title }}</h3>
            <span v-if="s.artist" class="text-sm text-[var(--text-soft)]">— {{ s.artist }}</span>
            <span class="rounded-full px-2 py-0.5 text-xs" :class="(statusMap[s.status] || {}).cls">
              {{ (statusMap[s.status] || { text: s.status }).text }}
            </span>
          </div>
          <p v-if="s.note" class="mt-1 whitespace-pre-wrap text-sm text-[var(--text-soft)]">{{ s.note }}</p>
          <div class="mt-1.5 flex flex-wrap items-center gap-3 text-xs text-[var(--text-soft)]">
            <span class="inline-flex items-center gap-1"><UserIcon :size="12" /> {{ s.username || '匿名' }}</span>
            <span class="inline-flex items-center gap-1"><Clock :size="12" /> {{ s.created_at }}</span>
          </div>
        </div>
      </div>
    </div>
    <div v-else-if="!loading && songEnabled" class="paper-card p-10 text-center text-[var(--text-soft)]">
      <Ban :size="28" class="mx-auto mb-2 opacity-40" /> 还没有人点歌，来点一首吧～
    </div>

    <el-pagination
      v-if="total > pageSize"
      class="mt-5 justify-center"
      layout="prev, pager, next"
      :total="total"
      :page-size="pageSize"
      :current-page="page"
      @current-change="(p: number) => { page = p; load() }"
      background
    />
  </div>
</template>

<style scoped>
.input-box {
  border-radius: 10px;
  border: 1px solid var(--border);
  background: var(--surface-2);
  padding: 9px 12px;
  font-size: 14px;
  outline: none;
}
.input-box:focus {
  border-color: var(--grape);
}
/* 带图标输入框：左侧留出图标空间（置于 .input-box 之后以覆盖其 padding 简写） */
.input-box--with-icon {
  padding-left: 36px;
}
</style>
