import { useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import { api } from '../api'
import { Table, PageHeader, Card, Spinner, ErrorMsg } from '../components/Table'

export function VersionSprawl() {
  const [q, setQ] = useState('')
  const [name, setName] = useState('')

  const { data, isLoading, error } = useQuery({
    queryKey: ['version-sprawl', name],
    queryFn: () => api.versionSprawl(name),
    enabled: name.length > 0,
  })

  return (
    <>
      <PageHeader title="Version sprawl"
        subtitle="See how many hosts run different versions of a package" />
      <div style={{ display: 'flex', gap: 8, marginBottom: 24 }}>
        <input value={q} onChange={e => setQ(e.target.value)}
          onKeyDown={e => e.key === 'Enter' && setName(q)}
          placeholder="Package name (exact)…"
          style={{ padding: '6px 10px', background: 'var(--surface)', border: '1px solid var(--border)', color: 'var(--text)', borderRadius: 4, fontSize: 13, width: 320 }} />
        <button onClick={() => setName(q)}
          style={{ padding: '6px 14px', background: 'var(--accent)', color: '#fff', border: 'none', borderRadius: 4, cursor: 'pointer' }}>
          Look up
        </button>
      </div>

      {isLoading && <Spinner />}
      {error && <ErrorMsg msg={String(error)} />}

      {data && (
        <Card>
          <div style={{ padding: '12px 16px', borderBottom: '1px solid var(--border)', fontSize: 13, color: 'var(--text-muted)' }}>
            {data.length} version{data.length !== 1 ? 's' : ''} of <strong style={{ color: 'var(--text)' }}>{name}</strong>
          </div>
          <Table
            columns={[
              { key: 'version', header: 'Version', render: r => <span className="tabular">{r.version ?? '(unversioned)'}</span> },
              {
                key: 'host_count', header: 'Hosts', render: r => {
                  const max = Math.max(...data.map(x => x.host_count))
                  const pct = max > 0 ? (r.host_count / max) * 100 : 0
                  return (
                    <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
                      <div style={{ width: 120, height: 6, background: 'var(--border)', borderRadius: 3, overflow: 'hidden' }}>
                        <div style={{ width: `${pct}%`, height: '100%', background: 'var(--accent)', borderRadius: 3 }} />
                      </div>
                      <span className="tabular">{r.host_count}</span>
                    </div>
                  )
                }
              },
            ]}
            data={data}
            keyFn={r => r.version ?? 'null'}
          />
        </Card>
      )}
    </>
  )
}
