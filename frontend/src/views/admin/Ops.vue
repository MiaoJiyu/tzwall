<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { HardDrive, Database, Cloud, Archive, RefreshCw, CircleCheck, CircleAlert, CircleOff } from 'lucide-vue-next'
import { ElMessage } from 'element-plus'
import { opsApi } from '@/api'

const data = ref<any>(null)
const loading = ref(false)

function fmtBytes(b: number) {
  if (!b) return '0 B'
  const u = ['B', 'KB', 'MB', 'GB', 'TB']
  const i = Math.floor(Math.log(b) / Math.log(1024))
  return (b / Math.pow(1024, i)).toFixed(i === 0 ? 0 : 1) + ' ' + u[i]
}

function healthIcon(status: string) {
  if (status === 'ok') return CircleCheck
  if (status === 'warn') return CircleAlert
  return CircleOff
}

function healthClass(status: string) {
  if (status === 'ok') return 'text-[#2F7D5C]'
  if (status === 'warn') return 'text-[#C8862B]'
  if (status === 'down') return 'text-[#B3453A]'
  return 'text-[var(--text-soft)]'
}

function healthText(status: string) {
  return status === 'ok' ? '正常' : status === 'warn' ? '警告' : status === 'down' ? '异常' : '未启用'
}

async function load() {
  loading.value = true
  try {
    data.value = await opsApi.overview()
  } catch {
    ElMessage.error('运维数据加载失败')
  } finally {
    loading.value = false
  }
}

onMounted(load)
</script>

<template>
  <div>
    <div class="mb-5 flex flex-wrap items-center justify-between gap-3">
      <h1 class="font-serif-sc text-2xl font-bold text-[var(--grape)]">运维面板</h1>
      <button class="flex items-center gap-1 rounded-full border border-[var(--border)] px-4 py-1.5 text-sm hover:text-[var(--grape)]" :class="{ 'animate-pulse': loading }" @click="load">
        <RefreshCw :size="14" /> 刷新
      </button>
    </div>

    <!-- 告警条 -->
    <div v-if="data?.alerts?.length" class="mb-4 space-y-2">
      <div
        v-for="(a, i) in data.alerts"
        :key="i"
        class="flex items-center gap-2 rounded-xl border border-[#B3453A]/30 bg-[#B3453A]/10 px-4 py-3 text-sm text-[#B3453A]"
      >
        <CircleAlert :size="16" /> {{ a }}
      </div>
    </div>

    <!-- 指标卡 -->
    <div class="mb-4 grid gap-3 sm:grid-cols-2 xl:grid-cols-4">
      <div class="paper-card p-4" :class="data?.disk?.used_percent >= 85 ? 'border-[#B3453A]/40' : ''">
        <div class="mb-2 flex items-center gap-2 text-[var(--text-soft)]"><HardDrive :size="16" /> 磁盘</div>
        <p class="font-serif-sc text-xl font-bold">{{ data ? fmtBytes(data.disk.free) : '—' }} 可用</p>
        <p class="mt-1 text-xs text-[var(--text-soft)]">
          共 {{ data ? fmtBytes(data.disk.total) : '—' }} · 已用 {{ data ? data.disk.used_percent.toFixed(1) : '—' }}%
        </p>
        <div class="mt-2 h-1.5 overflow-hidden rounded-full bg-[var(--surface-2)]">
          <div class="h-full rounded-full bg-grape-500" :style="{ width: (data?.disk?.used_percent || 0) + '%' }"></div>
        </div>
      </div>

      <div class="paper-card p-4">
        <div class="mb-2 flex items-center gap-2 text-[var(--text-soft)]"><Database :size="16" /> 数据库</div>
        <p class="font-serif-sc text-xl font-bold">{{ data ? fmtBytes(data.database.bytes) : '—' }}</p>
        <p class="mt-1 text-xs text-[var(--text-soft)]">PostgreSQL 当前体积</p>
      </div>

      <div class="paper-card p-4" :class="data?.cos?.exceeded ? 'border-[#C8862B]/40' : ''">
        <div class="mb-2 flex items-center gap-2 text-[var(--text-soft)]"><Cloud :size="16" /> COS 估算用量</div>
        <p class="font-serif-sc text-xl font-bold">{{ data ? fmtBytes(data.cos.bytes) : '—' }}</p>
        <p class="mt-1 text-xs text-[var(--text-soft)]">{{ data?.cos?.objects || 0 }} 个对象 · 告警阈值 {{ data?.cos?.alert_gb || 0 }} GB</p>
      </div>

      <div class="paper-card p-4">
        <div class="mb-2 flex items-center gap-2 text-[var(--text-soft)]"><HardDrive :size="16" /> 本地上传目录</div>
        <p class="font-serif-sc text-xl font-bold">{{ data ? fmtBytes(data.uploads.bytes) : '—' }}</p>
        <p class="mt-1 truncate text-xs text-[var(--text-soft)]" :title="data?.uploads?.path">{{ data?.uploads?.path }}</p>
      </div>
    </div>

    <div class="grid gap-4 lg:grid-cols-2">
      <!-- 健康状态 -->
      <div class="paper-card p-5">
        <h2 class="mb-4 font-serif-sc text-lg font-semibold">服务健康</h2>
        <div class="space-y-2">
          <div v-for="h in data?.health || []" :key="h.key" class="flex items-center gap-3 rounded-lg bg-[var(--surface-2)] px-3 py-2.5">
            <component :is="healthIcon(h.status)" :size="18" :class="healthClass(h.status)" />
            <span class="text-sm font-medium">{{ h.label }}</span>
            <span class="ml-auto truncate text-xs text-[var(--text-soft)]" :title="h.detail">{{ h.detail }}</span>
            <span class="shrink-0 text-xs" :class="healthClass(h.status)">{{ healthText(h.status) }}</span>
          </div>
        </div>
      </div>

      <!-- 备份 -->
      <div class="paper-card p-5">
        <div class="mb-4 flex items-center justify-between">
          <h2 class="flex items-center gap-2 font-serif-sc text-lg font-semibold"><Archive :size="16" /> 数据库备份</h2>
          <span class="text-xs text-[var(--text-soft)]">
            最近备份：{{ data?.backups?.last_run_at ? new Date(data.backups.last_run_at).toLocaleString('zh-CN') : '无记录' }}
          </span>
        </div>
        <div class="overflow-x-auto">
          <table class="w-full text-sm">
            <thead>
              <tr class="border-b border-[var(--border)] text-left text-[var(--text-soft)]">
                <th class="py-2">文件</th>
                <th class="py-2">大小</th>
                <th class="py-2">时间</th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="f in data?.backups?.files || []" :key="f.name" class="border-b border-[var(--border)]">
                <td class="max-w-[220px] truncate py-2 font-mono text-xs">{{ f.name }}</td>
                <td class="py-2 text-xs text-[var(--text-soft)]">{{ fmtBytes(f.size) }}</td>
                <td class="py-2 text-xs text-[var(--text-soft)]">{{ new Date(f.updated_at).toLocaleString('zh-CN') }}</td>
              </tr>
            </tbody>
          </table>
          <p v-if="!(data?.backups?.files || []).length" class="py-8 text-center text-sm text-[var(--text-soft)]">
            目录 {{ data?.backups?.dir || '/var/backups/tzwall' }} 下暂无备份文件
          </p>
        </div>
        <p class="mt-3 text-xs text-[var(--text-soft)]">每日备份由 systemd timer 执行（保留 7 份），无需在后台手动触发。</p>
      </div>
    </div>
  </div>
</template>
