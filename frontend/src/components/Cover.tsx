type Props = {
  id: string
  title: string
}

export function Cover({ id, title }: Props) {
  const hue = hashHue(id)
  const initial = (title.trim()[0] || '♫').toUpperCase()
  return (
    <div
      className="cover"
      style={{
        background: `conic-gradient(from 210deg at 30% 20%, hsl(${hue} 42% 22%), hsl(${(hue + 40) % 360} 50% 14%), hsl(${(hue + 18) % 360} 28% 8%))`,
      }}
      aria-hidden
    >
      <span>{initial}</span>
    </div>
  )
}

function hashHue(id: string) {
  let n = 0
  for (const ch of id) {
    n = (n * 33 + ch.charCodeAt(0)) >>> 0
  }
  return 12 + (n % 48)
}
