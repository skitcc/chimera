import { Link } from 'react-router-dom'
import type { Track } from '../api/types'
import { useAuth } from '../auth/AuthContext'
import { useLikes } from '../likes/LikesContext'
import { usePlayer } from '../player/PlayerContext'
import { Cover } from './Cover'

type Props = {
  track: Track
  showStatus?: boolean
}

export function TrackRow({ track, showStatus = false }: Props) {
  const { play, track: current, playing } = usePlayer()
  const { user } = useAuth()
  const { ids, toggle } = useLikes()
  const active = current?.id === track.id
  const liked = ids.has(track.id)
  const ready = track.status === 'ready'

  return (
    <article className={`row ${active ? 'row-active' : ''}`}>
      <button
        className="row-play"
        type="button"
        disabled={!ready}
        onClick={() => ready && play(track)}
        aria-label={active && playing ? 'Пауза' : 'Играть'}
      >
        <Cover id={track.id} lit={active && playing} />
        <span className="row-glyph">{active && playing ? '❚❚' : '▶'}</span>
      </button>
      <div className="row-meta">
        <h3>{track.title}</h3>
        <p>
          {track.artist ? (
            <Link to={`/artist?q=${encodeURIComponent(track.artist)}`}>{track.artist}</Link>
          ) : (
            <span>без артиста</span>
          )}
          {showStatus && track.status !== 'ready' ? <em className="status"> · {track.status}</em> : null}
        </p>
      </div>
      {user ? (
        <button
          className={`heart ${liked ? 'on' : ''}`}
          type="button"
          disabled={!ready}
          onClick={() => void toggle(track.id)}
          aria-label={liked ? 'Убрать лайк' : 'Лайк'}
        >
          {liked ? '♥' : '♡'}
        </button>
      ) : null}
    </article>
  )
}
