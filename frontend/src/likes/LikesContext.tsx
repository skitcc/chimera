import { createContext, useContext, useEffect, useMemo, useState } from 'react'
import type { ReactNode } from 'react'
import { api } from '../api/client'
import { ApiError } from '../api/types'
import { useAuth } from '../auth/AuthContext'

type LikesState = {
  ids: Set<string>
  toggle: (id: string) => Promise<void>
}

const LikesContext = createContext<LikesState | null>(null)

export function LikesProvider({ children }: { children: ReactNode }) {
  const { user } = useAuth()
  const [ids, setIds] = useState<Set<string>>(new Set())

  useEffect(() => {
    if (!user) {
      setIds(new Set())
      return
    }
    api
      .liked()
      .then((page) => setIds(new Set(page.items.map((t) => t.id))))
      .catch(() => setIds(new Set()))
  }, [user])

  const value = useMemo<LikesState>(
    () => ({
      ids,
      toggle: async (id) => {
        const liked = ids.has(id)
        const next = new Set(ids)
        try {
          if (liked) {
            next.delete(id)
            setIds(next)
            await api.unlike(id)
            return
          }
          next.add(id)
          setIds(next)
          await api.like(id)
        } catch (err) {
          if (err instanceof ApiError && err.code === 'conflict') {
            next.add(id)
            setIds(next)
            return
          }
          setIds(ids)
          throw err
        }
      },
    }),
    [ids],
  )

  return <LikesContext.Provider value={value}>{children}</LikesContext.Provider>
}

export function useLikes() {
  const ctx = useContext(LikesContext)
  if (!ctx) {
    throw new Error('useLikes outside provider')
  }
  return ctx
}
