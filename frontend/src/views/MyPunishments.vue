<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { ShieldAlert } from 'lucide-vue-next'
import { punishmentApi, reportApi } from '@/api'

const tab = ref('punish')
const items = ref<any[]>([])
const reports = ref<any[]>([])

const typeLabel: Record<string, string> = {
  warn: '警告',
  mute: '禁言',
  ban_user: '封禁用户名',
  ban_ip: '封禁 IP',
  force_reset: '强制修改用户名与密码',
}
const resultLabel: Record<string, string> = {
  remove: '已撤下内容',
  dismiss: '未发现违规',
}

async function load() {
  const p = await punishmentApi.mine()
  items.value = p.items || []
  const r = await reportApi.mine()
  reports.value = r.items || []
}
onMounted(load)
</script>

<template>
  <div class="mx-auto max-w-3xl px-4 py-6">
    <h1 class="mb-5 font-serif-sc text-2xl font-bold text-[var(--grape)]">我的权限变更记录</h1>

    <div class="mb-5 flex gap-2">
      <button :class="tab === 'punish' ? 'bg-grape-600 text-white' : 'bg-[var(--surface-2)] text-[var(--text-soft)]'" class="rounded-full px-4 py-1.5 text-sm" @click="tab = 'punish'">
        处罚记录（{{ items.length }}）
      </button>
      <button :class="tab === 'report' ? 'bg-grape-600 text-white' : 'bg-[var(--surface-2)] text-[var(--text-soft)]'" class="rounded-full px-4 py-1.5 text-sm" @click="tab = 'report'">
        我的举报（{{ reports.length }}）
      </button>
    </div>

    <!-- 处罚记录 -->
    <div v-if="tab === 'punish'" class="space-y-3">
      <div v-for="p in items" :key="p.id" class="paper-card p-4">
        <div class="flex flex-wrap items-center gap-2">
          <span class="rounded-full bg-grape-50 px-2 py-0.5 text-xs font-medium text-[var(--grape)]">{{ typeLabel[p.type] || p.type }}</span>
          <span v-if="p.revoked" class="rounded-full bg-[var(--surface-2)] px-2 py-0.5 text-xs text-[var(--text-soft)]">已撤销</span>
          <span v-else-if="p.permanent" class="rounded-full bg-[var(--danger)]/15 px-2 py-0.5 text-xs text-[var(--danger)]">永久</span>
          <span v-else-if="p.expires_at" class="rounded-full bg-warn/15 px-2 py-0.5 text-xs text-warn">至 {{ p.expires_at }}</span>
          <span class="ml-auto text-xs text-[var(--text-soft)]">{{ p.created_at }}</span>
        </div>
        <p class="mt-2 text-sm leading-relaxed">理由：{{ p.reason }}</p>
        <p v-if="p.target_ip" class="mt-1 text-xs text-[var(--text-soft)]">目标 IP：{{ p.target_ip }}</p>
        <p v-if="p.new_username" class="mt-1 text-xs text-[var(--text-soft)]">新用户名：{{ p.new_username }}</p>
        <p v-if="p.type === 'warn'" class="mt-1 text-xs" :class="p.ack_at ? 'text-[var(--jade)]' : 'text-warn'">
          {{ p.ack_at ? '已确认' : '待确认' }}
        </p>
      </div>
      <div v-if="!items.length" class="paper-card p-10 text-center text-[var(--text-soft)]">暂无处罚记录，请继续遵守社区规范～</div>
    </div>

    <!-- 我的举报 -->
    <div v-else class="space-y-3">
      <div v-for="r in reports" :key="r.id" class="paper-card p-4">
        <div class="flex flex-wrap items-center gap-2">
          <ShieldAlert :size="14" class="text-[var(--grape)]" />
          <span class="rounded-full bg-[var(--surface-2)] px-2 py-0.5 text-xs">{{ r.target_type === 'submission' ? '投稿' : '评论' }} #{{ r.target_id }}</span>
          <span v-if="r.status === 'pending'" class="rounded-full bg-warn/15 px-2 py-0.5 text-xs text-warn">待处理</span>
          <span v-else class="rounded-full bg-[var(--jade)]/15 px-2 py-0.5 text-xs text-[var(--jade)]">{{ resultLabel[r.result] || '已处理' }}</span>
          <span class="ml-auto text-xs text-[var(--text-soft)]">{{ r.created_at }}</span>
        </div>
        <p class="mt-2 text-sm">举报理由：{{ r.reason }}</p>
        <p v-if="r.result_reason" class="mt-1 text-sm text-[var(--text-soft)]">处理结果：{{ r.result_reason }}</p>
      </div>
      <div v-if="!reports.length" class="paper-card p-10 text-center text-[var(--text-soft)]">暂无举报记录</div>
    </div>
  </div>
</template>
