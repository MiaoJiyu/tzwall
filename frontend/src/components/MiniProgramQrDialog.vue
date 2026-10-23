<script setup lang="ts">
defineProps<{ modelValue: boolean; title?: string; name?: string; qrcode?: string; desc?: string }>()
const emit = defineEmits(['update:modelValue'])
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
        <img v-if="qrcode" :src="qrcode" class="h-full w-full object-contain" alt="小程序码" />
        <span v-else class="px-3 text-xs leading-relaxed text-[var(--text-soft)]">管理员尚未配置小程序码</span>
      </div>
      <p v-if="desc" class="whitespace-pre-wrap text-sm leading-relaxed text-[var(--text-soft)]">{{ desc }}</p>
    </div>
  </el-dialog>
</template>
