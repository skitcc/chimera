import { Link } from 'react-router-dom'

export function NotFoundPage() {
  return (
    <section className="page">
      <p className="eyebrow">404</p>
      <h1>Такой страницы нет</h1>
      <p className="muted">Этот адрес никуда не ведёт.</p>
      <Link to="/" className="text-btn">
        На ленту
      </Link>
    </section>
  )
}
