import { useState } from 'react'
import { useParams, useNavigate } from 'react-router-dom'
import { useQuery, useQueryClient } from '@tanstack/react-query'
import { api, type AgentConfig } from '../api'
import { PageHeader, Card, Spinner, ErrorMsg, Badge } from '../components/Table'

const role = () => localStorage.getItem('role')

export function AgentDetail() {
  const { agentID } = useParams<{ agentID: string }>()
  const navigate = useNavigate()
  const qc = useQueryClient()
  const [msg, setMsg] = useState('')
  const [err, setErr] = useState('')
  const [showConfig, setShowConfig] = useState(false)

  const { data: agent, isLoading, error } = useQuery({
    queryKey: ['agent', agentID],
    queryFn: () => api.agent(agentID!),
  })

  // Config form state — pre-filled with agent's current config
  const [cfg, setCfg] = useState<Partial<AgentConfig>>({})

  async function act(fn: () => Promise<unknown>, label: string) {
    setMsg(''); setErr('')
    try {
      await fn()
      setMsg(`${label} queued.`)
      qc.invalidateQueries({ queryKey: ['agents'] })
      qc.invalidateQueries({ queryKey: ['agent', agentID] })
    } catch (e) {
      setErr(String(e))
    }
  }

  if (isLoading) return <Spinner />
  if (error || !agent) return <ErrorMsg msg={String(error ?? 'Not found')} />

  return (
    <>
      <PageHeader title={agent.hostname ?? agentID!} subtitle={`Agent ${agentID}`} />

      <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: 16, marginBottom: 16 }}>
        <Card style={{ padding: 16 }}>
          <table style={{ width: '100%', borderCollapse: 'collapse' }}>
            <tbody>
              {[
                ['Status', <Badge color={agent.status === 'active' ? 'var(--success)' : 'var(--danger)'}>{agent.status}</Badge>],
                ['OS', agent.os],
                ['IP', agent.primary_ip ?? '—'],
                ['Agent version', agent.agent_version],
                ['Config version', agent.config_version],
                ['Last checkin', agent.last_checkin ? new Date(agent.last_checkin).toLocaleString() : '—'],
                ['Last heartbeat', agent.last_heartbeat ? new Date(agent.last_heartbeat).toLocaleString() : '—'],
              ].map(([k, v]) => (
                <tr key={String(k)} style={{ borderBottom: '1px solid var(--border)' }}>
                  <td style={{ padding: '7px 0', color: 'var(--text-muted)', width: 140, fontSize: 13 }}>{k}</td>
                  <td style={{ padding: '7px 0', fontSize: 13 }}>{v as React.ReactNode}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </Card>

        {role() === 'admin' && (
          <Card style={{ padding: 16 }}>
            <p style={{ margin: '0 0 12px', fontWeight: 600, fontSize: 13 }}>Actions</p>
            <div style={{ display: 'flex', flexDirection: 'column', gap: 8 }}>
              <ActionBtn onClick={() => act(() => api.scanNow(agentID!), 'Scan now')}>
                Trigger scan now
              </ActionBtn>
              <ActionBtn onClick={() => setShowConfig(s => !s)}>
                {showConfig ? 'Cancel config push' : 'Push config update'}
              </ActionBtn>
              <ActionBtn danger onClick={() => {
                if (confirm('Decommission this agent? It will stop reporting.')) {
                  act(() => api.decommission(agentID!), 'Decommission')
                    .then(() => navigate('/fleet'))
                }
              }}>
                Decommission agent
              </ActionBtn>
            </div>
            {msg && <p style={{ color: 'var(--success)', fontSize: 13, marginTop: 10 }}>{msg}</p>}
            {err && <p style={{ color: 'var(--danger)', fontSize: 13, marginTop: 10 }}>{err}</p>}
          </Card>
        )}
      </div>

      {showConfig && role() === 'admin' && (
        <Card style={{ padding: 16, marginBottom: 16 }}>
          <p style={{ margin: '0 0 14px', fontWeight: 600, fontSize: 13 }}>Config push</p>
          <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr 1fr', gap: 12, marginBottom: 14 }}>
            {([
              ['process_interval_s', 'Process interval (s)'],
              ['installed_interval_s', 'Installed interval (s)'],
              ['hash_concurrency', 'Hash concurrency'],
              ['spool_max_bytes', 'Spool max bytes'],
              ['mem_limit_bytes', 'Mem limit bytes'],
            ] as [keyof AgentConfig, string][]).map(([key, label]) => (
              <label key={key} style={{ display: 'flex', flexDirection: 'column', gap: 4 }}>
                <span style={{ fontSize: 11, color: 'var(--text-muted)' }}>{label}</span>
                <input type="number" value={(cfg[key] as number) ?? ''}
                  onChange={e => setCfg(c => ({ ...c, [key]: Number(e.target.value) }))}
                  style={{ padding: '6px 8px', background: 'var(--surface-2)', border: '1px solid var(--border)', color: 'var(--text)', borderRadius: 4, fontSize: 13 }} />
              </label>
            ))}
          </div>
          <button
            onClick={() => act(() => api.pushConfig(agentID!, cfg), 'Config push')}
            style={{ padding: '7px 18px', background: 'var(--accent)', color: '#fff', border: 'none', borderRadius: 4, cursor: 'pointer', fontSize: 13 }}>
            Push config
          </button>
        </Card>
      )}

      <div>
        <button onClick={() => navigate(`/hosts/${agentID}`)}
          style={{ color: 'var(--accent)', background: 'none', border: 'none', cursor: 'pointer', fontSize: 13, padding: 0 }}>
          ← View software inventory for this host
        </button>
      </div>
    </>
  )
}

function ActionBtn({ children, onClick, danger }: { children: React.ReactNode; onClick: () => void; danger?: boolean }) {
  return (
    <button onClick={onClick} style={{
      padding: '7px 14px', cursor: 'pointer', borderRadius: 4, fontSize: 13, textAlign: 'left',
      border: `1px solid ${danger ? 'var(--danger)' : 'var(--border)'}`,
      color: danger ? 'var(--danger)' : 'var(--text)',
      background: 'none',
    }}>
      {children}
    </button>
  )
}
