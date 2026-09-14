import { Cover } from './Cover'
import { usePlayer } from '../player/PlayerContext'

export function PlayerBar() {
  const { track, playing, progress, duration, toggle, seek } = usePlayer()
  if (!track) {
    return <footer className="player player-empty">Тишина. Выберите трек.</footer>
  }

  const ratio = duration ? progress / duration : 0

  return (
    <footer className="player">
      <Cover id={track.id} lit={playing} />
      <div className="player-copy">
        <strong>{track.title}</strong>
        <span>{track.artist || 'неизвестный артист'}</span>
      </div>
      <button type="button" className="play-btn" onClick={toggle} aria-label={playing ? 'Пауза' : 'Играть'}>
        {playing ? '❚❚' : '▶'}
      </button>
      <div className="scrub">
        <span>{fmt(progress)}</span>
        <input
          type="range"
          min={0}
          max={1000}
          value={Math.round(ratio * 1000)}
          onChange={(e) => seek(Number(e.target.value) / 1000)}
        />
        <span>{fmt(duration)}</span>
      </div>
    </footer>
  )
}

function fmt(sec: number) {
  if (!sec || Number.isNaN(sec)) {
    return '0:00'
  }
  const m = Math.floor(sec / 60)
  const s = Math.floor(sec % 60)
    .toString()
    .padStart(2, '0')
  return `${m}:${s}`
}
