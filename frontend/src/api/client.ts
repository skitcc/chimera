import { ApiError } from './types'
import type { AuthResponse, Track, TrackPage, UploadSession, User } from './types'

const base = import.meta.env.VITE_API_URL ?? ''

type Session = {
  token: string | null
}

const session: Session = { token: localStorage.getItem('chimera.token') }

export function setToken(token: string | null) {
  session.token = token
  if (token) {
    localStorage.setItem('chimera.token', token)
    return
  }
  localStorage.removeItem('chimera.token')
}

export function streamUrl(id: string) {
  return `${base}/v1/tracks/${id}/stream`
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
    throw new ApiError(err.code ?? 'error', err.message ?? res.statusText)
  }
  return data as T
}

export const api = {
  register: (email: string, password: string, name: string) =>
    request<AuthResponse>('/v1/auth/register', {
      method: 'POST',
      body: JSON.stringify({ email, password, name }),
    }),

  login: (email: string, password: string) =>
    request<AuthResponse>('/v1/auth/login', {
      method: 'POST',
      body: JSON.stringify({ email, password }),
    }),

  me: () => request<User>('/v1/me'),

  feed: (params: { cursor?: string; artist?: string } = {}) => {
    const q = new URLSearchParams({ limit: '40' })
    if (params.cursor) q.set('cursor', params.cursor)
    if (params.artist) q.set('artist', params.artist)
    return request<TrackPage>(`/v1/tracks?${q}`)
  },

  myTracks: () => request<TrackPage>('/v1/me/tracks?limit=100'),

  liked: () => request<TrackPage>('/v1/me/likes?limit=100'),

  userTracks: (userId: string) => request<TrackPage>(`/v1/users/${userId}/tracks?limit=100`),

  like: (id: string) => request<void>(`/v1/tracks/${id}/like`, { method: 'POST' }),

  unlike: (id: string) => request<void>(`/v1/tracks/${id}/like`, { method: 'DELETE' }),

  initUpload: (title: string, artist: string, size: number) =>
    request<UploadSession>('/v1/tracks/upload-init', {
      method: 'POST',
      body: JSON.stringify({ title, artist, size }),
    }),

  completeUpload: (id: string) =>
    request<Track>(`/v1/tracks/${id}/upload-complete`, { method: 'POST' }),

  putFile: async (url: string, file: File) => {
    const res = await fetch(url, { method: 'PUT', body: file })
    if (!res.ok) {
      throw new ApiError('upload', 'не удалось залить файл в хранилище')
    }
  },
}
