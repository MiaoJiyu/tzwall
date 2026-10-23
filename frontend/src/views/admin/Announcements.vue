<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { Megaphone, Pin, CheckCircle2, Pencil, Trash2, Plus } from 'lucide-vue-next'
import { announcementApi } from '@/api'

const items = ref<any[]>([])
const loading = ref(false)
const dialog = ref(false)
const editing = ref<any | null>(null)
const form = ref({ title: '', content: '', require_confirm: false, pinned: false, active: true })
const saving = ref(false)

async function load() {
  loading.value = true
  try {
    const r = await announcementApi.adminList()
    items.value = r.items || []
  } finally {
    loading.value = false
  }
}
function openCreate() {
  editing.value = null
  form.value = { title: '', content: '', require_confirm: false, pinned: false, active: true }
  dialog.value = true
}
function openEdit(a: any) {
  editing.value = a
  form.value = {
    title: a.title,
    content: a.content,
    require_confirm: a.require_confirm,
    pinned: a.pinned,
    active: a.active,
  }
  dialog.value = true
}
async function save() {
  if (!form.value.title.trim() || !form.value.content.trim()) {
    ElMessage.warning('标题与内容均为必填')
    return
  }
  saving.value = true
  try {
    if (editing.value) {
      await announcementApi.update(editing.value.id, { ...form.value })
      ElMessage.success('公告已更新')
    } else {
      await announcementApi.create({ ...form.value })
      ElMessage.success('公告已发布')
    }
    dialog.value = false
    await load()
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '保存失败')
  } finally {
    saving.value = false
  }
}
async function remove(a: any) {
  try {
    await ElMessageBox.confirm('确认删除公告「' + a.title + '」？此操作不可恢复。', '删除确认', { type: 'warning' })
  } catch {
    return
  }
  try {
    await announcementApi.remove(a.id)
    ElMessage.success('已删除')
    await load()
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '删除失败')
  }
}
onMounted(load)
</script>

<template>
  <div class="mx-auto max-w-4xl space-y-5 p-4 sm:p-6">
    <div class="flex items-center justify-between">
      <h1 class="flex items-center gap-2 text-lg font-semibold"><Megaphone :size="20" /> 公告管理</h1>
      <button class="btn-grape flex items-center gap-1 rounded-full px-4 py-2 text-sm" @click="openCreate">
        <Plus :size="16" /> 发布公告
      </button>
    </div>

    <div class="paper-card divide-y divide-[var(--border)]">
      <div v-if="loading" class="p-6 text-center text-sm text-[var(--text-soft)]">加载中…</div>
      <div v-else-if="!items.length" class="p-6 text-center text-sm text-[var(--text-soft)]">暂无公告</div>
      <div v-for="a in items" :key="a.id" class="flex items-start gap-3 p-4">
        <div class="min-w-0 flex-1">
          <div class="flex flex-wrap items-center gap-1.5">
            <span v-if="a.pinned" class="flex items-center gap-0.5 rounded bg-[var(--grape)]/10 px-1.5 py-0.5 text-[11px] font-medium text-[var(--grape)]"><Pin :size="11" /> 置顶</span>
            <span class="font-medium">{{ a.title }}</span>
            <span v-if="a.require_confirm" class="flex items-center gap-0.5 rounded bg-warn/15 px-1.5 py-0.5 text-[11px] text-warn"><CheckCircle2 :size="11" /> 需确认</span>
            <span v-if="!a.active" class="rounded bg-[var(--surface-2)] px-1.5 py-0.5 text-[11px] text-[var(--text-soft)]">未发布</span>
          </div>
          <p class="mt-1 line-clamp-2 whitespace-pre-wrap text-sm text-[var(--text-soft)]">{{ a.content }}</p>
        </div>
        <div class="flex shrink-0 items-center gap-1">
          <button class="rounded-md p-1.5 hover:bg-[var(--surface-2)]" title="编辑" @click="openEdit(a)"><Pencil :size="15" /></button>
          <button class="rounded-md p-1.5 text-[var(--danger)] hover:bg-[var(--danger)]/10" title="删除" @click="remove(a)"><Trash2 :size="15" /></button>
        </div>
      </div>
    </div>

    <el-dialog v-model="dialog" :title="editing ? '修改公告' : '发布公告'" width="520px">
      <div class="space-y-3">
        <div>
          <label class="mb-1 block text-xs text-[var(--text-soft)]">标题</label>
          <input v-model="form.title" maxlength="200" placeholder="公告标题" class="input-style" />
        </div>
        <div>
          <label class="mb-1 block text-xs text-[var(--text-soft)]">内容</label>
          <textarea v-model="form.content" rows="5" placeholder="公告内容（纯文本，支持多行）" class="input-style h-auto py-2"></textarea>
        </div>
        <div class="flex flex-wrap gap-4 text-sm">
          <label class="flex items-center gap-1.5"><el-switch v-model="form.active" /> 发布（可见）</label>
          <label class="flex items-center gap-1.5"><el-switch v-model="form.pinned" /> 置顶</label>
          <label class="flex items-center gap-1.5"><el-switch v-model="form.require_confirm" /> 要求全员确认</label>
        </div>
      </div>
      <template #footer>
        <div class="flex justify-end gap-2">
          <button class="rounded-full px-4 py-1.5 text-sm" @click="dialog = false">取消</button>
          <button class="btn-grape rounded-full px-4 py-1.5 text-sm text-white" :disabled="saving" @click="save">{{ saving ? '保存中…' : '保存' }}</button>
        </div>
      </template>
    </el-dialog>
  </div>
</template>
