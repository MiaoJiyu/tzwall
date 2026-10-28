<script setup lang="ts">
import { ref, watch } from 'vue'
import { X } from 'lucide-vue-next'

const props = defineProps<{ modelValue: boolean; pinned?: boolean }>()
const emit = defineEmits<{
  'update:modelValue': [boolean]
  confirm: [payload: { order: number; expiresAt: string }]
}>()

const visible = ref(false)
const order = ref(0)
const expiresAt = ref('') // 留空=永久置顶

watch(
  () => props.modelValue,
  (v) => {
    visible.value = v
    if (v) {
      order.value = 0
      expiresAt.value = ''
    }
  },
)

function close() {
  emit('update:modelValue', false)
}
function submit() {
  emit('confirm', { order: order.value, expiresAt: expiresAt.value || '' })
  close()
}
</script>

<template>
  <!-- Teleport 到 body：完全脱离投稿卡片等任何可能带 transform 的祖先，
       保证遮罩始终以浏览器视口为基准居中，不随卡片滚动/位移/布局变化而偏移 -->
  <Teleport to="body">
    <div v-if="visible" class="fixed inset-0 z-[9997] flex items-center justify-center bg-black/55 px-4">
      <div class="paper-card w-full max-w-md animate-fadeup p-6">
        <div class="flex items-center justify-between">
          <h3 class="font-serif-sc text-lg font-bold">{{ pinned ? '修改置顶设置' : '置顶投稿' }}</h3>
          <button class="text-[var(--text-soft)] transition hover:text-[var(--text)]" @click="close"><X :size="18" /></button>
        </div>

        <div class="mt-4 space-y-3 text-sm">
          <div>
            <label class="mb-1 block text-[var(--text-soft)]">排序权重（数字越小越靠前）</label>
            <el-input-number v-model="order" :min="0" :max="9999" />
          </div>
          <div>
            <label class="mb-1 block text-[var(--text-soft)]">置顶有效期（可选，留空为永久置顶）</label>
            <el-date-picker
              v-model="expiresAt"
              type="datetime"
              placeholder="留空 = 永久置顶"
              format="YYYY-MM-DD HH:mm:ss"
              value-format="YYYY-MM-DD HH:mm:ss"
              :clearable="true"
              style="width: 100%"
            />
          </div>
          <p class="text-xs leading-relaxed text-[var(--text-soft)]">
            到期时间一到，该投稿会自动取消置顶（无需手动操作）。
          </p>
        </div>

        <div class="mt-5 flex justify-end gap-2">
          <button class="rounded-full border border-[var(--border)] px-4 py-2 text-sm transition hover:bg-[var(--surface-2)]" @click="close">取消</button>
          <button class="btn-grape rounded-full px-5 py-2 text-sm text-white" @click="submit">{{ pinned ? '保存' : '确认置顶' }}</button>
        </div>
      </div>
    </div>
  </Teleport>
</template>
