import { Navigate } from 'react-router-dom'
import { useEffect, useState } from 'react'
import { api } from '../api/client'
import type { Track } from '../api/types'
import { useAuth } from '../auth/AuthContext'
import { useLikes } from '../likes/LikesContext'
import { TrackList } from '../components/TrackList'

export function LikesPage() {
  const { user, ready } = useAuth()
  const { ids } = useLikes()
  const [tracks, setTracks] = useState<Track[]>([])
  const [loading, setLoading] = useState(true)

  useEffect(() => {
    if (!user) {
      return
    }
    api
      .liked()
      .then((page) => setTracks(page.items))
      .finally(() => setLoading(false))
  }, [user])

  if (ready && !user) {
    return <Navigate to="/auth" replace />
  }

  return (
    <TrackList
      kicker="Только ваше"
      title="Любимое"
      tracks={tracks.filter((t) => ids.has(t.id))}
      loading={loading}
      empty={<p className="muted">Отметьте сердце на треке — он осядет здесь.</p>}
    />
  )
}
