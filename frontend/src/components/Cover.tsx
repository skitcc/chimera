import { Eyes } from './Eyes'

type Props = {
  id: string
  lit?: boolean
}

// Every track gets its own squint, so a list reads as a crowd rather than copies.
export function Cover({ id, lit = false }: Props) {
  const n = hash(id)
  const squint = 0.55 + (n % 46) / 100
  return (
    <div className={`cover ${lit ? 'lit' : ''}`} aria-hidden>
      <Eyes squint={lit ? 1 : squint} lit={lit} glow={0.7} />
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
