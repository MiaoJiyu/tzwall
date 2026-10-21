import axios from 'axios'
import { ElMessage } from 'element-plus'

const http = axios.create({ baseURL: '/api', timeout: 15000 })

http.interceptors.request.use((cfg) => {
  const t = localStorage.getItem('token')
  if (t) cfg.headers.Authorization = 'Bearer ' + t
  if (crypto && crypto.randomUUID) cfg.headers['X-Request-Id'] = crypto.randomUUID()
  return cfg
})

http.interceptors.response.use(
  (resp) => {
    const body = resp.data
    if (body && body.code !== 0) {
      switch (body.code) {
        case 40100:
          localStorage.removeItem('token')
          localStorage.removeItem('tzwall_user')
          ElMessage.error(body.message || '登录已失效')
          if (!location.pathname.startsWith('/login') && !location.pathname.startsWith('/register')) {
            location.href = '/login'
          }
          break
        case 41011:
          ElMessage.error('请求端点失败，请联系系统管理员')
          break
        case 42900:
          ElMessage.warning('操作过于频繁，请稍后再试')
          break
        default:
          ElMessage.error(body.message || '请求失败')
      }
      return Promise.reject(body)
    }
    return body.data
  },
  (err) => {
    ElMessage.error('网络错误，请稍后重试')
    return Promise.reject(err)
  },
)

export default http
