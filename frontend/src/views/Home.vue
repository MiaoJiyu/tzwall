<script setup lang="ts">
import { onMounted, ref, watch, computed } from 'vue'
import { useRouter } from 'vue-router'
import { Flame, Clock, Plus, Search } from 'lucide-vue-next'
import { ElMessage, ElMessageBox } from 'element-plus'
import { submissionApi, adminApi, announcementApi } from '@/api'
import { useAuthStore } from '@/stores/auth'
import { useMetaStore } from '@/stores/meta'
import SubmissionCard from '@/components/SubmissionCard.vue'
import PollWidget from '@/components/PollWidget.vue'
import AnnouncementBoard from '@/components/AnnouncementBoard.vue'
import PenaltyDialog from '@/components/PenaltyDialog.vue'

const router = useRouter()
const auth = useAuthStore()
const meta = useMetaStore()

// 首页公告（常驻于校园热榜下方）；要求确认者首次展示时弹窗，按用户记录确认态。
const announcements = ref<any[]>([])
const confirmDialog = ref(false)
const confirmTarget = ref<any>(null)

async function loadAnnouncements() {
  try {
    const r = await announcementApi.list()
    announcements.value = r.items || []
    maybeConfirm()
  } catch {
    announcements.value = []
  }
}
// 找到首个“需确认且本人未确认”的公告，弹出确认窗口（仅登录用户；游客不弹）
function maybeConfirm() {
  if (!auth.isLoggedIn) return
  const target = announcements.value.find((a: any) => a.require_confirm && !a.confirmed)
  if (target) {
    confirmTarget.value = target
    confirmDialog.value = true
  }
}
async function confirmAnn() {
  if (!confirmTarget.value) return
  const id = confirmTarget.value.id
  try {
    await announcementApi.confirm(id)
  } catch {
    /* 忽略网络异常，仅本地标记 */
  }
  const t = announcements.value.find((a: any) => a.id === id)
  if (t) t.confirmed = true
  confirmTarget.value = null
  confirmDialog.value = false
  maybeConfirm() // 弹出下一个待确认
}
const items = ref<any[]>([])
const total = ref(0)
const page = ref(1)
// 首页每页投稿数：后台可配置（默认 10）
const pageSize = computed(() => Number(meta.site?.limits?.home_page_size) || 10)
const sort = ref('latest')
const keyword = ref('')
const activeTag = ref('')
const hot = ref<any[]>([])
const tags = ref<any[]>([])

async function load() {
  const p = await submissionApi.list({ sort: sort.value, keyword: keyword.value, tag: activeTag.value, page: page.value, page_size: pageSize.value })
  items.value = p.items
  total.value = p.total
}
async function loadHot() {
  const p = await submissionApi.list({ sort: 'hot', page: 1, page_size: 6 })
  hot.value = p.items
}
async function loadTags() {
  const p = await submissionApi.tags()
  tags.value = p.items || []
}

function changeSort(s: string) {
  sort.value = s
  page.value = 1
  load()
}
function pickTag(name: string) {
  activeTag.value = activeTag.value === name ? '' : name
  page.value = 1
  load()
}
function onSearch() {
  page.value = 1
  load()
}

watch(() => page.value, load)
// 后台调整每页数量后：回到第一页并重新加载
watch(pageSize, () => {
  page.value = 1
  load()
})

// 管理员快捷撤下 / 处罚
const showPenalty = ref(false)
const penaltyUser = ref<number | null>(null)
const penaltyName = ref('')
function onPenalty(item: any) {
  if (!item.user_id) return
  penaltyUser.value = item.user_id
  penaltyName.value = item.anonymous ? '匿名用户' : item.author
  showPenalty.value = true
}
async function onTakedown(item: any) {
  await ElMessageBox.confirm('确定快捷撤下该投稿？撤下后不再公开展示。', '快捷撤下', { type: 'warning' })
  await adminApi.reviewSubmission(item.id, { status: 'rejected', note: '管理员快捷撤下' })
  ElMessage.success('稿件已撤下')
  await load()
  await loadHot()
}
async function onPin(payload: { id: number; order: number; expiresAt: string }) {
  try {
    await adminApi.pinSubmission(payload.id, {
      order: payload.order,
      expires_at: payload.expiresAt || undefined,
    })
    ElMessage.success('已置顶')
    await load()
    await loadHot()
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '置顶失败')
  }
}
async function onUnpin(id: number) {
  try {
    await adminApi.unpinSubmission(id)
    ElMessage.success('已取消置顶')
    await load()
    await loadHot()
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '操作失败')
  }
}

onMounted(() => {
  load()
  loadHot()
  loadTags()
  loadAnnouncements()
})
</script>

<template>
  <div class="mx-auto max-w-[1200px] px-4 py-6">
    <!-- Hero -->
    <section class="mb-8 overflow-hidden rounded-xl2 border border-[var(--border)] bg-gradient-to-br from-grape-600 to-grape-700 p-8 text-white animate-fadeup">
      <h1 class="font-serif-sc text-3xl font-bold sm:text-4xl">校园墙</h1>
      <p class="mt-2 max-w-xl text-sm text-white/85">爱国 严谨 勤奋 奉献。在这里分享校园故事、求助与精彩瞬间。</p>
      <button v-if="auth.isLoggedIn" class="btn-grape mt-5 rounded-full bg-white/95 px-5 py-2.5 text-sm font-semibold text-grape-700" @click="router.push('/publish')">
        <Plus :size="16" class="inline" /> 发布投稿
      </button>
      <button v-else class="mt-5 rounded-full border border-white/60 px-5 py-2.5 text-sm font-semibold" @click="router.push('/login')">登录后发布</button>
    </section>

    <!-- 关联投票：全宽展示，为独立卡片框，不嵌套在单个投稿卡片内 -->
    <div class="mb-8"><PollWidget /></div>

    <div class="grid grid-cols-1 gap-6 lg:grid-cols-[1fr_320px]">
      <!-- 列表 -->
      <div>
        <div class="mb-4 flex flex-wrap items-center gap-3">
          <div class="flex rounded-full border border-[var(--border)] bg-[var(--surface)] p-1">
            <button :class="sort === 'latest' ? 'bg-grape-600 text-white' : 'text-[var(--text-soft)]'" class="flex items-center gap-1 rounded-full px-4 py-1.5 text-sm" @click="changeSort('latest')">
              <Clock :size="14" /> 最新
            </button>
            <button :class="sort === 'hot' ? 'bg-grape-600 text-white' : 'text-[var(--text-soft)]'" class="flex items-center gap-1 rounded-full px-4 py-1.5 text-sm" @click="changeSort('hot')">
              <Flame :size="14" /> 热门
            </button>
          </div>
          <div class="relative flex-1 min-w-[180px]">
            <Search :size="16" class="absolute left-3 top-1/2 -translate-y-1/2 text-[var(--text-soft)]" />
            <input v-model="keyword" @keyup.enter="onSearch" placeholder="搜索…" class="h-9 w-full rounded-full border border-[var(--border)] bg-[var(--surface)] pl-9 pr-4 text-sm outline-none focus:border-[var(--grape)]" />
          </div>
        </div>

        <div v-if="tags.length" class="mb-4 flex flex-wrap gap-2">
          <button v-for="t in tags" :key="t.id" @click="pickTag(t.name)" :class="activeTag === t.name ? 'bg-grape-600 text-white' : 'bg-[var(--surface-2)] text-[var(--text-soft)]'" class="rounded-full px-3 py-1 text-xs">
            #{{ t.name }}
          </button>
        </div>

        <div class="lg:hidden"><AnnouncementBoard :items="announcements" /></div>

        <div v-if="items.length" class="space-y-4">
          <SubmissionCard v-for="it in items" :key="it.id" :item="it" @penalty="onPenalty" @takedown="onTakedown" @pin="onPin" @unpin="onUnpin" />
        </div>
        <div v-else class="paper-card p-10 text-center text-[var(--text-soft)]">暂无投稿，快来发布第一条吧～</div>

        <el-pagination
          v-if="total > pageSize"
          class="mt-6 justify-center"
          layout="prev, pager, next"
          :total="total"
          :page-size="pageSize"
          :current-page="page"
          @current-change="(p: number) => (page = p)"
          background
        />
      </div>

      <!-- 侧栏热榜 -->
      <aside class="hidden lg:block">
        <div class="paper-card p-5">
          <h3 class="mb-3 flex items-center gap-1 font-serif-sc font-bold text-[var(--grape)]"><Flame :size="16" /> 校园热榜</h3>
          <ol class="space-y-3">
            <li v-for="(h, i) in hot" :key="h.id" class="flex cursor-pointer gap-2 text-sm" @click="router.push('/submission/' + h.id)">
              <span class="font-serif-sc text-lg font-bold text-grape-300">{{ i + 1 }}</span>
              <span class="line-clamp-2 text-[var(--text)] hover:text-[var(--grape)]">{{ h.title }}</span>
            </li>
          </ol>
        </div>
        <!-- 校园公告：常驻显示于校园热榜下方 -->
        <AnnouncementBoard :items="announcements" />
      </aside>
    </div>

    <PenaltyDialog v-model="showPenalty" :user-id="penaltyUser" :username="penaltyName" @done="load" />

    <!-- 要求确认公告首次展示时弹出，确认后按用户不再重复 -->
    <el-dialog v-model="confirmDialog" title="公告确认" width="420px" :close-on-click-modal="false">
      <p class="font-medium text-[var(--text)]">{{ confirmTarget?.title }}</p>
      <p class="mt-2 whitespace-pre-wrap text-sm leading-relaxed text-[var(--text-soft)]">{{ confirmTarget?.content }}</p>
      <template #footer>
        <div class="flex justify-end">
          <button class="btn-grape rounded-full px-5 py-2 text-sm font-medium text-white" @click="confirmAnn">我已阅读并确认</button>
        </div>
      </template>
    </el-dialog>
  </div>
</template>
