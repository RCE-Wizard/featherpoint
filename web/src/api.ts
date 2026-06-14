const BASE = '/api'

function getToken() {
  return localStorage.getItem('token') ?? ''
}

function authHeaders(): HeadersInit {
  return { Authorization: `Bearer ${getToken()}`, 'Content-Type': 'application/json' }
}

async function get<T>(path: string): Promise<T> {
  const res = await fetch(BASE + path, { headers: authHeaders() })
  if (!res.ok) throw new Error(`${res.status} ${await res.text()}`)
  return res.json()
}

async function post<T>(path: string, body: unknown): Promise<T> {
  const res = await fetch(BASE + path, { method: 'POST', headers: authHeaders(), body: JSON.stringify(body) })
  if (!res.ok) throw new Error(`${res.status} ${await res.text()}`)
  return res.json()
}

export interface Paged<T> { data: T[]; total: number }

export const api = {
  login: (username: string, password: string) =>
    post<{ token: string; role: string }>('/login', { username, password }),

  agents: () => get<Agent[]>('/agents'),

  hostSoftware: (hostID: string, source?: string, offset = 0, limit = 100) =>
    get<Paged<HostSoftwareRow>>(`/hosts/${hostID}/software?source=${source ?? ''}&offset=${offset}&limit=${limit}`),

  softwareHosts: (catalogID: string, offset = 0) =>
    get<Paged<SoftwareHostRow>>(`/catalog/${catalogID}/hosts?offset=${offset}`),

  versionSprawl: (name: string) =>
    get<VersionSprawlRow[]>(`/reports/version-sprawl?name=${encodeURIComponent(name)}`),

  unsigned: (offset = 0, q = '') =>
    get<Paged<UnsignedRow>>(`/reports/unsigned?offset=${offset}&q=${encodeURIComponent(q)}`),

  dormant: (offset = 0, q = '') =>
    get<Paged<DormantRow>>(`/reports/dormant?offset=${offset}&q=${encodeURIComponent(q)}`),

  catalogSearch: (q: string) =>
    get<Paged<CatalogEntry>>(`/catalog?q=${encodeURIComponent(q)}`),

  createCommand: (agentID: string, type: string, payload: Record<string, unknown> = {}) =>
    post<{ id: string }>(`/agents/${agentID}/commands`, { type, payload }),
}

// --- Types ---
export interface Agent {
  id: string
  hostname: string
  os: string
  primary_ip: string | null
  agent_version: string
  status: string
  last_checkin: string | null
  last_heartbeat: string | null
  config_version: number
}

export interface HostSoftwareRow {
  catalog_id: string
  name: string
  publisher: string | null
  version: string | null
  source: string
  signed: boolean | null
  signer: string | null
  exe_path: string | null
  install_location: string | null
  owning_user: string | null
  is_running: boolean
  last_seen: string
}

export interface SoftwareHostRow {
  host_id: string
  hostname: string
  os: string
  source: string
  last_seen: string
}

export interface VersionSprawlRow {
  version: string | null
  host_count: number
}

export interface UnsignedRow {
  host_id: string
  hostname: string
  name: string
  exe_path: string | null
  source: string
  last_seen: string
}

export interface DormantRow {
  host_id: string
  hostname: string
  name: string
  version: string | null
  publisher: string | null
  install_location: string | null
  last_seen: string
}

export interface CatalogEntry {
  id: string
  name: string
  publisher: string | null
  version: string | null
  source: string
  signed: boolean | null
  sha256: string | null
}
