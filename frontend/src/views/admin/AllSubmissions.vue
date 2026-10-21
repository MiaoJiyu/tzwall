<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import { Search, Gavel, FileSearch, Eye, X, Ban, RotateCcw } from 'lucide-vue-next'
import { ElMessage } from 'element-plus'
import { adminApi, commentApi } from '@/api'
import PenaltyDialog from '@/components/PenaltyDialog.vue'

const router = useRouter()
const list = ref<any[]>([])
const page = ref(1)
const total = ref(0)
const loading = ref(false)
const filters = ref({ status: 'all', keyword: '' })

const statusMap: Record<string, { text: string; cls: string }> = {
  pending: { text: '待审核', cls: 'bg-warn text-white' },
  approved: { text: '已通过', cls: 'bg-[var(--jade)] text-white' },
  need_edit: { text: '需修改', cls: 'bg-warn text-white' },
  rejected: { text: '已驳回', cls: 'bg-[var(--danger)] text-white' },
}

// 处罚对话框
const showPenalty = ref(false)
const penaltyUser = ref<number | null>(null)
const penaltyName = ref('')

// 快捷查看弹窗
const viewItem = ref<any>(null)
const viewComments = ref<any[]>([])

async function load() {
  loading.value = true
  try {
    const p = await adminApi.allSubmissions({
      page: page.value,
      page_size: 20,
      status: filters.value.status,
      keyword: filters.value.keyword.trim(),
    })
    list.value = p.items || []
    total.value = p.total
  } finally {
    loading.value = false
  }
}

function search() {
  page.value = 1
  load()
}

function openPenalty(user: number | null, name: string) {
  if (!user) return
  penaltyUser.value = user
  penaltyName.value = name
  showPenalty.value = true
}

function flatten(nodes: any[], depth = 0): any[] {
  const out: any[] = []
  for (const n of nodes || []) {
    out.push({ ...n, _depth: depth })
    if (n.children?.length) out.push(...flatten(n.children, depth + 1))
  }
  return out
}

const confirmTakeDown = ref(false)

async function openView(s: any) {
  viewItem.value = s
  confirmTakeDown.value = false
  viewComments.value = []
  try {
    const p = await commentApi.list(s.id)
    viewComments.value = flatten(p.items || [])
  } catch {
    viewComments.value = []
  }
}

async function takeDownSubmission() {
  if (!viewItem.value) return
  await adminApi.reviewSubmission(viewItem.value.id, { status: 'rejected', note: '管理员快捷撤下' })
  viewItem.value.status = 'rejected'
  confirmTakeDown.value = false
  ElMessage.success('稿件已撤下')
  load()
}

async function takeDownComment(c: any) {
  await adminApi.removeComment(c.id)
  c.deleted = true
  ElMessage.success('评论已撤下')
}

async function restoreSubmission() {
  if (!viewItem.value) return
  await adminApi.restoreSubmission(viewItem.value.id)
  viewItem.value.status = 'approved'
  ElMessage.success('稿件已恢复')
  load()
}

async function restoreRow(s: any) {
  await adminApi.restoreSubmission(s.id)
  ElMessage.success('稿件已恢复')
  load()
}

async function restoreComment(c: any) {
  await adminApi.restoreComment(c.id)
  ElMessage.success('评论已恢复')
  c.deleted = false
}

function auditChip(a: any) {
  const kind = a.kind === 'ai' ? 'AI' : '腾讯云'
  const map: Record<string, { text: string; cls: string }> = {
    pass: { text: '正常', cls: 'border border-[var(--jade)] text-[var(--jade)]' },
    block: { text: '驳回', cls: 'border border-[var(--danger)] text-[var(--danger)]' },
    review: { text: '可疑', cls: 'border border-warn text-warn' },
    failed: { text: '异常', cls: 'border border-warn text-warn' },
  }
  const m = map[a.status] || { text: a.label || a.status, cls: 'border border-[var(--border)] text-[var(--text-soft)]' }
  return { kind, ...m }
}

onMounted(load)
</script>

<template>
  <div>
    <h1 class="mb-5 font-serif-sc text-2xl font-bold text-[var(--grape)]">所有稿件</h1>

    <div class="paper-card mb-4 flex flex-wrap items-center gap-3 p-4">
      <select v-model="filters.status" class="input-box w-36" @change="search">
        <option value="all">全部状态</option>
        <option value="pending">待审核</option>
        <option value="approved">已通过</option>
        <option value="need_edit">需修改</option>
        <option value="rejected">已驳回</option>
      </select>
      <div class="relative flex-1">
        <Search :size="16" class="absolute left-3 top-1/2 -translate-y-1/2 text-[var(--text-soft)]" />
        <input v-model="filters.keyword" class="input-box pl-9" placeholder="搜索标题或内容" @keyup.enter="search" />
      </div>
      <button class="btn-grape rounded-full px-5 py-2 text-sm" @click="search">搜索</button>
    </div>

    <div class="paper-card overflow-x-auto">
      <table class="w-full text-sm">
        <thead>
          <tr class="border-b border-[var(--border)] text-left text-[var(--text-soft)]">
            <th class="px-4 py-3 font-medium">ID</th>
            <th class="px-4 py-3 font-medium">标题</th>
            <th class="px-4 py-3 font-medium">作者</th>
            <th class="px-4 py-3 font-medium">状态</th>
            <th class="px-4 py-3 font-medium">机审</th>
            <th class="px-4 py-3 font-medium">提交时间</th>
            <th class="px-4 py-3 text-right font-medium">操作</th>
          </tr>
        </thead>
        <tbody>
          <tr
            v-for="s in list"
            :key="s.id"
            class="cursor-pointer border-b border-[var(--border)] last:border-0 hover:bg-[var(--surface-2)]"
            @click="openView(s)"
          >
            <td class="px-4 py-3 text-[var(--text-soft)]">#{{ s.id }}</td>
            <td class="max-w-[240px] px-4 py-3">
              <div class="truncate font-medium text-[var(--grape)]">{{ s.title || '(无标题)' }} <span v-if="s.videos?.length" class="ml-1 text-[var(--danger)]">🎬{{ s.videos.length }}</span></div>
              <div class="truncate text-xs text-[var(--text-soft)]">{{ s.content }}</div>
            </td>
            <td class="px-4 py-3">
              <span v-if="s.anonymous" class="text-[var(--text-soft)]">匿名</span>
              <span v-else>{{ s.author }}</span>
            </td>
            <td class="px-4 py-3">
              <span class="rounded-full px-2 py-0.5 text-xs text-white" :class="(statusMap[s.status] || { cls: 'bg-[var(--text-soft)]' }).cls">
                {{ (statusMap[s.status] || { text: s.status }).text }}
              </span>
            </td>
            <td class="px-4 py-3">
              <div v-if="s.audit?.length" class="flex flex-wrap gap-1">
                <span v-for="(a, i) in s.audit" :key="i" class="rounded-full px-1.5 py-0.5 text-[11px]" :class="auditChip(a).cls">
                  {{ auditChip(a).kind }}·{{ auditChip(a).text }}
                </span>
              </div>
              <span v-else class="text-xs text-[var(--text-soft)]">—</span>
            </td>
            <td class="whitespace-nowrap px-4 py-3 text-xs text-[var(--text-soft)]">{{ s.created_at }}</td>
            <td class="px-4 py-3">
              <div class="flex items-center justify-end gap-2">
                <button class="flex items-center gap-1 rounded-full border border-[var(--border)] px-3 py-1 text-xs hover:text-[var(--grape)]" @click.stop="openView(s)">
                  <Eye :size="13" /> 查看
                </button>
                <button v-if="s.status === 'pending'" class="flex items-center gap-1 rounded-full border border-[var(--border)] px-3 py-1 text-xs hover:text-[var(--grape)]" @click.stop="router.push('/admin/submissions')">
                  <FileSearch :size="13" /> 审核
                </button>
                <button v-if="s.status === 'rejected'" class="flex items-center gap-1 rounded-full bg-[var(--jade)] px-3 py-1 text-xs text-white" @click.stop="restoreRow(s)">
                  <RotateCcw :size="13" /> 恢复
                </button>
                <button v-if="s.user_id" class="flex items-center gap-1 rounded-full border border-[var(--border)] px-3 py-1 text-xs hover:text-[var(--danger)]" @click.stop="openPenalty(s.user_id, s.anonymous ? '匿名用户' : s.author)">
                  <Gavel :size="13" /> 处罚作者
                </button>
              </div>
            </td>
          </tr>
          <tr v-if="!loading && !list.length">
            <td colspan="7" class="px-4 py-10 text-center text-[var(--text-soft)]">暂无稿件</td>
          </tr>
        </tbody>
      </table>
    </div>

    <el-pagination
      v-if="total > 20"
      class="mt-4 justify-center"
      layout="prev, pager, next"
      :total="total"
      :page-size="20"
      :current-page="page"
      @current-change="(p: number) => { page = p; load() }"
      background
    />

    <!-- 快捷查看弹窗（含投稿与评论的撤下/处罚） -->
    <div v-if="viewItem" class="fixed inset-0 z-[9995] flex items-start justify-center overflow-auto bg-black/55 px-4 py-8">
      <div class="paper-card w-full max-w-2xl p-6">
        <div class="flex items-start justify-between">
          <div>
            <h3 class="font-serif-sc text-xl font-bold">{{ viewItem.title || '(无标题)' }}</h3>
            <div class="mt-1 flex flex-wrap items-center gap-2 text-xs text-[var(--text-soft)]">
              <span class="rounded-full bg-[var(--surface-2)] px-2 py-0.5 text-[var(--grape)]">{{ viewItem.anonymous ? '匿名用户' : viewItem.author }}</span>
              <span>{{ viewItem.created_at }}</span>
              <span class="rounded-full px-2 py-0.5 text-white" :class="(statusMap[viewItem.status] || { cls: 'bg-[var(--text-soft)]' }).cls">{{ (statusMap[viewItem.status] || { text: viewItem.status }).text }}</span>
            </div>
          </div>
          <button class="text-[var(--text-soft)] hover:text-[var(--text)]" @click="viewItem = null"><X :size="18" /></button>
        </div>

        <div v-if="viewItem.audit?.length" class="mt-3 flex flex-wrap items-center gap-2 text-xs">
          <span class="text-[var(--text-soft)]">机审：</span>
          <span v-for="(a, i) in viewItem.audit" :key="i" class="rounded-full px-2 py-0.5" :class="auditChip(a).cls">{{ auditChip(a).kind }}·{{ auditChip(a).text }}</span>
          <span v-for="(a, i) in viewItem.audit.filter((x: any) => x.status === 'block' && x.reason)" :key="'r' + i" class="text-[var(--danger)]">（理由：{{ a.reason }}）</span>
        </div>

        <!-- AI 审核评论（仅管理员可见）：通过/拒绝原因 -->
        <div v-if="viewItem.audit_comment" class="mt-3 rounded-lg bg-[var(--surface-2)] px-3 py-2 text-xs text-[var(--grape)]">
          <span class="font-semibold">AI 审核意见：</span>{{ viewItem.audit_comment }}
        </div>

        <p v-if="viewItem.content" class="mt-3 whitespace-pre-wrap text-sm leading-relaxed text-[var(--text-soft)]">{{ viewItem.content }}</p>
        <div v-if="viewItem.images?.length" class="mt-3 grid grid-cols-3 gap-2">
          <img v-for="im in viewItem.images" :key="im.id" :src="im.url" class="h-28 w-full rounded-lg object-cover" alt="" @error="$event.target.style.display = 'none'" />
        </div>
        <div v-if="viewItem.videos?.length" class="mt-3 grid grid-cols-2 gap-2">
          <video v-for="v in viewItem.videos" :key="v.id" :src="v.url" controls preload="metadata" class="h-40 w-full rounded-lg object-cover bg-black"></video>
        </div>
        <div v-if="viewItem.tags?.length" class="mt-2 flex flex-wrap gap-1.5">
          <span v-for="t in viewItem.tags" :key="t.id" class="rounded-full bg-grape-50 px-2 py-0.5 text-xs text-[var(--grape)]">#{{ t.name }}</span>
        </div>

        <div class="mt-4 flex flex-wrap items-center gap-2 border-t border-[var(--border)] pt-3">
          <template v-if="viewItem.status !== 'rejected'">
            <template v-if="!confirmTakeDown">
              <button class="flex items-center gap-1 rounded-full bg-[var(--danger)] px-4 py-1.5 text-sm text-white" @click="confirmTakeDown = true"><Ban :size="14" /> 快捷撤下稿件</button>
            </template>
            <template v-else>
              <span class="text-sm text-[var(--danger)]">确认撤下该稿件？</span>
              <button class="rounded-full bg-[var(--danger)] px-4 py-1.5 text-sm text-white" @click="takeDownSubmission">确认撤下</button>
              <button class="rounded-full border border-[var(--border)] px-4 py-1.5 text-sm" @click="confirmTakeDown = false">取消</button>
            </template>
          </template>
          <button v-else class="flex items-center gap-1 rounded-full bg-[var(--jade)] px-4 py-1.5 text-sm text-white" @click="restoreSubmission"><RotateCcw :size="14" /> 恢复稿件</button>
          <button v-if="viewItem.user_id" class="flex items-center gap-1 rounded-full border border-[var(--border)] px-4 py-1.5 text-sm hover:text-[var(--danger)]" @click="openPenalty(viewItem.user_id, viewItem.anonymous ? '匿名用户' : viewItem.author)"><Gavel :size="14" /> 处罚作者</button>
        </div>

        <!-- 评论管理 -->
        <div class="mt-5">
          <h4 class="mb-2 text-sm font-semibold">评论（{{ viewComments.length }}）</h4>
          <div v-if="viewComments.length" class="space-y-2">
            <div
              v-for="c in viewComments"
              :key="c.id"
              class="rounded-lg border border-[var(--border)] p-3"
              :style="{ marginLeft: (c._depth || 0) * 16 + 'px' }"
            >
              <div class="flex items-center gap-2 text-xs text-[var(--text-soft)]">
                <span class="font-medium text-[var(--text)]">{{ c.deleted ? '匿名用户' : c.username }}</span>
                <span>{{ c.created_at }}</span>
                <span v-if="c.deleted" class="rounded bg-[var(--surface-2)] px-1.5 text-[var(--text-soft)]">已删除</span>
              </div>
              <div class="mt-1 text-sm" :class="c.deleted ? 'text-[var(--text-soft)]' : ''">
                <template v-if="c.deleted">【已删除】{{ c.content }}</template>
                <template v-else>{{ c.content }}</template>
              </div>
              <div class="mt-1.5 flex gap-2">
                <button v-if="!c.deleted" class="flex items-center gap-1 rounded-full border border-[var(--border)] px-2.5 py-0.5 text-xs hover:text-[var(--danger)]" @click="takeDownComment(c)"><Ban :size="12" /> 撤下</button>
                <button v-else class="flex items-center gap-1 rounded-full border border-[var(--border)] px-2.5 py-0.5 text-xs hover:text-[var(--jade)]" @click="restoreComment(c)"><RotateCcw :size="12" /> 恢复</button>
                <button v-if="c.user_id" class="flex items-center gap-1 rounded-full border border-[var(--border)] px-2.5 py-0.5 text-xs hover:text-[var(--danger)]" @click="openPenalty(c.user_id, c.username || '用户')"><Gavel :size="12" /> 处罚</button>
              </div>
            </div>
          </div>
          <p v-else class="text-xs text-[var(--text-soft)]">暂无评论</p>
        </div>
      </div>
    </div>

    <PenaltyDialog v-model="showPenalty" :user-id="penaltyUser" :username="penaltyName" @done="load" />
  </div>
</template>

<style scoped>
.input-box {
  border-radius: 10px;
  border: 1px solid var(--border);
  background: var(--surface-2);
  padding: 8px 12px;
  font-size: 14px;
  outline: none;
}
.input-box:focus {
  border-color: var(--grape);
}
</style>
