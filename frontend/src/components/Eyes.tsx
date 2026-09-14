import eyes from '../assets/eyes.png'

type Props = {
  className?: string
  lit?: boolean
}

export function Eyes({ className = '', lit = false }: Props) {
  return (
    <img
      src={eyes}
      alt=""
      className={`eyes ${lit ? 'lit' : ''} ${className}`.trim()}
      draggable={false}
    />
  )
}
