<script setup lang="ts">
import { Reply, Trash2, Flag, Ban, Gavel, RotateCcw } from 'lucide-vue-next'
import { useAuthStore } from '@/stores/auth'

const props = defineProps<{ node: any; submissionId: number }>()
const emit = defineEmits(['reply', 'delete', 'report', 'moderate-remove', 'moderate-restore', 'penalty'])
const auth = useAuthStore()
</script>

<template>
  <div class="comment">
    <div class="flex gap-2">
      <div class="flex h-8 w-8 shrink-0 items-center justify-center rounded-full bg-grape-100 text-sm font-semibold text-[var(--grape)]">
        {{ node.deleted ? '—' : (node.username || '匿')?.[0] || '匿' }}
      </div>
      <div class="flex-1">
        <div class="text-sm font-medium text-[var(--text)]">
          {{ node.deleted ? (auth.isAdmin ? node.username : '匿名用户') : node.username }}
        </div>
        <!-- 已删除：管理员见原文（标注已删除），普通用户仅见占位文案 -->
        <div v-if="!node.deleted" class="mt-0.5 text-sm leading-relaxed text-[var(--text-soft)]">{{ node.content }}</div>
        <div v-else-if="auth.isAdmin" class="mt-0.5 text-sm leading-relaxed text-[var(--text-soft)]">
          【已删除】{{ node.content }}
        </div>
        <div v-else class="mt-0.5 text-sm italic text-[var(--text-soft)]">内容已删除</div>

        <div class="mt-1 flex gap-3 text-xs text-[var(--text-soft)]">
          <span>{{ node.created_at }}</span>
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
