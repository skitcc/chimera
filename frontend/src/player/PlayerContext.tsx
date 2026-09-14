import { createContext, useContext, useEffect, useMemo, useRef, useState } from 'react'
import type { ReactNode } from 'react'
import { streamUrl } from '../api/client'
import type { Track } from '../api/types'

type PlayerState = {
  track: Track | null
  playing: boolean
  progress: number
  duration: number
  play: (track: Track) => void
  toggle: () => void
  seek: (ratio: number) => void
}

const PlayerContext = createContext<PlayerState | null>(null)

export function PlayerProvider({ children }: { children: ReactNode }) {
  const audioRef = useRef(new Audio())
  const [track, setTrack] = useState<Track | null>(null)
  const [playing, setPlaying] = useState(false)
  const [progress, setProgress] = useState(0)
  const [duration, setDuration] = useState(0)

  useEffect(() => {
    const audio = audioRef.current
    const onTime = () => setProgress(audio.currentTime)
    const onMeta = () => setDuration(audio.duration || 0)
    const onEnd = () => setPlaying(false)
    audio.addEventListener('timeupdate', onTime)
    audio.addEventListener('loadedmetadata', onMeta)
    audio.addEventListener('ended', onEnd)
    return () => {
      audio.pause()
      audio.removeEventListener('timeupdate', onTime)
      audio.removeEventListener('loadedmetadata', onMeta)
      audio.removeEventListener('ended', onEnd)
    }
  }, [])

  const value = useMemo<PlayerState>(
    () => ({
      track,
      playing,
      progress,
      duration,
      play: (next) => {
        const audio = audioRef.current
        if (track?.id === next.id) {
          if (audio.paused) {
            void audio.play()
            setPlaying(true)
            return
          }
          audio.pause()
          setPlaying(false)
          return
        }
        audio.src = streamUrl(next.id)
        void audio.play()
        setTrack(next)
        setPlaying(true)
      },
      toggle: () => {
        const audio = audioRef.current
        if (!track) {
          return
        }
        if (audio.paused) {
          void audio.play()
          setPlaying(true)
          return
        }
        audio.pause()
        setPlaying(false)
      },
      seek: (ratio) => {
        const audio = audioRef.current
        if (!duration) {
          return
        }
        audio.currentTime = ratio * duration
      },
    }),
    [track, playing, progress, duration],
  )

  return <PlayerContext.Provider value={value}>{children}</PlayerContext.Provider>
}

export function usePlayer() {
  const ctx = useContext(PlayerContext)
  if (!ctx) {
    throw new Error('usePlayer outside provider')
  }
  return ctx
}
