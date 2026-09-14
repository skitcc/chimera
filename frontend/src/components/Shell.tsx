import { NavLink, Outlet, useNavigate } from 'react-router-dom'
import { useState } from 'react'
import { useAuth } from '../auth/AuthContext'
import { usePlayer } from '../player/PlayerContext'
import { Eyes } from './Eyes'
import { PlayerBar } from './PlayerBar'

export function Shell() {
  const { user, logout } = useAuth()
  const { playing } = usePlayer()
  const navigate = useNavigate()
  const [q, setQ] = useState('')

  return (
    <div className="app">
      <aside className="rail">
        <NavLink to="/" className="mark">
          <Eyes lit={playing} className="mark-eyes" />
          <span>Sound Chimera</span>
        </NavLink>
        <nav>
          <NavLink to="/" end>
            Лента
          </NavLink>
          <NavLink to="/library">Мои треки</NavLink>
          <NavLink to="/likes">Любимое</NavLink>
          <NavLink to="/upload">Загрузить</NavLink>
        </nav>
        <div className="rail-foot">
          {user ? (
            <>
              <p className="who">{user.name || user.email}</p>
              <button type="button" className="text-btn" onClick={logout}>
                Выйти
              </button>
            </>
          ) : (
            <NavLink to="/auth" className="text-btn">
              Войти
            </NavLink>
          )}
        </div>
      </aside>
      <main className="stage">
        <form
          className="seek"
          onSubmit={(e) => {
            e.preventDefault()
            const artist = q.trim()
            if (artist) {
              navigate(`/artist?q=${encodeURIComponent(artist)}`)
            }
          }}
        >
          <input
            value={q}
            onChange={(e) => setQ(e.target.value)}
            placeholder="Артист"
            aria-label="Поиск по артисту"
          />
        </form>
        <Outlet />
      </main>
      <PlayerBar />
    </div>
  )
}
