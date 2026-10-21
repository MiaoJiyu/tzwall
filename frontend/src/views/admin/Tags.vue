<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { Check, X, Merge, Plus, Pencil, Trash2, Search } from 'lucide-vue-next'
import { ElMessage, ElMessageBox } from 'element-plus'
import { adminApi, submissionApi } from '@/api'

const pending = ref<any[]>([])
const all = ref<any[]>([])
const approved = ref<any[]>([])
const mergeFrom = ref<number | null>(null)
const mergeTo = ref<number | null>(null)
const newTag = ref('')
const keyword = ref('')

const statusMap: Record<string, { text: string; cls: string }> = {
  approved: { text: '已通过', cls: 'bg-[var(--jade)]/15 text-[var(--jade)]' },
  pending: { text: '待审核', cls: 'bg-warn/15 text-warn' },
  rejected: { text: '已拒绝', cls: 'bg-[var(--danger)]/15 text-[var(--danger)]' },
}

const filteredAll = computed(() => {
  const kw = keyword.value.trim()
  if (!kw) return all.value
  return all.value.filter((t) => (t.name || '').includes(kw))
})

async function loadPending() {
  const p = await adminApi.pendingTags()
  pending.value = p.items || []
}
async function loadAll() {
  const p = await adminApi.allTags()
  all.value = p.items || []
}
async function loadApproved() {
  const p = await submissionApi.tags()
  approved.value = p.items || []
}

async function approve(id: number, ok: boolean) {
  await adminApi.approveTag(id, { approve: ok })
  ElMessage.success(ok ? '已通过' : '已拒绝')
  refresh()
}

async function addTag() {
  const name = newTag.value.trim()
  if (!name) {
    ElMessage.warning('请输入标签名')
    return
  }
  try {
    await adminApi.createTag({ name })
    ElMessage.success('已新增标签')
    newTag.value = ''
    loadAll()
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '新增失败')
  }
}

async function editTag(t: any) {
  try {
    const { value } = await ElMessageBox.prompt('修改标签名', '编辑标签', {
      inputValue: t.name,
      inputValidator: (v: string) => (v && v.trim() ? true : '标签名不能为空'),
    })
    await adminApi.updateTag(t.id, { name: value.trim() })
    ElMessage.success('已修改')
    refresh()
  } catch (e: any) {
    if (e !== 'cancel') ElMessage.error(e?.response?.data?.message || '修改失败')
  }
}

async function delTag(t: any) {
  try {
    await ElMessageBox.confirm(`确定删除标签 #${t.name}？将从相关投稿中解除关联。`, '删除标签', { type: 'warning' })
    await adminApi.deleteTag(t.id)
    ElMessage.success('已删除')
    refresh()
  } catch (e: any) {
    if (e !== 'cancel') ElMessage.error(e?.response?.data?.message || '删除失败')
  }
}

async function merge() {
  if (!mergeFrom.value || !mergeTo.value) {
    ElMessage.warning('请选择合并的源与目标标签')
    return
  }
  await adminApi.mergeTag({ from: mergeFrom.value, to: mergeTo.value })
  ElMessage.success('已合并')
  mergeFrom.value = null
  mergeTo.value = null
  refresh()
}

function refresh() {
  loadPending()
  loadAll()
  loadApproved()
}

onMounted(refresh)
</script>

<template>
  <div>
    <h1 class="mb-5 font-serif-sc text-2xl font-bold text-[var(--grape)]">标签管理</h1>

    <!-- 全部标签 + 新增/修改/删除 -->
    <div class="paper-card mb-5 p-5">
      <div class="mb-3 flex flex-wrap items-center justify-between gap-2">
        <h2 class="font-semibold">全部标签（{{ all.length }}）</h2>
        <div class="flex items-center gap-2">
          <div class="relative">
            <Search :size="15" class="absolute left-3 top-1/2 -translate-y-1/2 text-[var(--text-soft)]" />
            <input v-model="keyword" placeholder="搜索标签" class="rounded-lg border border-[var(--border)] bg-[var(--surface-2)] py-1.5 pl-9 pr-3 text-sm outline-none" />
          </div>
          <input v-model="newTag" placeholder="新增标签名" class="rounded-lg border border-[var(--border)] bg-[var(--surface-2)] px-3 py-1.5 text-sm outline-none" @keyup.enter="addTag" />
          <button class="btn-grape flex items-center gap-1 rounded-full px-4 py-1.5 text-xs" @click="addTag"><Plus :size="13" /> 新增</button>
        </div>
      </div>

      <div v-if="filteredAll.length" class="flex flex-wrap gap-2">
        <div v-for="t in filteredAll" :key="t.id" class="flex items-center gap-2 rounded-full bg-[var(--surface-2)] px-3 py-1.5 text-sm">
          <span>#{{ t.name }}</span>
          <span class="rounded-full px-1.5 py-0.5 text-[11px]" :class="(statusMap[t.status] || {}).cls">{{ (statusMap[t.status] || { text: t.status }).text }}</span>
          <button class="text-[var(--text-soft)] hover:text-[var(--grape)]" title="修改" @click="editTag(t)"><Pencil :size="13" /></button>
          <button class="text-[var(--text-soft)] hover:text-[var(--danger)]" title="删除" @click="delTag(t)"><Trash2 :size="13" /></button>
        </div>
      </div>
      <p v-else class="text-sm text-[var(--text-soft)]">暂无标签</p>
    </div>

    <!-- 待审核标签 -->
    <div class="paper-card mb-5 p-5">
      <h2 class="mb-3 font-semibold">待审核标签（{{ pending.length }}）</h2>
      <div v-if="pending.length" class="space-y-2">
        <div v-for="t in pending" :key="t.id" class="flex items-center justify-between rounded-lg bg-[var(--surface-2)] px-3 py-2 text-sm">
          <span>#{{ t.name }}</span>
          <div class="flex gap-2">
            <button class="flex items-center gap-1 rounded-full bg-[var(--jade)] px-3 py-1 text-xs text-white" @click="approve(t.id, true)"><Check :size="13" /> 通过</button>
            <button class="flex items-center gap-1 rounded-full bg-[var(--danger)] px-3 py-1 text-xs text-white" @click="approve(t.id, false)"><X :size="13" /> 拒绝</button>
          </div>
        </div>
      </div>
      <p v-else class="text-sm text-[var(--text-soft)]">暂无待审核标签</p>
    </div>

    <!-- 合并去重 -->
    <div class="paper-card p-5">
      <h2 class="mb-3 font-semibold">合并去重</h2>
      <div class="flex flex-wrap items-center gap-2 text-sm">
        <select v-model="mergeFrom" class="rounded-lg border border-[var(--border)] bg-[var(--surface-2)] px-2 py-1.5">
          <option :value="null">选择源标签</option>
          <option v-for="t in all" :key="t.id" :value="t.id">#{{ t.name }}</option>
        </select>
        <span class="text-[var(--text-soft)]">→</span>
        <select v-model="mergeTo" class="rounded-lg border border-[var(--border)] bg-[var(--surface-2)] px-2 py-1.5">
          <option :value="null">选择目标标签</option>
          <option v-for="t in approved" :key="t.id" :value="t.id">#{{ t.name }}</option>
        </select>
        <button class="btn-grape flex items-center gap-1 rounded-full px-4 py-1.5 text-xs" @click="merge"><Merge :size="13" /> 合并</button>
      </div>
    </div>
  </div>
</template>
