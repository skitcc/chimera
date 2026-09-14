import { useEffect, useState } from 'react'
import { api } from '../api/client'
import type { Track } from '../api/types'
import { TrackList } from '../components/TrackList'

export function FeedPage() {
  const [tracks, setTracks] = useState<Track[]>([])
  const [loading, setLoading] = useState(true)

  useEffect(() => {
    api
      .feed()
      .then((page) => setTracks(page.items))
      .finally(() => setLoading(false))
  }, [])

  return (
    <TrackList
      kicker="Эфир"
      title="Лента"
      tracks={tracks}
      loading={loading}
      empty={<p className="muted">Пока пусто. Первый трек разбудит её.</p>}
    />
  )
}
