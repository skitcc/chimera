import { useSearchParams } from 'react-router-dom'
import { useEffect, useState } from 'react'
import { api } from '../api/client'
import type { Track } from '../api/types'
import { TrackList } from '../components/TrackList'

export function ArtistPage() {
  const [params] = useSearchParams()
  const artist = params.get('q') ?? ''
  const [tracks, setTracks] = useState<Track[]>([])
  const [loading, setLoading] = useState(true)

  useEffect(() => {
    if (!artist) {
      setTracks([])
      setLoading(false)
      return
    }
    setLoading(true)
    api
      .feed({ artist })
      .then((page) => setTracks(page.items))
      .finally(() => setLoading(false))
  }, [artist])

  return (
    <TrackList
      kicker="По кредиту"
      title={artist || 'Артист'}
      tracks={tracks}
      loading={loading}
      empty={<p className="muted">Готовых треков с таким артистом нет.</p>}
    />
  )
}
