import { createContext, useContext, useState, useEffect } from 'react'
import { authApi } from '../api/auth.js'

const AuthContext = createContext(null)

export function AuthProvider({ children }) {
  const [user, setUser] = useState(null)
  const [token, setToken] = useState(localStorage.getItem('veer_token'))
  const [loading, setLoading] = useState(true)

  // Validate existing token on mount
  useEffect(() => {
    if (token) {
      authApi.me()
        .then((res) => {
          setUser(res)
        })
        .catch(() => {
          localStorage.removeItem('veer_token')
          setToken(null)
          setUser(null)
        })
        .finally(() => setLoading(false))
    } else {
      setLoading(false)
    }
  }, [])

  const login = async (username, password) => {
    const res = await authApi.login(username, password)
    const { token: newToken, username: uname } = res
    localStorage.setItem('veer_token', newToken)
    setToken(newToken)
    setUser({ username: uname })
    return res
  }

  const logout = async () => {
    try {
      await authApi.logout()
    } catch {
      // Ignore errors during logout - best-effort cleanup
    }
    localStorage.removeItem('veer_token')
    setToken(null)
    setUser(null)
  }

  return (
    <AuthContext.Provider value={{ user, token, login, logout, isAuthenticated: !!token, loading }}>
      {children}
    </AuthContext.Provider>
  )
}

export function useAuth() {
  const ctx = useContext(AuthContext)
  if (!ctx) throw new Error('useAuth must be used within AuthProvider')
  return ctx
}

export default AuthContext
