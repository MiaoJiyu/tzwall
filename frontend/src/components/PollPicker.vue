<script setup lang="ts">
import { ElMessage } from 'element-plus'
import { Check } from 'lucide-vue-next'

const props = defineProps<{ modelValue: number[]; polls: any[]; max: number; currentSubmissionId?: number }>()
const emit = defineEmits(['update:modelValue'])

function linkable(p: any) {
  return !p.expired && (!p.linked_submission_id || p.linked_submission_id === props.currentSubmissionId)
}
function toggle(p: any) {
  if (!linkable(p)) {
    ElMessage.warning('该投票已截止或已关联其他投稿')
    return
  }
  const cur = [...props.modelValue]
  const i = cur.indexOf(p.id)
  if (i >= 0) {
    cur.splice(i, 1)
  } else {
    if (cur.length >= props.max) {
      ElMessage.warning(`最多关联 ${props.max} 个投票`)
      return
    }
    cur.push(p.id)
  }
  emit('update:modelValue', cur)
}
</script>

<template>
  <div>
    <p class="mb-2 text-xs leading-relaxed text-[var(--text-soft)]">
      最多可关联 <b>{{ max }}</b> 个，当前已选 <b>{{ modelValue.length }}</b> 个。只能选择自己发起、未截止且未被其他投稿占用的投票。
    </p>
    <div v-if="!polls.length" class="rounded-lg border border-dashed border-[var(--border)] py-5 text-center text-sm text-[var(--text-soft)]">
      你还没有发起过投票，<router-link to="/polls" class="text-[var(--grape)] hover:underline">去发起一个</router-link>。
    </div>
    <div v-else class="max-h-64 space-y-2 overflow-y-auto">
      <button
        v-for="p in polls"
        :key="p.id"
        @click="toggle(p)"
        :disabled="!linkable(p)"
        class="flex w-full items-start gap-2 rounded-lg border px-3 py-2 text-left text-sm transition"
        :class="[modelValue.includes(p.id) ? 'border-[var(--grape)] bg-grape-50' : 'border-[var(--border)]', !linkable(p) ? 'cursor-not-allowed opacity-50' : 'hover:border-[var(--grape)]/50']"
      >
        <span class="mt-0.5 flex h-4 w-4 shrink-0 items-center justify-center rounded border text-white" :class="modelValue.includes(p.id) ? 'border-[var(--grape)] bg-[var(--grape)]' : 'border-[var(--border)]'">
          <Check v-if="modelValue.includes(p.id)" :size="10" />
        </span>
        <span class="min-w-0 flex-1">
          <span class="block truncate font-medium">{{ p.title }}</span>
          <span class="text-xs text-[var(--text-soft)]">
            {{ p.expired ? '已截止' : (p.total_votes + ' 票') }}
            <template v-if="p.linked_submission_id && p.linked_submission_id !== currentSubmissionId"> · 已关联其他投稿</template>
          </span>
        </span>
      </button>
    </div>
  </div>
</template>
