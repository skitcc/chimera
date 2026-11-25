import { ApiError } from './types'
import type { AuthResponse, Session, Track, TrackPage, UploadSession, User } from './types'

const base = import.meta.env.VITE_API_URL ?? ''

const TOKEN_KEY = 'chimera.token'
const USER_ID_KEY = 'chimera.user_id'

type ClientSession = {
  token: string | null
  userId: string | null
}

const session: ClientSession = {
  token: localStorage.getItem(TOKEN_KEY),
  userId: localStorage.getItem(USER_ID_KEY),
}

export function setToken(token: string | null) {
  session.token = token
  if (token) {
    localStorage.setItem(TOKEN_KEY, token)
    return
  }
  localStorage.removeItem(TOKEN_KEY)
  setUserId(null)
}

function setUserId(id: string | null) {
  session.userId = id
  if (id) {
    localStorage.setItem(USER_ID_KEY, id)
    return
  }
  localStorage.removeItem(USER_ID_KEY)
}

function currentUserId(): string {
  if (!session.userId) {
    throw new ApiError('unauthorized', 'missing session')
  }
  return session.userId
}

export function streamUrl(id: string) {
  return `${base}/api/v1/tracks/${id}/stream`
}

async function request<T>(path: string, init: RequestInit = {}): Promise<T> {
  const headers = new Headers(init.headers)
  if (init.body && !headers.has('Content-Type')) {
    headers.set('Content-Type', 'application/json')
  }
  if (session.token) {
    headers.set('Authorization', `Bearer ${session.token}`)
  }

  const res = await fetch(`${base}${path}`, { ...init, headers })
  if (res.status === 204) {
    return undefined as T
  }

  const data: unknown = await res.json().catch(() => ({}))
  if (!res.ok) {
    const err = data as { code?: string; message?: string }
    if (res.status === 401 && session.token && (err.message === 'invalid token' || err.message === 'missing token')) {
      setToken(null)
      localStorage.removeItem('chimera.user')
      window.dispatchEvent(new Event('chimera:logout'))
    }
    throw new ApiError(err.code ?? 'error', err.message ?? res.statusText)
  }
  return data as T
}

export type Challenge = { challenge_id: string; expires_at: number }
async function finishSession(created: Session): Promise<AuthResponse> {
  setUserId(created.user_id)
  setToken(created.token)
  const user = await request<User>(`/api/v1/users/${created.user_id}`)
  return { token: created.token, user }
}
async function login(email: string, password: string): Promise<AuthResponse | Challenge> {
  const result = await request<Session | Challenge>('/api/v1/sessions', {
    method: 'POST', body: JSON.stringify({ email, password }),
  })
  return 'challenge_id' in result ? result : finishSession(result)
}

export const api = {
  register: async (email: string, password: string, name: string) => {
    await request<User>('/api/v1/users', {
      method: 'POST',
      body: JSON.stringify({ email, password, name }),
    })
    return login(email, password)
  },

  login,
  verify: async (challenge_id: string, code: string) => finishSession(await request<Session>('/api/v1/sessions/verify', {
    method: 'POST', body: JSON.stringify({ challenge_id, code }),
  })),
  recover: (email: string) => request<{message: string}>('/api/v1/password-recovery', { method: 'POST', body: JSON.stringify({ email }) }),
  change: (current_password: string) => request<Challenge>('/api/v1/password-change', { method: 'POST', body: JSON.stringify({ current_password }) }),
  passwordVerify: (purpose: 'recovery' | 'change', challenge_id: string, code: string, new_password: string, email: string) => request<void>(`/api/v1/password-${purpose}/verify`, {
    method: 'POST', body: JSON.stringify({ challenge_id, code, new_password, email }),
  }),

  me: async () => {
    await request<void>('/api/v1/session')
    return request<User>(`/api/v1/users/${currentUserId()}`)
  },

  feed: (params: { cursor?: string; artist?: string } = {}) => {
    const q = new URLSearchParams({ limit: '40' })
    if (params.cursor) q.set('cursor', params.cursor)
    if (params.artist) q.set('artist', params.artist)
    return request<TrackPage>(`/api/v1/tracks?${q}`)
  },

  myTracks: () => request<TrackPage>(`/api/v1/tracks?user_id=${currentUserId()}&limit=100`),

  liked: () => request<TrackPage>(`/api/v1/tracks?liked_by=${currentUserId()}&limit=100`),

  userTracks: (userId: string) => request<TrackPage>(`/api/v1/tracks?user_id=${userId}&status=ready&limit=100`),

  like: (id: string) => request<void>(`/api/v1/users/${currentUserId()}/likes/${id}`, { method: 'PUT' }),

  unlike: (id: string) => request<void>(`/api/v1/users/${currentUserId()}/likes/${id}`, { method: 'DELETE' }),

  initUpload: (title: string, artist: string, size: number) =>
    request<UploadSession>('/api/v1/tracks', {
      method: 'POST',
      body: JSON.stringify({ title, artist, size }),
    }),

  completeUpload: (id: string) =>
    request<Track>(`/api/v1/tracks/${id}`, {
      method: 'PATCH',
      body: JSON.stringify({ status: 'ready' }),
    }),

  putFile: async (url: string, file: File) => {
    const res = await fetch(url, { method: 'PUT', body: file })
    if (!res.ok) {
      throw new ApiError('upload', 'не удалось залить файл в хранилище')
    }
  },
}
