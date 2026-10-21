<script setup lang="ts">
import { ref } from 'vue'
import { Image as ImageIcon, X } from 'lucide-vue-next'
import { ElMessage } from 'element-plus'
import { submissionApi } from '@/api'

const props = defineProps<{ modelValue: any[]; max?: number }>()
const emit = defineEmits(['update:modelValue'])
const uploading = ref(false)

async function onFile(e: any) {
  const files: FileList = e.target.files
  if (!files || !files.length) return
  for (const f of Array.from(files)) {
    if (props.max && props.modelValue.length >= props.max) {
      ElMessage.warning(`最多上传 ${props.max} 张图片`)
      break
    }
    uploading.value = true
    try {
      const r = await submissionApi.uploadImage(f)
      emit('update:modelValue', [
        ...props.modelValue,
        { id: r.id, url: r.url || submissionApi.imageUrl(r.id), content_type: r.content_type },
      ])
    } catch {
      ElMessage.error('上传失败，请重试')
    } finally {
      uploading.value = false
    }
  }
  e.target.value = ''
}
function remove(idx: number) {
  const arr = [...props.modelValue]
  arr.splice(idx, 1)
  emit('update:modelValue', arr)
}
</script>

<template>
  <div>
    <div class="grid grid-cols-3 gap-3 sm:grid-cols-4">
      <div v-for="(im, idx) in modelValue" :key="im.id" class="group relative h-24 overflow-hidden rounded-lg border border-[var(--border)]">
        <img :src="im.url" class="h-full w-full object-cover" alt="" />
        <button class="absolute right-1 top-1 rounded-full bg-black/50 p-1 text-white opacity-0 transition group-hover:opacity-100" @click="remove(idx)">
          <X :size="14" />
        </button>
      </div>
      <label class="flex h-24 cursor-pointer flex-col items-center justify-center gap-1 rounded-lg border border-dashed border-[var(--border)] text-[var(--text-soft)] transition hover:border-[var(--grape)] hover:text-[var(--grape)]">
        <ImageIcon :size="20" />
        <span class="text-xs">{{ uploading ? '上传中…' : '上传图片' }}</span>
        <input type="file" accept="image/*" multiple class="hidden" @change="onFile" :disabled="uploading" />
      </label>
    </div>
  </div>
</template>
