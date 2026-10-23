<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { ElMessage } from 'element-plus'
import { ticketApi } from '@/api'

const list = ref<any[]>([])
const total = ref(0)
const page = ref(1)
const fStatus = ref('all')
const fType = ref('all')
const keyword = ref('')

const statusText: Record<string, string> = { pending: '待处理', processing: '处理中', resolved: '已解决', rejected: '已驳回', closed: '已关闭' }
const statusCls: Record<string, string> = {
  pending: 'bg-warn/15 text-warn',
  processing: 'bg-grape-50 text-[var(--grape)]',
  resolved: 'bg-[var(--jade)]/15 text-[var(--jade)]',
  rejected: 'bg-[var(--danger)]/15 text-[var(--danger)]',
  closed: 'bg-[var(--surface-2)] text-[var(--text-soft)]',
}
const typeText: Record<string, string> = { bug: '功能故障', feedback: '意见反馈', report: '违规举报', appeal: '处罚申诉', account: '账号/安全', other: '其他' }
const transitions: Record<string, { v: string; l: string }[]> = {
  pending: [
    { v: 'processing', l: '开始处理' },
    { v: 'resolved', l: '标记解决' },
    { v: 'rejected', l: '驳回' },
    { v: 'closed', l: '关闭' },
  ],
  processing: [
    { v: 'pending', l: '转回待处理' },
    { v: 'resolved', l: '标记解决' },
    { v: 'rejected', l: '驳回' },
    { v: 'closed', l: '关闭' },
  ],
  resolved: [{ v: 'closed', l: '关闭' }],
  rejected: [],
  closed: [],
}

async function load() {
  const p = await ticketApi.adminList({ page: page.value, page_size: 20, status: fStatus.value, type: fType.value, keyword: keyword.value })
  list.value = p.items || []
  total.value = p.total || 0
}
onMounted(load)
function search() {
  page.value = 1
  load()
}

const detail = ref<any>(null)
const detailVisible = ref(false)
const replyText = ref('')
const replying = ref(false)
async function openDetail(it: any) {
  detailVisible.value = true
  replyText.value = ''
  try {
    detail.value = await ticketApi.adminGet(it.id)
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '加载失败')
  }
}
async function refresh() {
  detail.value = await ticketApi.adminGet(detail.value.id)
  load()
}
async function reply() {
  if (!replyText.value.trim()) {
    ElMessage.warning('请输入回复内容')
    return
  }
  replying.value = true
  try {
    await ticketApi.reply(detail.value.id, { content: replyText.value })
    ElMessage.success('已回复')
    replyText.value = ''
    await refresh()
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '回复失败')
  } finally {
    replying.value = false
  }
}
async function setStatus(s: string) {
  try {
    await ticketApi.setStatus(detail.value.id, { status: s })
    ElMessage.success('状态已更新')
    await refresh()
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '操作失败')
  }
}
</script>

<template>
  <div>
    <h1 class="mb-5 font-serif-sc text-2xl font-bold text-[var(--grape)]">工单管理</h1>
    <div class="paper-card p-5">
      <div class="mb-4 flex flex-wrap items-center gap-2">
        <select v-model="fStatus" @change="search" class="input-style w-auto">
          <option value="all">全部状态</option>
          <option value="pending">待处理</option>
          <option value="processing">处理中</option>
          <option value="resolved">已解决</option>
          <option value="rejected">已驳回</option>
          <option value="closed">已关闭</option>
        </select>
        <select v-model="fType" @change="search" class="input-style w-auto">
          <option value="all">全部类型</option>
          <option value="bug">功能故障</option>
          <option value="feedback">意见反馈</option>
          <option value="report">违规举报</option>
          <option value="appeal">处罚申诉</option>
          <option value="account">账号/安全</option>
          <option value="other">其他</option>
        </select>
        <input v-model="keyword" @keyup.enter="search" placeholder="搜索标题/描述" class="input-style w-56" />
        <button class="rounded-full bg-grape-500 px-4 py-2 text-sm text-white" @click="search">查询</button>
      </div>

      <div class="overflow-x-auto">
        <table class="w-full text-sm">
          <thead>
            <tr class="border-b border-[var(--border)] text-left text-[var(--text-soft)]">
              <th class="py-2">ID</th>
              <th class="py-2">类型</th>
              <th class="py-2">标题</th>
              <th class="py-2">提交人</th>
              <th class="py-2">状态</th>
              <th class="py-2">提交时间</th>
              <th class="py-2">操作</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="it in list" :key="it.id" class="border-b border-[var(--border)]">
              <td class="py-2">#{{ it.id }}</td>
              <td class="py-2 text-[var(--text-soft)]">{{ typeText[it.type] || it.type }}</td>
              <td class="max-w-[240px] truncate py-2 font-medium" :title="it.title">{{ it.title }}</td>
              <td class="py-2 text-[var(--text-soft)]">{{ it.username || '#' + it.user_id }}</td>
              <td class="py-2"><span class="rounded-full px-2 py-0.5 text-xs" :class="statusCls[it.status] || 'bg-[var(--surface-2)]'">{{ statusText[it.status] || it.status }}</span></td>
              <td class="py-2 text-[var(--text-soft)]">{{ it.created_at }}</td>
              <td class="py-2"><button class="text-xs text-[var(--grape)] hover:underline" @click="openDetail(it)">查看/处理</button></td>
            </tr>
          </tbody>
        </table>
      </div>
      <div v-if="!list.length" class="py-10 text-center text-sm text-[var(--text-soft)]">暂无工单</div>
      <el-pagination v-if="total > 20" class="mt-4 justify-center" layout="prev, pager, next" :total="total" :page-size="20" :current-page="page" @current-change="(p: number) => { page = p; load() }" background />
    </div>

    <el-dialog v-model="detailVisible" title="工单详情与处理" width="600px">
      <div v-if="detail" class="space-y-4 text-sm">
        <div class="flex items-center justify-between">
          <span class="font-medium">{{ typeText[detail.type] || detail.type }}</span>
          <span class="rounded-full px-2 py-0.5 text-xs" :class="statusCls[detail.status] || 'bg-[var(--surface-2)]'">{{ statusText[detail.status] || detail.status }}</span>
        </div>
        <h3 class="font-serif-sc text-lg font-bold">{{ detail.title }}</h3>
        <div class="text-xs text-[var(--text-soft)]">提交人：{{ detail.username || '#' + detail.user_id }} · {{ detail.created_at }}</div>
        <p class="whitespace-pre-wrap leading-relaxed">{{ detail.content }}</p>
        <div v-if="detail.attachments?.length" class="grid grid-cols-4 gap-2">
          <img v-for="a in detail.attachments" :key="a.id" :src="a.url" class="h-20 w-full rounded object-cover" alt="" />
        </div>

        <div class="border-t border-[var(--border)] pt-3">
          <p class="mb-2 text-xs font-medium text-[var(--text-soft)]">处理记录</p>
          <div v-if="detail.replies?.length" class="space-y-2">
            <div v-for="(r, idx) in detail.replies" :key="idx" class="rounded-lg bg-[var(--surface-2)] p-2.5">
              <div class="mb-1 flex items-center gap-2 text-xs text-[var(--text-soft)]">
                <span class="font-medium text-[var(--text)]">{{ r.status_to ? '系统' : r.is_admin ? '管理员' : '用户' }}{{ r.username ? ' · ' + r.username : '' }}</span>
                <span>{{ r.created_at }}</span>
              </div>
              <p class="leading-relaxed">{{ r.content }}</p>
            </div>
          </div>
          <p v-else class="text-xs text-[var(--text-soft)]">暂无记录</p>
        </div>

        <div class="border-t border-[var(--border)] pt-3">
          <p class="mb-2 text-xs font-medium text-[var(--text-soft)]">状态流转</p>
          <div class="flex flex-wrap gap-2">
            <button v-for="tr in transitions[detail.status] || []" :key="tr.v" class="rounded-full border border-[var(--border)] px-3 py-1 text-xs hover:text-[var(--grape)]" @click="setStatus(tr.v)">{{ tr.l }}</button>
            <span v-if="!(transitions[detail.status] || []).length" class="text-xs text-[var(--text-soft)]">该状态为终态，无法流转</span>
          </div>
        </div>

        <div class="border-t border-[var(--border)] pt-3">
          <textarea v-model="replyText" rows="3" placeholder="输入回复内容（用户可见）" class="w-full rounded-lg border border-[var(--border)] bg-[var(--surface-2)] p-3 text-sm outline-none focus:border-[var(--grape)]"></textarea>
          <div class="mt-2 text-right">
            <button class="btn-grape rounded-full px-4 py-2 text-sm text-white disabled:opacity-60" :disabled="replying" @click="reply">{{ replying ? '发送中…' : '回复' }}</button>
          </div>
        </div>
      </div>
    </el-dialog>
  </div>
</template>

<style scoped>
.input-style {
  height: 36px;
  border-radius: 10px;
  border: 1px solid var(--border);
  background: var(--surface-2);
  padding: 0 12px;
  outline: none;
  font-size: 14px;
}
</style>
