<script setup lang="ts">
import { ref } from 'vue'
import { useRouter } from 'vue-router'
import { Reply, Trash2, Flag, Ban, Gavel, RotateCcw, ThumbsUp } from 'lucide-vue-next'
import { ElMessage } from 'element-plus'
import { useAuthStore } from '@/stores/auth'
import { commentApi } from '@/api'
import TitleBadge from '@/components/TitleBadge.vue'

const props = defineProps<{ node: any; submissionId: number }>()
const emit = defineEmits(['reply', 'delete', 'report', 'moderate-remove', 'moderate-restore', 'penalty'])
const auth = useAuthStore()
const router = useRouter()

// 点赞（含防抖：请求进行中忽略重复点击；未登录给引导提示）
const liking = ref(false)
async function onLike() {
  if (props.node.deleted) return
  if (!auth.isLoggedIn) {
    ElMessage.warning('请先登录后再点赞')
    router.push('/login')
    return
  }
  if (liking.value) return
  liking.value = true
  try {
    const r: any = await commentApi.like(props.node.id)
    props.node.my_liked = !!r.liked
    props.node.like_count = r.like_count ?? 0
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '操作失败，请稍后再试')
  } finally {
    liking.value = false
  }
}
</script>

<template>
  <div class="comment">
    <div class="flex gap-2">
      <div class="h-8 w-8 shrink-0 overflow-hidden rounded-full bg-grape-100">
        <img v-if="node.avatar && !node.deleted" :src="node.avatar" class="h-full w-full object-cover" alt="" />
        <div v-else class="flex h-full w-full items-center justify-center text-sm font-semibold text-[var(--grape)]">
          {{ node.deleted ? '—' : (node.username || '匿')?.[0] || '匿' }}
        </div>
      </div>
      <div class="flex-1">
        <div class="flex items-center gap-1.5 text-sm font-medium text-[var(--text)]">
          <span>{{ node.deleted ? (auth.isAdmin ? node.username : '匿名用户') : node.username }}</span>
          <TitleBadge v-if="!node.deleted" :title="node.title" />
        </div>
        <!-- 已删除：管理员见原文（标注已删除），普通用户仅见占位文案 -->
        <div v-if="!node.deleted" class="mt-0.5 text-sm leading-relaxed text-[var(--text-soft)]">{{ node.content }}</div>
        <div v-else-if="auth.isAdmin" class="mt-0.5 text-sm leading-relaxed text-[var(--text-soft)]">
          【已删除】{{ node.content }}
        </div>
        <div v-else class="mt-0.5 text-sm italic text-[var(--text-soft)]">内容已删除</div>

        <div class="mt-1 flex gap-3 text-xs text-[var(--text-soft)]">
          <span>{{ node.created_at }}</span>
          <!-- 点赞：任意评论/回复均可点赞；未登录点击引导登录；请求中禁用防抖 -->
          <button
            v-if="!node.deleted"
            class="inline-flex items-center gap-0.5 hover:text-[var(--grape)]"
            :class="[node.my_liked ? 'text-[var(--grape)]' : '', liking ? 'pointer-events-none opacity-60' : '']"
            @click="onLike"
          >
            <ThumbsUp :size="13" :fill="node.my_liked ? 'currentColor' : 'none'" />
            {{ node.like_count || 0 }}
          </button>
          <button v-if="!node.deleted && auth.isLoggedIn" class="inline-flex items-center gap-0.5 hover:text-[var(--grape)]" @click="emit('reply', node)">
            <Reply :size="13" /> 回复
          </button>
          <button
            v-if="!node.deleted && auth.isLoggedIn && auth.user?.id !== node.user_id"
            class="inline-flex items-center gap-0.5 hover:text-[var(--danger)]"
            @click="emit('report', node)"
          >
            <Flag :size="13" /> 举报
          </button>
          <button
            v-if="!node.deleted && auth.isLoggedIn && auth.user?.id === node.user_id"
            class="inline-flex items-center gap-0.5 hover:text-[var(--danger)]"
            @click="emit('delete', node)"
          >
            <Trash2 :size="13" /> 删除
          </button>
          <!-- 管理员操作 -->
          <button
            v-if="auth.isAdmin && !node.deleted"
            class="inline-flex items-center gap-0.5 hover:text-[var(--danger)]"
            @click="emit('moderate-remove', node)"
          >
            <Ban :size="13" /> 撤下
          </button>
          <button
            v-if="auth.isAdmin && node.deleted"
            class="inline-flex items-center gap-0.5 hover:text-[var(--jade)]"
            @click="emit('moderate-restore', node)"
          >
            <RotateCcw :size="13" /> 恢复
          </button>
          <button
            v-if="auth.isAdmin && node.user_id"
            class="inline-flex items-center gap-0.5 hover:text-[var(--danger)]"
            @click="emit('penalty', node)"
          >
            <Gavel :size="13" /> 处罚
          </button>
        </div>

        <div v-if="node.children?.length" class="mt-3 space-y-3 border-l border-[var(--border)] pl-4">
          <CommentTree
            v-for="c in node.children"
            :key="c.id"
            :node="c"
            :submission-id="submissionId"
            @reply="(n) => emit('reply', n)"
            @delete="(n) => emit('delete', n)"
            @report="(n) => emit('report', n)"
            @moderate-remove="(n) => emit('moderate-remove', n)"
            @moderate-restore="(n) => emit('moderate-restore', n)"
            @penalty="(n) => emit('penalty', n)"
          />
        </div>
      </div>
    </div>
  </div>
</template>
