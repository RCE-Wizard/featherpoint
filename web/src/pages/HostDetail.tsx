import { useState } from 'react'
import { useParams } from 'react-router-dom'
import { useQuery } from '@tanstack/react-query'
import { api, type HostSoftwareRow } from '../api'
import { Table, Badge, PageHeader, Card, Spinner, ErrorMsg } from '../components/Table'

export function HostDetail() {
  const { hostID } = useParams<{ hostID: string }>()
  const [source, setSource] = useState('')
  const [offset, setOffset] = useState(0)
  const limit = 100

  const { data, isLoading, error } = useQuery({
    queryKey: ['host-software', hostID, source, offset],
    queryFn: () => api.hostSoftware(hostID!, source, offset, limit),
  })

  const columns = [
    { key: 'name', header: 'Name' },
    { key: 'version', header: 'Version', render: (r: HostSoftwareRow) => <span className="tabular">{r.version ?? '—'}</span> },
    { key: 'publisher', header: 'Publisher', render: (r: HostSoftwareRow) => <span style={{ color: 'var(--text-muted)' }}>{r.publisher ?? '—'}</span> },
    { key: 'source', header: 'Source', render: (r: HostSoftwareRow) =>
      <Badge color={r.source === 'running' ? 'var(--accent)' : 'var(--text-muted)'}>{r.source}</Badge>
    },
    { key: 'signed', header: 'Signed', render: (r: HostSoftwareRow) =>
      r.signed === null ? <span style={{ color: 'var(--text-muted)' }}>—</span>
        : r.signed ? <Badge color="var(--success)">signed</Badge>
        : <Badge color="var(--danger)">unsigned</Badge>
    },
    { key: 'owning_user', header: 'User', render: (r: HostSoftwareRow) => <span style={{ color: 'var(--text-muted)' }}>{r.owning_user ?? '—'}</span> },
    { key: 'last_seen', header: 'Last seen', render: (r: HostSoftwareRow) => <span className="tabular">{r.last_seen.slice(0, 19)}</span> },
  ]

  return (
    <>
      <PageHeader title={`Host: ${hostID}`} subtitle="Installed and running software" />
      <div style={{ display: 'flex', gap: 8, marginBottom: 16 }}>
        {(['', 'running', 'installed'] as const).map(s => (
          <button key={s} onClick={() => { setSource(s); setOffset(0) }} style={{
            padding: '4px 12px', borderRadius: 4, cursor: 'pointer', fontSize: 13,
            background: source === s ? 'var(--accent)' : 'var(--surface)',
            color: source === s ? '#fff' : 'var(--text-muted)',
            border: `1px solid ${source === s ? 'var(--accent)' : 'var(--border)'}`,
          }}>
            {s === '' ? 'All' : s}
          </button>
        ))}
        {data && <span style={{ marginLeft: 'auto', color: 'var(--text-muted)', fontSize: 13, alignSelf: 'center' }}>
          {data.total} items
        </span>}
      </div>
      <Card>
        {isLoading && <div style={{ padding: 24 }}><Spinner /></div>}
        {error && <div style={{ padding: 24 }}><ErrorMsg msg={String(error)} /></div>}
        {data && <Table columns={columns} data={data.data ?? []} keyFn={r => r.catalog_id + r.source} />}
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
