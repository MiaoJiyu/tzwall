<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { Plus, X, Check, Trash2, Pin, Clock } from 'lucide-vue-next'
import { ElMessage, ElMessageBox } from 'element-plus'
import { adminApi } from '@/api'

const items = ref<any[]>([])
const total = ref(0)
const page = ref(1)
const pageSize = 20
const loading = ref(false)
const keyword = ref('')
const showCreate = ref(false)
const submitting = ref(false)
const form = ref({ title: '', description: '', multiple: false, pinned: false, deadline: '', options: ['', ''] })

async function load() {
  loading.value = true
  try {
    const p = await adminApi.polls({ page: page.value, page_size: pageSize, keyword: keyword.value.trim() })
    items.value = p.items || []
    total.value = p.total
  } finally {
    loading.value = false
  }
}

function search() {
  page.value = 1
  load()
}

function addOption() {
  form.value.options.push('')
}
function removeOption(i: number) {
  if (form.value.options.length <= 2) return
  form.value.options.splice(i, 1)
}

async function create() {
  const opts = form.value.options.map((x) => x.trim()).filter(Boolean)
  if (!form.value.title.trim()) {
    ElMessage.warning('请填写标题')
    return
  }
  if (opts.length < 2) {
    ElMessage.warning('至少两个选项')
    return
  }
  submitting.value = true
  try {
    await adminApi.createPoll({
      title: form.value.title.trim(),
      description: form.value.description.trim(),
      multiple: form.value.multiple,
      pinned: form.value.pinned,
      deadline: form.value.deadline || undefined,
      options: opts,
    })
    ElMessage.success('已发布')
    showCreate.value = false
    form.value = { title: '', description: '', multiple: false, pinned: false, deadline: '', options: ['', ''] }
    page.value = 1
    await load()
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '发布失败')
  } finally {
    submitting.value = false
  }
}

async function remove(p: any) {
  await ElMessageBox.confirm(`确定强删投票《${p.title}》？将连同选项与投票记录一并删除。`, '强删投票', { type: 'warning' })
  await adminApi.deletePoll(p.id)
  ElMessage.success('已删除')
  load()
}

onMounted(load)
</script>

<template>
  <div>
    <div class="mb-5 flex items-center justify-between">
      <h1 class="font-serif-sc text-2xl font-bold text-[var(--grape)]">投票管理</h1>
      <button class="btn-grape flex items-center gap-1 rounded-full px-4 py-2 text-sm" @click="showCreate = !showCreate">
        <Plus :size="15" /> 发起投票
      </button>
    </div>

    <!-- 发起投票（管理员，可选置顶） -->
    <div v-if="showCreate" class="paper-card mb-5 p-5">
      <div class="flex items-center justify-between">
        <h2 class="font-semibold">管理员发起投票</h2>
        <button class="text-[var(--text-soft)]" @click="showCreate = false"><X :size="18" /></button>
      </div>
      <input v-model="form.title" placeholder="投票标题" class="input-box mt-3 w-full" />
      <textarea v-model="form.description" rows="2" placeholder="说明（选填）" class="input-box mt-3 w-full"></textarea>
      <div class="mt-3 flex flex-wrap items-center gap-4 text-sm">
        <label class="flex items-center gap-2"><input v-model="form.multiple" type="checkbox" /> 多选</label>
        <label class="flex items-center gap-2"><input v-model="form.pinned" type="checkbox" /> 置顶高亮</label>
        <label class="flex items-center gap-2 text-[var(--text-soft)]"><Clock :size="14" /> 截止时间 <input v-model="form.deadline" type="datetime-local" class="input-box" /></label>
      </div>
      <div class="mt-3 space-y-2">
        <div v-for="(_, i) in form.options" :key="i" class="flex items-center gap-2">
          <span class="w-6 text-right text-sm text-[var(--text-soft)]">{{ i + 1 }}.</span>
          <input v-model="form.options[i]" :placeholder="`选项 ${i + 1}`" class="input-box flex-1" />
          <button class="text-[var(--text-soft)] hover:text-[var(--danger)]" @click="removeOption(i)"><X :size="16" /></button>
        </div>
      </div>
      <div class="mt-3 flex items-center justify-between">
        <button class="text-sm text-[var(--grape)]" @click="addOption">+ 添加选项</button>
        <button class="btn-grape flex items-center gap-1 rounded-full px-5 py-2 text-sm" :disabled="submitting" @click="create">
          <Check :size="14" /> {{ submitting ? '提交中…' : '发布' }}
        </button>
      </div>
    </div>

    <div class="paper-card mb-4 flex flex-wrap items-center gap-3 p-4">
      <input v-model="keyword" class="input-box flex-1" placeholder="搜索标题" @keyup.enter="search" />
      <button class="btn-grape rounded-full px-5 py-2 text-sm" @click="search">搜索</button>
    </div>

    <div class="space-y-3">
      <div v-for="p in items" :key="p.id" class="paper-card p-4">
        <div class="flex flex-wrap items-center gap-2">
          <span v-if="p.pinned" class="flex items-center gap-1 rounded-full bg-[var(--grape)] px-2 py-0.5 text-xs text-white"><Pin :size="11" /> 置顶</span>
          <h3 class="font-medium">{{ p.title }}</h3>
          <span class="rounded-full px-2 py-0.5 text-xs" :class="p.expired ? 'bg-[var(--surface-2)] text-[var(--text-soft)]' : 'bg-[var(--jade)]/15 text-[var(--jade)]'">{{ p.expired ? '已截止' : '进行中' }}</span>
          <span class="text-xs text-[var(--text-soft)]">创建人：{{ p.creator || '匿名' }}</span>
          <span class="text-xs text-[var(--text-soft)]">共 {{ p.total_votes }} 票</span>
          <button class="ml-auto flex items-center gap-1 rounded-full border border-[var(--border)] px-3 py-1 text-xs hover:text-[var(--danger)]" @click="remove(p)">
            <Trash2 :size="12" /> 强删
          </button>
        </div>
        <div class="mt-2 flex flex-wrap gap-2 text-xs text-[var(--text-soft)]">
          <span v-for="o in p.options" :key="o.id" class="rounded-full bg-[var(--surface-2)] px-2 py-0.5">{{ o.text }}：{{ o.count }}</span>
        </div>
      </div>
      <div v-if="!loading && !items.length" class="paper-card p-10 text-center text-[var(--text-soft)]">暂无投票</div>
    </div>

    <el-pagination
      v-if="total > pageSize"
      class="mt-4 justify-center"
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
  padding: 8px 12px;
  font-size: 14px;
  outline: none;
}
.input-box:focus {
  border-color: var(--grape);
}
</style>
