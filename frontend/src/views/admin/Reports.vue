<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { Flag, Check, X, Gavel } from 'lucide-vue-next'
import { ElMessage } from 'element-plus'
import { reportApi } from '@/api'

const list = ref<any[]>([])
const page = ref(1)
const total = ref(0)
const status = ref('')
const current = ref<any>(null)
const form = ref({ result: 'dismiss', reason: '', note: '', punish_type: '', punish_target: '', permanent: false, punish_reason: '', expires_at: '' })

// 被举报内容预览
const preview = ref<any>(null)
const previewLoading = ref(false)
async function openPreview() {
  if (!current.value) return
  previewLoading.value = true
  try {
    preview.value = await reportApi.target(current.value.id)
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '加载被举报内容失败')
  } finally {
    previewLoading.value = false
  }
}
function flatComments(nodes: any[], depth = 0): any[] {
  const out: any[] = []
  for (const n of nodes || []) {
    out.push({ ...n, _depth: depth })
    if (n.children?.length) out.push(...flatComments(n.children, depth + 1))
  }
  return out
}

async function load() {
  const d = await reportApi.list({ page: page.value, page_size: 20, status: status.value })
  list.value = d.items || []
  total.value = d.total
}
async function resolve() {
  if (!current.value) return
  if (!form.value.reason.trim()) {
    ElMessage.warning('处理理由必填（将对举报人展示）')
    return
  }
  await reportApi.resolve(current.value.id, {
    result: form.value.result,
    reason: form.value.reason,
    note: form.value.note,
    punish_type: form.value.punish_type || undefined,
    punish_target: form.value.punish_target || undefined,
    permanent: form.value.permanent,
    punish_reason: form.value.punish_reason || undefined,
    expires_at: form.value.permanent ? undefined : form.value.expires_at || undefined,
  })
  ElMessage.success('已处理')
  current.value = null
  form.value = { result: 'dismiss', reason: '', note: '', punish_type: '', punish_target: '', permanent: false, punish_reason: '', expires_at: '' }
  load()
}
function open(r: any) {
  current.value = r
  // 已处理的举报：回填原有处理结果，支持复核修改
  form.value.result = r.result || 'dismiss'
  form.value.reason = r.result_reason || ''
  form.value.note = r.admin_note || ''
  form.value.punish_type = ''
  form.value.punish_target = ''
  form.value.permanent = false
  form.value.punish_reason = ''
  form.value.expires_at = ''
}

onMounted(load)
</script>

<template>
  <div>
    <h1 class="mb-5 font-serif-sc text-2xl font-bold text-[var(--grape)]">举报审核</h1>

    <div class="mb-4 flex flex-wrap gap-2">
      <button :class="status === '' ? 'bg-grape-600 text-white' : 'bg-[var(--surface-2)] text-[var(--text-soft)]'" class="rounded-full px-4 py-1.5 text-sm" @click="(status = ''), load()">全部</button>
      <button :class="status === 'pending' ? 'bg-grape-600 text-white' : 'bg-[var(--surface-2)] text-[var(--text-soft)]'" class="rounded-full px-4 py-1.5 text-sm" @click="(status = 'pending'), load()">待处理</button>
      <button :class="status === 'resolved' ? 'bg-grape-600 text-white' : 'bg-[var(--surface-2)] text-[var(--text-soft)]'" class="rounded-full px-4 py-1.5 text-sm" @click="(status = 'resolved'), load()">已处理</button>
    </div>

    <div v-if="list.length" class="paper-card divide-y divide-[var(--border)]">
      <div v-for="r in list" :key="r.id" class="flex cursor-pointer flex-wrap items-center gap-2 p-4 text-sm hover:bg-[var(--surface-2)]" @click="open(r)">
        <Flag :size="15" class="text-[var(--grape)]" />
        <span class="rounded-full bg-[var(--surface-2)] px-2 py-0.5 text-xs">{{ r.target_type === 'submission' ? '投稿' : '评论' }} #{{ r.target_id }}</span>
        <span v-if="r.status === 'pending'" class="rounded-full bg-warn/15 px-2 py-0.5 text-xs text-warn">待处理</span>
        <span v-else class="rounded-full bg-[var(--jade)]/15 px-2 py-0.5 text-xs text-[var(--jade)]">{{ r.result === 'remove' ? '已撤下' : '无违规' }}</span>
        <span class="line-clamp-1 flex-1 text-[var(--text-soft)]">{{ r.reason }}</span>
        <span class="text-xs text-[var(--text-soft)]">{{ r.created_at }}</span>
        <button class="rounded-full border border-[var(--border)] px-3 py-1 text-xs hover:text-[var(--grape)]" @click.stop="open(r)">
          {{ r.status === 'pending' ? '处理' : '查看/修改' }}
        </button>
      </div>

      <el-pagination
        v-if="total > 20"
        class="p-3"
        layout="prev, pager, next"
        :total="total"
        :page-size="20"
        :current-page="page"
        @current-change="(p: number) => { page = p; load() }"
        background
      />
    </div>
    <div v-else class="paper-card p-10 text-center text-[var(--text-soft)]">暂无举报</div>

    <!-- 处理抽屉 -->
    <div v-if="current" class="fixed inset-0 z-[9996] flex items-center justify-center bg-black/55 px-4">
      <div class="paper-card w-full max-w-lg p-6 animate-fadeup">
        <h3 class="font-serif-sc text-lg font-bold">{{ current.status === 'resolved' ? '复核举报' : '处理举报' }} #{{ current.id }}</h3>
        <p class="mt-2 rounded-lg bg-[var(--surface-2)] p-3 text-sm">举报理由：{{ current.reason }}</p>
        <button class="mt-2 text-sm text-[var(--grape)] underline" :disabled="previewLoading" @click="openPreview">
          {{ previewLoading ? '加载中…' : '查看被举报内容（含评论）' }}
        </button>

        <div class="mt-4">
          <label class="mb-1 block text-sm font-medium">处理结果</label>
          <div class="flex gap-2">
            <button :class="form.result === 'dismiss' ? 'bg-[var(--jade)] text-white' : 'bg-[var(--surface-2)]'" class="flex items-center gap-1 rounded-full px-4 py-1.5 text-sm" @click="form.result = 'dismiss'">
              <Check :size="14" /> 无明显违规
            </button>
            <button :class="form.result === 'remove' ? 'bg-[var(--danger)] text-white' : 'bg-[var(--surface-2)]'" class="flex items-center gap-1 rounded-full px-4 py-1.5 text-sm" @click="form.result = 'remove'">
              <X :size="14" /> 撤下内容
            </button>
          </div>
        </div>

        <div class="mt-3">
          <label class="mb-1 block text-sm font-medium">处理理由（必填，对举报人展示）</label>
          <textarea v-model="form.reason" rows="2" class="input-box"></textarea>
        </div>
        <div class="mt-3">
          <label class="mb-1 block text-sm font-medium">内部备注（可选）</label>
          <textarea v-model="form.note" rows="2" class="input-box"></textarea>
        </div>

        <div class="mt-3 grid grid-cols-2 gap-2">
          <div>
            <label class="mb-1 block text-sm font-medium">处罚类型（可选）</label>
            <select v-model="form.punish_type" class="input-box">
              <option value="">不处罚</option>
              <option value="warn">警告</option>
              <option value="mute">禁言</option>
              <option value="ban_user">封禁用户名</option>
              <option value="ban_ip">封禁 IP</option>
            </select>
          </div>
          <div>
            <label class="mb-1 block text-sm font-medium">处罚对象</label>
            <select v-model="form.punish_target" class="input-box">
              <option value="">—</option>
              <option value="author">被举报内容作者</option>
              <option value="reporter">举报人</option>
            </select>
          </div>
        </div>

        <!-- 选择处罚类型时，展示处罚有效时间与理由（issue 4） -->
        <template v-if="form.punish_type">
          <div v-if="!form.permanent" class="mt-3">
            <label class="mb-1 block text-sm font-medium">处罚有效时间（临时处罚必填）</label>
            <input v-model="form.expires_at" type="datetime-local" class="input-box" />
          </div>
          <div class="mt-3">
            <label class="mb-1 block text-sm font-medium">处罚理由（必填）</label>
            <textarea v-model="form.punish_reason" rows="2" class="input-box" placeholder="对违规用户的处罚说明"></textarea>
          </div>
        </template>
        <label class="mt-2 flex items-center gap-2 text-sm"><input type="checkbox" v-model="form.permanent" /> 永久处罚</label>

        <div class="mt-5 flex justify-end gap-2">
          <button class="rounded-full border border-[var(--border)] px-4 py-2 text-sm" @click="current = null">取消</button>
          <button class="btn-grape flex items-center gap-1 rounded-full px-5 py-2 text-sm" @click="resolve"><Gavel :size="14" /> 提交处理</button>
        </div>
      </div>
    </div>

    <!-- 被举报内容预览（投稿 + 评论） -->
    <div v-if="preview" class="fixed inset-0 z-[9998] flex items-start justify-center overflow-auto bg-black/60 px-4 py-8">
      <div class="paper-card w-full max-w-2xl p-6">
        <div class="flex items-start justify-between">
          <div>
            <h3 class="font-serif-sc text-lg font-bold">{{ preview.submission?.title || '(无标题)' }}</h3>
            <div class="mt-1 flex flex-wrap items-center gap-2 text-xs text-[var(--text-soft)]">
              <span class="rounded-full bg-[var(--surface-2)] px-2 py-0.5 text-[var(--grape)]">{{ preview.submission?.anonymous ? '匿名用户' : preview.submission?.author }}</span>
              <span>{{ preview.submission?.created_at }}</span>
              <span class="rounded-full bg-[var(--surface-2)] px-2 py-0.5">状态：{{ preview.submission?.status }}</span>
              <span v-if="preview.target_type === 'comment'" class="rounded-full bg-warn/15 px-2 py-0.5 text-warn">被举报为评论 #{{ current?.target_id }}</span>
            </div>
          </div>
          <button class="text-[var(--text-soft)] hover:text-[var(--text)]" @click="preview = null"><X :size="18" /></button>
        </div>

        <p v-if="preview.submission?.content" class="mt-3 whitespace-pre-wrap text-sm leading-relaxed text-[var(--text-soft)]">{{ preview.submission.content }}</p>
        <div v-if="preview.submission?.images?.length" class="mt-3 grid grid-cols-3 gap-2">
          <img v-for="im in preview.submission.images" :key="im.id" :src="im.url" class="h-28 w-full rounded-lg object-cover" alt="" @error="$event.target.style.display = 'none'" />
        </div>
        <div v-if="preview.submission?.tags?.length" class="mt-2 flex flex-wrap gap-1.5">
          <span v-for="t in preview.submission.tags" :key="t.id" class="rounded-full bg-grape-50 px-2 py-0.5 text-xs text-[var(--grape)]">#{{ t.name }}</span>
        </div>

        <h4 class="mt-4 mb-2 text-sm font-semibold">评论（{{ flatComments(preview.comments).length }}）</h4>
        <div v-if="preview.comments?.length" class="space-y-2">
          <div
            v-for="c in flatComments(preview.comments)"
            :key="c.id"
            class="rounded-lg border p-3"
            :class="c.id === preview.focus_comment_id ? 'border-[var(--danger)] bg-[var(--danger)]/5' : 'border-[var(--border)]'"
            :style="{ marginLeft: (c._depth || 0) * 16 + 'px' }"
          >
            <div class="flex items-center gap-2 text-xs text-[var(--text-soft)]">
              <span class="font-medium text-[var(--text)]">{{ c.deleted ? '匿名用户' : c.username }}</span>
              <span>{{ c.created_at }}</span>
              <span v-if="c.deleted" class="rounded bg-[var(--surface-2)] px-1.5">已删除</span>
              <span v-if="c.id === preview.focus_comment_id" class="rounded bg-[var(--danger)] px-1.5 text-white">被举报</span>
            </div>
            <div class="mt-1 text-sm" :class="c.deleted ? 'text-[var(--text-soft)]' : ''">
              <template v-if="c.deleted">【已删除】{{ c.content }}</template>
              <template v-else>{{ c.content }}</template>
            </div>
          </div>
        </div>
        <p v-else class="text-xs text-[var(--text-soft)]">暂无评论</p>
      </div>
    </div>
  </div>
</template>

<style scoped>
.input-box {
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
</style>
