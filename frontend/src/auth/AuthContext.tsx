import { createContext, useContext, useEffect, useMemo, useState } from 'react'
import type { ReactNode } from 'react'
import { api, setToken } from '../api/client'
import type { User } from '../api/types'

type AuthState = {
  user: User | null
  ready: boolean
  challenge: string | null
  cancelChallenge: () => void
  verify: (code: string) => Promise<void>
  login: (email: string, password: string) => Promise<void>
  register: (email: string, password: string, name: string) => Promise<void>
  logout: () => void
}

const AuthContext = createContext<AuthState | null>(null)

const USER_KEY = 'chimera.user'

export function AuthProvider({ children }: { children: ReactNode }) {
  const [user, setUser] = useState<User | null>(() => {
    const raw = localStorage.getItem(USER_KEY)
    return raw ? (JSON.parse(raw) as User) : null
  })
  const [ready, setReady] = useState(false)
  const [challenge, setChallenge] = useState<string | null>(null)
  useEffect(() => {
    const revoked = () => { setUser(null); window.location.assign(`${import.meta.env.VITE_ROUTER_BASENAME || '/legacy'}/auth`) }
    window.addEventListener('chimera:logout', revoked)
    return () => window.removeEventListener('chimera:logout', revoked)
  }, [])

  useEffect(() => {
    const token = localStorage.getItem('chimera.token')
    if (!token) {
      setReady(true)
      return
    }
    api
      .me()
      .then((me) => {
        setUser(me)
        localStorage.setItem(USER_KEY, JSON.stringify(me))
      })
      .catch(() => {
        setToken(null)
        localStorage.removeItem(USER_KEY)
        setUser(null)
      })
      .finally(() => setReady(true))
  }, [])

  const value = useMemo<AuthState>(
    () => ({
      user,
      ready,
      challenge,
      cancelChallenge: () => setChallenge(null),
      verify: async (code) => {
        if (!challenge) return
        const res = await api.verify(challenge, code)
        setToken(res.token); setUser(res.user); setChallenge(null)
        localStorage.setItem(USER_KEY, JSON.stringify(res.user))
      },
      login: async (email, password) => {
        const res = await api.login(email, password)
        if ('challenge_id' in res) { setChallenge(res.challenge_id); return }
        setToken(res.token)
        setUser(res.user)
        localStorage.setItem(USER_KEY, JSON.stringify(res.user))
      },
      register: async (email, password, name) => {
        const res = await api.register(email, password, name)
        if ('challenge_id' in res) { setChallenge(res.challenge_id); return }
        setToken(res.token)
        setUser(res.user)
        localStorage.setItem(USER_KEY, JSON.stringify(res.user))
      },
      logout: () => {
        setChallenge(null)
        setToken(null)
        localStorage.removeItem(USER_KEY)
        setUser(null)
      },
    }),
    [user, ready, challenge],
  )

  return <AuthContext.Provider value={value}>{children}</AuthContext.Provider>
}

export function useAuth() {
  const ctx = useContext(AuthContext)
  if (!ctx) {
    throw new Error('useAuth outside provider')
  }
  return ctx
}
