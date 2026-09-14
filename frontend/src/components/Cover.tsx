type Props = {
  id: string
  lit?: boolean
}

export function Cover({ id, lit = false }: Props) {
  const n = hash(id)
  const gap = 10 + (n % 6)
  const squat = 0.42 + ((n >> 3) % 12) / 100
  return (
    <div className={`cover ${lit ? 'lit' : ''}`} aria-hidden>
      <span className="cover-eye" style={{ transform: `translateX(-${gap}px) scaleY(${squat})` }} />
      <span className="cover-eye" style={{ transform: `translateX(${gap}px) scaleY(${squat})` }} />
    </div>
  )
}

function hash(id: string) {
  let n = 0
  for (const ch of id) {
    n = (n * 33 + ch.charCodeAt(0)) >>> 0
  }
  return n
}
