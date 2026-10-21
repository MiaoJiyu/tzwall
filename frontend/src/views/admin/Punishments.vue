<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { Plus, Gavel, Ban, Undo2 } from 'lucide-vue-next'
import { ElMessage, ElMessageBox } from 'element-plus'
import { punishmentApi, adminApi } from '@/api'

const tab = ref('list')
const items = ref<any[]>([])
const ipBans = ref<any[]>([])
const total = ref(0)
const page = ref(1)

const form = ref({ user_id: null as number | null, type: 'warn', reason: '', permanent: false, expires_at: '', new_username: '' })
const ipForm = ref({ cidr: '', reason: '', permanent: false, expires_at: '' })

// 用户搜索（支持用户名 / 邮箱 / 用户 ID），issue 7
const userKeyword = ref('')
const userOptions = ref<any[]>([])
const selectedUser = ref<any>(null)
let userSearchTimer: any = null
function onUserSearch() {
  clearTimeout(userSearchTimer)
  const kw = userKeyword.value.trim()
  if (!kw) {
    userOptions.value = []
    return
  }
  userSearchTimer = setTimeout(async () => {
    try {
      const r = await adminApi.users({ keyword: kw, page: 1, page_size: 10 })
      userOptions.value = r.items || []
    } catch {
      userOptions.value = []
    }
  }, 300)
}
function selectUser(u: any) {
  form.value.user_id = u.id
  selectedUser.value = u
  userOptions.value = []
  userKeyword.value = ''
}

const typeLabel: Record<string, string> = {
  warn: '警告',
  mute: '禁言',
  ban_user: '封禁用户名',
  ban_ip: '封禁 IP',
  force_reset: '强制修改用户名与密码',
}

async function load() {
  const d = await punishmentApi.list({ page: page.value, page_size: 20 })
  items.value = d.items || []
  total.value = d.total
}
async function loadIP() {
  const d = await punishmentApi.ipBans()
  ipBans.value = d.items || []
}
async function create() {
  if (!form.value.user_id) {
    ElMessage.warning('请填写用户 ID')
    return
  }
  if (!form.value.reason) {
    ElMessage.warning('请填写处罚理由')
    return
  }
  const p: any = {
    user_id: form.value.user_id,
    type: form.value.type,
    reason: form.value.reason,
    permanent: form.value.permanent,
    expires_at: form.value.permanent ? undefined : form.value.expires_at || undefined,
  }
  if (form.value.type === 'force_reset' && form.value.new_username) p.new_username = form.value.new_username
  const r = await punishmentApi.create(p)
  if (r?.temp_password) {
    ElMessage.success('已执行强制改密，临时密码：' + r.temp_password)
  } else {
    ElMessage.success('处罚已生效')
  }
  form.value = { user_id: null, type: 'warn', reason: '', permanent: false, expires_at: '', new_username: '' }
  tab.value = 'list'
  load()
}
async function revoke(id: number) {
  try {
    await ElMessageBox.confirm('撤销后记录保留并标记已撤销，强制改密不恢复原值。确认撤销？', '撤销处罚', { type: 'warning' })
  } catch {
    return
  }
  await punishmentApi.revoke(id)
  ElMessage.success('已撤销')
  load()
}
async function createIPBan() {
  if (!ipForm.value.cidr || !ipForm.value.reason) {
    ElMessage.warning('IP 与理由均为必填')
    return
  }
  await punishmentApi.createIpBan({
    cidr: ipForm.value.cidr,
    reason: ipForm.value.reason,
    permanent: ipForm.value.permanent,
    expires_at: ipForm.value.permanent ? undefined : ipForm.value.expires_at || undefined,
  })
  ElMessage.success('IP 封禁已生效')
  ipForm.value = { cidr: '', reason: '', permanent: false, expires_at: '' }
  loadIP()
  load()
}
async function revokeIP(id: number) {
  await punishmentApi.revokeIpBan(id)
  ElMessage.success('已撤销 IP 封禁')
  loadIP()
}

onMounted(() => {
  load()
  loadIP()
})
</script>

<template>
  <div>
    <h1 class="mb-5 font-serif-sc text-2xl font-bold text-[var(--grape)]">处罚管理</h1>

    <div class="mb-4 flex flex-wrap gap-2">
      <button :class="tab === 'list' ? 'bg-grape-600 text-white' : 'bg-[var(--surface-2)] text-[var(--text-soft)]'" class="rounded-full px-4 py-1.5 text-sm" @click="tab = 'list'">处罚记录</button>
      <button :class="tab === 'new' ? 'bg-grape-600 text-white' : 'bg-[var(--surface-2)] text-[var(--text-soft)]'" class="flex items-center gap-1 rounded-full px-4 py-1.5 text-sm" @click="tab = 'new'"><Plus :size="14" /> 新增处罚</button>
      <button :class="tab === 'ip' ? 'bg-grape-600 text-white' : 'bg-[var(--surface-2)] text-[var(--text-soft)]'" class="flex items-center gap-1 rounded-full px-4 py-1.5 text-sm" @click="tab = 'ip'"><Ban :size="14" /> IP 封禁</button>
    </div>

    <!-- 新增处罚 -->
    <div v-if="tab === 'new'" class="paper-card mb-5 p-5">
      <div class="grid gap-3 sm:grid-cols-2">
        <!-- 用户搜索（用户名 / 邮箱 / 用户 ID），issue 7 -->
        <div class="relative">
          <input v-model="userKeyword" @input="onUserSearch" placeholder="搜索用户（用户名/邮箱/ID）" class="input-box" />
          <div v-if="userOptions.length" class="absolute z-10 mt-1 max-h-56 w-full overflow-auto rounded-lg border border-[var(--border)] bg-[var(--surface)] py-1 shadow-soft">
            <button
              v-for="u in userOptions"
              :key="u.id"
              type="button"
              class="block w-full px-3 py-1.5 text-left text-sm hover:bg-[var(--surface-2)]"
              @click="selectUser(u)"
            >
              <span class="font-medium">#{{ u.id }}</span> {{ u.username }}
              <span class="text-[var(--text-soft)]">{{ u.email }}</span>
            </button>
          </div>
          <p v-if="selectedUser" class="mt-1 text-xs text-[var(--text-soft)]">
            已选：#{{ selectedUser.id }} {{ selectedUser.username }}（{{ selectedUser.email }}）
          </p>
        </div>
        <select v-model="form.type" class="input-box">
          <option value="warn">警告</option>
          <option value="mute">禁言</option>
          <option value="ban_user">封禁用户名</option>
          <option value="force_reset">强制修改用户名与密码</option>
        </select>
        <input v-if="form.type === 'force_reset'" v-model="form.new_username" placeholder="新用户名（可选）" class="input-box" />
        <label class="flex items-center gap-2 text-sm"><input type="checkbox" v-model="form.permanent" /> 永久处罚</label>
        <input v-if="!form.permanent" v-model="form.expires_at" type="datetime-local" class="input-box" />
      </div>
      <textarea v-model="form.reason" rows="2" placeholder="处罚理由（必填）" class="input-box mt-3"></textarea>
      <button class="btn-grape mt-3 flex items-center gap-1 rounded-full px-5 py-2 text-sm" @click="create"><Gavel :size="14" /> 执行处罚</button>
    </div>

    <!-- IP 封禁 -->
    <div v-if="tab === 'ip'" class="paper-card mb-5 p-5">
      <div class="grid gap-3 sm:grid-cols-3">
        <input v-model="ipForm.cidr" placeholder="IP / CIDR（支持 IPv4、IPv6）" class="input-box" />
        <input v-model="ipForm.reason" placeholder="封禁理由（必填）" class="input-box" />
        <label class="flex items-center gap-2 text-sm"><input type="checkbox" v-model="ipForm.permanent" /> 永久</label>
      </div>
      <input v-if="!ipForm.permanent" v-model="ipForm.expires_at" type="datetime-local" placeholder="临时封禁到期时间" class="input-box mt-3" />
      <button class="btn-grape mt-3 flex items-center gap-1 rounded-full px-5 py-2 text-sm" @click="createIPBan"><Ban :size="14" /> 封禁 IP</button>

      <div class="mt-4 space-y-2">
        <div v-for="b in ipBans" :key="b.id" class="flex flex-wrap items-center gap-2 rounded-lg bg-[var(--surface-2)] px-3 py-2 text-sm">
          <code class="text-xs">{{ b.cidr }}</code>
          <span class="text-[var(--text-soft)]">{{ b.reason }}</span>
          <span v-if="b.revoked" class="rounded bg-[var(--surface-2)] px-2 py-0.5 text-xs">已撤销</span>
          <span v-else-if="b.permanent" class="rounded bg-[var(--danger)]/15 px-2 py-0.5 text-xs text-[var(--danger)]">永久</span>
          <span v-else-if="b.expires_at" class="rounded bg-warn/15 px-2 py-0.5 text-xs text-warn">至 {{ b.expires_at }}</span>
          <button v-if="!b.revoked" class="ml-auto text-xs text-[var(--grape)]" @click="revokeIP(b.id)">撤销</button>
        </div>
      </div>
    </div>

    <!-- 处罚记录 -->
    <div v-if="tab === 'list'" class="paper-card divide-y divide-[var(--border)]">
      <div v-for="p in items" :key="p.id" class="flex flex-wrap items-center gap-2 p-4 text-sm">
        <span class="rounded-full bg-grape-50 px-2 py-0.5 text-xs font-medium text-[var(--grape)]">{{ typeLabel[p.type] || p.type }}</span>
        <span class="text-xs text-[var(--text-soft)]">用户 #{{ p.user_id }}</span>
        <span v-if="p.revoked" class="rounded bg-[var(--surface-2)] px-2 py-0.5 text-xs">已撤销</span>
        <span v-else-if="p.permanent" class="rounded bg-[var(--danger)]/15 px-2 py-0.5 text-xs text-[var(--danger)]">永久</span>
        <span v-else-if="p.expires_at" class="rounded bg-warn/15 px-2 py-0.5 text-xs text-warn">至 {{ p.expires_at }}</span>
        <span class="line-clamp-1 flex-1 text-[var(--text-soft)]">{{ p.reason }}</span>
        <button v-if="!p.revoked" class="flex items-center gap-1 rounded-full border border-[var(--border)] px-3 py-1 text-xs hover:text-[var(--grape)]" @click="revoke(p.id)">
          <Undo2 :size="12" /> 撤销
        </button>
      </div>
      <div v-if="!items.length" class="p-10 text-center text-[var(--text-soft)]">暂无处罚记录</div>
      <el-pagination
        v-if="total > 20"
        class="p-3"
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
.input-box {
  height: 40px;
  width: 100%;
  border-radius: 10px;
  border: 1px solid var(--border);
  background: var(--surface-2);
  padding: 8px 12px;
  font-size: 14px;
  outline: none;
}
.input-box:focus {
  border-color: var(--grape);
}
textarea.input-box {
  height: auto;
}
</style>
