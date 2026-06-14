import { useParams } from 'react-router-dom'
import { useQuery } from '@tanstack/react-query'
import { api, type SoftwareHostRow } from '../api'
import { Table, PageHeader, Card, Spinner, ErrorMsg } from '../components/Table'

export function SoftwareHosts() {
  const { catalogID } = useParams<{ catalogID: string }>()

  const { data, isLoading, error } = useQuery({
    queryKey: ['software-hosts', catalogID],
    queryFn: () => api.softwareHosts(catalogID!),
  })

  const columns = [
    { key: 'hostname', header: 'Host' },
    { key: 'os', header: 'OS', render: (r: SoftwareHostRow) => <span style={{ color: 'var(--text-muted)' }}>{r.os}</span> },
    { key: 'source', header: 'Source', render: (r: SoftwareHostRow) => <span style={{ color: 'var(--text-muted)' }}>{r.source}</span> },
    { key: 'last_seen', header: 'Last seen', render: (r: SoftwareHostRow) => <span className="tabular">{r.last_seen.slice(0, 19)}</span> },
  ]

  return (
    <>
      <PageHeader title="Software distribution" subtitle={`Hosts with catalog entry ${catalogID}`} />
      <Card>
        {isLoading && <div style={{ padding: 24 }}><Spinner /></div>}
        {error && <div style={{ padding: 24 }}><ErrorMsg msg={String(error)} /></div>}
        {data && <Table columns={columns} data={data.data ?? []} keyFn={r => r.host_id + r.source}
          emptyText="No hosts found for this catalog entry." />}
      </Card>
    </>
  )
}
