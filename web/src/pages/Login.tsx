import { useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { api } from '../api'

export function Login() {
  const [u, setU] = useState('')
  const [p, setP] = useState('')
  const [err, setErr] = useState('')
  const navigate = useNavigate()

  async function submit(e: React.FormEvent) {
    e.preventDefault()
    setErr('')
    try {
      const res = await api.login(u, p)
      localStorage.setItem('token', res.token)
      localStorage.setItem('role', res.role)
      navigate('/fleet')
    } catch {
      setErr('Invalid credentials')
    }
  }

  const input: React.CSSProperties = {
    display: 'block', width: '100%', padding: '8px 10px',
    background: 'var(--surface-2)', border: '1px solid var(--border)',
    borderRadius: 4, color: 'var(--text)', fontSize: 14, marginTop: 4,
  }

  return (
    <div style={{ minHeight: '100vh', display: 'flex', alignItems: 'center', justifyContent: 'center', background: 'var(--bg)' }}>
      <div style={{ width: 340, background: 'var(--surface)', border: '1px solid var(--border)', borderRadius: 8, padding: 32 }}>
        <h1 style={{ margin: '0 0 8px', fontSize: 18, fontWeight: 700, color: 'var(--accent)' }}>featherpoint</h1>
        <p style={{ margin: '0 0 24px', color: 'var(--text-muted)', fontSize: 13 }}>Software inventory</p>
        <form onSubmit={submit}>
          <label style={{ display: 'block', marginBottom: 14 }}>
            <span style={{ fontSize: 12, color: 'var(--text-muted)' }}>Username</span>
            <input style={input} value={u} onChange={e => setU(e.target.value)} autoFocus />
          </label>
          <label style={{ display: 'block', marginBottom: 20 }}>
            <span style={{ fontSize: 12, color: 'var(--text-muted)' }}>Password</span>
            <input style={input} type="password" value={p} onChange={e => setP(e.target.value)} />
          </label>
          {err && <p style={{ color: 'var(--danger)', fontSize: 13, margin: '0 0 12px' }}>{err}</p>}
          <button type="submit" style={{
            width: '100%', padding: '9px', background: 'var(--accent)', color: '#fff',
            border: 'none', borderRadius: 4, cursor: 'pointer', fontSize: 14, fontWeight: 500,
          }}>
            Sign in
          </button>
        </form>
      </div>
    </div>
  )
}
