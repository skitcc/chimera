import { useEffect, useId, useRef } from 'react'

type Props = {
  className?: string
  /** Lids are open; false keeps them shut so a parent can stage the reveal. */
  open?: boolean
  /** Brighter glow, used while a track is playing. */
  lit?: boolean
  /** Random two-phase blinks every few seconds. */
  blink?: boolean
  /** Eyes drift toward the pointer and wander when it rests. */
  gaze?: boolean
  glow?: number
}

// One eye in a 400x160 box, mirrored for the other side.
const EYE = 'M372 58C330 60 292 66 266 76C246 84 246 106 268 108C300 108 340 88 372 58Z'

const BLINK_MS = 360

export function Eyes({ className = '', open = true, lit = false, blink = false, gaze = false, glow = 1 }: Props) {
  const uid = useId()
  const clipId = `eyes-clip-${uid}`
  const glowId = `eyes-glow-${uid}`
  const svgRef = useRef<SVGSVGElement>(null)
  const gazeRef = useRef<SVGGElement>(null)

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

  useEffect(() => {
    if (!gaze || window.matchMedia('(prefers-reduced-motion: reduce)').matches) {
      return
    }
    const g = gazeRef.current!
    let tx = 0
    let ty = 0
    let x = 0
    let y = 0
    let vx = 0
    let vy = 0
    let lastMove = 0
    let nextWander = performance.now() + 2500
    let frame = 0

    const onMove = (e: PointerEvent) => {
      tx = (e.clientX / window.innerWidth - 0.5) * 22
      ty = (e.clientY / window.innerHeight - 0.5) * 12
      lastMove = performance.now()
    }

    const tick = (now: number) => {
      // When the pointer has been still for a while the eyes look around on their own.
      if (now - lastMove > 3500 && now > nextWander) {
        tx = (Math.random() - 0.5) * 18
        ty = (Math.random() - 0.5) * 8
        nextWander = now + 1800 + Math.random() * 3200
      }
      // Critically damped spring: quick saccade, soft settle, no rubbery bounce.
      vx += (tx - x) * 0.012 - vx * 0.18
      vy += (ty - y) * 0.012 - vy * 0.18
      x += vx
      y += vy
      g.style.transform = `translate(${x.toFixed(2)}px, ${y.toFixed(2)}px)`
      frame = requestAnimationFrame(tick)
    }

    window.addEventListener('pointermove', onMove)
    frame = requestAnimationFrame(tick)
    return () => {
      window.removeEventListener('pointermove', onMove)
      cancelAnimationFrame(frame)
    }
  }, [gaze])

  const cls = ['eyes', open ? 'open' : 'shut', lit ? 'lit' : '', className].filter(Boolean).join(' ')

  return (
    <svg ref={svgRef} className={cls} viewBox="0 0 400 160" aria-hidden style={{ '--glow': glow } as React.CSSProperties}>
      <defs>
        <clipPath id={clipId}>
          <path d={EYE} />
          <path d={EYE} transform="matrix(-1 0 0 1 400 0)" />
        </clipPath>
        <filter id={glowId} x="-40%" y="-120%" width="180%" height="340%">
          <feGaussianBlur stdDeviation="9" />
        </filter>
      </defs>
      <g ref={gazeRef} className="gaze">
        <g className="lid halo" filter={`url(#${glowId})`}>
          <path d={EYE} />
          <path d={EYE} transform="matrix(-1 0 0 1 400 0)" />
        </g>
        <g clipPath={`url(#${clipId})`}>
          <rect className="lid" x="0" y="52" width="400" height="60" />
        </g>
      </g>
    </svg>
  )
}
