<script setup lang="ts">
import { useRouter } from 'vue-router'
import { Heart, MessageCircle, Eye, Image as ImageIcon, Video as VideoIcon, Ban, Gavel } from 'lucide-vue-next'
import { useAuthStore } from '@/stores/auth'

const props = defineProps<{ item: any }>()
const emit = defineEmits(['penalty', 'takedown'])
const auth = useAuthStore()
const router = useRouter()
function open() {
  router.push('/submission/' + props.item.id)
}
</script>

<template>
  <article class="paper-card cursor-pointer p-5 animate-fadeup" @click="open">
    <div class="flex items-center gap-2 text-xs text-[var(--text-soft)]">
      <span class="rounded-full bg-[var(--surface-2)] px-2 py-0.5 font-medium text-[var(--grape)]">{{ item.anonymous ? '匿名用户' : item.author }}</span>
      <span>{{ item.created_at }}</span>
      <span v-if="item.status && item.status !== 'approved'" class="rounded bg-warn/20 px-1.5 py-0.5 text-warn">{{ item.status }}</span>
    </div>
    <h3 class="mt-2 font-serif-sc text-lg font-bold leading-snug">{{ item.title }}</h3>
    <p v-if="item.content" class="mt-2 line-clamp-3 text-sm text-[var(--text-soft)]">{{ item.content }}</p>
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
      <span class="inline-flex items-center gap-1"><Heart :size="14" /> {{ item.like_count - item.dislike_count }}</span>
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
