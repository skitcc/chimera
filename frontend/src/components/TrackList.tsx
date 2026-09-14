import type { ReactNode } from 'react'
import type { Track } from '../api/types'
import { TrackRow } from '../components/TrackRow'

type Props = {
  title: string
  kicker?: string
  tracks: Track[]
  empty: ReactNode
  showStatus?: boolean
  loading?: boolean
}

export function TrackList({ title, kicker, tracks, empty, showStatus, loading }: Props) {
  return (
    <section className="page">
      {kicker ? <p className="eyebrow">{kicker}</p> : null}
      <h1>{title}</h1>
      {loading ? <p className="muted">Собираем пластинку…</p> : null}
      {!loading && tracks.length === 0 ? empty : null}
      <div className="stack">
        {tracks.map((track) => (
          <TrackRow key={track.id} track={track} showStatus={showStatus} />
        ))}
      </div>
    </section>
  )
}
