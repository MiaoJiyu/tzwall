<script setup lang="ts">
import { reactive } from 'vue'
import { useRouter } from 'vue-router'
import { Check } from 'lucide-vue-next'
import { ElMessage } from 'element-plus'
import { pollApi } from '@/api'
import { useAuthStore } from '@/stores/auth'

const props = defineProps<{ poll: any }>()
const emit = defineEmits(['voted'])
const auth = useAuthStore()
const router = useRouter()
const selected = reactive<number[]>(Array.isArray(props.poll.my_option_ids) ? [...props.poll.my_option_ids] : [])

function toggle(oid: number) {
  if (props.poll.expired) return
  if (props.poll.multiple) {
    const i = selected.indexOf(oid)
    if (i >= 0) selected.splice(i, 1)
    else selected.push(oid)
  } else {
    selected.splice(0, selected.length, oid)
  }
}
async function vote() {
  if (!auth.isLoggedIn) {
    router.push('/login')
    return
  }
  if (props.poll.expired) {
    ElMessage.warning('投票已截止')
    return
  }
  if (!selected.length) {
    ElMessage.warning('请先选择选项')
    return
  }
  await pollApi.vote(props.poll.id, { option_ids: [...selected] })
  ElMessage.success('投票成功')
  emit('voted')
}
</script>

<template>
  <div class="rounded-lg border border-[var(--border)] p-3">
    <div class="flex items-center justify-between gap-2">
      <p class="text-sm font-medium">{{ poll.title }}</p>
      <span v-if="poll.expired" class="shrink-0 text-xs text-[var(--text-soft)]">已截止</span>
      <span v-else class="shrink-0 text-xs text-[var(--text-soft)]">共 {{ poll.total_votes }} 票</span>
    </div>
    <p v-if="poll.description" class="mt-0.5 text-xs text-[var(--text-soft)]">{{ poll.description }}</p>
    <div class="mt-2 space-y-1.5">
      <button
        v-for="o in poll.options"
        :key="o.id"
        @click="toggle(o.id)"
        :disabled="poll.expired"
        class="flex w-full items-center gap-2 rounded-lg border px-2.5 py-1.5 text-left text-xs transition"
        :class="selected.includes(o.id) ? 'border-[var(--grape)] bg-grape-50' : 'border-[var(--border)] hover:border-[var(--grape)]/50'"
      >
        <span class="flex h-3.5 w-3.5 items-center justify-center rounded-full border" :class="selected.includes(o.id) ? 'border-[var(--grape)] bg-[var(--grape)] text-white' : 'border-[var(--border)]'">
          <Check v-if="selected.includes(o.id)" :size="9" />
        </span>
        <span class="flex-1">{{ o.text }}</span>
        <span class="text-[var(--text-soft)]">{{ o.count }}</span>
      </button>
    </div>
    <div v-if="!poll.expired" class="mt-1.5 flex justify-end">
      <button class="rounded-full bg-grape-500 px-3 py-1 text-xs text-white" @click="vote">{{ selected.length ? '改票' : '投票' }}</button>
    </div>
  </div>
</template>
