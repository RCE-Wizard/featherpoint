import { useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import { useNavigate } from 'react-router-dom'
import { api, type CatalogEntry } from '../api'
import { Table, Badge, PageHeader, Card, Spinner, ErrorMsg } from '../components/Table'

export function Catalog() {
  const [q, setQ] = useState('')
  const [search, setSearch] = useState('')
  const navigate = useNavigate()

  const { data, isLoading, error } = useQuery({
    queryKey: ['catalog', search],
    queryFn: () => api.catalogSearch(search),
    enabled: search.length > 0,
  })

  const columns = [
    { key: 'name', header: 'Name', render: (r: CatalogEntry) => (
      <button onClick={() => navigate(`/catalog/${r.id}/hosts`)}
        style={{ color: 'var(--accent)', background: 'none', border: 'none', cursor: 'pointer', padding: 0, fontSize: 14 }}>
        {r.name}
      </button>
    )},
    { key: 'version', header: 'Version', render: (r: CatalogEntry) => <span className="tabular">{r.version ?? '—'}</span> },
    { key: 'publisher', header: 'Publisher', render: (r: CatalogEntry) => <span style={{ color: 'var(--text-muted)' }}>{r.publisher ?? '—'}</span> },
    { key: 'source', header: 'Source', render: (r: CatalogEntry) =>
      <Badge color={r.source === 'running' ? 'var(--accent)' : 'var(--text-muted)'}>{r.source}</Badge>
    },
    { key: 'signed', header: 'Signed', render: (r: CatalogEntry) =>
      r.signed === null ? <span style={{ color: 'var(--text-muted)' }}>—</span>
        : r.signed ? <Badge color="var(--success)">✓</Badge>
        : <Badge color="var(--danger)">✗</Badge>
    },
  ]

  return (
    <>
      <PageHeader title="Software catalog"
        subtitle="Search for a package across the fleet — click a name to see which hosts have it" />
      <div style={{ display: 'flex', gap: 8, marginBottom: 16 }}>
        <input value={q} onChange={e => setQ(e.target.value)}
          onKeyDown={e => e.key === 'Enter' && setSearch(q)}
          placeholder="Search by name…"
          style={{ padding: '6px 10px', background: 'var(--surface)', border: '1px solid var(--border)', color: 'var(--text)', borderRadius: 4, fontSize: 13, width: 320 }} />
        <button onClick={() => setSearch(q)}
          style={{ padding: '6px 14px', background: 'var(--accent)', color: '#fff', border: 'none', borderRadius: 4, cursor: 'pointer' }}>
          Search
        </button>
        {data && <span style={{ marginLeft: 'auto', color: 'var(--text-muted)', fontSize: 13, alignSelf: 'center' }}>
          {data.total} results
        </span>}
      </div>
      <Card>
        {isLoading && <div style={{ padding: 24 }}><Spinner /></div>}
        {error && <div style={{ padding: 24 }}><ErrorMsg msg={String(error)} /></div>}
        {data && <Table columns={columns} data={data.data ?? []} keyFn={r => r.id} />}
        {!search && <div style={{ padding: 24, color: 'var(--text-muted)', textAlign: 'center' }}>Enter a package name to search</div>}
      </Card>
    </>
  )
}
