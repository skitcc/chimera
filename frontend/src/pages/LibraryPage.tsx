import { Navigate } from 'react-router-dom'
import { useEffect, useState } from 'react'
import { api } from '../api/client'
import type { Track } from '../api/types'
import { useAuth } from '../auth/AuthContext'
import { TrackList } from '../components/TrackList'

export function LibraryPage() {
  const { user, ready } = useAuth()
  const [tracks, setTracks] = useState<Track[]>([])
  const [loading, setLoading] = useState(true)

  useEffect(() => {
    if (!user) {
      return
    }
    api
      .myTracks()
      .then((page) => setTracks(page.items))
      .finally(() => setLoading(false))
  }, [user])

  if (ready && !user) {
    return <Navigate to="/auth" replace />
  }

  return (
    <TrackList
      kicker="Архив автора"
      title="Мои треки"
      tracks={tracks}
      loading={loading}
      showStatus
      empty={<p className="muted">Здесь будут ваши загрузки, включая черновики.</p>}
    />
  )
}
