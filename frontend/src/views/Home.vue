<script setup lang="ts">
import { onMounted, ref, watch } from 'vue'
import { useRouter } from 'vue-router'
import { Flame, Clock, Plus, Search } from 'lucide-vue-next'
import { ElMessage, ElMessageBox } from 'element-plus'
import { submissionApi, adminApi } from '@/api'
import { useAuthStore } from '@/stores/auth'
import SubmissionCard from '@/components/SubmissionCard.vue'
import PollWidget from '@/components/PollWidget.vue'
import PenaltyDialog from '@/components/PenaltyDialog.vue'

const router = useRouter()
const auth = useAuthStore()
const items = ref<any[]>([])
const total = ref(0)
const page = ref(1)
const pageSize = 20
const sort = ref('latest')
const keyword = ref('')
const activeTag = ref('')
const hot = ref<any[]>([])
const tags = ref<any[]>([])

async function load() {
  const p = await submissionApi.list({ sort: sort.value, keyword: keyword.value, tag: activeTag.value, page: page.value, page_size: pageSize })
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

onMounted(() => {
  load()
  loadHot()
  loadTags()
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

        <PollWidget />

        <div v-if="items.length" class="space-y-4">
          <SubmissionCard v-for="it in items" :key="it.id" :item="it" @penalty="onPenalty" @takedown="onTakedown" />
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
      </aside>
    </div>

    <PenaltyDialog v-model="showPenalty" :user-id="penaltyUser" :username="penaltyName" @done="load" />
  </div>
</template>
