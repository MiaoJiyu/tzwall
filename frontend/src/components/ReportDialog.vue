<script setup lang="ts">
import { ref, watch } from 'vue'
import { Flag } from 'lucide-vue-next'
import { ElMessage } from 'element-plus'
import { reportApi } from '@/api'

const props = defineProps<{ modelValue: boolean; targetType: string; targetId: number }>()
const emit = defineEmits(['update:modelValue', 'done'])
const reason = ref('')
const loading = ref(false)

watch(
  () => props.modelValue,
  (v) => {
    if (v) reason.value = ''
  },
)

async function submit() {
  if (!reason.value.trim()) {
    ElMessage.warning('请填写举报理由')
    return
  }
  loading.value = true
  try {
    await reportApi.create({ target_type: props.targetType, target_id: props.targetId, reason: reason.value })
    ElMessage.success('举报已提交，管理员会尽快处理')
    emit('update:modelValue', false)
    emit('done')
  } finally {
    loading.value = false
  }
}
</script>

<template>
  <div v-if="modelValue" class="fixed inset-0 z-[9996] flex items-center justify-center bg-black/55 px-4">
    <div class="paper-card w-full max-w-md p-6 animate-fadeup">
      <h3 class="flex items-center gap-2 font-serif-sc text-lg font-bold text-[var(--grape)]">
        <Flag :size="18" /> 举报内容
      </h3>
      <p class="mt-1 text-xs text-[var(--text-soft)]">同一内容只能举报一次，请如实填写理由。</p>
      <textarea
        v-model="reason"
        rows="4"
        maxlength="200"
        placeholder="请说明举报理由（必填，200 字以内）"
        class="mt-3 w-full rounded-lg border border-[var(--border)] bg-[var(--surface-2)] p-3 text-sm outline-none focus:border-[var(--grape)]"
      ></textarea>
      <div class="mt-4 flex justify-end gap-2">
        <button class="rounded-full border border-[var(--border)] px-4 py-2 text-sm" @click="emit('update:modelValue', false)">取消</button>
        <button class="btn-grape rounded-full px-5 py-2 text-sm" :disabled="loading" @click="submit">
          {{ loading ? '提交中…' : '提交举报' }}
        </button>
      </div>
    </div>
  </div>
</template>
