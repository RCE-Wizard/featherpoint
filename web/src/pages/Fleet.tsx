import { useQuery } from '@tanstack/react-query'
import { useNavigate } from 'react-router-dom'
import { api, type Agent } from '../api'
import { Table, Badge, PageHeader, Card, Spinner, ErrorMsg } from '../components/Table'

function statusColor(s: string) {
  return s === 'active' ? 'var(--success)' : 'var(--danger)'
}

function ago(ts: string | null) {
  if (!ts) return '—'
  const diff = Date.now() - new Date(ts).getTime()
  const m = Math.floor(diff / 60000)
  if (m < 1) return 'just now'
  if (m < 60) return `${m}m ago`
  const h = Math.floor(m / 60)
  if (h < 24) return `${h}h ago`
  return `${Math.floor(h / 24)}d ago`
}

export function Fleet() {
  const navigate = useNavigate()
  const { data, isLoading, error } = useQuery({ queryKey: ['agents'], queryFn: api.agents })

  const columns = [
    { key: 'hostname', header: 'Host', render: (r: Agent) => (
      <button onClick={() => navigate(`/agents/${r.id}`)}
        style={{ color: 'var(--accent)', background: 'none', border: 'none', cursor: 'pointer', padding: 0, fontSize: 14 }}>
        {r.hostname ?? '—'}
      </button>
    )},
    { key: 'os', header: 'OS', render: (r: Agent) => <span style={{ color: 'var(--text-muted)' }}>{r.os}</span> },
    { key: 'primary_ip', header: 'IP', render: (r: Agent) => <span className="tabular">{r.primary_ip ?? '—'}</span> },
    { key: 'agent_version', header: 'Version', render: (r: Agent) => <span className="tabular">{r.agent_version}</span> },
    { key: 'status', header: 'Status', render: (r: Agent) => <Badge color={statusColor(r.status)}>{r.status}</Badge> },
    { key: 'last_checkin', header: 'Last checkin', render: (r: Agent) => <span className="tabular">{ago(r.last_checkin)}</span> },
    { key: 'last_heartbeat', header: 'Last heartbeat', render: (r: Agent) => <span className="tabular">{ago(r.last_heartbeat)}</span> },
  ]

  return (
    <>
      <PageHeader title="Fleet" subtitle="Click a host to manage it or view its software" />
      <Card>
        {isLoading && <div style={{ padding: 24 }}><Spinner /></div>}
        {error && <div style={{ padding: 24 }}><ErrorMsg msg={String(error)} /></div>}
        {data && <Table columns={columns} data={data} keyFn={r => r.id} emptyText="No agents enrolled yet." />}
      </Card>
    </>
  )
}
