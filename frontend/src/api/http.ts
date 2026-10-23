import axios from 'axios'
import { ElMessage } from 'element-plus'

const http = axios.create({ baseURL: '/api', timeout: 15000 })

http.interceptors.request.use((cfg) => {
  const t = localStorage.getItem('token')
  if (t) cfg.headers.Authorization = 'Bearer ' + t
  if (crypto && crypto.randomUUID) cfg.headers['X-Request-Id'] = crypto.randomUUID()
  return cfg
})

function onAuthPage() {
  return location.pathname.startsWith('/login') || location.pathname.startsWith('/register')
}

// 会话失效统一处理：清除本地登录态。
// 仅当「原本有 token 且非静默请求」时才提示并跳转登录页；
// 登出后的滞后请求、静默请求（如登出接口本身）不提示、不跳转，避免误导。
function handleSessionExpired(silent: boolean) {
  const hadToken = !!localStorage.getItem('token')
  localStorage.removeItem('token')
  localStorage.removeItem('tzwall_user')
  if (silent || !hadToken) return
  if (!onAuthPage()) location.href = '/login'
  ElMessage.error('登录已失效，请重新登录')
}

// 业务错误（HTTP 200 且 code!=0）
function handleBizError(code: number, message: string, silent: boolean) {
  if (silent) return
  switch (code) {
    case 40100:
      handleSessionExpired(false)
      break
    case 41011:
      ElMessage.error('请求端点失败，请联系系统管理员')
      break
    case 42900:
      ElMessage.warning('操作过于频繁，请稍后再试')
      break
    default:
      ElMessage.error(message || '请求失败')
  }
}

http.interceptors.response.use(
  (resp) => {
    const body = resp.data
    const silent = !!(resp.config as any)?.silent
    if (body && body.code !== 0) {
      handleBizError(body.code, body.message, silent)
      return Promise.reject(body)
    }
    return body.data
  },
  (err) => {
    const resp = err?.response
    const silent = !!(err?.config as any)?.silent

    // 无 response = 真正的网络错误（超时 / 断网 / 服务不可达）
    if (!resp) {
      if (!silent) ElMessage.error('网络错误，请稍后重试')
      return Promise.reject(err)
    }

    // 有 response 但 HTTP 非 2xx：这是服务端明确返回的业务/会话错误，不是网络错误
    const body = resp.data || {}
    const code = body.code
    if (resp.status === 401 || code === 40100) {
      handleSessionExpired(silent) // 会话失效：清本地，非静默时提示并跳登录
    } else if (resp.status === 403 || code === 41011) {
      if (!silent) ElMessage.error(body.message || '请求端点失败，请联系系统管理员')
    } else if (resp.status === 429 || code === 42900) {
      if (!silent) ElMessage.warning('操作过于频繁，请稍后再试')
    } else if (resp.status === 404 || code === 40400) {
      if (!silent) ElMessage.error(body.message || '资源不存在')
    } else {
      if (!silent) ElMessage.error(body.message || '请求失败')
    }
    return Promise.reject(body.message ? body : err)
  },
)

export default http
