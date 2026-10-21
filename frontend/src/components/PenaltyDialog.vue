<script setup lang="ts">
import { ref, watch } from 'vue'
import { Gavel, X } from 'lucide-vue-next'
import { ElMessage } from 'element-plus'
import { punishmentApi } from '@/api'

const props = defineProps<{
  modelValue: boolean
  userId: number | null
  username?: string
}>()
const emit = defineEmits<{
  'update:modelValue': [boolean]
  done: []
}>()

const form = ref({ type: 'warn', reason: '', permanent: false, expires_at: '' })
const submitting = ref(false)

// 每次打开重置表单
watch(
  () => props.modelValue,
  (v) => {
    if (v) {
      form.value = { type: 'warn', reason: '', permanent: false, expires_at: '' }
    }
  },
)

async function submit() {
  if (!props.userId) return
  if (!form.value.reason.trim()) {
    ElMessage.warning('处罚理由必填')
    return
  }
  if (!form.value.permanent && !form.value.expires_at) {
    ElMessage.warning('临时处罚必须指定有效时间')
    return
  }
  submitting.value = true
  try {
    await punishmentApi.create({
      user_id: props.userId,
      type: form.value.type,
      reason: form.value.reason,
      permanent: form.value.permanent,
      expires_at: form.value.permanent ? undefined : form.value.expires_at || undefined,
    })
    ElMessage.success('已对 ' + (props.username || '用户#' + props.userId) + ' 执行处罚')
    emit('done')
    emit('update:modelValue', false)
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '处罚失败')
  } finally {
    submitting.value = false
  }
}
</script>

<template>
  <div v-if="modelValue" class="fixed inset-0 z-[9997] flex items-center justify-center bg-black/55 px-4">
    <div class="paper-card w-full max-w-md p-6 animate-fadeup">
      <div class="flex items-center justify-between">
        <h3 class="flex items-center gap-1 font-serif-sc text-lg font-bold">
          <Gavel :size="16" /> 处罚用户
        </h3>
        <button class="text-[var(--text-soft)] hover:text-[var(--text)]" @click="emit('update:modelValue', false)">
          <X :size="18" />
        </button>
      </div>
      <p class="mt-2 text-sm text-[var(--text-soft)]">
        处罚对象：<b>{{ username || '用户#' + userId }}</b>
      </p>

      <div class="mt-4">
        <label class="mb-1 block text-sm font-medium">处罚类型</label>
        <select v-model="form.type" class="input-box">
          <option value="warn">警告</option>
          <option value="mute">禁言</option>
          <option value="ban_user">封禁用户名</option>
        </select>
      </div>

      <div v-if="!form.permanent" class="mt-3">
        <label class="mb-1 block text-sm font-medium">处罚有效时间（临时处罚必填）</label>
        <input v-model="form.expires_at" type="datetime-local" class="input-box" />
      </div>

      <div class="mt-3">
        <label class="mb-1 block text-sm font-medium">处罚理由（必填）</label>
        <textarea v-model="form.reason" rows="3" class="input-box" placeholder="违规说明"></textarea>
      </div>

      <label class="mt-3 flex items-center gap-2 text-sm">
        <input type="checkbox" v-model="form.permanent" /> 永久处罚
      </label>

      <div class="mt-5 flex justify-end gap-2">
        <button class="rounded-full border border-[var(--border)] px-4 py-2 text-sm" @click="emit('update:modelValue', false)">取消</button>
        <button class="btn-grape rounded-full px-5 py-2 text-sm" :disabled="submitting" @click="submit">确认处罚</button>
      </div>
    </div>
  </div>
</template>

<style scoped>
.input-box {
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
