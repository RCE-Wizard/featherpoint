interface Column<T> {
  key: string
  header: string
  render?: (row: T) => React.ReactNode
  style?: React.CSSProperties
}

interface Props<T> {
  columns: Column<T>[]
  data: T[]
  keyFn: (row: T) => string
  emptyText?: string
}

export function Table<T>({ columns, data, keyFn, emptyText = 'No data' }: Props<T>) {
  return (
    <div style={{ overflowX: 'auto' }}>
      <table style={{ width: '100%', borderCollapse: 'collapse' }}>
        <thead>
          <tr>
            {columns.map(c => (
              <th key={c.key} style={{
                textAlign: 'left',
                padding: '6px 12px',
                borderBottom: '1px solid var(--border)',
                color: 'var(--text-muted)',
                fontSize: 11,
                fontWeight: 500,
                letterSpacing: '.06em',
                textTransform: 'uppercase',
                ...c.style,
              }}>
                {c.header}
              </th>
            ))}
          </tr>
        </thead>
        <tbody>
          {data.length === 0 ? (
            <tr>
              <td colSpan={columns.length}
                style={{ padding: '24px 12px', color: 'var(--text-muted)', textAlign: 'center' }}>
                {emptyText}
              </td>
            </tr>
          ) : data.map(row => (
            <tr key={keyFn(row)} style={{ borderBottom: '1px solid var(--border)' }}
              onMouseEnter={e => (e.currentTarget.style.background = 'var(--surface-2)')}
              onMouseLeave={e => (e.currentTarget.style.background = '')}>
              {columns.map(c => (
                <td key={c.key} style={{ padding: '8px 12px', verticalAlign: 'middle', ...c.style }}>
                  {c.render ? c.render(row) : String((row as Record<string, unknown>)[c.key] ?? '')}
                </td>
              ))}
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  )
}

export function Badge({ children, color = 'var(--text-muted)' }: { children: React.ReactNode; color?: string }) {
  return (
    <span style={{
      display: 'inline-block', padding: '1px 6px',
      borderRadius: 3, fontSize: 11, fontWeight: 500,
      border: `1px solid ${color}`, color,
    }}>
      {children}
    </span>
  )
}

export function PageHeader({ title, subtitle }: { title: string; subtitle?: string }) {
  return (
    <div style={{ marginBottom: 24 }}>
      <h1 style={{ margin: 0, fontSize: 18, fontWeight: 600 }}>{title}</h1>
      {subtitle && <p style={{ margin: '4px 0 0', color: 'var(--text-muted)', fontSize: 13 }}>{subtitle}</p>}
    </div>
  )
}

export function Card({ children, style }: { children: React.ReactNode; style?: React.CSSProperties }) {
  return (
    <div style={{
      background: 'var(--surface)',
      border: '1px solid var(--border)',
      borderRadius: 6,
      ...style,
    }}>
      {children}
    </div>
  )
}

export function Spinner() {
  return <span style={{ color: 'var(--text-muted)' }}>Loading…</span>
}

export function ErrorMsg({ msg }: { msg: string }) {
  return <span style={{ color: 'var(--danger)' }}>{msg}</span>
}
