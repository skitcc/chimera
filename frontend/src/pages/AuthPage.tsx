import { useEffect, useState } from 'react'
import { Link, useLocation, useNavigate } from 'react-router-dom'
import { ApiError } from '../api/types'
import { api } from '../api/client'
import { useAuth } from '../auth/AuthContext'
import { Eyes } from '../components/Eyes'

export function AuthPage() {
  const { user, login, register, challenge, cancelChallenge, verify, logout } = useAuth()
  const navigate = useNavigate()
  const location = useLocation()
  const changing = location.pathname === '/password-change'
  const [mode, setMode] = useState<'login' | 'register' | 'recovery'>( 'login')
  const [email, setEmail] = useState('')
  const [password, setPassword] = useState('')
  const [newPassword, setNewPassword] = useState('')
  const [code, setCode] = useState('')
  const [name, setName] = useState('')
  const [passwordChallenge, setPasswordChallenge] = useState<string | null>(null)
  const [error, setError] = useState('')
  const [notice, setNotice] = useState('')
  const [busy, setBusy] = useState(false)
  const recovering = mode === 'recovery'
  const pending = changing || recovering ? passwordChallenge : challenge

  useEffect(() => {
    if (user && !changing) navigate('/', { replace: true })
    if (changing && !user) navigate('/auth', { replace: true })
  }, [user, changing, navigate])

  async function submit() {
    setError(''); setNotice(''); setBusy(true)
    try {
      if (pending) {
        if (changing || recovering) {
          await api.passwordVerify(changing ? 'change' : 'recovery', pending, code, newPassword, email)
          setPasswordChallenge(null); setPassword(''); setNewPassword(''); setCode('')
          if (changing) logout()
          setMode('login'); setNotice('Пароль изменён. Войдите с новым паролем и email-кодом.'); navigate('/auth')
        } else await verify(code)
      } else if (changing) {
        setPasswordChallenge((await api.change(password)).challenge_id)
      } else if (recovering) {
        await api.recover(email); setPasswordChallenge('recovery')
        setNotice('Если аккаунт существует, письмо с кодом отправлено.')
      } else if (mode === 'register') await register(email, password, name)
      else await login(email, password)
    } catch (err) { setError(err instanceof ApiError ? err.message : 'Не удалось выполнить запрос') }
    finally { setBusy(false) }
  }

  return <div className="gate">
    <Eyes className="gate-eyes" blink glow={1.3} />
    <div className="gate-card">
      <p className="wordmark">Sound Chimera</p>
      <h1>{changing ? 'Смена пароля' : recovering ? 'Восстановление доступа' : 'Слушает в темноте.'}</h1>
      {!changing && !pending && <div className="tabs">
        <button type="button" onClick={() => setMode('login')}>Вход</button>
        <button type="button" onClick={() => setMode('register')}>Регистрация</button>
        <button type="button" onClick={() => setMode('recovery')}>Забыли пароль?</button>
      </div>}
      <form onSubmit={(e) => {e.preventDefault(); void submit()}}>
        {pending ? <>
          <p>Введите код из письма. Код действует 5 минут.</p>
          <label>Email-код<input value={code} onChange={e => setCode(e.target.value)} inputMode="numeric" autoComplete="one-time-code" pattern="[0-9]{6}" maxLength={6} required /></label>
          {(changing || recovering) && <label>Новый пароль<input type="password" value={newPassword} onChange={e => setNewPassword(e.target.value)} autoComplete="new-password" minLength={8} required /></label>}
        </> : <>
          {mode === 'register' && !changing && <label>Имя<input value={name} onChange={e => setName(e.target.value)} required /></label>}
          {!changing && <label>Email<input type="email" value={email} onChange={e => setEmail(e.target.value)} required /></label>}
          {!recovering && <label>{changing ? 'Текущий пароль' : 'Пароль'}<input type="password" value={password} onChange={e => setPassword(e.target.value)} autoComplete="current-password" minLength={8} required /></label>}
        </>}
        {error && <p className="error" role="alert">{error}</p>}
        {notice && <p role="status">{notice}</p>}
        <button type="submit" className="primary" disabled={busy}>{pending ? 'Подтвердить' : 'Продолжить'}</button>
      </form>
      {pending && <button type="button" className="ghost" disabled={busy} onClick={() => {
        setBusy(true); setError('')
        const resend = changing ? api.change(password) : recovering ? api.recover(email) : login(email,password)
        void resend.then(res => { if (res && 'challenge_id' in res && changing) setPasswordChallenge(res.challenge_id) })
          .catch((err: unknown) => setError(err instanceof ApiError ? err.message : 'Не удалось отправить код'))
          .finally(() => setBusy(false))
      }}>Отправить снова (через 60 секунд)</button>}
      {pending && <button type="button" className="ghost" disabled={busy} onClick={() => {
        cancelChallenge(); setPasswordChallenge(null); setCode(''); setError('')
      }}>Начать заново</button>}
      <Link to="/" className="ghost">Слушать без входа</Link>
    </div>
  </div>
}
