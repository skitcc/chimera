import { useEffect, useState } from 'react'
import { Eyes } from './Eyes'

type Phase = 'dark' | 'awake' | 'brand' | 'gone'

// Cold open: silence, then the eyes open in the void and the name surfaces.
export function Splash({ onDone }: { onDone: () => void }) {
  const [phase, setPhase] = useState<Phase>('dark')

  useEffect(() => {
    const reduce = window.matchMedia('(prefers-reduced-motion: reduce)').matches
    const steps: Array<[Phase, number]> = reduce
      ? [
          ['brand', 100],
          ['gone', 900],
        ]
      : [
          ['awake', 500],
          ['brand', 1700],
          ['gone', 3300],
        ]
    const timers = steps.map(([next, at]) => window.setTimeout(() => setPhase(next), at))
    return () => timers.forEach(window.clearTimeout)
  }, [])

  useEffect(() => {
    if (phase !== 'gone') {
      return
    }
    const t = window.setTimeout(onDone, 900)
    return () => window.clearTimeout(t)
  }, [phase, onDone])

  return (
    <div className={`splash ${phase}`} onClick={() => setPhase('gone')} role="presentation">
      <div className="splash-rings">
        <span />
        <span />
        <span />
      </div>
      <Eyes className="splash-eyes" open={phase !== 'dark'} gaze glow={1.4} />
      <p className="splash-word">
        {'SOUND CHIMERA'.split('').map((ch, i) => (
          <span key={i} style={{ '--i': i } as React.CSSProperties}>
            {ch === ' ' ? '\u00a0' : ch}
          </span>
        ))}
      </p>
    </div>
  )
}
