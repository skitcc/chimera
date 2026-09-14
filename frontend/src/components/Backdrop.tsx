import { useLocation } from 'react-router-dom'
import { usePlayer } from '../player/PlayerContext'
import { Eyes } from './Eyes'

// Something is always watching from behind the UI. It wakes up when music plays.
export function Backdrop() {
  const { playing } = usePlayer()
  const { pathname } = useLocation()
  // The gate has its own pair of eyes; two pairs would be a crowd.
  const quiet = pathname === '/auth'
  return (
    <div className={`backdrop ${playing ? 'awake' : ''} ${quiet ? 'quiet' : ''}`} aria-hidden>
      <div className="backdrop-glow" />
      <Eyes className="backdrop-eyes" lit={playing} blink gaze glow={playing ? 1.6 : 0.9} />
      <div className="backdrop-grain" />
    </div>
  )
}
