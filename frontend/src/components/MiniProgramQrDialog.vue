<script setup lang="ts">
defineProps<{
  modelValue: boolean
  title?: string
  name?: string
  qrcode?: string
  imageBase64?: string // 动态生成的小程序码（自动绑定模式，优先展示）
  desc?: string
  code?: string
  codeHint?: string
  showRefresh?: boolean
}>()
const emit = defineEmits(['update:modelValue', 'refresh'])
</script>

<template>
  <el-dialog
    :model-value="modelValue"
    :title="title || '微信扫码'"
    width="400px"
    @update:model-value="(v: boolean) => emit('update:modelValue', v)"
  >
    <div class="space-y-3 text-center">
      <p v-if="name" class="text-sm font-medium text-[var(--text)]">{{ name }}</p>
      <div class="mx-auto flex h-56 w-56 items-center justify-center rounded-xl border border-[var(--border)] bg-[var(--surface-2)] p-2">
        <img v-if="imageBase64" :src="imageBase64" class="h-full w-full object-contain" alt="小程序码" />
        <img v-else-if="qrcode" :src="qrcode" class="h-full w-full object-contain" alt="小程序码" />
        <span v-else class="px-3 text-xs leading-relaxed text-[var(--text-soft)]">管理员尚未配置小程序码</span>
      </div>
      <div v-if="code" class="rounded-lg border border-[var(--border)] bg-[var(--surface-2)] p-4">
        <div class="text-xs text-[var(--text-soft)]">授权码</div>
        <div class="mt-1 font-mono text-2xl font-bold tracking-widest text-[var(--grape)]">{{ code }}</div>
        <div class="mt-1 text-xs leading-relaxed text-[var(--text-soft)]">{{ codeHint || '在小程序「绑定微信」页输入此授权码，10 分钟内有效、一次使用' }}</div>
      </div>
      <p v-if="desc" class="whitespace-pre-wrap text-sm leading-relaxed text-[var(--text-soft)]">{{ desc }}</p>
    </div>
    <template v-if="showRefresh" #footer>
      <div class="flex justify-end gap-2">
        <button class="rounded-full px-4 py-1.5 text-sm" @click="emit('update:modelValue', false)">关闭</button>
        <button class="btn-grape rounded-full px-4 py-1.5 text-sm text-white" @click="emit('refresh')">我已绑定，刷新</button>
      </div>
    </template>
  </el-dialog>
</template>
