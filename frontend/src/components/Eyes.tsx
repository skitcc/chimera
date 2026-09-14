import { useEffect, useId, useRef } from 'react'

type Props = {
  className?: string
  /** Lids are open; false keeps them shut so a parent can stage the reveal. */
  open?: boolean
  /** Brighter glow, used while a track is playing. */
  lit?: boolean
  /** Random blinks every few seconds. */
  blink?: boolean
  /** Eyes drift toward the pointer. */
  gaze?: boolean
  /** 0..1, how far the lids can open. Lets covers look squinted. */
  squint?: number
  glow?: number
}

// One eye in a 400x160 box, mirrored for the other side.
const EYE = 'M372 58C330 60 292 66 266 76C246 84 246 106 268 108C300 108 340 88 372 58Z'

export function Eyes({
  className = '',
  open = true,
  lit = false,
  blink = false,
  gaze = false,
  squint = 1,
  glow = 1,
}: Props) {
  const uid = useId()
  const clipId = `eyes-clip-${uid}`
  const glowId = `eyes-glow-${uid}`
  const svgRef = useRef<SVGSVGElement>(null)
  const gazeRef = useRef<SVGGElement>(null)

  useEffect(() => {
    if (!blink) {
      return
    }
    const svg = svgRef.current!
    let timer = 0
    const schedule = () => {
      timer = window.setTimeout(() => {
        svg.classList.add('blink')
        window.setTimeout(() => svg.classList.remove('blink'), 140)
        schedule()
      }, 3200 + Math.random() * 5000)
    }
    schedule()
    return () => window.clearTimeout(timer)
  }, [blink])

  useEffect(() => {
    if (!gaze || window.matchMedia('(prefers-reduced-motion: reduce)').matches) {
      return
    }
    const g = gazeRef.current!
    let tx = 0
    let ty = 0
    let x = 0
    let y = 0
    let frame = 0
    const onMove = (e: PointerEvent) => {
      tx = (e.clientX / window.innerWidth - 0.5) * 22
      ty = (e.clientY / window.innerHeight - 0.5) * 12
    }
    const tick = () => {
      x += (tx - x) * 0.06
      y += (ty - y) * 0.06
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
    <svg
      ref={svgRef}
      className={cls}
      viewBox="0 0 400 160"
      aria-hidden
      style={{ '--squint': squint, '--glow': glow } as React.CSSProperties}
    >
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
