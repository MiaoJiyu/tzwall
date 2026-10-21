<script setup lang="ts">
import { ref } from 'vue'
import { Video, X } from 'lucide-vue-next'
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
      ElMessage.warning(`最多上传 ${props.max} 个视频`)
      break
    }
    uploading.value = true
    try {
      const r = await submissionApi.uploadVideo(f)
      emit('update:modelValue', [
        ...props.modelValue,
        { id: r.id, url: r.url || submissionApi.videoUrl(r.id), content_type: r.content_type, size: r.size },
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
function fmtSize(n?: number) {
  if (!n) return ''
  return (n / 1024 / 1024).toFixed(1) + ' MB'
}
</script>

<template>
  <div>
    <div class="grid grid-cols-2 gap-3 sm:grid-cols-3">
      <div v-for="(v, idx) in modelValue" :key="v.id" class="group relative overflow-hidden rounded-lg border border-[var(--border)]">
        <video :src="v.url" class="h-24 w-full bg-black object-cover" preload="metadata" controls></video>
        <div class="truncate px-2 py-1 text-xs text-[var(--text-soft)]">{{ fmtSize(v.size) }}</div>
        <button class="absolute right-1 top-1 rounded-full bg-black/60 p-1 text-white opacity-0 transition group-hover:opacity-100" @click="remove(idx)">
          <X :size="14" />
        </button>
      </div>
      <label class="flex h-24 cursor-pointer flex-col items-center justify-center gap-1 rounded-lg border border-dashed border-[var(--border)] text-[var(--text-soft)] transition hover:border-[var(--grape)] hover:text-[var(--grape)]">
        <Video :size="20" />
        <span class="text-xs">{{ uploading ? '上传中…' : '上传视频' }}</span>
        <input type="file" accept="video/mp4,video/webm,video/quicktime,video/*" multiple class="hidden" @change="onFile" :disabled="uploading" />
      </label>
    </div>
  </div>
</template>
