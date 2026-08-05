// api/client.ts — Axios 客户端封装，统一拦截器
//   - request 拦截器：自动附带 JWT Bearer Token
//   - response 拦截器：401 时自动清除本地登录态（token + user），防止过期 Token 残留
//   - 超时 30s，baseURL=/api（开发时由 Vite proxy 转发到后端 8080）
import axios from 'axios'

const api = axios.create({ baseURL: '/api', timeout: 30000 })

// 请求拦截：自动注入 JWT
api.interceptors.request.use(config => {
  const token = localStorage.getItem('token')
  if (token) config.headers.Authorization = `Bearer ${token}`
  return config
})

// 响应拦截：401 自动登出
api.interceptors.response.use(
  res => res.data,
  err => {
    if (err.response?.status === 401) {
      localStorage.removeItem('token')
      localStorage.removeItem('user')
    }
    return Promise.reject(err)
  }
)

export default api
