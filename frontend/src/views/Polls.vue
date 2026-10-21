<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'
import { Plus, X, Check, Trash2, Clock, User as UserIcon, Pin, BarChart3 } from 'lucide-vue-next'
import { ElMessage, ElMessageBox } from 'element-plus'
import { pollApi } from '@/api'
import { useAuthStore } from '@/stores/auth'

const auth = useAuthStore()
const items = ref<any[]>([])
const total = ref(0)
const page = ref(1)
const pageSize = 10
const loading = ref(false)
const selected = reactive<Record<number, number[]>>({})

const showCreate = ref(false)
const submitting = ref(false)
const form = ref({ title: '', description: '', multiple: false, deadline: '', options: ['', ''] })

const statusText = (p: any) => (p.expired ? '已截止' : '进行中')

async function load() {
  loading.value = true
  try {
    const p = await pollApi.list({ page: page.value, page_size: pageSize })
    items.value = p.items || []
    total.value = p.total
    for (const it of items.value) {
      selected[it.id] = Array.isArray(it.my_option_ids) ? [...it.my_option_ids] : []
    }
  } finally {
    loading.value = false
  }
}

function toggleOption(p: any, oid: number) {
  if (p.expired) return
  const cur = selected[p.id] || []
  if (p.multiple) {
    selected[p.id] = cur.includes(oid) ? cur.filter((x) => x !== oid) : [...cur, oid]
  } else {
    selected[p.id] = [oid]
  }
}

async function vote(p: any) {
  if (!auth.isLoggedIn) {
    ElMessage.warning('请先登录')
    return
  }
  if (p.expired) {
    ElMessage.warning('投票已截止')
    return
  }
  const ids = selected[p.id] || []
  if (!ids.length) {
    ElMessage.warning('请先选择选项')
    return
  }
  try {
    await pollApi.vote(p.id, { option_ids: ids })
    ElMessage.success('投票成功')
    await load()
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '投票失败')
  }
}

async function removePoll(p: any) {
  await ElMessageBox.confirm('确定删除该投票？', '删除投票', { type: 'warning' })
  await pollApi.remove(p.id)
  ElMessage.success('已删除')
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
  if (!auth.isLoggedIn) {
    ElMessage.warning('请先登录')
    return
  }
  const opts = form.value.options.map((x) => x.trim()).filter(Boolean)
  if (!form.value.title.trim()) {
    ElMessage.warning('请填写投票标题')
    return
  }
  if (opts.length < 2) {
    ElMessage.warning('至少两个选项')
    return
  }
  submitting.value = true
  try {
    await pollApi.create({
      title: form.value.title.trim(),
      description: form.value.description.trim(),
      multiple: form.value.multiple,
      deadline: form.value.deadline || undefined,
      options: opts,
    })
    ElMessage.success('投票已发起')
    showCreate.value = false
    form.value = { title: '', description: '', multiple: false, deadline: '', options: ['', ''] }
    page.value = 1
    await load()
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '发起失败')
  } finally {
    submitting.value = false
  }
}

function pct(p: any, count: number) {
  return p.total_votes > 0 ? Math.round((count / p.total_votes) * 100) : 0
}
function votersText(voters: string[]) {
  if (!voters || !voters.length) return ''
  const head = voters.slice(0, 5).join('、')
  return voters.length > 5 ? `${head} 等 ${voters.length} 人` : head
}

onMounted(load)
</script>

<template>
  <div class="mx-auto max-w-[900px] px-4 py-6">
    <div class="mb-5 flex items-end justify-between">
      <div>
        <h1 class="flex items-center gap-2 font-serif-sc text-2xl font-bold text-[var(--grape)]"><BarChart3 :size="24" /> 校园投票</h1>
        <p class="mt-1 text-sm text-[var(--text-soft)]">发起话题投票，看看大家的想法。</p>
      </div>
      <button v-if="auth.isLoggedIn" class="btn-grape flex items-center gap-1 rounded-full px-4 py-2 text-sm" @click="showCreate = !showCreate">
        <Plus :size="15" /> 发起投票
      </button>
    </div>

    <!-- 发起投票 -->
    <div v-if="showCreate" class="paper-card mb-6 p-5">
      <div class="flex items-center justify-between">
        <h2 class="font-semibold">发起投票</h2>
        <button class="text-[var(--text-soft)]" @click="showCreate = false"><X :size="18" /></button>
      </div>
      <input v-model="form.title" placeholder="投票标题" class="input-box mt-3 w-full" />
      <textarea v-model="form.description" rows="2" placeholder="说明（选填）" class="input-box mt-3 w-full"></textarea>
      <div class="mt-3 flex flex-wrap items-center gap-4 text-sm">
        <label class="flex items-center gap-2"><input v-model="form.multiple" type="checkbox" /> 多选</label>
        <label class="flex items-center gap-2 text-[var(--text-soft)]">
          <Clock :size="14" /> 截止时间
          <input v-model="form.deadline" type="datetime-local" class="input-box" />
        </label>
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
      <p class="mt-2 text-xs text-[var(--text-soft)]">选项数量上限由后台配置（默认 10 个）。</p>
    </div>

    <!-- 投票列表 -->
    <div v-if="items.length" class="space-y-4">
      <div
        v-for="p in items"
        :key="p.id"
        class="paper-card p-5"
        :class="p.pinned ? 'ring-2 ring-[var(--grape)]/40 bg-grape-50/40' : ''"
      >
        <div class="flex flex-wrap items-center gap-2">
          <span v-if="p.pinned" class="flex items-center gap-1 rounded-full bg-[var(--grape)] px-2 py-0.5 text-xs text-white"><Pin :size="11" /> 置顶</span>
          <h3 class="font-serif-sc text-lg font-bold">{{ p.title }}</h3>
          <span class="rounded-full px-2 py-0.5 text-xs" :class="p.expired ? 'bg-[var(--surface-2)] text-[var(--text-soft)]' : 'bg-[var(--jade)]/15 text-[var(--jade)]'">{{ statusText(p) }}</span>
          <span v-if="p.multiple" class="rounded-full bg-[var(--surface-2)] px-2 py-0.5 text-xs text-[var(--text-soft)]">多选</span>
        </div>
        <p v-if="p.description" class="mt-1 text-sm text-[var(--text-soft)]">{{ p.description }}</p>
        <div class="mt-1 flex flex-wrap items-center gap-3 text-xs text-[var(--text-soft)]">
          <span class="inline-flex items-center gap-1"><UserIcon :size="12" /> {{ p.creator || '匿名' }}</span>
          <span class="inline-flex items-center gap-1"><Clock :size="12" /> {{ p.created_at }}</span>
          <span v-if="p.deadline">截止 {{ p.deadline }}</span>
          <span>共 {{ p.total_votes }} 票</span>
        </div>

        <div class="mt-3 space-y-2">
          <button
            v-for="o in p.options"
            :key="o.id"
            class="relative w-full overflow-hidden rounded-lg border px-3 py-2 text-left text-sm transition"
            :class="(selected[p.id] || []).includes(o.id) ? 'border-[var(--grape)] bg-grape-50' : 'border-[var(--border)] hover:border-[var(--grape)]/50'"
            :disabled="p.expired"
            @click="toggleOption(p, o.id)"
          >
            <span class="absolute inset-y-0 left-0 bg-[var(--grape)]/10" :style="{ width: pct(p, o.count) + '%' }"></span>
            <span class="relative flex items-center justify-between gap-2">
              <span class="flex items-center gap-2">
                <span class="flex h-4 w-4 items-center justify-center rounded-full border" :class="(selected[p.id] || []).includes(o.id) ? 'border-[var(--grape)] bg-[var(--grape)] text-white' : 'border-[var(--border)]'">
                  <Check v-if="(selected[p.id] || []).includes(o.id)" :size="10" />
                </span>
                {{ o.text }}
              </span>
              <span class="text-xs text-[var(--text-soft)]">{{ o.count }} 票 · {{ pct(p, o.count) }}%</span>
            </span>
            <span v-if="votersText(o.voters)" class="relative mt-0.5 block text-[11px] text-[var(--text-soft)]">{{ votersText(o.voters) }}</span>
          </button>
        </div>

        <div class="mt-3 flex items-center justify-between">
          <span class="text-xs text-[var(--text-soft)]">
            {{ (p.my_option_ids || []).length ? '你已投过票，可重新选择后改票' : '选择后点击投票' }}
          </span>
          <div class="flex items-center gap-2">
            <button v-if="auth.isLoggedIn && auth.user?.id === p.user_id" class="flex items-center gap-1 rounded-full border border-[var(--border)] px-3 py-1 text-xs hover:text-[var(--danger)]" @click="removePoll(p)">
              <Trash2 :size="12" /> 删除
            </button>
            <button class="btn-grape rounded-full px-4 py-1.5 text-xs disabled:cursor-not-allowed disabled:opacity-50" :disabled="p.expired" @click="vote(p)">
              {{ (p.my_option_ids || []).length ? '改票' : '投票' }}
            </button>
          </div>
        </div>
      </div>
    </div>
    <div v-else-if="!loading" class="paper-card p-10 text-center text-[var(--text-soft)]">还没有投票，来发起第一个吧～</div>

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
</style>
