<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import { submissionApi } from '@/api'
import TagPicker from '@/components/TagPicker.vue'
import ImageUploader from '@/components/ImageUploader.vue'

const router = useRouter()
const title = ref('')
const content = ref('')
const selectedTagIds = ref<number[]>([])
const customTagNames = ref<string[]>([])
const images = ref<any[]>([])
const anonymous = ref(false)
const contactVisible = ref('login')
const contact = ref('')
const availableTags = ref<any[]>([])
const loading = ref(false)

onMounted(async () => {
  const p = await submissionApi.tags()
  availableTags.value = p.items || []
})

function onAddCustom(name: string) {
  if (!customTagNames.value.includes(name)) customTagNames.value.push(name)
}

async function submit() {
  if (!title.value.trim()) {
    ElMessage.warning('请填写标题')
    return
  }
  if (!content.value.trim() && images.value.length === 0) {
    ElMessage.warning('请填写内容或上传图片')
    return
  }
  loading.value = true
  try {
    await submissionApi.create({
      title: title.value,
      content: content.value,
      tag_ids: selectedTagIds.value,
      tag_names: customTagNames.value,
      image_ids: images.value.map((i) => i.id),
      anonymous: anonymous.value,
      contact: contact.value,
      contact_visible: contactVisible.value,
    })
    ElMessage.success('投稿已提交，等待审核')
    router.push('/my')
  } finally {
    loading.value = false
  }
}
</script>

<template>
  <div class="mx-auto max-w-2xl px-4 py-6">
    <h1 class="mb-5 font-serif-sc text-2xl font-bold text-[var(--grape)]">发布投稿</h1>

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
        <label class="mb-1 block text-sm font-medium">内容</label>
        <textarea v-model="content" rows="6" placeholder="分享你的故事（纯文本）" class="input-style h-auto py-3"></textarea>
      </div>

      <div>
        <label class="mb-1 block text-sm font-medium">图片（可选）</label>
        <ImageUploader v-model="images" :max="9" />
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

      <button class="btn-grape w-full rounded-full py-2.5 font-medium" :disabled="loading" @click="submit">
        {{ loading ? '提交中…' : '提交投稿' }}
      </button>
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
