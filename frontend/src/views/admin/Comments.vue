<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import { Search, Ban, RotateCcw, Gavel, Trash2, MessageSquare } from 'lucide-vue-next'
import { ElMessage, ElMessageBox } from 'element-plus'
import { adminApi, punishmentApi } from '@/api'

const router = useRouter()

// ── 筛选 ─────────────────────────────────────────────
const q = ref({ keyword: '', status: '', submission_id: '', author: '' })
const items = ref<any[]>([])
const total = ref(0)
const page = ref(1)
const pageSize = 20
const loading = ref(false)

async function load() {
  loading.value = true
  try {
    const params: any = { page: page.value, page_size: pageSize }
    if (q.value.keyword.trim()) params.keyword = q.value.keyword.trim()
    if (q.value.status) params.status = q.value.status
    if (q.value.submission_id.trim()) params.submission_id = q.value.submission_id.trim()
    if (q.value.author.trim()) params.author = q.value.author.trim()
    const d = await adminApi.comments(params)
    items.value = d.items || []
    total.value = d.total || 0
  } catch {
    /* 错误已由拦截器提示 */
  } finally {
    loading.value = false
  }
}
function search() {
  page.value = 1
  load()
}
function reset() {
  q.value = { keyword: '', status: '', submission_id: '', author: '' }
  page.value = 1
  load()
}

// ── 状态展示 ─────────────────────────────────────────
function statusOf(r: any) {
  if (r.deleted) return { key: 'deleted', label: '已撤回', cls: 'bg-[var(--surface-2)] text-[var(--text-soft)]' }
  if (r.audit_status === 'block') return { key: 'blocked', label: '已屏蔽', cls: 'bg-[var(--danger)]/15 text-[var(--danger)]' }
  return { key: 'normal', label: '正常', cls: 'bg-[var(--jade)]/15 text-[var(--jade)]' }
}
function fmtTime(t: string) {
  return t ? String(t).replace('T', ' ').slice(0, 16) : '—'
}

// ── 撤回 / 恢复 ──────────────────────────────────────
async function withdraw(r: any) {
  try {
    await ElMessageBox.confirm(
      `确认撤回该评论？撤回后公开展示为「内容已删除」，正文仍保留供追溯。\n\n评论 #${r.id}（作者：${r.author}）`,
      '撤回评论',
      { type: 'warning', confirmButtonText: '撤回', cancelButtonText: '取消' },
    )
  } catch {
    return
  }
  await adminApi.removeComment(r.id)
  ElMessage.success('已撤回，即时生效')
  load()
}
async function restore(r: any) {
  try {
    await ElMessageBox.confirm(`确认恢复评论 #${r.id}？恢复后将重新公开展示。`, '恢复评论', { type: 'warning' })
  } catch {
    return
  }
  await adminApi.restoreComment(r.id)
  ElMessage.success('已恢复，即时生效')
  load()
}

// ── 处罚作者 ─────────────────────────────────────────
const punishDialog = ref(false)
const punishRow = ref<any>(null)
const punishForm = ref({ type: 'mute', reason: '', permanent: false, expires_at: '', also_withdraw: true })

// 各处罚类型的影响范围（弹窗中明确告知）
const punishTypes = [
  { value: 'warn', label: '警告', impact: '仅弹窗提醒用户，不限制任何功能' },
  { value: 'mute', label: '禁言', impact: '禁止投稿、评论、点赞、分享、举报（到期或撤销后自动解除）' },
  { value: 'ban_user', label: '封禁账号', impact: '账号完全无法登录与使用站点，直到撤销' },
]
function impactOf(type: string) {
  return punishTypes.find((t) => t.value === type)?.impact || ''
}
function openPunish(r: any) {
  punishRow.value = r
  punishForm.value = { type: 'mute', reason: '', permanent: false, expires_at: '', also_withdraw: true }
  punishDialog.value = true
}
async function submitPunish() {
  const r = punishRow.value
  if (!r) return
  if (!punishForm.value.reason.trim()) {
    ElMessage.warning('请填写处罚理由')
    return
  }
  if (!punishForm.value.permanent && !punishForm.value.expires_at) {
    ElMessage.warning('请选择到期时间，或勾选「永久」')
    return
  }
  const typeLabel = punishTypes.find((t) => t.value === punishForm.value.type)?.label || punishForm.value.type
  const duration = punishForm.value.permanent ? '永久' : `至 ${punishForm.value.expires_at.replace('T', ' ')}`
  // 二次确认：明确影响范围
  try {
    await ElMessageBox.confirm(
      `将对用户「${r.author}」（#${r.user_id}）执行：\n\n` +
        `· 处罚类型：${typeLabel}\n` +
        `· 影响范围：${impactOf(punishForm.value.type)}\n` +
        `· 时长：${duration}\n` +
        `· 理由：${punishForm.value.reason.trim()}\n` +
        (punishForm.value.also_withdraw ? `· 同时撤回该评论 #${r.id}\n` : '') +
        `\n确认执行？`,
      '处罚确认',
      { type: 'warning', confirmButtonText: '确认执行', cancelButtonText: '取消', customClass: 'punish-confirm' },
    )
  } catch {
    return
  }
  try {
    await punishmentApi.create({
      user_id: r.user_id,
      type: punishForm.value.type,
      reason: punishForm.value.reason.trim(),
      permanent: punishForm.value.permanent,
      expires_at: punishForm.value.permanent ? undefined : punishForm.value.expires_at,
    })
    if (punishForm.value.also_withdraw && !r.deleted) {
      await adminApi.removeComment(r.id)
    }
    ElMessage.success('处罚已生效' + (punishForm.value.also_withdraw && !r.deleted ? '，评论已撤回' : ''))
    punishDialog.value = false
    load()
  } catch {
    /* 错误已由拦截器提示 */
  }
}

onMounted(load)
</script>

<template>
  <div>
    <h1 class="mb-5 font-serif-sc text-2xl font-bold text-[var(--grape)]">评论管理</h1>

    <!-- 筛选 -->
    <div class="paper-card mb-4 p-4">
      <div class="grid gap-3 sm:grid-cols-2 lg:grid-cols-4">
        <div class="relative">
          <Search :size="15" class="absolute left-3 top-1/2 -translate-y-1/2 text-[var(--text-soft)]" />
          <input v-model="q.keyword" placeholder="关键词（评论内容/投稿标题）" class="input-box pl-9" @keyup.enter="search" />
        </div>
        <input v-model="q.submission_id" placeholder="投稿 ID" class="input-box" @keyup.enter="search" />
        <input v-model="q.author" placeholder="作者（用户名/昵称）" class="input-box" @keyup.enter="search" />
        <select v-model="q.status" class="input-box">
          <option value="">全部状态</option>
          <option value="normal">正常</option>
          <option value="deleted">已撤回</option>
          <option value="blocked">已屏蔽</option>
        </select>
      </div>
      <div class="mt-3 flex gap-2">
        <button class="btn-grape flex items-center gap-1 rounded-full px-4 py-1.5 text-sm" @click="search">
          <Search :size="14" /> 查询
        </button>
        <button class="rounded-full border border-[var(--border)] px-4 py-1.5 text-sm hover:text-[var(--grape)]" @click="reset">重置</button>
        <span class="ml-auto self-center text-xs text-[var(--text-soft)]">共 {{ total }} 条</span>
      </div>
    </div>

    <!-- 列表 -->
    <div class="paper-card overflow-hidden">
      <div class="overflow-x-auto">
        <table class="w-full text-sm">
          <thead class="bg-[var(--surface-2)] text-left text-xs text-[var(--text-soft)]">
            <tr>
              <th class="px-3 py-2.5 font-medium">ID</th>
              <th class="px-3 py-2.5 font-medium">所属投稿</th>
              <th class="px-3 py-2.5 font-medium">作者</th>
              <th class="px-3 py-2.5 font-medium">内容</th>
              <th class="px-3 py-2.5 font-medium">状态</th>
              <th class="px-3 py-2.5 font-medium">点赞</th>
              <th class="px-3 py-2.5 font-medium">时间</th>
              <th class="px-3 py-2.5 text-right font-medium">操作</th>
            </tr>
          </thead>
          <tbody class="divide-y divide-[var(--border)]">
            <tr v-for="r in items" :key="r.id" class="hover:bg-[var(--surface-2)]/50">
              <td class="px-3 py-2.5 align-top">
                <span class="font-mono text-xs">#{{ r.id }}</span>
                <span v-if="r.parent_id" class="ml-1 rounded bg-grape-50 px-1.5 py-0.5 text-[10px] text-[var(--grape)]">回复</span>
              </td>
              <td class="max-w-[180px] px-3 py-2.5 align-top">
                <button class="line-clamp-2 text-left text-xs text-[var(--grape)] hover:underline" @click="router.push('/submission/' + r.submission_id)">
                  #{{ r.submission_id }} {{ r.submission_title || '（投稿已删除）' }}
                </button>
              </td>
              <td class="px-3 py-2.5 align-top">
                <button class="text-xs text-[var(--grape)] hover:underline" @click="router.push('/admin/users?keyword=' + encodeURIComponent(r.author))">
                  {{ r.author }}
                </button>
                <div class="text-[10px] text-[var(--text-soft)]">#{{ r.user_id }}</div>
              </td>
              <td class="max-w-[280px] px-3 py-2.5 align-top">
                <p class="line-clamp-2 text-xs leading-relaxed text-[var(--text-soft)]">{{ r.content || '（内容已隐藏）' }}</p>
              </td>
              <td class="px-3 py-2.5 align-top">
                <span class="rounded px-2 py-0.5 text-xs" :class="statusOf(r).cls">{{ statusOf(r).label }}</span>
              </td>
              <td class="px-3 py-2.5 align-top text-xs">{{ r.like_count || 0 }}</td>
              <td class="px-3 py-2.5 align-top text-xs text-[var(--text-soft)]">{{ fmtTime(r.created_at) }}</td>
              <td class="px-3 py-2.5 align-top">
                <div class="flex justify-end gap-1.5">
                  <button
                    v-if="!r.deleted"
                    class="inline-flex items-center gap-1 rounded-full border border-[var(--border)] px-2.5 py-1 text-xs hover:text-[var(--danger)]"
                    @click="withdraw(r)"
                  >
                    <Ban :size="12" /> 撤回
                  </button>
                  <button
                    v-else
                    class="inline-flex items-center gap-1 rounded-full border border-[var(--border)] px-2.5 py-1 text-xs hover:text-[var(--jade)]"
                    @click="restore(r)"
                  >
                    <RotateCcw :size="12" /> 恢复
                  </button>
                  <button
                    class="inline-flex items-center gap-1 rounded-full border border-[var(--border)] px-2.5 py-1 text-xs hover:text-[var(--danger)]"
                    @click="openPunish(r)"
                  >
                    <Gavel :size="12" /> 处罚作者
                  </button>
                </div>
              </td>
            </tr>
            <tr v-if="!items.length">
              <td colspan="8" class="px-3 py-12 text-center text-[var(--text-soft)]">
                <MessageSquare :size="28" class="mx-auto mb-2 opacity-40" />
                {{ loading ? '加载中…' : '没有符合条件的评论' }}
              </td>
            </tr>
          </tbody>
        </table>
      </div>
      <el-pagination
        v-if="total > pageSize"
        class="border-t border-[var(--border)] p-3"
        layout="prev, pager, next"
        :total="total"
        :page-size="pageSize"
        :current-page="page"
        @current-change="(p: number) => { page = p; load() }"
        background
      />
    </div>

    <!-- 处罚作者 -->
    <el-dialog v-model="punishDialog" title="处罚评论作者" width="480px">
      <div v-if="punishRow" class="space-y-3 text-sm">
        <p class="rounded-lg bg-[var(--surface-2)] p-3 text-xs leading-relaxed">
          评论 #{{ punishRow.id }} · 作者「{{ punishRow.author }}」（#{{ punishRow.user_id }}）<br />
          <span class="text-[var(--text-soft)]">{{ punishRow.content }}</span>
        </p>
        <div>
          <label class="mb-1 block text-sm font-medium">处罚类型</label>
          <select v-model="punishForm.type" class="input-box">
            <option v-for="t in punishTypes" :key="t.value" :value="t.value">{{ t.label }}</option>
          </select>
          <p class="mt-1 text-xs leading-relaxed text-[var(--warn)]">影响范围：{{ impactOf(punishForm.type) }}</p>
        </div>
        <div>
          <label class="mb-1 block text-sm font-medium">时长</label>
          <label class="flex items-center gap-2 text-sm"><input type="checkbox" v-model="punishForm.permanent" /> 永久处罚</label>
          <input v-if="!punishForm.permanent" v-model="punishForm.expires_at" type="datetime-local" class="input-box mt-2" />
        </div>
        <div>
          <label class="mb-1 block text-sm font-medium">处罚理由（必填）</label>
          <textarea v-model="punishForm.reason" rows="2" placeholder="请说明处罚原因，将展示给用户" class="input-box"></textarea>
        </div>
        <label class="flex items-center gap-2 text-sm">
          <input type="checkbox" v-model="punishForm.also_withdraw" /> 同时撤回该评论
        </label>
      </div>
      <template #footer>
        <div class="flex justify-end gap-2">
          <button class="rounded-full px-4 py-1.5 text-sm" @click="punishDialog = false">取消</button>
          <button class="btn-grape inline-flex items-center gap-1 rounded-full px-4 py-1.5 text-sm text-white" @click="submitPunish">
            <Gavel :size="14" /> 执行处罚
          </button>
        </div>
      </template>
    </el-dialog>
  </div>
</template>

<style scoped>
.input-box {
  height: 40px;
  width: 100%;
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
textarea.input-box {
  height: auto;
}
</style>
