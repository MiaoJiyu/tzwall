<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'
import { useRouter } from 'vue-router'
import { BarChart3, Check } from 'lucide-vue-next'
import { ElMessage } from 'element-plus'
import { pollApi } from '@/api'
import { useAuthStore } from '@/stores/auth'

const auth = useAuthStore()
const router = useRouter()
const polls = ref<any[]>([])
const selected = reactive<Record<number, number[]>>({})

async function load() {
  try {
    const p = await pollApi.list({ page: 1, page_size: 3 })
    polls.value = (p.items || []).filter((x: any) => !x.expired)
    for (const it of polls.value) selected[it.id] = Array.isArray(it.my_option_ids) ? [...it.my_option_ids] : []
  } catch {
    /* 忽略：投票组件不阻塞首页 */
  }
}
function toggle(p: any, oid: number) {
  const cur = selected[p.id] || []
  selected[p.id] = p.multiple ? (cur.includes(oid) ? cur.filter((x) => x !== oid) : [...cur, oid]) : [oid]
}
async function vote(p: any) {
  if (!auth.isLoggedIn) {
    router.push('/login')
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
onMounted(load)
</script>

<template>
  <div v-if="polls.length" class="paper-card mb-4 p-5">
    <div class="mb-3 flex items-center justify-between">
      <h3 class="flex items-center gap-1 font-serif-sc font-bold text-[var(--grape)]"><BarChart3 :size="16" /> 进行中的投票</h3>
      <router-link to="/polls" class="text-xs text-[var(--grape)] hover:underline">更多 →</router-link>
    </div>
    <div v-for="p in polls" :key="p.id" class="mb-4 last:mb-0">
      <p class="text-sm font-medium">{{ p.title }}</p>
      <div class="mt-2 space-y-1.5">
        <button
          v-for="o in p.options"
          :key="o.id"
          @click="toggle(p, o.id)"
          class="flex w-full items-center gap-2 rounded-lg border px-2.5 py-1.5 text-left text-xs transition"
          :class="(selected[p.id] || []).includes(o.id) ? 'border-[var(--grape)] bg-grape-50' : 'border-[var(--border)] hover:border-[var(--grape)]/50'"
        >
          <span class="flex h-3.5 w-3.5 items-center justify-center rounded-full border" :class="(selected[p.id] || []).includes(o.id) ? 'border-[var(--grape)] bg-[var(--grape)] text-white' : 'border-[var(--border)]'">
            <Check v-if="(selected[p.id] || []).includes(o.id)" :size="9" />
          </span>
          <span class="flex-1">{{ o.text }}</span>
          <span class="text-[var(--text-soft)]">{{ o.count }}</span>
        </button>
      </div>
      <div class="mt-1.5 flex justify-end">
        <button class="rounded-full bg-grape-500 px-3 py-1 text-xs text-white" @click="vote(p)">{{ (p.my_option_ids || []).length ? '改票' : '投票' }}</button>
      </div>
    </div>
  </div>
</template>
