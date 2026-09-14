export type User = {
  id: string
  email: string
  name: string
}

export type Track = {
  id: string
  user_id: string
  title: string
  artist: string
  status: 'pending' | 'processing' | 'ready' | string
  size_bytes: number
}

export type TrackPage = {
  items: Track[]
  next_cursor?: string
  limit: number
}

export type AuthResponse = {
  token: string
  user: User
}

export type UploadSession = {
  track: Track
  upload_url: string
}

export class ApiError extends Error {
  code: string

  constructor(code: string, message: string) {
    super(message)
    this.name = 'ApiError'
    this.code = code
  }
}
