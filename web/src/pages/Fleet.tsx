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
  const role = localStorage.getItem('role')

  async function sendCommand(agentID: string, type: string) {
    try {
      await api.createCommand(agentID, type)
      alert(`Command "${type}" queued.`)
    } catch (e) {
      alert(`Error: ${e}`)
    }
  }

  const columns = [
    { key: 'hostname', header: 'Host', render: (r: Agent) => (
      <button onClick={() => navigate(`/hosts/${r.id}`)}
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
    ...(role === 'admin' ? [{
      key: 'actions', header: 'Actions',
      render: (r: Agent) => (
        <div style={{ display: 'flex', gap: 6 }}>
          <Btn onClick={() => sendCommand(r.id, 'scan_now')}>Scan now</Btn>
          <Btn onClick={() => sendCommand(r.id, 'decommission')} danger>Decommission</Btn>
        </div>
      ),
    }] : []),
  ]

  return (
    <>
      <PageHeader title="Fleet" subtitle="All enrolled agents" />
      <Card>
        {isLoading && <div style={{ padding: 24 }}><Spinner /></div>}
        {error && <div style={{ padding: 24 }}><ErrorMsg msg={String(error)} /></div>}
        {data && <Table columns={columns} data={data} keyFn={r => r.id} emptyText="No agents enrolled yet." />}
      </Card>
    </>
  )
}

function Btn({ children, onClick, danger }: { children: React.ReactNode; onClick: () => void; danger?: boolean }) {
  return (
    <button onClick={onClick} style={{
      padding: '2px 8px', fontSize: 12, cursor: 'pointer', borderRadius: 3,
      border: `1px solid ${danger ? 'var(--danger)' : 'var(--border)'}`,
      color: danger ? 'var(--danger)' : 'var(--text-muted)',
      background: 'none',
    }}>
      {children}
    </button>
  )
}
