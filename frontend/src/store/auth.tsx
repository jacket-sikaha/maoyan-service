// store/auth.tsx — AuthContext 全局认证状态
//   管理模式：JWT Token + User 对象双双持久化到 localStorage
//   页面刷新时从 localStorage 恢复登录态
//   401 自动清理由 api/client.ts 的 axios 拦截器处理
//   提供 login/register/logout 三个方法 + useAuth() hook
import { createContext, useContext, useState, useEffect, ReactNode } from 'react'
import { login as apiLogin, register as apiRegister } from '../api/endpoints'

interface User { id: string; email: string }
interface AuthContextType {
  user: User | null
  token: string | null
  login: (email: string, password: string) => Promise<void>
  register: (email: string, password: string) => Promise<void>
  logout: () => void
}

const AuthContext = createContext<AuthContextType | null>(null)

export function AuthProvider({ children }: { children: ReactNode }) {
  const [user, setUser] = useState<User | null>(null)
  const [token, setToken] = useState<string | null>(null)

  // 应用启动时从 localStorage 恢复登录态
  useEffect(() => {
    const savedToken = localStorage.getItem('token')
    const savedUser = localStorage.getItem('user')
    if (savedToken && savedUser) {
      setToken(savedToken)
      setUser(JSON.parse(savedUser))
    }
  }, [])

  const login = async (email: string, password: string) => {
    const res = await apiLogin({ email, password })
    if (res.code === 0) {
      setToken(res.data.token)
      setUser(res.data.user)
      localStorage.setItem('token', res.data.token)
      localStorage.setItem('user', JSON.stringify(res.data.user))
    } else {
      throw new Error(res.msg)
    }
  }

  const register = async (email: string, password: string) => {
    const res = await apiRegister({ email, password })
    if (res.code === 0) {
      setToken(res.data.token)
      setUser(res.data.user)
      localStorage.setItem('token', res.data.token)
      localStorage.setItem('user', JSON.stringify(res.data.user))
    } else {
      throw new Error(res.msg)
    }
  }

  const logout = () => {
    setToken(null)
    setUser(null)
    localStorage.removeItem('token')
    localStorage.removeItem('user')
  }

  return (
    <AuthContext.Provider value={{ user, token, login, register, logout }}>
      {children}
    </AuthContext.Provider>
  )
}

// useAuth hook — 在任何组件中获取认证状态和操作方法
export function useAuth() {
  const ctx = useContext(AuthContext)
  if (!ctx) throw new Error('useAuth must be within AuthProvider')
  return ctx
}
