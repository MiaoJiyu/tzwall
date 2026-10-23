<script setup lang="ts">
import { computed, ref } from 'vue'
import ContentMarkdown from './ContentMarkdown.vue'

const props = defineProps<{ modelValue: string; placeholder?: string }>()
const emit = defineEmits(['update:modelValue'])
const ta = ref<HTMLTextAreaElement | null>(null)
const mobileTab = ref<'edit' | 'preview'>('edit')

const val = computed({
  get: () => props.modelValue,
  set: (v: string) => emit('update:modelValue', v),
})

// 在光标处包裹选中文本（加粗/斜体/行内代码/行内公式/链接）
function surround(before: string, after: string, placeholder: string) {
  const el = ta.value
  const cur = props.modelValue || ''
  if (!el) {
    emit('update:modelValue', cur + before + placeholder + after)
    return
  }
  const s = el.selectionStart
  const e = el.selectionEnd
  const sel = cur.slice(s, e) || placeholder
  const next = cur.slice(0, s) + before + sel + after + cur.slice(e)
  emit('update:modelValue', next)
  requestAnimationFrame(() => {
    el.focus()
    const pos = s + before.length + sel.length
    el.setSelectionRange(pos, pos)
  })
}

// 在光标所在行行首插入前缀（标题/引用/列表）
function linePrefix(prefix: string) {
  const el = ta.value
  const cur = props.modelValue || ''
  if (!el) {
    emit('update:modelValue', prefix + cur)
    return
  }
  const s = el.selectionStart
  const lineStart = cur.lastIndexOf('\n', Math.max(0, s - 1)) + 1
  const next = cur.slice(0, lineStart) + prefix + cur.slice(lineStart)
  emit('update:modelValue', next)
  requestAnimationFrame(() => {
    el.focus()
    const pos = lineStart + prefix.length
    el.setSelectionRange(pos, pos)
  })
}

// 追加块级内容（代码块/公式块）
function insertBlock(text: string) {
  const cur = props.modelValue || ''
  const sep = cur && !cur.endsWith('\n') ? '\n' : ''
  emit('update:modelValue', cur + sep + text)
}

const tools: { label: string; title: string; fn: () => void }[] = [
  { label: 'B', title: '加粗', fn: () => surround('**', '**', '加粗文本') },
  { label: 'I', title: '斜体', fn: () => surround('*', '*', '斜体文本') },
  { label: 'H', title: '标题', fn: () => linePrefix('## ') },
  { label: '❝', title: '引用', fn: () => linePrefix('> ') },
  { label: '• 列表', title: '无序列表', fn: () => linePrefix('- ') },
  { label: '1. 列表', title: '有序列表', fn: () => linePrefix('1. ') },
  { label: '链接', title: '链接', fn: () => surround('[', '](https://)', '链接文字') },
  { label: '</>', title: '行内代码', fn: () => surround('`', '`', 'code') },
  { label: '代码块', title: '代码块', fn: () => insertBlock('\n```js\n// 在此编写代码\n```\n') },
  { label: 'x²', title: '行内公式', fn: () => surround('$', '$', 'E=mc^2') },
  { label: '∑ 块', title: '独立公式块', fn: () => insertBlock('\n$$\n\\int_0^1 x\\,dx = \\frac{1}{2}\n$$\n') },
]
</script>

<template>
  <div class="md-editor">
    <!-- 工具栏 -->
    <div class="flex flex-wrap items-center gap-1 rounded-t-xl border border-[var(--border)] bg-[var(--surface-2)] p-1.5">
      <button
        v-for="(t, i) in tools"
        :key="i"
        type="button"
        :title="t.title"
        class="rounded-md px-2 py-1 text-xs text-[var(--text-soft)] hover:bg-grape-50 hover:text-[var(--grape)]"
        @click="t.fn"
      >
        {{ t.label }}
      </button>
      <!-- 移动端：编辑/预览切换 -->
      <div class="ml-auto flex gap-1 md:hidden">
        <button type="button" :class="mobileTab === 'edit' ? 'bg-grape-500 text-white' : 'text-[var(--text-soft)]'" class="rounded-md px-2.5 py-1 text-xs" @click="mobileTab = 'edit'">编辑</button>
        <button type="button" :class="mobileTab === 'preview' ? 'bg-grape-500 text-white' : 'text-[var(--text-soft)]'" class="rounded-md px-2.5 py-1 text-xs" @click="mobileTab = 'preview'">预览</button>
      </div>
    </div>

    <div class="grid grid-cols-1 md:grid-cols-2">
      <!-- 编辑区 -->
      <textarea
        ref="ta"
        v-model="val"
        :placeholder="placeholder || '支持 Markdown 与 LaTeX，例如 $E=mc^2$ 或 $$\\int_0^1 x\\,dx$$'"
        :class="[mobileTab === 'preview' ? 'hidden' : 'block', 'md:block']"
        class="min-h-[240px] w-full resize-y rounded-bl-xl border border-[var(--border)] border-t-0 bg-[var(--surface-2)] p-3 text-sm leading-relaxed outline-none focus:border-[var(--grape)]"
      ></textarea>
      <!-- 预览区（桌面常显，移动端按 tab） -->
      <div
        :class="[mobileTab === 'preview' ? 'block' : 'hidden', 'md:block']"
        class="md-content-scroll min-h-[240px] overflow-y-auto rounded-br-xl border border-[var(--border)] border-l-0 border-t-0 bg-[var(--surface)] p-3 text-sm md:border-l"
      >
        <ContentMarkdown :source="modelValue" />
        <p v-if="!modelValue" class="text-sm text-[var(--text-soft)]">预览区域：实时显示渲染后的效果。</p>
      </div>
    </div>
  </div>
</template>
