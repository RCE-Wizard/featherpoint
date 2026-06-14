import { useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import { api, type DormantRow } from '../api'
import { Table, PageHeader, Card, Spinner, ErrorMsg } from '../components/Table'

export function Dormant() {
  const [offset, setOffset] = useState(0)
  const [q, setQ] = useState('')
  const [search, setSearch] = useState('')
  const limit = 100

  const { data, isLoading, error } = useQuery({
    queryKey: ['dormant', offset, search],
    queryFn: () => api.dormant(offset, search),
  })

  const columns = [
    { key: 'hostname', header: 'Host' },
    { key: 'name', header: 'Package' },
    { key: 'version', header: 'Version', render: (r: DormantRow) => <span className="tabular">{r.version ?? '—'}</span> },
    { key: 'publisher', header: 'Publisher', render: (r: DormantRow) => <span style={{ color: 'var(--text-muted)' }}>{r.publisher ?? '—'}</span> },
    { key: 'install_location', header: 'Location', render: (r: DormantRow) =>
      <span style={{ color: 'var(--text-muted)', fontFamily: 'monospace', fontSize: 12 }}>{r.install_location ?? '—'}</span>
    },
    { key: 'last_seen', header: 'Last seen', render: (r: DormantRow) =>
      <span className="tabular">{r.last_seen.slice(0, 19)}</span>
    },
  ]

  return (
    <>
      <PageHeader title="Dormant software"
        subtitle="Installed packages not seen running — potential attack surface" />
      <div style={{ display: 'flex', gap: 8, marginBottom: 16 }}>
        <input value={q} onChange={e => setQ(e.target.value)}
          onKeyDown={e => e.key === 'Enter' && setSearch(q)}
          placeholder="Filter…"
          style={{ padding: '6px 10px', background: 'var(--surface)', border: '1px solid var(--border)', color: 'var(--text)', borderRadius: 4, fontSize: 13, width: 280 }} />
        <button onClick={() => { setSearch(q); setOffset(0) }}
          style={{ padding: '6px 14px', background: 'var(--accent)', color: '#fff', border: 'none', borderRadius: 4, cursor: 'pointer' }}>
          Search
        </button>
        {data && <span style={{ marginLeft: 'auto', color: 'var(--text-muted)', fontSize: 13, alignSelf: 'center' }}>
          {data.total} items
        </span>}
      </div>
      <Card>
        {isLoading && <div style={{ padding: 24 }}><Spinner /></div>}
        {error && <div style={{ padding: 24 }}><ErrorMsg msg={String(error)} /></div>}
        {data && <Table columns={columns} data={data.data ?? []} keyFn={r => r.host_id + r.name} />}
      </Card>
      {data && data.total > limit && (
        <div style={{ marginTop: 12, display: 'flex', gap: 8 }}>
          <button onClick={() => setOffset(Math.max(0, offset - limit))} disabled={offset === 0}
            style={{ padding: '4px 12px', cursor: 'pointer', background: 'var(--surface)', border: '1px solid var(--border)', color: 'var(--text)', borderRadius: 4 }}>
            ← Prev
          </button>
          <button onClick={() => setOffset(offset + limit)} disabled={offset + limit >= data.total}
            style={{ padding: '4px 12px', cursor: 'pointer', background: 'var(--surface)', border: '1px solid var(--border)', color: 'var(--text)', borderRadius: 4 }}>
            Next →
          </button>
        </div>
      )}
    </>
  )
}
