<script setup lang="ts">
import { ref } from 'vue'
import { useRouter } from 'vue-router'
import { Heart, MessageCircle, Eye, Image as ImageIcon, Video as VideoIcon, Ban, Gavel } from 'lucide-vue-next'
import { ElMessage } from 'element-plus'
import { useAuthStore } from '@/stores/auth'
import { interactionApi } from '@/api'
import TitleBadge from '@/components/TitleBadge.vue'

const props = defineProps<{ item: any }>()
const emit = defineEmits(['penalty', 'takedown'])
const auth = useAuthStore()
const router = useRouter()
const statusText: Record<string, string> = { pending: '审核中', approved: '已通过', rejected: '未通过', need_edit: '需修改', withdrawn: '已撤回' }
// plainText 将 Markdown/LaTeX 粗略转为纯文本，用于信息流卡片预览（不显示原始标记）。
function plainText(md: string) {
  if (!md) return ''
  return md
    .replace(/```[\s\S]*?```/g, ' [代码] ')
    .replace(/`([^`]+)`/g, '$1')
    .replace(/\$\$[\s\S]*?\$\$/g, ' [公式] ')
    .replace(/\$([^$]+)\$/g, ' [公式] ')
    .replace(/!\[[^\]]*\]\([^)]*\)/g, ' ')
    .replace(/\[([^\]]+)\]\([^)]*\)/g, '$1')
    .replace(/^\s{0,3}#{1,6}\s+/gm, '')
    .replace(/^\s{0,3}>\s?/gm, '')
    .replace(/^\s{0,3}[-*+]\s+/gm, '')
    .replace(/^\s{0,3}\d+\.\s+/gm, '')
    .replace(/[*_~]{1,2}/g, '')
    .replace(/\s+/g, ' ')
    .trim()
}
function open() {
  router.push('/submission/' + props.item.id)
}
const liking = ref(false)
async function toggleLike() {
  if (!auth.isLoggedIn) {
    router.push('/login')
    return
  }
  if (liking.value) return
  const it = props.item
  const liked = it.my_attitude === 1
  const next = liked ? 0 : 1
  liking.value = true
  it.my_attitude = next
  it.like_count = (it.like_count || 0) + (liked ? -1 : 1)
  try {
    await interactionApi.react({ target_type: 'submission', target_id: it.id, attitude: next })
  } catch (e: any) {
    it.my_attitude = liked ? 1 : 0
    it.like_count = (it.like_count || 0) + (liked ? 1 : -1)
    ElMessage.error(e?.response?.data?.message || '操作失败')
  } finally {
    liking.value = false
  }
}
</script>

<template>
  <article class="paper-card cursor-pointer p-5 animate-fadeup" @click="open">
    <div class="flex items-center gap-2 text-xs text-[var(--text-soft)]">
      <span class="rounded-full bg-[var(--surface-2)] px-2 py-0.5 font-medium text-[var(--grape)]">{{ item.author }}</span>
      <TitleBadge v-if="!item.anonymous" :title="item.author_title" />
      <span>{{ item.created_at }}</span>
      <span v-if="item.status && item.status !== 'approved'" class="rounded bg-warn/20 px-1.5 py-0.5 text-warn">{{ statusText[item.status] || item.status }}</span>
    </div>
    <h3 class="mt-2 font-serif-sc text-lg font-bold leading-snug">{{ item.title }}</h3>
    <p v-if="item.content" class="mt-2 line-clamp-3 text-sm text-[var(--text-soft)]">{{ plainText(item.content) }}</p>
    <div v-if="item.images?.length" class="mt-3 grid grid-cols-3 gap-2">
      <img
        v-for="im in item.images.slice(0, 3)"
        :key="im.id"
        :src="im.url"
        class="h-24 w-full rounded-lg object-cover"
        alt=""
        @error="$event.target.style.display = 'none'"
      />
      <div v-if="item.images.length > 3" class="relative flex h-24 items-center justify-center rounded-lg bg-[var(--surface-2)] text-sm text-[var(--text-soft)]">
        <ImageIcon :size="18" /> +{{ item.images.length - 3 }}
      </div>
    </div>
    <div v-if="item.videos?.length" class="mt-3 grid grid-cols-3 gap-2">
      <div v-for="v in item.videos.slice(0, 3)" :key="v.id" class="relative h-24 overflow-hidden rounded-lg bg-black">
        <video :src="v.url" class="h-full w-full object-cover" preload="metadata"></video>
        <div class="absolute inset-0 flex items-center justify-center"><VideoIcon :size="20" class="text-white/80" /></div>
      </div>
      <div v-if="item.videos.length > 3" class="relative flex h-24 items-center justify-center rounded-lg bg-[var(--surface-2)] text-sm text-[var(--text-soft)]">
        <VideoIcon :size="18" /> +{{ item.videos.length - 3 }}
      </div>
    </div>
    <div v-if="item.tags?.length" class="mt-3 flex flex-wrap gap-1.5">
      <span v-for="t in item.tags" :key="t.id" class="rounded-full bg-grape-50 px-2 py-0.5 text-xs text-[var(--grape)]">#{{ t.name }}</span>
    </div>
    <div class="mt-3 flex items-center gap-4 text-xs text-[var(--text-soft)]">
      <button class="inline-flex items-center gap-1 transition-colors" :class="item.my_attitude === 1 ? 'text-[var(--grape)]' : 'hover:text-[var(--grape)]'" @click.stop="toggleLike">
        <Heart :size="14" :fill="item.my_attitude === 1 ? 'currentColor' : 'none'" /> {{ item.like_count - item.dislike_count }}
      </button>
      <span class="inline-flex items-center gap-1"><MessageCircle :size="14" /> {{ item.comment_count }}</span>
      <span class="inline-flex items-center gap-1"><Eye :size="14" /> {{ item.view_count }}</span>
    </div>

    <!-- 管理员快捷操作（对每条投稿） -->
    <div v-if="auth.isAdmin" class="mt-3 flex flex-wrap gap-2 border-t border-[var(--border)] pt-2" @click.stop>
      <button class="inline-flex items-center gap-1 rounded-full bg-[var(--danger)] px-3 py-1 text-xs text-white" @click.stop="emit('takedown', item)">
        <Ban :size="12" /> 快捷撤下
      </button>
      <button v-if="item.user_id" class="inline-flex items-center gap-1 rounded-full border border-[var(--border)] px-3 py-1 text-xs hover:text-[var(--danger)]" @click.stop="emit('penalty', item)">
        <Gavel :size="12" /> 处罚作者
      </button>
    </div>
  </article>
</template>
