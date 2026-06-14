import { useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import { api, type UnsignedRow } from '../api'
import { Table, PageHeader, Card, Spinner, ErrorMsg } from '../components/Table'

export function Unsigned() {
  const [offset, setOffset] = useState(0)
  const [q, setQ] = useState('')
  const [search, setSearch] = useState('')

  const { data, isLoading, error } = useQuery({
    queryKey: ['unsigned', offset, search],
    queryFn: () => api.unsigned(offset, search),
  })

  const columns = [
    { key: 'hostname', header: 'Host' },
    { key: 'name', header: 'Binary' },
    { key: 'exe_path', header: 'Path', render: (r: UnsignedRow) =>
      <span style={{ color: 'var(--text-muted)', fontFamily: 'monospace', fontSize: 12 }}>{r.exe_path ?? '—'}</span>
    },
    { key: 'source', header: 'Source', render: (r: UnsignedRow) =>
      <span style={{ color: 'var(--text-muted)' }}>{r.source}</span>
    },
    { key: 'last_seen', header: 'Last seen', render: (r: UnsignedRow) =>
      <span className="tabular">{r.last_seen.slice(0, 19)}</span>
    },
  ]

  return (
    <>
      <PageHeader title="Unsigned binaries"
        subtitle="Running executables with no verified code signature" />
      <div style={{ display: 'flex', gap: 8, marginBottom: 16 }}>
        <input value={q} onChange={e => setQ(e.target.value)}
          onKeyDown={e => e.key === 'Enter' && setSearch(q)}
          placeholder="Filter by host or name…"
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
        {data && <Table columns={columns} data={data.data ?? []} keyFn={r => r.host_id + r.name + r.exe_path} />}
      </Card>
    </>
  )
}
