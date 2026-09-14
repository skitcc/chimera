import { Navigate, useNavigate } from 'react-router-dom'
import { useState } from 'react'
import { api } from '../api/client'
import { ApiError } from '../api/types'
import { useAuth } from '../auth/AuthContext'

export function UploadPage() {
  const { user, ready } = useAuth()
  const navigate = useNavigate()
  const [title, setTitle] = useState('')
  const [artist, setArtist] = useState('')
  const [file, setFile] = useState<File | null>(null)
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState('')

  if (ready && !user) {
    return <Navigate to="/auth" replace />
  }

  return (
    <section className="page">
      <p className="eyebrow">Сигнал</p>
      <h1>Загрузить трек</h1>
      <p className="lede">Файл уходит в хранилище напрямую. API только открывает глаза и ставит ready.</p>
      <form
        className="upload"
        onSubmit={(e) => {
          e.preventDefault()
          if (!file) {
            return
          }
          setBusy(true)
          setError('')
          void (async () => {
            try {
              const session = await api.initUpload(title, artist, file.size)
              await api.putFile(session.upload_url, file)
              await api.completeUpload(session.track.id)
              navigate('/library')
            } catch (err) {
              setError(err instanceof ApiError ? err.message : 'загрузка сорвалась')
            } finally {
              setBusy(false)
            }
          })()
        }}
      >
        <label>
          Название
          <input value={title} onChange={(e) => setTitle(e.target.value)} required />
        </label>
        <label>
          Артист
          <input value={artist} onChange={(e) => setArtist(e.target.value)} />
        </label>
        <label className="file">
          Аудиофайл
          <input
            type="file"
            accept="audio/mpeg,audio/*"
            required
            onChange={(e) => setFile(e.target.files?.[0] ?? null)}
          />
          <span>{file ? `${file.name} · ${Math.ceil(file.size / 1024)} КБ` : 'выберите mp3'}</span>
        </label>
        {error ? <p className="error">{error}</p> : null}
        <button type="submit" className="primary" disabled={busy}>
          {busy ? 'Заливаем…' : 'Опубликовать'}
        </button>
      </form>
    </section>
  )
}
