<script setup lang="ts">
import { ref } from 'vue'
import { useRouter } from 'vue-router'
import { Search, Plus, Home, FileText, User as UserIcon, Shield, ShieldAlert, LogOut, Menu } from 'lucide-vue-next'
import { useAuthStore } from '@/stores/auth'
import { useMetaStore } from '@/stores/meta'
import ThemeToggle from './ThemeToggle.vue'

const auth = useAuthStore()
const meta = useMetaStore()
const router = useRouter()
const keyword = ref('')
const mobileOpen = ref(false)

function doSearch() {
  router.push({ path: '/', query: keyword.value ? { keyword: keyword.value } : {} })
}
function go(path: string) {
  mobileOpen.value = false
  router.push(path)
}
function logout() {
  auth.logout()
  mobileOpen.value = false
  router.push('/')
}
</script>

<template>
  <header class="sticky top-0 z-40 border-b border-[var(--border)] bg-[var(--surface)]/90 backdrop-blur">
    <div class="mx-auto flex h-16 max-w-[1200px] items-center gap-4 px-4">
      <a class="flex items-center gap-2" @click="go('/')">
        <img src="/favicon.svg" class="h-8 w-8" alt="校徽" />
        <span class="font-serif-sc text-lg font-bold text-[var(--grape)]">{{ meta.site.site_name || '泰兴中学校园墙' }}</span>
      </a>

      <div class="relative hidden flex-1 md:block">
        <Search :size="16" class="absolute left-3 top-1/2 -translate-y-1/2 text-[var(--text-soft)]" />
        <input
          v-model="keyword"
          @keyup.enter="doSearch"
          placeholder="搜索投稿、标签…"
          class="h-9 w-full max-w-md rounded-full border border-[var(--border)] bg-[var(--surface-2)] pl-9 pr-4 text-sm outline-none focus:border-[var(--grape)]"
        />
      </div>

      <nav class="ml-auto hidden items-center gap-1 md:flex">
        <button class="nav-btn" @click="go('/')"><Home :size="16" /> 首页</button>
        <button v-if="auth.isLoggedIn" class="nav-btn" @click="go('/publish')"><Plus :size="16" /> 发布</button>
        <button v-if="auth.isLoggedIn" class="nav-btn" @click="go('/my')"><FileText :size="16" /> 我的</button>
        <ThemeToggle />
        <template v-if="auth.isLoggedIn">
          <el-dropdown trigger="click">
            <button class="ml-1 flex h-9 items-center gap-1 rounded-full border border-[var(--border)] bg-[var(--surface-2)] px-3 text-sm">
              <UserIcon :size="16" /> {{ auth.user?.username }}
            </button>
            <template #dropdown>
              <el-dropdown-menu>
                <el-dropdown-item @click="go('/my')"><FileText :size="14" /> 我的投稿</el-dropdown-item>
                <el-dropdown-item @click="go('/profile')"><UserIcon :size="14" /> 账号安全</el-dropdown-item>
                <el-dropdown-item @click="go('/my/punishments')"><ShieldAlert :size="14" /> 权限变更记录</el-dropdown-item>
                <el-dropdown-item v-if="auth.isAdmin" @click="go('/admin')"><Shield :size="14" /> 后台管理</el-dropdown-item>
                <el-dropdown-item divided @click="logout"><LogOut :size="14" /> 退出登录</el-dropdown-item>
              </el-dropdown-menu>
            </template>
          </el-dropdown>
        </template>
        <template v-else>
          <button class="nav-btn" @click="go('/login')">登录</button>
          <button class="btn-grape rounded-full px-4 py-2 text-sm font-medium" @click="go('/register')">注册</button>
        </template>
      </nav>

      <button class="ml-auto md:hidden" @click="mobileOpen = !mobileOpen"><Menu :size="22" /></button>
    </div>

    <div v-if="mobileOpen" class="border-t border-[var(--border)] px-4 py-3 md:hidden">
      <input
        v-model="keyword"
        @keyup.enter="doSearch"
        placeholder="搜索投稿、标签…"
        class="mb-3 h-9 w-full rounded-full border border-[var(--border)] bg-[var(--surface-2)] px-4 text-sm outline-none"
      />
      <div class="flex flex-col gap-1">
        <button class="nav-btn justify-start" @click="go('/')"><Home :size="16" /> 首页</button>
        <button v-if="auth.isLoggedIn" class="nav-btn justify-start" @click="go('/publish')"><Plus :size="16" /> 发布投稿</button>
        <button v-if="auth.isLoggedIn" class="nav-btn justify-start" @click="go('/my')"><FileText :size="16" /> 我的投稿</button>
        <button v-if="auth.isLoggedIn" class="nav-btn justify-start" @click="go('/profile')"><UserIcon :size="16" /> 账号安全</button>
        <button v-if="auth.isLoggedIn && auth.isAdmin" class="nav-btn justify-start" @click="go('/admin')"><Shield :size="16" /> 后台管理</button>
        <button v-if="auth.isLoggedIn" class="nav-btn justify-start" @click="logout"><LogOut :size="16" /> 退出登录</button>
        <button v-else class="nav-btn justify-start" @click="go('/login')">登录</button>
        <button v-if="!auth.isLoggedIn" class="btn-grape rounded-full px-4 py-2 text-sm" @click="go('/register')">注册</button>
      </div>
    </div>
  </header>
</template>

<style scoped>
.nav-btn {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  border-radius: 9999px;
  padding: 8px 14px;
  font-size: 14px;
  color: var(--text-soft);
  transition: color 0.2s, background 0.2s;
}
.nav-btn:hover {
  color: var(--grape);
  background: var(--surface-2);
}
</style>
