<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { Check, Pencil, X, Gavel } from 'lucide-vue-next'
import { ElMessage } from 'element-plus'
import { adminApi } from '@/api'
import PenaltyDialog from '@/components/PenaltyDialog.vue'

const list = ref<any[]>([])
const page = ref(1)
const total = ref(0)
const note = ref<Record<number, string>>({})

// 处罚对话框状态（issue 5）
const showPenalty = ref(false)
const penaltyUser = ref<number | null>(null)
const penaltyName = ref('')

async function load() {
  const p = await adminApi.pendingSubmissions({ page: page.value, page_size: 20 })
  list.value = p.items || []
  total.value = p.total
}

async function review(id: number, status: string) {
  await adminApi.reviewSubmission(id, { status, note: note.value[id] || '' })
  ElMessage.success('已处理')
  load()
}

function openPenalty(s: any) {
  if (!s.user_id) return
  penaltyUser.value = s.user_id
  penaltyName.value = s.anonymous ? '匿名用户' : s.author
  showPenalty.value = true
}

// 机审结果展示（issue 11）
function auditChip(a: any) {
  const kind = a.kind === 'ai' ? 'AI' : '腾讯云'
  const map: Record<string, { text: string; cls: string }> = {
    pass: { text: '正常', cls: 'border border-[var(--jade)] text-[var(--jade)]' },
    block: { text: '驳回', cls: 'border border-[var(--danger)] text-[var(--danger)]' },
    review: { text: '可疑', cls: 'border border-warn text-warn' },
    failed: { text: '异常', cls: 'border border-warn text-warn' },
  }
  const m = map[a.status] || { text: a.label || a.status, cls: 'bg-[var(--surface-2)] text-[var(--text-soft)]' }
  return { kind, ...m }
}

onMounted(load)
</script>

<template>
  <div>
    <h1 class="mb-5 font-serif-sc text-2xl font-bold text-[var(--grape)]">投稿审核</h1>

    <div v-if="list.length" class="space-y-4">
      <div v-for="s in list" :key="s.id" class="paper-card p-5">
        <div class="flex items-center gap-2 text-xs text-[var(--text-soft)]">
          <span class="rounded-full bg-[var(--surface-2)] px-2 py-0.5 text-[var(--grape)]">{{ s.author }}</span>
          <span>{{ s.created_at }}</span>
          <span v-if="s.real_author" class="text-[var(--text-soft)]">（真实：{{ s.real_author }}）</span>
        </div>
        <h3 class="mt-2 font-serif-sc text-lg font-bold">{{ s.title }}</h3>
        <p v-if="s.content" class="mt-2 whitespace-pre-wrap text-sm text-[var(--text-soft)]">{{ s.content }}</p>
        <div v-if="s.images?.length" class="mt-3 grid grid-cols-3 gap-2">
          <img v-for="im in s.images" :key="im.id" :src="im.url" class="h-24 w-full rounded-lg object-cover" alt="" />
        </div>
        <div v-if="s.videos?.length" class="mt-3 grid grid-cols-3 gap-2">
          <video v-for="v in s.videos" :key="v.id" :src="v.url" controls preload="metadata" class="h-24 w-full rounded-lg object-cover bg-black"></video>
        </div>
        <div v-if="s.tags?.length" class="mt-2 flex flex-wrap gap-1.5">
          <span v-for="t in s.tags" :key="t.id" class="rounded-full bg-grape-50 px-2 py-0.5 text-xs text-[var(--grape)]">#{{ t.name }}</span>
        </div>

        <!-- 机审判定依据（腾讯云 / AI 驳回理由），issue 11 -->
        <div v-if="s.audit?.length" class="mt-3 flex flex-wrap items-center gap-2 text-xs">
          <span class="text-[var(--text-soft)]">机审：</span>
          <span
            v-for="(a, i) in s.audit"
            :key="i"
            class="rounded-full px-2 py-0.5"
            :class="auditChip(a).cls"
          >{{ auditChip(a).kind }}·{{ auditChip(a).text }}</span>
          <span v-for="(a, i) in s.audit.filter((x: any) => x.status === 'block' && x.reason)" :key="'r' + i" class="text-[var(--danger)]">
            （理由：{{ a.reason }}）
          </span>
        </div>

        <!-- AI 审核评论（仅管理员可见）：通过/拒绝原因 -->
        <div v-if="s.audit_comment" class="mt-2 rounded-lg bg-[var(--surface-2)] px-3 py-2 text-xs text-[var(--grape)]">
          <span class="font-semibold">AI 审核意见：</span>{{ s.audit_comment }}
        </div>

        <textarea v-model="note[s.id]" rows="2" placeholder="审核意见（仅投稿人可见）" class="input-style mt-3 h-auto py-2"></textarea>

        <div class="mt-3 flex flex-wrap gap-2">
          <button class="flex items-center gap-1 rounded-full bg-[var(--jade)] px-4 py-1.5 text-sm text-white" @click="review(s.id, 'approved')"><Check :size="14" /> 通过</button>
          <button class="flex items-center gap-1 rounded-full bg-warn px-4 py-1.5 text-sm text-white" @click="review(s.id, 'need_edit')"><Pencil :size="14" /> 需要修改</button>
          <button class="flex items-center gap-1 rounded-full bg-[var(--danger)] px-4 py-1.5 text-sm text-white" @click="review(s.id, 'rejected')"><X :size="14" /> 不通过</button>
          <button v-if="s.user_id" class="flex items-center gap-1 rounded-full border border-[var(--border)] px-4 py-1.5 text-sm hover:text-[var(--danger)]" @click="openPenalty(s)"><Gavel :size="14" /> 处罚作者</button>
        </div>
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
    </div>
    <div v-else class="paper-card p-10 text-center text-[var(--text-soft)]">暂无待审核投稿 🎉</div>

    <PenaltyDialog v-model="showPenalty" :user-id="penaltyUser" :username="penaltyName" />
  </div>
</template>

<style scoped>
.input-style {
  width: 100%;
  border-radius: 12px;
  border: 1px solid var(--border);
  background: var(--surface-2);
  padding: 8px 12px;
  outline: none;
  font-size: 14px;
}
.input-style:focus {
  border-color: var(--grape);
}
</style>
