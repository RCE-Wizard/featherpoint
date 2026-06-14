import { NavLink, useNavigate } from 'react-router-dom'

const nav = [
  { to: '/fleet', label: 'Fleet' },
  { to: '/reports/unsigned', label: 'Unsigned' },
  { to: '/reports/dormant', label: 'Dormant' },
  { to: '/reports/version-sprawl', label: 'Version sprawl' },
  { to: '/catalog', label: 'Catalog' },
]

export function Layout({ children }: { children: React.ReactNode }) {
  const navigate = useNavigate()
  const role = localStorage.getItem('role')

  function logout() {
    localStorage.removeItem('token')
    localStorage.removeItem('role')
    navigate('/login')
  }

  return (
    <div className="min-h-screen flex flex-col" style={{ background: 'var(--bg)' }}>
      <header style={{ background: 'var(--surface)', borderBottom: '1px solid var(--border)' }}
        className="flex items-center gap-6 px-6 h-12 shrink-0">
        <span style={{ color: 'var(--accent)', fontWeight: 700, letterSpacing: '-.02em' }}>
          featherpoint
        </span>
        <nav className="flex gap-1">
          {nav.map(({ to, label }) => (
            <NavLink key={to} to={to}
              style={({ isActive }) => ({
                padding: '4px 10px',
                borderRadius: 4,
                color: isActive ? 'var(--accent)' : 'var(--text-muted)',
                background: isActive ? 'rgba(99,102,241,.12)' : 'transparent',
                textDecoration: 'none',
                fontSize: 13,
              })}>
              {label}
            </NavLink>
          ))}
        </nav>
        <div className="ml-auto flex items-center gap-3">
          <span style={{ color: 'var(--text-muted)', fontSize: 12 }}>{role}</span>
          <button onClick={logout}
            style={{ color: 'var(--text-muted)', fontSize: 12, cursor: 'pointer', border: 'none', background: 'none' }}>
            sign out
          </button>
        </div>
      </header>
      <main className="flex-1 p-6">{children}</main>
    </div>
  )
}
