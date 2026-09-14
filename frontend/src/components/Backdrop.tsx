import { useLocation } from 'react-router-dom'
import { Eyes } from './Eyes'

// The eyes stay fixed in the void and only blink from time to time.
export function Backdrop() {
  const { pathname } = useLocation()
  // The gate has its own pair of eyes; two pairs would be a crowd.
  const quiet = pathname === '/auth'
  return (
    <div className={`backdrop ${quiet ? 'quiet' : ''}`} aria-hidden>
      <Eyes className="backdrop-eyes" blink glow={1.35} />
      <div className="backdrop-grain" />
    </div>
  )
}
