<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { useRoute } from 'vue-router'
import { Heart, ThumbsDown, Share2, MessageCircle, Send, Flag, Ban, Gavel } from 'lucide-vue-next'
import { ElMessage, ElMessageBox } from 'element-plus'
import { submissionApi, commentApi, interactionApi, punishmentApi, adminApi } from '@/api'
import { useAuthStore } from '@/stores/auth'
import CommentTree from '@/components/CommentTree.vue'
import ReportDialog from '@/components/ReportDialog.vue'
import PenaltyDialog from '@/components/PenaltyDialog.vue'

const route = useRoute()
const auth = useAuthStore()
const id = Number(route.params.id)
const detail = ref<any>(null)
const comments = ref<any[]>([])
const newText = ref('')
const replyTo = ref<any>(null)
const myAttitude = ref(0)
const previewImg = ref('')
const muted = ref(false)
const showReport = ref(false)
const reportTarget = ref<{ type: string; id: number }>({ type: 'submission', id: 0 })

function openReport(type: string, targetId: number) {
  reportTarget.value = { type, id: targetId }
  showReport.value = true
}

async function load() {
  detail.value = await submissionApi.get(id)
}
async function loadComments() {
  const p = await commentApi.list(id)
  comments.value = p.items || []
}
async function loadStatus() {
  if (!auth.isLoggedIn) return
  try {
    const s = await punishmentApi.status()
    muted.value = !!s?.muted
  } catch {
    muted.value = false
  }
}

async function react(att: number) {
  if (!auth.isLoggedIn) {
    ElMessage.warning('请先登录')
    return
  }
  if (muted.value) {
    ElMessage.warning('您已被禁言，无法表态')
    return
  }
  const next = myAttitude.value === att ? 0 : att
  await interactionApi.react({ target_type: 'submission', target_id: id, attitude: next })
  myAttitude.value = next
  if (next === 1) detail.value.like_count++
  else if (next === -1) detail.value.dislike_count++
  await load()
}
async function submitComment() {
  if (!auth.isLoggedIn) {
    ElMessage.warning('请先登录')
    return
  }
  if (!newText.value.trim()) return
  await commentApi.create(id, { content: newText.value, parent_id: replyTo.value?.id || 0 })
  newText.value = ''
  replyTo.value = null
  await loadComments()
  await load()
}
function onReply(node: any) {
  replyTo.value = node
  newText.value = ''
}
async function onDelete(node: any) {
  await commentApi.remove(node.id)
  await loadComments()
}

// 管理员快捷操作
const showPenalty = ref(false)
const penaltyUser = ref<number | null>(null)
const penaltyName = ref('')
function openPenalty(userId: number | null, name: string) {
  if (!userId) return
  penaltyUser.value = userId
  penaltyName.value = name
  showPenalty.value = true
}
async function onModerateRemove(node: any) {
  await adminApi.removeComment(node.id)
  ElMessage.success('评论已撤下')
  await loadComments()
}
async function onModerateRestore(node: any) {
  await adminApi.restoreComment(node.id)
  ElMessage.success('评论已恢复')
  await loadComments()
}
async function takeDownSubmission() {
  await ElMessageBox.confirm('确定快捷撤下该投稿？撤下后不再公开展示。', '快捷撤下', { type: 'warning' })
  await adminApi.reviewSubmission(id, { status: 'rejected', note: '管理员快捷撤下' })
  ElMessage.success('稿件已撤下')
  await load()
}
function share() {
  navigator.clipboard?.writeText(location.href)
  ElMessage.success('链接已复制，去分享吧')
}

onMounted(() => {
  load()
  loadComments()
  loadStatus()
})
</script>

<template>
  <div class="mx-auto max-w-[860px] px-4 py-6" v-if="detail">
    <button class="mb-3 text-sm text-[var(--text-soft)]" @click="$router.back()">← 返回</button>

    <article class="paper-card p-6 animate-fadeup">
      <div class="flex items-center gap-2 text-xs text-[var(--text-soft)]">
        <span class="rounded-full bg-[var(--surface-2)] px-2 py-0.5 font-medium text-[var(--grape)]">{{ detail.anonymous ? '匿名用户' : detail.author }}</span>
        <span>{{ detail.created_at }}</span>
        <span v-if="detail.status !== 'approved'" class="rounded bg-warn/20 px-2 py-0.5 text-warn">状态：{{ detail.status }}</span>
      </div>
      <h1 class="mt-2 font-serif-sc text-2xl font-bold">{{ detail.title }}</h1>

      <p v-if="detail.content" class="mt-4 whitespace-pre-wrap leading-relaxed">{{ detail.content }}</p>

      <div v-if="detail.images?.length" class="mt-4 grid grid-cols-2 gap-3 sm:grid-cols-3">
        <img v-for="im in detail.images" :key="im.id" :src="im.url" class="h-40 w-full cursor-zoom-in rounded-lg object-cover" @click="previewImg = im.url" alt="" @error="$event.target.style.display = 'none'" />
      </div>

      <div v-if="detail.videos?.length" class="mt-4 grid grid-cols-1 gap-3 sm:grid-cols-2">
        <video v-for="v in detail.videos" :key="v.id" :src="v.url" controls preload="metadata" class="w-full rounded-lg bg-black" @error="$event.target.style.display = 'none'"></video>
      </div>

      <div v-if="detail.tags?.length" class="mt-4 flex flex-wrap gap-1.5">
        <span v-for="t in detail.tags" :key="t.id" class="rounded-full bg-grape-50 px-2 py-0.5 text-xs text-[var(--grape)]">#{{ t.name }}</span>
      </div>

      <div v-if="detail.contact" class="mt-4 rounded-lg border border-[var(--border)] bg-[var(--surface-2)] p-3 text-sm">
        联系方式：{{ detail.contact }}
      </div>

      <div class="mt-5 flex items-center gap-5 border-t border-[var(--border)] pt-4 text-sm">
        <button class="flex items-center gap-1.5" :class="myAttitude === 1 ? 'text-[var(--grape)]' : 'text-[var(--text-soft)]'" @click="react(1)"><Heart :size="18" /> {{ detail.like_count }}</button>
        <button class="flex items-center gap-1.5" :class="myAttitude === -1 ? 'text-[var(--danger)]' : 'text-[var(--text-soft)]'" @click="react(-1)"><ThumbsDown :size="18" /> {{ detail.dislike_count }}</button>
        <button
          class="flex items-center gap-1.5"
          :class="muted ? 'cursor-not-allowed text-[var(--text-soft)]/50' : 'text-[var(--text-soft)]'"
          :disabled="muted"
          @click="share"
        >
          <Share2 :size="18" /> 分享
        </button>
        <button
          v-if="auth.isLoggedIn"
          class="flex items-center gap-1.5"
          :class="muted ? 'cursor-not-allowed text-[var(--text-soft)]/50' : 'text-[var(--text-soft)] hover:text-[var(--danger)]'"
          :disabled="muted"
          @click="openReport('submission', id)"
        >
          <Flag :size="18" /> 举报
        </button>
        <span class="flex items-center gap-1.5 text-[var(--text-soft)]"><MessageCircle :size="18" /> {{ detail.comment_count }}</span>
      </div>

      <!-- 管理员快捷操作（针对本投稿） -->
      <div v-if="auth.isAdmin" class="mt-3 flex flex-wrap gap-2 border-t border-[var(--border)] pt-3">
        <button class="inline-flex items-center gap-1 rounded-full bg-[var(--danger)] px-3 py-1 text-sm text-white" @click="takeDownSubmission"><Ban :size="14" /> 快捷撤下稿件</button>
        <button v-if="detail.user_id" class="inline-flex items-center gap-1 rounded-full border border-[var(--border)] px-3 py-1 text-sm hover:text-[var(--danger)]" @click="openPenalty(detail.user_id, detail.anonymous ? '匿名用户' : detail.author)"><Gavel :size="14" /> 处罚作者</button>
      </div>
    </article>

    <!-- 评论 -->
    <section class="mt-6">
      <h2 class="mb-3 font-serif-sc text-lg font-bold">评论 {{ detail.comment_count }}</h2>
      <div v-if="auth.isLoggedIn" class="paper-card mb-4 p-4">
        <div v-if="replyTo" class="mb-2 text-xs text-[var(--text-soft)]">回复 @{{ replyTo.username }} <button class="ml-2 text-[var(--danger)]" @click="replyTo = null">取消</button></div>
        <textarea v-model="newText" rows="3" placeholder="友善发言，理性交流…" class="w-full rounded-lg border border-[var(--border)] bg-[var(--surface-2)] p-3 text-sm outline-none focus:border-[var(--grape)]"></textarea>
        <div class="mt-2 flex justify-end">
          <button class="btn-grape rounded-full px-4 py-2 text-sm" @click="submitComment"><Send :size="14" /> 发表</button>
        </div>
      </div>
      <div v-else class="paper-card mb-4 p-4 text-center text-sm text-[var(--text-soft)]">
        请<button class="mx-1 text-[var(--grape)]" @click="$router.push('/login')">登录</button>后参与评论
      </div>

      <div v-if="comments.length" class="space-y-4">
        <CommentTree
          v-for="c in comments"
          :key="c.id"
          :node="c"
          :submission-id="id"
          @reply="onReply"
          @delete="onDelete"
          @report="(n: any) => openReport('comment', n.id)"
          @moderate-remove="onModerateRemove"
          @moderate-restore="onModerateRestore"
          @penalty="(n: any) => openPenalty(n.user_id, n.username || '用户')"
        />
      </div>
      <p v-else class="text-center text-sm text-[var(--text-soft)]">还没有评论，抢沙发～</p>
    </section>

    <div v-if="previewImg" class="fixed inset-0 z-[9999] flex items-center justify-center bg-black/80" @click="previewImg = ''">
      <img :src="previewImg" class="max-h-[90vh] max-w-[90vw] rounded-lg" alt="" />
    </div>

    <ReportDialog v-model="showReport" :target-type="reportTarget.type" :target-id="reportTarget.id" @done="loadStatus" />
    <PenaltyDialog v-model="showPenalty" :user-id="penaltyUser" :username="penaltyName" />
  </div>
</template>
