<script setup lang="ts">
import { ref } from 'vue'
import { ScanLine, Loader2, UploadCloud, ImageIcon, ShieldCheck, ShieldAlert } from 'lucide-vue-next'
import { ElMessage } from 'element-plus'
import { watermarkApi } from '@/api'

const file = ref<File | null>(null)
const preview = ref('')
const loading = ref(false)
const result = ref<any>(null)

function onPick(e: Event) {
  const t = e.target as HTMLInputElement
  const f = t.files?.[0]
  if (!f) return
  if (!f.type.startsWith('image/')) {
    ElMessage.warning('请选择图片文件')
    return
  }
  if (f.size > 20 * 1024 * 1024) {
    ElMessage.warning('图片不能超过 20MB')
    return
  }
  file.value = f
  result.value = null
  if (preview.value) URL.revokeObjectURL(preview.value)
  preview.value = URL.createObjectURL(f)
}

async function recognize() {
  if (!file.value) {
    ElMessage.warning('请先选择待识别的图片')
    return
  }
  loading.value = true
  result.value = null
  try {
    result.value = await watermarkApi.extract(file.value)
  } catch {
    ElMessage.error('识别失败，请重试')
  } finally {
    loading.value = false
  }
}
</script>

<template>
  <div>
    <div class="mb-5 flex items-center gap-2">
      <ScanLine :size="22" class="text-[var(--grape)]" />
      <h1 class="font-serif-sc text-2xl font-bold text-[var(--grape)]">盲水印识别</h1>
    </div>

    <div class="paper-card mb-4 p-5">
      <p class="mb-1 text-sm font-medium">泄露溯源</p>
      <p class="mb-4 text-sm text-[var(--text-soft)]">
        本站图片在对外分发时会为每位访客嵌入不可见的<strong>文字盲水印</strong>（含访客 IP 与分发时间）。
        若发现图片被截图外泄，可在此上传该图片，还原其中嵌入的访客信息以定位泄露者。本页面仅超级管理员可见。
      </p>
      <p class="text-xs text-[var(--text-soft)]">
        提示：仅能识别经 <strong>version 3.0</strong> 新方案分发的图片；图片被过度缩放/压缩可能导致无法提取。
      </p>
    </div>

    <div class="paper-card mb-4 p-5">
      <label
        class="flex cursor-pointer flex-col items-center justify-center gap-2 rounded-xl border-2 border-dashed border-[var(--border)] bg-[var(--surface-2)] px-4 py-10 text-[var(--text-soft)] transition hover:border-[var(--grape)] hover:text-[var(--grape)]"
      >
        <UploadCloud :size="32" />
        <span class="text-sm">{{ file ? file.name : '点击选择待识别的图片' }}</span>
        <span class="text-xs">支持 JPG / PNG / WebP 等，不超过 20MB</span>
        <input type="file" accept="image/*" class="hidden" @change="onPick" />
      </label>

      <div class="mt-4 flex items-center gap-3">
        <button
          class="flex items-center gap-1.5 rounded-xl bg-grape-500 px-5 py-2.5 text-sm font-medium text-white transition hover:bg-grape-600 disabled:opacity-60"
          :disabled="loading || !file"
          @click="recognize"
        >
          <Loader2 v-if="loading" :size="16" class="animate-spin" />
          <ScanLine v-else :size="16" />
          {{ loading ? '识别中…' : '开始识别' }}
        </button>
        <span v-if="file" class="text-xs text-[var(--text-soft)]">已选择：{{ file.name }}</span>
      </div>
    </div>

    <!-- 预览 + 结果 -->
    <div v-if="preview || result" class="grid gap-4 lg:grid-cols-2">
      <div v-if="preview" class="paper-card flex items-center justify-center p-4">
        <img :src="preview" alt="待识别图片" class="max-h-[360px] rounded-lg object-contain" />
      </div>

      <div v-if="result" class="paper-card p-5">
        <div class="mb-4 flex items-center gap-2">
          <component :is="result.found ? ShieldCheck : ShieldAlert" :size="18" :class="result.found ? 'text-[#2F7D5C]' : 'text-[#B3453A]'" />
          <span
            class="rounded-full px-3 py-1 text-xs font-medium"
            :class="result.found ? 'bg-[#2F7D5C]/15 text-[#2F7D5C]' : 'bg-[#B3453A]/15 text-[#B3453A]'"
          >
            {{ result.found ? '已识别到盲水印' : '未识别到盲水印' }}
          </span>
        </div>

        <div v-if="result.found" class="space-y-3">
          <div class="rounded-xl bg-[var(--surface-2)] p-4">
            <p class="mb-1 text-xs text-[var(--text-soft)]">访客 IP</p>
            <p class="font-mono text-lg font-semibold">{{ result.ip || '—' }}</p>
          </div>
          <div class="rounded-xl bg-[var(--surface-2)] p-4">
            <p class="mb-1 text-xs text-[var(--text-soft)]">分发时间</p>
            <p class="font-mono text-lg font-semibold">{{ result.embedded_at || '—' }}</p>
          </div>
          <div class="rounded-xl bg-[var(--surface-2)] p-4">
            <p class="mb-1 text-xs text-[var(--text-soft)]">原始文本</p>
            <p class="break-all font-mono text-sm">{{ result.raw }}</p>
          </div>
          <div class="rounded-xl bg-[var(--surface-2)] p-4">
            <p class="mb-1 text-xs text-[var(--text-soft)]">解码文本</p>
            <p class="break-all font-mono text-sm">{{ result.decoded }}</p>
          </div>
        </div>

        <p v-else class="text-sm text-[var(--text-soft)]">
          {{ result.note || '该图片未检测到盲水印。' }}
        </p>
      </div>
    </div>

    <div v-else class="paper-card flex flex-col items-center justify-center gap-3 p-12 text-[var(--text-soft)]">
      <ImageIcon :size="40" />
      <p class="text-sm">上传一张待识别的图片即可还原其中嵌入的盲水印</p>
    </div>
  </div>
</template>
