<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { Search, MailPlus, Check } from 'lucide-vue-next'
import { ElMessage } from 'element-plus'
import { adminApi } from '@/api'
import { useAuthStore } from '@/stores/auth'

const auth = useAuthStore()
const list = ref<any[]>([])
const total = ref(0)
const page = ref(1)
const keyword = ref('')

async function load() {
  const p = await adminApi.users({ page: page.value, page_size: 20, keyword: keyword.value })
  list.value = p.items || []
  total.value = p.total
}
async function setRole(u: any, role: string) {
  await adminApi.setRole(u.id, { role })
  ElMessage.success('已更新角色')
  load()
}
async function resendVerify(u: any) {
  try {
    await adminApi.resendUserVerify(u.id)
    ElMessage.success(`已向 ${u.email} 重发验证邮件`)
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || e?.message || '发送失败')
  }
}
async function activate(u: any) {
  try {
    await adminApi.activateUser(u.id)
    ElMessage.success('已直接激活该用户')
    load()
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || e?.message || '激活失败')
  }
}
function onSearch() {
  page.value = 1
  load()
}

onMounted(load)
</script>

<template>
  <div>
    <h1 class="mb-5 font-serif-sc text-2xl font-bold text-[var(--grape)]">用户管理</h1>
    <div class="paper-card p-5">
      <div class="relative mb-4 max-w-sm">
        <Search :size="16" class="absolute left-3 top-1/2 -translate-y-1/2 text-[var(--text-soft)]" />
        <input v-model="keyword" @keyup.enter="onSearch" placeholder="搜索用户名/邮箱/姓名/ID" class="input-style pl-9" />
      </div>

      <div class="overflow-x-auto">
        <table class="w-full text-sm">
          <thead>
            <tr class="border-b border-[var(--border)] text-left text-[var(--text-soft)]">
              <th class="py-2">用户名</th>
              <th class="py-2">邮箱</th>
              <th class="py-2">邮箱验证</th>
              <th class="py-2">角色</th>
              <th class="py-2">状态</th>
              <th class="py-2" v-if="auth.isAdmin">操作</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="u in list" :key="u.id" class="border-b border-[var(--border)]">
              <td class="py-2 font-medium">{{ u.username }}</td>
              <td class="py-2 text-[var(--text-soft)]">{{ u.email }}</td>
              <td class="py-2">
                <span v-if="u.email_verified" class="rounded-full bg-[var(--jade)]/15 px-2 py-0.5 text-xs text-[var(--jade)]">已验证</span>
                <span v-else class="rounded-full bg-warn/15 px-2 py-0.5 text-xs text-warn">未验证</span>
              </td>
              <td class="py-2">
                <span class="rounded-full bg-grape-50 px-2 py-0.5 text-xs text-[var(--grape)]">{{ u.role }}</span>
              </td>
              <td class="py-2 text-[var(--text-soft)]">{{ u.status }}</td>
              <td class="py-2" v-if="auth.isAdmin">
                <div class="flex items-center gap-2">
                  <button
                    v-if="!u.email_verified"
                    class="inline-flex items-center gap-1 rounded-full border border-[var(--border)] px-3 py-1 text-xs hover:text-[var(--grape)]"
                    @click="resendVerify(u)"
                  >
                    <MailPlus :size="13" /> 重发验证
                  </button>
                  <button
                    v-if="!u.email_verified && auth.user?.role === 'super_admin'"
                    class="inline-flex items-center gap-1 rounded-full bg-[var(--jade)] px-3 py-1 text-xs text-white"
                    @click="activate(u)"
                  >
                    <Check :size="13" /> 直接激活
                  </button>
                  <el-dropdown v-if="auth.user?.role === 'super_admin'" trigger="click" @command="(r: string) => setRole(u, r)">
                    <button class="rounded-full border border-[var(--border)] px-3 py-1 text-xs">设为</button>
                    <template #dropdown>
                      <el-dropdown-menu>
                        <el-dropdown-item command="user">普通用户</el-dropdown-item>
                        <el-dropdown-item command="admin">普通管理员</el-dropdown-item>
                      </el-dropdown-menu>
                    </template>
                  </el-dropdown>
                </div>
              </td>
            </tr>
          </tbody>
        </table>
      </div>

      <el-pagination
        v-if="total > 20"
        class="mt-4 justify-center"
        layout="prev, pager, next"
        :total="total"
        :page-size="20"
        :current-page="page"
        @current-change="(p: number) => { page = p; load() }"
        background
      />
    </div>
  </div>
</template>

<style scoped>
.input-style {
  height: 40px;
  width: 100%;
  border-radius: 12px;
  border: 1px solid var(--border);
  background: var(--surface-2);
  padding: 0 12px;
  padding-left: 2.5rem;
  outline: none;
  font-size: 14px;
}
.input-style:focus {
  border-color: var(--grape);
}
</style>
