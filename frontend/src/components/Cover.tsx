type Props = {
  id: string
  lit?: boolean
}

const BARS = 9

// A small waveform seeded by the track id: every track gets its own silhouette.
export function Cover({ id, lit = false }: Props) {
  let n = hash(id)
  const bars = Array.from({ length: BARS }, (_, i) => {
    n = (n * 1103515245 + 12345) >>> 0
    return { h: 0.25 + (n % 60) / 100, i }
  })
  return (
    <div className={`cover ${lit ? 'lit' : ''}`} aria-hidden>
      <span className="wave">
        {bars.map((b) => (
          <i key={b.i} style={{ '--h': b.h, '--i': b.i } as React.CSSProperties} />
        ))}
      </span>
    </div>
  )
}

function hash(id: string) {
  let n = 2166136261
  for (const ch of id) {
    n = ((n ^ ch.charCodeAt(0)) * 16777619) >>> 0
  }
  return n
}
