<script setup lang="ts">
import { ref } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import { ticketApi } from '@/api'
import ImageUploader from '@/components/ImageUploader.vue'

const router = useRouter()
const type = ref('bug')
const title = ref('')
const content = ref('')
const attachments = ref<any[]>([])
const loading = ref(false)
const types = [
  { v: 'bug', l: '功能故障' },
  { v: 'feedback', l: '意见反馈' },
  { v: 'report', l: '违规举报' },
  { v: 'appeal', l: '处罚申诉' },
  { v: 'account', l: '账号/安全' },
  { v: 'other', l: '其他' },
]

async function submit() {
  if (!title.value.trim()) {
    ElMessage.warning('请填写工单标题')
    return
  }
  if (!content.value.trim()) {
    ElMessage.warning('请填写工单描述')
    return
  }
  loading.value = true
  try {
    const r = await ticketApi.create({
      type: type.value,
      title: title.value,
      content: content.value,
      attachments: attachments.value.map((a) => ({ id: a.id, url: a.url, content_type: a.content_type })),
    })
    ElMessage.success('工单已提交，我们会尽快处理')
    router.push('/tickets/' + r.id)
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '提交失败，请稍后再试')
  } finally {
    loading.value = false
  }
}
</script>

<template>
  <div class="mx-auto max-w-2xl px-4 py-6">
    <h1 class="mb-5 font-serif-sc text-2xl font-bold text-[var(--grape)]">提交工单</h1>
    <div class="paper-card space-y-5 p-6">
      <div>
        <label class="mb-2 block text-sm font-medium">工单类型 <span class="text-[var(--danger)]">*</span></label>
        <div class="flex flex-wrap gap-2">
          <button
            v-for="t in types"
            :key="t.v"
            @click="type = t.v"
            :class="type === t.v ? 'bg-grape-500 text-white' : 'bg-[var(--surface-2)] text-[var(--text-soft)]'"
            class="rounded-full px-4 py-1.5 text-sm"
          >
            {{ t.l }}
          </button>
        </div>
      </div>
      <div>
        <label class="mb-1 block text-sm font-medium">标题 <span class="text-[var(--danger)]">*</span></label>
        <input v-model="title" maxlength="100" placeholder="一句话概括（≤100 字）" class="input-style" />
      </div>
      <div>
        <label class="mb-1 block text-sm font-medium">描述 <span class="text-[var(--danger)]">*</span></label>
        <textarea v-model="content" rows="6" maxlength="5000" placeholder="请详细描述你遇到的问题或建议" class="input-style h-auto py-3"></textarea>
      </div>
      <div>
        <label class="mb-1 block text-sm font-medium">附件（可选，图片，最多 9 张）</label>
        <ImageUploader v-model="attachments" :max="9" />
      </div>
      <button class="btn-grape w-full rounded-full py-2.5 font-medium" :disabled="loading" @click="submit">
        {{ loading ? '提交中…' : '提交工单' }}
      </button>
    </div>
  </div>
</template>

<style scoped>
.input-style {
  width: 100%;
  border-radius: 12px;
  border: 1px solid var(--border);
  background: var(--surface-2);
  padding: 10px 12px;
  outline: none;
  font-size: 14px;
}
.input-style:focus {
  border-color: var(--grape);
}
</style>
