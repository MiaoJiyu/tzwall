<script setup lang="ts">
import { computed } from 'vue'
import MarkdownIt from 'markdown-it'
import * as katexPluginModule from '@vscode/markdown-it-katex'
import katex from 'katex'
import hljs from 'highlight.js'
import DOMPurify from 'dompurify'
import 'katex/dist/katex.min.css'
import 'highlight.js/styles/github.css'

// Markdown 渲染器：标题/列表/链接/代码块/表格 + KaTeX 行内($..$)与独立($$..$$)公式。
// html:false 转义原始 HTML 防 XSS；breaks:true 让单换行生效以兼容旧纯文本显示；
// 代码块用 highlight.js 高亮；最终经 DOMPurify 白名单净化（保留 KaTeX 的 class/style）。
const md = new MarkdownIt({ html: false, linkify: true, breaks: true, typographer: false })
md.options.highlight = (code: string, lang: string) => {
  let out = ''
  try {
    out = lang && hljs.getLanguage(lang)
      ? hljs.highlight(code, { language: lang, ignoreIllegals: true }).value
      : hljs.highlightAuto(code).value
  } catch {
    out = md.utils.escapeHtml(code)
  }
  return '<pre class="hljs"><code>' + out + '</code></pre>'
}
// @vscode/markdown-it-katex 的导出在不同打包器下互操作形状不一致，稳健取出插件函数。
const katexPlugin: any = (katexPluginModule as any).default?.default || (katexPluginModule as any).default || (katexPluginModule as any)
md.use(katexPlugin, katex)

const props = defineProps<{ source?: string }>()

const rendered = computed(() => {
  const src = props.source || ''
  if (!src.trim()) return ''
  const raw = md.render(src)
  return DOMPurify.sanitize(raw, {
    ADD_ATTR: ['style', 'target', 'rel', 'xmlns', 'encoding', 'aria-hidden', 'aria-label', 'aria-describedby'],
    ADD_TAGS: ['math', 'semantics', 'mrow', 'mi', 'mo', 'mn', 'msup', 'msub', 'mfrac', 'msqrt', 'annotation'],
  }) as string
})
</script>

<template>
  <div v-if="rendered" class="md-content" v-html="rendered"></div>
</template>

<style scoped>
.md-content { line-height: 1.7; word-break: break-word; }
.md-content :deep(p) { margin: 8px 0; }
.md-content :deep(a) { color: var(--grape); text-decoration: underline; word-break: break-all; }
.md-content :deep(pre) { overflow-x: auto; border-radius: 10px; padding: 12px 14px; margin: 10px 0; font-size: 13px; }
.md-content :deep(pre code) { background: none; padding: 0; font-size: 13px; }
.md-content :deep(code) { background: var(--surface-2); border-radius: 4px; padding: 1px 5px; font-size: 0.9em; }
.md-content :deep(blockquote) { border-left: 3px solid var(--grape); margin: 10px 0; padding: 4px 12px; color: var(--text-soft); background: var(--surface-2); border-radius: 0 8px 8px 0; }
.md-content :deep(h1), .md-content :deep(h2), .md-content :deep(h3), .md-content :deep(h4) { font-weight: 700; margin: 14px 0 8px; line-height: 1.35; }
.md-content :deep(h1) { font-size: 1.4em; }
.md-content :deep(h2) { font-size: 1.25em; }
.md-content :deep(h3) { font-size: 1.1em; }
.md-content :deep(ul), .md-content :deep(ol) { padding-left: 1.4em; margin: 8px 0; }
.md-content :deep(table) { border-collapse: collapse; margin: 10px 0; display: block; overflow-x: auto; }
.md-content :deep(th), .md-content :deep(td) { border: 1px solid var(--border); padding: 6px 10px; }
.md-content :deep(img) { max-width: 100%; border-radius: 8px; }
.md-content :deep(hr) { border: none; border-top: 1px solid var(--border); margin: 14px 0; }
.md-content :deep(.katex-display) { overflow-x: auto; overflow-y: hidden; padding: 4px 0; }
</style>
