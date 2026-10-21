<script setup lang="ts">
import { ref } from 'vue'
import { Plus } from 'lucide-vue-next'
import { ElMessage } from 'element-plus'

const props = defineProps<{ modelValue: number[]; available: any[]; max?: number }>()
const emit = defineEmits(['update:modelValue', 'addCustom'])
const custom = ref('')

function toggle(id: number) {
  const set = new Set(props.modelValue)
  if (set.has(id)) {
    set.delete(id)
  } else {
    if (props.max && set.size >= props.max) {
      ElMessage.warning(`最多选择 ${props.max} 个标签`)
      return
    }
    set.add(id)
  }
  emit('update:modelValue', [...set])
}
function addCustom() {
  const name = custom.value.trim()
  if (!name) return
  emit('addCustom', name)
  custom.value = ''
}
</script>

<template>
  <div>
    <div class="flex flex-wrap gap-2">
      <button
        v-for="t in available"
        :key="t.id"
        type="button"
        @click="toggle(t.id)"
        :class="modelValue.includes(t.id) ? 'bg-grape-600 text-white' : 'bg-[var(--surface-2)] text-[var(--text-soft)]'"
        class="rounded-full px-3 py-1 text-sm transition"
      >
        #{{ t.name }}
      </button>
    </div>
    <div class="mt-2 flex items-center gap-2">
      <input
        v-model="custom"
        @keyup.enter="addCustom"
        placeholder="自定义新标签（需审核）"
        class="h-8 flex-1 rounded-full border border-[var(--border)] bg-[var(--surface-2)] px-3 text-sm outline-none focus:border-[var(--grape)]"
      />
      <button type="button" class="btn-grape rounded-full px-3 py-1.5 text-sm" @click="addCustom"><Plus :size="14" /> 添加</button>
    </div>
  </div>
</template>
