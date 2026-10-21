<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'
import { Search } from 'lucide-vue-next'
import { logApi } from '@/api'

const list = ref<any[]>([])
const total = ref(0)
const filters = reactive({ action: '', object_type: '', actor_id: '', from: '', to: '' })
const page = ref(1)

const actionLabel: Record<string, string> = {
  'config.update': '修改系统配置',
  'punishment.create': '创建处罚',
  'punishment.revoke': '撤销处罚',
  'ipban.revoke': '撤销 IP 封禁',
  'report.resolve': '处理举报',
  'set_role': '变更角色',
  'review_tag': '审核标签',
  'merge_tag': '合并标签',
  'permission.update': '更新权限矩阵',
  'agreement.publish': '发布协议版本',
  'wordfilter.add': '新增词条',
  'wordfilter.remove': '删除词条',
}

async function load() {
  const p: any = { page: page.value, page_size: 20 }
  for (const [k, v] of Object.entries(filters)) {
    if (v) p[k] = v
  }
  const d = await logApi.audit(p)
  list.value = d.items || []
  total.value = d.total
}

function search() {
  page.value = 1
  load()
}

onMounted(load)
</script>

<template>
  <div>
    <h1 class="mb-5 font-serif-sc text-2xl font-bold text-[var(--grape)]">审计日志</h1>

    <div class="paper-card mb-4 p-4">
      <div class="grid gap-2 sm:grid-cols-2 lg:grid-cols-5">
        <input v-model="filters.action" placeholder="动作（如 config.update）" class="input-box" @keyup.enter="search" />
        <input v-model="filters.object_type" placeholder="对象类型" class="input-box" @keyup.enter="search" />
        <input v-model="filters.actor_id" placeholder="操作人 ID" class="input-box" @keyup.enter="search" />
        <input v-model="filters.from" type="date" class="input-box" />
        <button class="btn-grape flex items-center justify-center gap-1 rounded-full py-2 text-sm" @click="search">
          <Search :size="14" /> 查询
        </button>
      </div>
      <p class="mt-2 text-xs text-[var(--text-soft)]">审计记录覆盖配置修改、处罚、审核、权限与协议变更；默认保留 365 天。</p>
    </div>

    <div class="paper-card overflow-hidden">
      <div class="overflow-x-auto">
        <table class="w-full text-sm">
          <thead>
            <tr class="border-b border-[var(--border)] text-left text-[var(--text-soft)]">
              <th class="px-4 py-2">时间</th>
              <th class="px-4 py-2">操作人</th>
              <th class="px-4 py-2">动作</th>
              <th class="px-4 py-2">对象</th>
              <th class="px-4 py-2">摘要</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="r in list" :key="r.id" class="border-b border-[var(--border)] hover:bg-[var(--surface-2)]">
              <td class="whitespace-nowrap px-4 py-2 text-xs text-[var(--text-soft)]">{{ r.created_at?.replace('T', ' ').slice(0, 19) }}</td>
              <td class="px-4 py-2">{{ r.actor_name || '#' + r.actor_id }}</td>
              <td class="px-4 py-2">
                <span class="rounded-full bg-grape-50 px-2 py-0.5 text-xs text-[var(--grape)]">{{ actionLabel[r.action] || r.action }}</span>
              </td>
              <td class="px-4 py-2 font-mono text-xs text-[var(--text-soft)]">{{ r.object_type }}{{ r.object_id ? '#' + r.object_id : '' }}</td>
              <td class="max-w-[320px] truncate px-4 py-2 text-xs text-[var(--text-soft)]">{{ r.detail }}</td>
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
