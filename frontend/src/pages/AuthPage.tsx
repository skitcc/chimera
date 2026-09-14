import { useEffect, useState } from 'react'
import { Link, useNavigate } from 'react-router-dom'
import { ApiError } from '../api/types'
import { useAuth } from '../auth/AuthContext'
import { Eyes } from '../components/Eyes'

export function AuthPage() {
  const { user, login, register } = useAuth()
  const navigate = useNavigate()
  const [mode, setMode] = useState<'login' | 'register'>('login')
  const [email, setEmail] = useState('')
  const [password, setPassword] = useState('')
  const [name, setName] = useState('')
  const [error, setError] = useState('')

  useEffect(() => {
    if (user) {
      navigate('/', { replace: true })
    }
  }, [user, navigate])

  return (
    <div className="gate">
      <Eyes className="gate-eyes" blink glow={1.3} />
      <div className="gate-card">
        <p className="wordmark">Sound Chimera</p>
        <h1>Слушает в темноте.</h1>
        <p className="lede">Войдите, чтобы заливать треки и собирать любимое. Ленту можно слушать сразу.</p>
        <div className="tabs">
          <button type="button" className={mode === 'login' ? 'on' : ''} onClick={() => setMode('login')}>
            Вход
          </button>
          <button type="button" className={mode === 'register' ? 'on' : ''} onClick={() => setMode('register')}>
            Регистрация
          </button>
        </div>
        <form
          onSubmit={(e) => {
            e.preventDefault()
            setError('')
            const run = mode === 'login' ? login(email, password) : register(email, password, name)
            void run.catch((err: unknown) => {
              setError(err instanceof ApiError ? err.message : 'не вышло')
            })
          }}
        >
          {mode === 'register' ? (
            <label>
              Имя
              <input value={name} onChange={(e) => setName(e.target.value)} required />
            </label>
          ) : null}
          <label>
            Email
            <input type="email" value={email} onChange={(e) => setEmail(e.target.value)} required />
          </label>
          <label>
            Пароль
            <input type="password" value={password} onChange={(e) => setPassword(e.target.value)} required minLength={8} />
          </label>
          {error ? <p className="error">{error}</p> : null}
          <button type="submit" className="primary">
            {mode === 'login' ? 'Войти' : 'Создать аккаунт'}
          </button>
        </form>
        <Link to="/" className="ghost">
          Слушать без входа
        </Link>
      </div>
    </div>
  )
}
