<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ElMessage, ElMessageBox } from 'element-plus'
import { submissionApi, pollApi } from '@/api'
import { useMetaStore } from '@/stores/meta'
import TagPicker from '@/components/TagPicker.vue'
import ImageUploader from '@/components/ImageUploader.vue'
import VideoUploader from '@/components/VideoUploader.vue'
import MarkdownEditor from '@/components/MarkdownEditor.vue'
import PollPicker from '@/components/PollPicker.vue'

const router = useRouter()
const route = useRoute()
const meta = useMetaStore()
const DRAFT_KEY = 'tzwall_submission_draft'

const title = ref('')
const content = ref('')
const selectedTagIds = ref<number[]>([])
const customTagNames = ref<string[]>([])
const images = ref<any[]>([])
const videos = ref<any[]>([])
const anonymous = ref(false)
const contactVisible = ref('login')
const contact = ref('')
const availableTags = ref<any[]>([])
const loading = ref(false)

// 关联投票（在投稿流程中统一选择）
const myPolls = ref<any[]>([])
const pollSel = ref<number[]>([])
const pollMax = ref(3)
async function loadMyPolls() {
  try {
    const r: any = await pollApi.mine()
    myPolls.value = r.items || []
  } catch {
    myPolls.value = []
  }
}

// 编辑模式：/publish?edit=<id>
const editingId = ref<number | null>(null)

// 本地草稿
const draftSavedAt = ref(0)
const showDraftBanner = ref(false)

function draftSnapshot() {
  return {
    title: title.value,
    content: content.value,
    selectedTagIds: selectedTagIds.value,
    customTagNames: customTagNames.value,
    images: images.value,
    videos: videos.value,
    anonymous: anonymous.value,
    contactVisible: contactVisible.value,
    contact: contact.value,
    pollSel: pollSel.value,
    savedAt: Date.now(),
  }
}
function saveDraft() {
  if (editingId.value) return
  if (!title.value.trim() && !content.value.trim() && images.value.length === 0 && videos.value.length === 0) {
    ElMessage.warning('暂无内容可保存为草稿')
    return
  }
  localStorage.setItem(DRAFT_KEY, JSON.stringify(draftSnapshot()))
  ElMessage.success('草稿已保存到本地浏览器')
}
function readDraft(): any | null {
  try {
    const raw = localStorage.getItem(DRAFT_KEY)
    return raw ? JSON.parse(raw) : null
  } catch {
    return null
  }
}
function applyDraft(d: any) {
  title.value = d.title || ''
  content.value = d.content || ''
  selectedTagIds.value = d.selectedTagIds || []
  customTagNames.value = d.customTagNames || []
  images.value = d.images || []
  videos.value = d.videos || []
  anonymous.value = !!d.anonymous
  contactVisible.value = d.contactVisible || 'login'
  contact.value = d.contact || ''
  pollSel.value = d.pollSel || []
}
function checkDraft() {
  if (editingId.value) return
  const d = readDraft()
  if (d && (d.title || d.content || (d.images || []).length || (d.videos || []).length)) {
    draftSavedAt.value = d.savedAt || 0
    showDraftBanner.value = true
  }
}
function restoreDraft() {
  const d = readDraft()
  if (d) applyDraft(d)
  showDraftBanner.value = false
  ElMessage.success('已恢复草稿')
}
async function discardDraft() {
  try {
    await ElMessageBox.confirm('确定丢弃本地草稿？此操作不可恢复。', '丢弃草稿', { type: 'warning' })
  } catch {
    return
  }
  localStorage.removeItem(DRAFT_KEY)
  showDraftBanner.value = false
  ElMessage.success('已丢弃草稿')
}
function clearDraft() {
  localStorage.removeItem(DRAFT_KEY)
  showDraftBanner.value = false
}
function fmtTime(ts: number) {
  if (!ts) return ''
  const d = new Date(ts)
  const p = (n: number) => String(n).padStart(2, '0')
  return `${d.getFullYear()}-${p(d.getMonth() + 1)}-${p(d.getDate())} ${p(d.getHours())}:${p(d.getMinutes())}`
}

async function loadForEdit(id: number) {
  loading.value = true
  try {
    const d = await submissionApi.get(id)
    editingId.value = id
    title.value = d.title || ''
    content.value = d.content || ''
    selectedTagIds.value = (d.tags || []).map((t: any) => t.id)
    customTagNames.value = []
    images.value = d.images || []
    videos.value = d.videos || []
    anonymous.value = !!d.anonymous
    contact.value = d.contact || ''
    contactVisible.value = d.contact_visible || 'login'
    try {
      const lp: any = await pollApi.submissionPolls(id)
      pollSel.value = (lp.items || []).map((x: any) => x.id)
    } catch {
      pollSel.value = []
    }
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '加载稿件失败')
    router.replace('/publish')
  } finally {
    loading.value = false
  }
}

onMounted(async () => {
  pollMax.value = meta.site?.limits?.poll_per_submission ?? 3
  await loadMyPolls()
  const p = await submissionApi.tags()
  availableTags.value = p.items || []
  const editId = Number(route.query.edit)
  if (editId) {
    await loadForEdit(editId)
  } else {
    checkDraft()
  }
})

function onAddCustom(name: string) {
  if (!customTagNames.value.includes(name)) customTagNames.value.push(name)
}

async function submit() {
  if (!title.value.trim()) {
    ElMessage.warning('请填写标题')
    return
  }
  if (!content.value.trim() && images.value.length === 0 && videos.value.length === 0) {
    ElMessage.warning('请填写内容或上传图片/视频')
    return
  }
  loading.value = true
  try {
    const payload = {
      title: title.value,
      content: content.value,
      tag_ids: selectedTagIds.value,
      tag_names: customTagNames.value,
      image_ids: images.value.map((i) => i.id),
      video_ids: videos.value.map((v) => v.id),
      anonymous: anonymous.value,
      contact: contact.value,
      contact_visible: contactVisible.value,
    }
    if (editingId.value) {
      await submissionApi.update(editingId.value, payload)
      try {
        await pollApi.setSubmissionPolls(editingId.value, pollSel.value)
      } catch {
        /* 关联失败已由拦截器提示（如越权） */
      }
      ElMessage.success('已保存修改，将重新进入审核')
    } else {
      const created: any = await submissionApi.create(payload)
      const sid = created?.id
      if (sid && pollSel.value.length) {
        try {
          await pollApi.setSubmissionPolls(sid, pollSel.value)
        } catch {
          /* 关联失败已由拦截器提示 */
        }
      }
      clearDraft()
      ElMessage.success('投稿已提交')
      router.push(sid ? '/submission/' + sid : '/my')
      return
    }
    router.push('/my')
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || e?.message || '提交失败，请稍后再试')
  } finally {
    loading.value = false
  }
}
</script>

<template>
  <div class="mx-auto max-w-2xl px-4 py-6">
    <h1 class="mb-5 font-serif-sc text-2xl font-bold text-[var(--grape)]">{{ editingId ? '编辑投稿' : '发布投稿' }}</h1>

    <div v-if="editingId" class="mb-4 rounded-lg border border-[var(--grape)]/30 bg-[var(--grape)]/10 p-3 text-sm text-[var(--text-soft)]">
      正在编辑稿件（#{{ editingId }}）。保存后将重新进入审核，通过前不在公开列表展示。
    </div>
    <div v-else-if="showDraftBanner" class="mb-4 flex flex-wrap items-center justify-between gap-2 rounded-lg border border-warn/30 bg-warn/10 p-3 text-sm">
      <span class="text-[var(--text-soft)]">检测到未提交的本地草稿{{ draftSavedAt ? '（保存于 ' + fmtTime(draftSavedAt) + '）' : '' }}</span>
      <span class="flex gap-2">
        <button class="rounded-full bg-grape-500 px-3 py-1 text-xs text-white" @click="restoreDraft">恢复草稿</button>
        <button class="rounded-full border border-[var(--border)] px-3 py-1 text-xs" @click="discardDraft">丢弃</button>
      </span>
    </div>

    <div class="paper-card space-y-5 p-6">
      <div>
        <label class="mb-1 block text-sm font-medium">标题 <span class="text-[var(--danger)]">*</span></label>
        <input v-model="title" placeholder="一句话标题" class="input-style" />
      </div>

      <div>
        <label class="mb-1 block text-sm font-medium">标签</label>
        <TagPicker v-model="selectedTagIds" :available="availableTags" :max="5" @add-custom="onAddCustom" />
        <div v-if="customTagNames.length" class="mt-2 flex flex-wrap gap-1.5">
          <span v-for="n in customTagNames" :key="n" class="rounded-full bg-warn/15 px-2 py-0.5 text-xs text-warn">待审：#{{ n }}</span>
        </div>
      </div>

      <div>
        <label class="mb-1 flex items-center justify-between text-sm font-medium">
          <span>内容</span>
          <span class="text-xs font-normal text-[var(--text-soft)]">支持 Markdown 与 LaTeX 公式</span>
        </label>
        <MarkdownEditor v-model="content" placeholder="分享你的故事…支持 Markdown 标题/列表/代码块与 $行内公式$、$$公式块$$" />
      </div>

      <div>
        <label class="mb-1 block text-sm font-medium">图片（可选）</label>
        <ImageUploader v-model="images" :max="9" />
      </div>

      <div>
        <label class="mb-1 block text-sm font-medium">视频（可选）</label>
        <VideoUploader v-model="videos" :max="3" />
      </div>

      <div>
        <label class="mb-1 block text-sm font-medium">联系方式（可选）</label>
        <input v-model="contact" placeholder="微信 / QQ / 手机等" class="input-style" />
        <div class="mt-2 flex gap-4 text-sm">
          <label class="flex items-center gap-1.5"><input type="radio" value="all" v-model="contactVisible" /> 所有人可见</label>
          <label class="flex items-center gap-1.5"><input type="radio" value="login" v-model="contactVisible" /> 仅登录用户可见</label>
        </div>
      </div>

      <label class="flex items-center gap-2 text-sm">
        <input type="checkbox" v-model="anonymous" /> 匿名投稿（公开页显示为“匿名用户”）
      </label>

      <div>
        <label class="mb-1 flex items-center justify-between text-sm font-medium">
          <span>关联投票（可选）</span>
          <span class="text-xs font-normal text-[var(--text-soft)]">最多 {{ pollMax }} 个</span>
        </label>
        <PollPicker v-model="pollSel" :polls="myPolls" :max="pollMax" :current-submission-id="editingId || undefined" />
      </div>

      <div class="flex items-center gap-3">
        <button
          v-if="!editingId"
          class="rounded-full border border-[var(--border)] px-5 py-2.5 text-sm font-medium hover:text-[var(--grape)] disabled:opacity-60"
          :disabled="loading"
          @click="saveDraft"
        >
          保存草稿
        </button>
        <button class="btn-grape flex-1 rounded-full py-2.5 font-medium" :disabled="loading" @click="submit">
          {{ loading ? '提交中…' : (editingId ? '保存修改' : '提交投稿') }}
        </button>
      </div>
    </div>
  </div>
</template>

<style scoped>
.input-style {
  width: 100%;
  border-radius: 12px;
  border: 1px solid var(--border);
  background: var(--surface-2);
  padding: 10px 12px;
  outline: none;
  font-size: 14px;
}
.input-style:focus {
  border-color: var(--grape);
}
</style>
