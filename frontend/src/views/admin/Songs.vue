<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { Search, Trash2, Play, RotateCcw, XCircle } from 'lucide-vue-next'
import { ElMessage, ElMessageBox } from 'element-plus'
import { adminApi } from '@/api'

const items = ref<any[]>([])
const total = ref(0)
const page = ref(1)
const pageSize = 20
const loading = ref(false)
const filters = ref({ status: 'all', keyword: '' })

const statusMap: Record<string, { text: string; cls: string }> = {
  pending: { text: '待播放', cls: 'bg-warn text-white' },
  played: { text: '已播放', cls: 'bg-[var(--jade)] text-white' },
  canceled: { text: '已取消', cls: 'bg-[var(--danger)] text-white' },
}

async function load() {
  loading.value = true
  try {
    const p = await adminApi.songs({ page: page.value, page_size: pageSize, status: filters.value.status, keyword: filters.value.keyword.trim() })
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

async function setStatus(s: any, status: string) {
  await adminApi.songStatus(s.id, status)
  ElMessage.success('已更新状态')
  load()
}

async function remove(s: any) {
  await ElMessageBox.confirm(`确定删除点歌《${s.title}》？`, '删除点歌', { type: 'warning' })
  await adminApi.deleteSong(s.id)
  ElMessage.success('已删除')
  load()
}

onMounted(load)
</script>

<template>
  <div>
    <h1 class="mb-5 font-serif-sc text-2xl font-bold text-[var(--grape)]">点歌管理</h1>

    <div class="paper-card mb-4 flex flex-wrap items-center gap-3 p-4">
      <select v-model="filters.status" class="input-box w-32" @change="search">
        <option value="all">全部状态</option>
        <option value="pending">待播放</option>
        <option value="played">已播放</option>
        <option value="canceled">已取消</option>
      </select>
      <div class="relative flex-1">
        <Search :size="16" class="absolute left-3 top-1/2 -translate-y-1/2 text-[var(--text-soft)]" />
        <input v-model="filters.keyword" class="input-box input-box--with-icon" placeholder="搜索歌名/歌手/留言" @keyup.enter="search" />
      </div>
      <button class="btn-grape rounded-full px-5 py-2 text-sm" @click="search">搜索</button>
    </div>

    <div class="paper-card overflow-x-auto">
      <table class="w-full text-sm">
        <thead>
          <tr class="border-b border-[var(--border)] text-left text-[var(--text-soft)]">
            <th class="px-4 py-3 font-medium">歌曲</th>
            <th class="px-4 py-3 font-medium">歌手</th>
            <th class="px-4 py-3 font-medium">留言</th>
            <th class="px-4 py-3 font-medium">点歌人</th>
            <th class="px-4 py-3 font-medium">状态</th>
            <th class="px-4 py-3 font-medium">时间</th>
            <th class="px-4 py-3 text-right font-medium">操作</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="s in items" :key="s.id" class="border-b border-[var(--border)] last:border-0">
            <td class="px-4 py-3 font-medium">{{ s.title }}</td>
            <td class="px-4 py-3 text-[var(--text-soft)]">{{ s.artist || '—' }}</td>
            <td class="max-w-[220px] truncate px-4 py-3 text-[var(--text-soft)]">{{ s.note || '—' }}</td>
            <td class="px-4 py-3">{{ s.username || '匿名' }}</td>
            <td class="px-4 py-3">
              <span class="rounded-full px-2 py-0.5 text-xs text-white" :class="(statusMap[s.status] || { cls: 'bg-[var(--text-soft)]' }).cls">
                {{ (statusMap[s.status] || { text: s.status }).text }}
              </span>
            </td>
            <td class="whitespace-nowrap px-4 py-3 text-xs text-[var(--text-soft)]">{{ s.created_at }}</td>
            <td class="px-4 py-3">
              <div class="flex items-center justify-end gap-2">
                <button v-if="s.status !== 'played'" class="flex items-center gap-1 rounded-full bg-[var(--jade)] px-3 py-1 text-xs text-white" @click="setStatus(s, 'played')">
                  <Play :size="12" /> 已播放
                </button>
                <button v-if="s.status !== 'pending'" class="flex items-center gap-1 rounded-full border border-[var(--border)] px-3 py-1 text-xs hover:text-warn" @click="setStatus(s, 'pending')">
                  <RotateCcw :size="12" /> 待播放
                </button>
                <button v-if="s.status !== 'canceled'" class="flex items-center gap-1 rounded-full border border-[var(--border)] px-3 py-1 text-xs hover:text-[var(--danger)]" @click="setStatus(s, 'canceled')">
                  <XCircle :size="12" /> 取消
                </button>
                <button class="flex items-center gap-1 rounded-full border border-[var(--border)] px-3 py-1 text-xs hover:text-[var(--danger)]" @click="remove(s)">
                  <Trash2 :size="12" /> 删除
                </button>
              </div>
            </td>
          </tr>
          <tr v-if="!loading && !items.length">
            <td colspan="7" class="px-4 py-10 text-center text-[var(--text-soft)]">暂无点歌</td>
          </tr>
        </tbody>
      </table>
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
.input-box--with-icon {
  padding-left: 36px;
}
</style>
