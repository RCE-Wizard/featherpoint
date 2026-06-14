import { useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import { api, type AuditRow } from '../api'
import { Table, PageHeader, Card, Spinner, ErrorMsg } from '../components/Table'

export function AuditLog() {
  const [offset, setOffset] = useState(0)
  const limit = 100

  const { data, isLoading, error } = useQuery({
    queryKey: ['audit', offset],
    queryFn: () => api.auditLog(offset),
  })

  const columns = [
    {
      key: 'at', header: 'Time',
      render: (r: AuditRow) => (
        <span className="tabular" style={{ color: 'var(--text-muted)', fontSize: 12 }}>
          {new Date(r.at).toLocaleString()}
        </span>
      ),
    },
    { key: 'actor', header: 'Actor', render: (r: AuditRow) => <span style={{ color: 'var(--accent)' }}>{r.actor}</span> },
    { key: 'action', header: 'Action', render: (r: AuditRow) => <code style={{ fontSize: 12 }}>{r.action}</code> },
    { key: 'target', header: 'Target', render: (r: AuditRow) => <span style={{ color: 'var(--text-muted)', fontSize: 12 }}>{r.target || '—'}</span> },
    {
      key: 'detail', header: 'Detail',
      render: (r: AuditRow) => r.detail && Object.keys(r.detail).length > 0
        ? <code style={{ fontSize: 11, color: 'var(--text-muted)' }}>{JSON.stringify(r.detail)}</code>
        : <span style={{ color: 'var(--text-muted)' }}>—</span>,
    },
  ]

  return (
    <>
      <PageHeader title="Audit log" subtitle="All administrative actions" />
      {data && (
        <p style={{ color: 'var(--text-muted)', fontSize: 13, margin: '0 0 12px' }}>
          {data.total} total entries
        </p>
      )}
      <Card>
        {isLoading && <div style={{ padding: 24 }}><Spinner /></div>}
        {error && <div style={{ padding: 24 }}><ErrorMsg msg={String(error)} /></div>}
        {data && <Table columns={columns} data={data.data ?? []} keyFn={r => String(r.id)} emptyText="No audit entries yet." />}
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
