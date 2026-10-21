<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { RotateCcw, Save, Plus, Trash2, History, AlertTriangle } from 'lucide-vue-next'
import { ElMessage, ElMessageBox } from 'element-plus'
import { configApi, agreementApi, wordApi } from '@/api'

const groups = ref<any[]>([])
const readOnly = ref<any[]>([])
const tab = ref('')
const saving = ref(false)
const dirty = ref<Record<string, string>>({})
const pristine = ref<Record<string, string>>({})

// 协议 Tab
const agreements = ref<any[]>([])
const activeKey = ref('user')
const publishForm = ref<any>({ title: '', version: '', content: '', effective_at: '' })

// 词表 Tab
const words = ref<any[]>([])
const wordTotal = ref(0)
const wordPage = ref(1)
const wordCategory = ref('forbidden')
const wordKeyword = ref('')
const wordInput = ref('')

const tabList = computed(() => [
  ...groups.value.map((g) => ({ key: g.key, label: g.label, kind: 'group' })),
  { key: 'agreements', label: '协议版本', kind: 'special' },
  { key: 'words', label: '词表管理', kind: 'special' },
])

async function loadConfigs() {
  const d = await configApi.list()
  groups.value = d.groups || []
  readOnly.value = d.read_only || []
  const p: Record<string, string> = {}
  for (const g of groups.value) {
    for (const it of g.items) {
      p[it.key] = it.value ?? ''
    }
  }
  pristine.value = { ...p }
  dirty.value = {}
  if (!tab.value && tabList.value.length) tab.value = tabList.value[0].key
}

async function save() {
  if (!Object.keys(dirty.value).length) {
    ElMessage.info('没有需要保存的修改')
    return
  }
  try {
    await ElMessageBox.confirm('保存后立即生效（标记“需重启”的项除外），确认保存？', '保存配置', { type: 'warning' })
  } catch {
    return
  }
  saving.value = true
  try {
    const r = await configApi.save(dirty.value)
    ElMessage.success(`已保存 ${(r.changed || []).length} 项配置`)
    await loadConfigs()
  } finally {
    saving.value = false
  }
}

function resetAll() {
  dirty.value = {}
}

function onInput(key: string, val: any) {
  const v = val === true || val === false ? String(val) : String(val ?? '')
  if (v === (pristine.value[key] ?? '')) {
    delete dirty.value[key]
  } else {
    dirty.value[key] = v
  }
}

// ---------- 协议 ----------
async function loadAgreements() {
  const d = await agreementApi.list()
  agreements.value = d.items || []
  const cur = agreements.value.find((a) => a.key === activeKey.value) || agreements.value[0]
  if (cur) {
    activeKey.value = cur.key
    publishForm.value = {
      title: cur.current?.title || '',
      version: '',
      content: cur.current?.content || '',
      effective_at: '',
    }
  }
}

async function publish() {
  if (!publishForm.value.content) {
    ElMessage.warning('协议正文不能为空')
    return
  }
  await agreementApi.publish(activeKey.value, publishForm.value)
  ElMessage.success('已发布新版本，已登录用户需重新确认')
  loadAgreements()
}

async function rollback(id: number, version: string) {
  try {
    await ElMessageBox.confirm(`将当前生效内容回滚到版本 ${version}？`, '回滚协议', { type: 'warning' })
  } catch {
    return
  }
  await agreementApi.rollback(id)
  ElMessage.success('已回滚')
  loadAgreements()
}

// ---------- 词表 ----------
async function loadWords() {
  const d = await wordApi.list({ page: wordPage.value, page_size: 50, category: wordCategory.value, keyword: wordKeyword.value })
  words.value = d.items || []
  wordTotal.value = d.total
}

async function addWords() {
  if (!wordInput.value.trim()) {
    ElMessage.warning('请输入词条')
    return
  }
  const r = await wordApi.add({ words: wordInput.value, category: wordCategory.value })
  ElMessage.success(`新增 ${r.added || 0} 个词条`)
  wordInput.value = ''
  loadWords()
}

async function removeWord(id: number) {
  await wordApi.remove(id)
  ElMessage.success('已删除')
  loadWords()
}

function switchTab(key: string) {
  if (Object.keys(dirty.value).length && key !== tab.value) {
    ElMessageBox.confirm('当前有未保存的配置修改，切换分组将丢弃，确认切换？', '未保存修改', { type: 'warning' })
      .then(() => {
        dirty.value = {}
        tab.value = key
        afterTab(key)
      })
      .catch(() => {})
    return
  }
  tab.value = key
  afterTab(key)
}

function afterTab(key: string) {
  if (key === 'agreements') loadAgreements()
  if (key === 'words') loadWords()
}

onMounted(loadConfigs)
</script>

<template>
  <div>
    <div class="mb-4 flex flex-wrap items-center justify-between gap-3">
      <h1 class="font-serif-sc text-2xl font-bold text-[var(--grape)]">系统配置</h1>
      <div v-if="tabList.some((t) => t.kind === 'group' && t.key === tab)" class="flex gap-2">
        <button class="flex items-center gap-1 rounded-full border border-[var(--border)] px-4 py-1.5 text-sm hover:text-[var(--grape)]" @click="resetAll">
          <RotateCcw :size="14" /> 重置
        </button>
        <button class="btn-grape flex items-center gap-1 rounded-full px-5 py-1.5 text-sm" :disabled="saving" @click="save">
          <Save :size="14" /> 保存{{ Object.keys(dirty).length ? `（${Object.keys(dirty).length}）` : '' }}
        </button>
      </div>
    </div>

    <div class="flex flex-col gap-4 lg:flex-row">
      <!-- 分组导航 -->
      <aside class="paper-card h-fit p-2 lg:w-52 lg:shrink-0">
        <button
          v-for="t in tabList"
          :key="t.key"
          class="w-full rounded-lg px-3 py-2 text-left text-sm transition"
          :class="tab === t.key ? 'bg-grape-50 font-medium text-[var(--grape)]' : 'text-[var(--text-soft)] hover:bg-[var(--surface-2)]'"
          @click="switchTab(t.key)"
        >
          {{ t.label }}
        </button>
      </aside>

      <!-- 右侧内容 -->
      <div class="min-w-0 flex-1 space-y-4">
        <!-- 配置分组 -->
        <template v-for="g in groups" :key="g.key">
          <div v-if="tab === g.key" class="paper-card p-5">
            <h2 class="mb-4 font-serif-sc text-lg font-semibold">{{ g.label }}</h2>
            <div class="space-y-4">
              <div v-for="it in g.items" :key="it.key" class="grid gap-2 sm:grid-cols-[200px_1fr] sm:items-start">
                <div>
                  <label class="block text-sm font-medium">{{ it.label }}</label>
                  <p v-if="it.desc" class="mt-0.5 text-xs text-[var(--text-soft)]">{{ it.desc }}</p>
                </div>
                <div>
                  <!-- 布尔 -->
                  <label v-if="it.type === 'bool'" class="flex cursor-pointer items-center gap-2 text-sm">
                    <input type="checkbox" :checked="(dirty[it.key] ?? it.value) === 'true'" @change="onInput(it.key, ($event.target as HTMLInputElement).checked ? 'true' : 'false')" />
                    <span>{{ (dirty[it.key] ?? it.value) === 'true' ? '开启' : '关闭' }}</span>
                  </label>
                  <!-- 整数 -->
                  <input
                    v-else-if="it.type === 'int'"
                    type="number"
                    :min="it.min"
                    :max="it.max"
                    class="input-box sm:max-w-[200px]"
                    :value="dirty[it.key] ?? it.value"
                    @input="onInput(it.key, ($event.target as HTMLInputElement).value)"
                  />
                  <!-- 敏感项 -->
                  <div v-else-if="it.secret" class="flex flex-wrap items-center gap-2">
                    <span v-if="it.has_set" class="rounded-full bg-[var(--surface-2)] px-3 py-1 font-mono text-xs text-[var(--text-soft)]">{{ it.masked }}</span>
                    <span v-else class="text-xs text-[var(--text-soft)]">未设置</span>
                    <input
                      class="input-box sm:max-w-[280px]"
                      :placeholder="it.has_set ? '留空表示不修改' : '输入新值'"
                      :value="dirty[it.key] ?? ''"
                      @input="onInput(it.key, ($event.target as HTMLInputElement).value)"
                    />
                  </div>
                  <!-- 多行文本 -->
                  <textarea
                    v-else-if="it.type === 'text'"
                    rows="4"
                    class="input-box"
                    :value="dirty[it.key] ?? it.value"
                    @input="onInput(it.key, ($event.target as HTMLTextAreaElement).value)"
                  ></textarea>
                  <!-- 单行文本 -->
                  <input v-else class="input-box" :value="dirty[it.key] ?? it.value" @input="onInput(it.key, ($event.target as HTMLInputElement).value)" />
                  <p v-if="it.restart" class="mt-1 flex items-center gap-1 text-xs text-[#C8862B]">
                    <AlertTriangle :size="12" /> 修改后需重启服务生效
                  </p>
                </div>
              </div>
            </div>
          </div>
        </template>

        <!-- .env 只读项 -->
        <div v-if="tab === groups[0]?.key" class="paper-card p-5">
          <h2 class="mb-1 font-serif-sc text-lg font-semibold">密钥与环境配置</h2>
          <p class="mb-4 text-xs text-[var(--text-soft)]">以下配置仅保存在服务器 .env 文件中，后台只展示状态，不可查看内容。</p>
          <div class="space-y-2">
            <div v-for="r in readOnly" :key="r.key" class="flex flex-wrap items-center gap-2 rounded-lg bg-[var(--surface-2)] px-3 py-2 text-sm">
              <span class="font-medium">{{ r.label }}</span>
              <span
                class="rounded-full px-2 py-0.5 text-xs"
                :class="r.status === 'configured' ? 'bg-[#2F7D5C]/15 text-[#2F7D5C]' : 'bg-[#C8862B]/15 text-[#C8862B]'"
              >{{ r.status === 'configured' ? '已配置' : '未配置' }}</span>
              <span class="text-xs text-[var(--text-soft)]">{{ r.detail }}</span>
            </div>
          </div>
        </div>

        <!-- 协议版本 -->
        <div v-if="tab === 'agreements'" class="grid gap-4 lg:grid-cols-[1fr_320px]">
          <div class="paper-card p-5">
            <div class="mb-4 flex flex-wrap items-center gap-2">
              <button
                v-for="a in agreements"
                :key="a.key"
                class="rounded-full px-3 py-1 text-sm"
                :class="activeKey === a.key ? 'bg-grape-600 text-white' : 'bg-[var(--surface-2)] text-[var(--text-soft)]'"
                @click="activeKey = a.key; loadAgreements()"
              >
                {{ a.key === 'user' ? '用户协议' : a.key === 'privacy' ? '隐私协议' : '数据保护协议' }}
              </button>
              <span v-if="agreements.find((a) => a.key === activeKey)?.current" class="ml-auto text-xs text-[var(--text-soft)]">
                当前版本 {{ agreements.find((a) => a.key === activeKey)?.current?.version }}
              </span>
            </div>
            <div class="space-y-3">
              <input v-model="publishForm.title" placeholder="协议标题" class="input-box" />
              <input v-model="publishForm.version" placeholder="新版本号（留空自动生成，如 2026.1021.1030）" class="input-box" />
              <input v-model="publishForm.effective_at" type="date" class="input-box sm:max-w-[200px]" />
              <textarea v-model="publishForm.content" rows="14" class="input-box" placeholder="协议正文"></textarea>
              <button class="btn-grape flex items-center gap-1 rounded-full px-5 py-2 text-sm" @click="publish">
                <Plus :size="14" /> 发布新版本
              </button>
            </div>
          </div>
          <div class="paper-card h-fit p-4">
            <h3 class="mb-3 flex items-center gap-1 text-sm font-semibold"><History :size="14" /> 版本历史</h3>
            <div class="space-y-2">
              <div
                v-for="v in agreements.find((a) => a.key === activeKey)?.versions || []"
                :key="v.id"
                class="rounded-lg border border-[var(--border)] p-3 text-xs"
              >
                <div class="flex items-center justify-between">
                  <span class="font-medium">{{ v.version }}</span>
                  <span class="text-[var(--text-soft)]">{{ v.effective_at?.slice(0, 10) }}</span>
                </div>
                <p class="mt-1 line-clamp-2 text-[var(--text-soft)]">{{ v.content }}</p>
                <button class="mt-2 text-xs text-[var(--grape)] hover:underline" @click="rollback(v.id, v.version)">回滚到此版本</button>
              </div>
              <p v-if="!(agreements.find((a) => a.key === activeKey)?.versions || []).length" class="py-6 text-center text-xs text-[var(--text-soft)]">暂无历史版本</p>
            </div>
          </div>
        </div>

        <!-- 词表管理 -->
        <div v-if="tab === 'words'" class="paper-card p-5">
          <div class="mb-4 flex flex-wrap items-center gap-2">
            <button
              class="rounded-full px-3 py-1 text-sm"
              :class="wordCategory === 'forbidden' ? 'bg-[#B3453A]/15 text-[#B3453A]' : 'bg-[var(--surface-2)] text-[var(--text-soft)]'"
              @click="wordCategory = 'forbidden'; wordPage = 1; loadWords()"
            >违禁词（拦截）</button>
            <button
              class="rounded-full px-3 py-1 text-sm"
              :class="wordCategory === 'common' ? 'bg-grape-600 text-white' : 'bg-[var(--surface-2)] text-[var(--text-soft)]'"
              @click="wordCategory = 'common'; wordPage = 1; loadWords()"
            >常用词（仅记录）</button>
            <input v-model="wordKeyword" placeholder="搜索词条" class="input-box ml-auto sm:max-w-[200px]" @keyup.enter="wordPage = 1; loadWords()" />
          </div>
          <textarea v-model="wordInput" rows="3" class="input-box" placeholder="批量添加：每行一个，或用逗号分隔"></textarea>
          <div class="mt-2 flex items-center gap-2">
            <button class="btn-grape flex items-center gap-1 rounded-full px-4 py-1.5 text-sm" @click="addWords">
              <Plus :size="14" /> 添加
            </button>
            <span class="text-xs text-[var(--text-soft)]">共 {{ wordTotal }} 个词条</span>
          </div>
          <div class="mt-4 flex flex-wrap gap-2">
            <span
              v-for="w in words"
              :key="w.id"
              class="flex items-center gap-1 rounded-full bg-[var(--surface-2)] px-3 py-1 text-sm"
            >
              {{ w.word }}
              <button class="text-[var(--text-soft)] hover:text-[#B3453A]" @click="removeWord(w.id)">
                <Trash2 :size="12" />
              </button>
            </span>
            <p v-if="!words.length" class="w-full py-8 text-center text-sm text-[var(--text-soft)]">暂无词条</p>
          </div>
          <el-pagination
            v-if="wordTotal > 50"
            class="mt-4"
            layout="prev, pager, next"
            :total="wordTotal"
            :page-size="50"
            :current-page="wordPage"
            @current-change="(p: number) => { wordPage = p; loadWords() }"
            background
          />
        </div>
      </div>
    </div>
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
