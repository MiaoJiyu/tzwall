<script setup lang="ts">
import { useRouter } from 'vue-router'
import { useAuthStore } from '@/stores/auth'
import { LayoutDashboard, Users, FileCheck, Files, Tags, Music, BarChart3, LogOut, Shield, Flag, Gavel, ShieldCheck, Settings, ScrollText, ClipboardList, Activity } from 'lucide-vue-next'
import ThemeToggle from '@/components/ThemeToggle.vue'

const auth = useAuthStore()
const router = useRouter()

function logout() {
  auth.logout()
  router.push('/')
}
</script>

<template>
  <div class="flex min-h-screen bg-[var(--bg)]">
    <aside class="flex w-60 flex-col border-r border-[var(--border)] bg-[var(--surface)]">
      <div class="flex h-16 items-center gap-2 border-b border-[var(--border)] px-5">
        <img src="/favicon.svg" class="h-8 w-8" alt="校徽" />
        <span class="font-serif-sc text-base font-bold text-[var(--grape)]">校园墙管理</span>
      </div>
      <nav class="flex-1 space-y-1 p-3">
        <router-link to="/admin/dashboard" class="admin-link"><LayoutDashboard :size="18" /> 数据概览</router-link>
        <router-link to="/admin/users" class="admin-link"><Users :size="18" /> 用户管理</router-link>
        <router-link to="/admin/submissions" class="admin-link"><FileCheck :size="18" /> 投稿审核</router-link>
        <router-link to="/admin/all-submissions" class="admin-link"><Files :size="18" /> 所有稿件</router-link>
        <router-link to="/admin/tags" class="admin-link"><Tags :size="18" /> 标签管理</router-link>
        <router-link to="/admin/songs" class="admin-link"><Music :size="18" /> 点歌管理</router-link>
        <router-link to="/admin/polls" class="admin-link"><BarChart3 :size="18" /> 投票管理</router-link>
        <router-link to="/admin/reports" class="admin-link"><Flag :size="18" /> 举报审核</router-link>
        <router-link to="/admin/punishments" class="admin-link"><Gavel :size="18" /> 处罚管理</router-link>
        <router-link v-if="auth.user?.role === 'super_admin'" to="/admin/permissions" class="admin-link">
          <ShieldCheck :size="18" /> 访问权限
        </router-link>
        <router-link v-if="auth.user?.role === 'super_admin'" to="/admin/configs" class="admin-link">
          <Settings :size="18" /> 系统配置
        </router-link>
        <router-link v-if="auth.user?.role === 'super_admin'" to="/admin/logs/access" class="admin-link">
          <ScrollText :size="18" /> 访问日志
        </router-link>
        <router-link v-if="auth.user?.role === 'super_admin'" to="/admin/logs/audit" class="admin-link">
          <ClipboardList :size="18" /> 审计日志
        </router-link>
        <router-link v-if="auth.user?.role === 'super_admin'" to="/admin/ops" class="admin-link">
          <Activity :size="18" /> 运维面板
        </router-link>
      </nav>
      <div class="border-t border-[var(--border)] p-3">
        <button class="admin-link w-full" @click="logout"><LogOut :size="18" /> 退出登录</button>
      </div>
    </aside>

    <div class="flex flex-1 flex-col">
      <header class="flex h-16 items-center justify-between border-b border-[var(--border)] bg-[var(--surface)] px-6">
        <div class="flex items-center gap-2 text-sm text-[var(--text-soft)]">
          <Shield :size="16" class="text-[var(--grape)]" />
          <span>管理员：{{ auth.user?.username }}</span>
        </div>
        <ThemeToggle />
      </header>
      <main class="flex-1 p-6">
        <router-view />
      </main>
    </div>
  </div>
</template>

<style scoped>
.admin-link {
  display: flex;
  align-items: center;
  gap: 10px;
  border-radius: 10px;
  padding: 10px 14px;
  font-size: 14px;
  color: var(--text-soft);
  transition: background 0.2s, color 0.2s;
}
.admin-link:hover {
  background: var(--surface-2);
  color: var(--grape);
}
.router-link-exact-active.admin-link,
.router-link-active.admin-link {
  background: var(--grape-50);
  color: var(--grape);
  font-weight: 600;
}
</style>
