<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'
import { Search } from 'lucide-vue-next'
import { logApi } from '@/api'

const list = ref<any[]>([])
const total = ref(0)
const filters = reactive({ path: '', method: '', status: '', user_id: '', ip: '', from: '', to: '' })
const page = ref(1)
const detail = ref<any>(null)
const drawer = ref(false)

async function load() {
  const p: any = { page: page.value, page_size: 20 }
  for (const [k, v] of Object.entries(filters)) {
    if (v) p[k] = v
  }
  const d = await logApi.access(p)
  list.value = d.items || []
  total.value = d.total
}

function search() {
  page.value = 1
  load()
}

function open(row: any) {
  detail.value = row
  drawer.value = true
}

function parseHeaders(s: string) {
  try {
    return JSON.parse(s)
  } catch {
    return {}
  }
}

function statusClass(code: number) {
  if (code >= 500) return 'bg-[#B3453A]/15 text-[#B3453A]'
  if (code >= 400) return 'bg-[#C8862B]/15 text-[#C8862B]'
  if (code >= 300) return 'bg-[#4A6FA5]/15 text-[#4A6FA5]'
  return 'bg-[#2F7D5C]/15 text-[#2F7D5C]'
}

onMounted(load)
</script>

<template>
  <div>
    <h1 class="mb-5 font-serif-sc text-2xl font-bold text-[var(--grape)]">访问日志</h1>

    <div class="paper-card mb-4 p-4">
      <div class="grid gap-2 sm:grid-cols-2 lg:grid-cols-4">
        <input v-model="filters.path" placeholder="路径关键字" class="input-box" @keyup.enter="search" />
        <input v-model="filters.method" placeholder="方法（GET/POST…）" class="input-box" @keyup.enter="search" />
        <input v-model="filters.status" placeholder="状态码" class="input-box" @keyup.enter="search" />
        <input v-model="filters.user_id" placeholder="用户 ID" class="input-box" @keyup.enter="search" />
        <input v-model="filters.ip" placeholder="IP" class="input-box" @keyup.enter="search" />
        <input v-model="filters.from" type="date" class="input-box" />
        <input v-model="filters.to" type="date" class="input-box" />
        <button class="btn-grape flex items-center justify-center gap-1 rounded-full py-2 text-sm" @click="search">
          <Search :size="14" /> 查询
        </button>
      </div>
      <p class="mt-2 text-xs text-[var(--text-soft)]">IP 与请求详情仅超级管理员可见；敏感字段已脱敏。默认查询最近 7 天。</p>
    </div>

    <div class="paper-card overflow-hidden">
      <div class="overflow-x-auto">
        <table class="w-full text-sm">
          <thead>
            <tr class="border-b border-[var(--border)] text-left text-[var(--text-soft)]">
              <th class="px-4 py-2">时间</th>
              <th class="px-4 py-2">方法</th>
              <th class="px-4 py-2">路径</th>
              <th class="px-4 py-2">状态</th>
              <th class="px-4 py-2">耗时</th>
              <th class="px-4 py-2">用户</th>
              <th class="px-4 py-2">IP</th>
              <th class="px-4 py-2"></th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="r in list" :key="r.id" class="border-b border-[var(--border)] hover:bg-[var(--surface-2)]">
              <td class="whitespace-nowrap px-4 py-2 text-xs text-[var(--text-soft)]">{{ r.created_at?.replace('T', ' ').slice(0, 19) }}</td>
              <td class="px-4 py-2"><span class="rounded bg-[var(--surface-2)] px-1.5 py-0.5 font-mono text-xs">{{ r.method }}</span></td>
              <td class="max-w-[280px] truncate px-4 py-2 font-mono text-xs">{{ r.path }}</td>
              <td class="px-4 py-2"><span class="rounded-full px-2 py-0.5 text-xs" :class="statusClass(r.status_code)">{{ r.status_code }}</span></td>
              <td class="px-4 py-2 text-xs text-[var(--text-soft)]">{{ r.duration_ms }} ms</td>
              <td class="px-4 py-2 text-xs">{{ r.username || (r.user_id ? '#' + r.user_id : '游客') }}</td>
              <td class="px-4 py-2 font-mono text-xs text-[var(--text-soft)]">{{ r.ip }}</td>
              <td class="px-4 py-2"><button class="text-xs text-[var(--grape)] hover:underline" @click="open(r)">详情</button></td>
            </tr>
          </tbody>
        </table>
        <div v-if="!list.length" class="p-10 text-center text-[var(--text-soft)]">暂无记录</div>
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

    <el-drawer v-model="drawer" title="请求详情" size="480px">
      <div v-if="detail" class="space-y-4 text-sm">
        <div>
          <h3 class="mb-1 text-xs font-semibold text-[var(--text-soft)]">请求行</h3>
          <p class="font-mono text-xs break-all">{{ detail.method }} {{ detail.path }}{{ detail.query ? '?' + detail.query : '' }}</p>
        </div>
        <div class="grid grid-cols-2 gap-2 text-xs">
          <div><span class="text-[var(--text-soft)]">状态码</span> {{ detail.status_code }}</div>
          <div><span class="text-[var(--text-soft)]">耗时</span> {{ detail.duration_ms }} ms</div>
          <div><span class="text-[var(--text-soft)]">IP</span> {{ detail.ip }}</div>
          <div><span class="text-[var(--text-soft)]">用户</span> {{ detail.username || (detail.user_id ? '#' + detail.user_id : '游客') }}</div>
          <div class="col-span-2"><span class="text-[var(--text-soft)]">请求体摘要</span> {{ detail.body_digest || '—' }}</div>
        </div>
        <div>
          <h3 class="mb-1 text-xs font-semibold text-[var(--text-soft)]">请求头</h3>
          <div class="space-y-1">
            <div v-for="(v, k) in parseHeaders(detail.headers)" :key="k" class="flex gap-2 font-mono text-xs">
              <span class="shrink-0 text-[var(--text-soft)]">{{ k }}:</span>
              <span class="break-all">{{ v }}</span>
            </div>
          </div>
        </div>
        <div v-if="detail.body_preview">
          <h3 class="mb-1 text-xs font-semibold text-[var(--text-soft)]">请求体预览</h3>
          <pre class="max-h-60 overflow-auto rounded-lg bg-[var(--surface-2)] p-3 font-mono text-xs whitespace-pre-wrap break-all">{{ detail.body_preview }}</pre>
        </div>
        <div v-if="detail.ua">
          <h3 class="mb-1 text-xs font-semibold text-[var(--text-soft)]">User-Agent</h3>
          <p class="font-mono text-xs break-all text-[var(--text-soft)]">{{ detail.ua }}</p>
        </div>
      </div>
    </el-drawer>
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
</style>
