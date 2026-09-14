import { useParams } from 'react-router-dom'
import { useEffect, useState } from 'react'
import { api } from '../api/client'
import type { Track } from '../api/types'
import { TrackList } from '../components/TrackList'

export function UserTracksPage() {
  const { id = '' } = useParams()
  const [tracks, setTracks] = useState<Track[]>([])
  const [loading, setLoading] = useState(true)

  useEffect(() => {
    api
      .userTracks(id)
      .then((page) => setTracks(page.items))
      .finally(() => setLoading(false))
  }, [id])

  return (
    <TrackList
      kicker="Каталог автора"
      title="Загрузки"
      tracks={tracks}
      loading={loading}
      empty={<p className="muted">У этого автора пока нет готовых треков.</p>}
    />
  )
}
