import { useEffect, useId, useRef } from 'react'

type Props = {
  className?: string
  /** Lids are open; false keeps them shut so a parent can stage the reveal. */
  open?: boolean
  /** Brighter glow, used while a track is playing. */
  lit?: boolean
  /** Random two-phase blinks every few seconds. */
  blink?: boolean
  glow?: number
}

// One eye in a 400x160 box, mirrored for the other side.
const EYE = 'M374 58C334 60 295 64 267 74C244 82 244 97 266 103C299 106 340 86 374 58Z'

const BLINK_MS = 560

export function Eyes({ className = '', open = true, lit = false, blink = false, glow = 1 }: Props) {
  const uid = useId()
  const glowId = `eyes-glow-${uid}`
  const svgRef = useRef<SVGSVGElement>(null)

  useEffect(() => {
    if (!blink || !open) {
      return
    }
    const svg = svgRef.current!
    let timer = 0
    const schedule = (delay: number) => {
      timer = window.setTimeout(() => {
        svg.classList.add('blink')
        window.setTimeout(() => svg.classList.remove('blink'), BLINK_MS)
        // Occasionally a double blink, like a real one.
        schedule(Math.random() < 0.2 ? BLINK_MS + 180 : 2800 + Math.random() * 5200)
      }, delay)
    }
    schedule(1800 + Math.random() * 3000)
    return () => window.clearTimeout(timer)
  }, [blink, open])

  const cls = ['eyes', open ? 'open' : 'shut', lit ? 'lit' : '', className].filter(Boolean).join(' ')

  return (
    <svg ref={svgRef} className={cls} viewBox="0 0 400 160" aria-hidden style={{ '--glow': glow } as React.CSSProperties}>
      <defs>
        <filter id={glowId} x="-40%" y="-120%" width="180%" height="340%">
          <feGaussianBlur stdDeviation="7" />
        </filter>
      </defs>
      <g className="lid">
        <g className="halo" filter={`url(#${glowId})`}>
          <path d={EYE} />
          <path d={EYE} transform="matrix(-1 0 0 1 400 0)" />
        </g>
        <path d={EYE} />
        <path d={EYE} transform="matrix(-1 0 0 1 400 0)" />
      </g>
    </svg>
  )
}
