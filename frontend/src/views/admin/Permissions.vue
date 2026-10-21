<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { Save, ShieldCheck } from 'lucide-vue-next'
import { ElMessage } from 'element-plus'
import { permissionApi } from '@/api'

const role = ref('user')
const userId = ref<number | null>(null)
const items = ref<any[]>([])
const dirty = ref(false)
const keyword = ref('')

async function load() {
  const d = await permissionApi.matrix({ role: role.value, user_id: userId.value || undefined })
  items.value = d.items || []
  dirty.value = false
}
async function save() {
  await permissionApi.save({
    role: role.value,
    user_id: userId.value || undefined,
    items: items.value.map((i) => ({
      endpoint_key: i.endpoint_key,
      allow: i.allow,
      rps: i.rps || 0,
      rpm: i.rpm || 0,
      rphour: i.rphour || 0,
      rpday: i.rpday || 0,
    })),
  })
  ElMessage.success('权限矩阵已保存')
  dirty.value = false
}

const filtered = computed(() =>
  keyword.value ? items.value.filter((i) => i.endpoint_key.toLowerCase().includes(keyword.value.toLowerCase())) : items.value,
)

watch([role, userId], load)
onMounted(load)
</script>

<template>
  <div>
    <div class="mb-5 flex flex-wrap items-end justify-between gap-3">
      <div>
        <h1 class="font-serif-sc text-2xl font-bold text-[var(--grape)]">访问权限矩阵</h1>
        <p class="mt-1 text-sm text-[var(--text-soft)]">优先级：用户单独配置 &gt; 角色默认权限集 &gt; 默认拒绝；超级管理员不受限制。</p>
      </div>
      <button class="btn-grape flex items-center gap-1 rounded-full px-5 py-2 text-sm" @click="save">
        <Save :size="15" /> {{ dirty ? '保存修改' : '保存' }}
      </button>
    </div>

    <div class="paper-card p-5">
      <div class="mb-4 flex flex-wrap items-center gap-3">
        <select v-model="role" class="rounded-lg border border-[var(--border)] bg-[var(--surface-2)] px-3 py-1.5 text-sm">
          <option value="user">普通用户默认权限集</option>
          <option value="admin">普通管理员默认权限集</option>
          <option value="public">未登录访客权限集</option>
        </select>
        <input v-model.number="userId" type="number" placeholder="用户 ID（单用户覆盖，可空）" class="h-9 w-56 rounded-lg border border-[var(--border)] bg-[var(--surface-2)] px-3 text-sm outline-none focus:border-[var(--grape)]" />
        <input v-model="keyword" placeholder="搜索端点…" class="h-9 flex-1 rounded-lg border border-[var(--border)] bg-[var(--surface-2)] px-3 text-sm outline-none focus:border-[var(--grape)]" />
      </div>

      <div class="overflow-x-auto">
        <table class="w-full text-sm">
          <thead>
            <tr class="border-b border-[var(--border)] text-left text-[var(--text-soft)]">
              <th class="py-2">端点</th>
              <th class="py-2 w-20">允许</th>
              <th class="py-2 w-24">rps</th>
              <th class="py-2 w-24">rpm</th>
              <th class="py-2 w-24">rphour</th>
              <th class="py-2 w-24">rpday</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="i in filtered" :key="i.endpoint_key" class="border-b border-[var(--border)]">
              <td class="py-2 font-mono text-xs">{{ i.endpoint_key }}</td>
              <td class="py-2">
                <input type="checkbox" v-model="i.allow" @change="dirty = true" />
              </td>
              <td class="py-2"><input type="number" v-model.number="i.rps" @change="dirty = true" class="num-input" /></td>
              <td class="py-2"><input type="number" v-model.number="i.rpm" @change="dirty = true" class="num-input" /></td>
              <td class="py-2"><input type="number" v-model.number="i.rphour" @change="dirty = true" class="num-input" /></td>
              <td class="py-2"><input type="number" v-model.number="i.rpday" @change="dirty = true" class="num-input" /></td>
            </tr>
          </tbody>
        </table>
      </div>

      <p class="mt-3 flex items-center gap-1 text-xs text-[var(--text-soft)]">
        <ShieldCheck :size="13" /> 速率 0 表示不限；四档可任选组合，超限返回 429。
      </p>
    </div>
  </div>
</template>

<style scoped>
.num-input {
  width: 72px;
  height: 32px;
  border-radius: 8px;
  border: 1px solid var(--border);
  background: var(--surface-2);
  padding: 0 8px;
  outline: none;
  font-size: 13px;
}
.num-input:focus {
  border-color: var(--grape);
}
</style>
