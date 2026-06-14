# CLAUDE.md — Software Inventory Agent (MVP)

Instruction set for building the MVP. Read this top to bottom before writing code.
Build in the phase order in §11. Each phase must run end to end before starting the next.

> Replace the placeholder Go module path `github.com/ORG/swinv` and the web package
> name `swinv-web` with the real names before phase 1.

---

## 1. Goal

A lightweight cross-platform agent collects **running processes** and **installed
software** from each host and forwards changes to a central Go service, which stores
them in PostgreSQL. A React web app reports on the fleet and manages the agents.

Endpoints in scope: desktop/server OSes only — **Linux, Windows, macOS**.
Out of scope: mobile, network hardware.

The agent must be cheap: minimal CPU and memory, small static binary, low network volume.

---

## 2. MVP scope

**In scope**
- Single static Go agent binary per OS, runs as a native service.
- Two collection loops: running processes (frequent) + installed software (slow cadence).
- Footprint controls: hash caching, delta sending, store-and-forward spool.
- Outbound-only communication over mTLS. Agent polls a command queue for management.
- Go ingestion/API service: enroll, ingest, checkin, heartbeat. Synchronous Postgres writes.
- PostgreSQL with a catalog-centric schema (software deduped, host/agent split).
- React SPA (dark theme) for reports + basic fleet management, with auth and basic RBAC.

**Explicitly deferred (do NOT build now — see §12 for why each is safe to add later)**
- Durable message queue (Kafka/NATS/etc.) — MVP writes synchronously.
- ClickHouse / separate history store — Postgres holds everything for MVP.
- Backend-for-Frontend layer — SPA talks straight to the API.
- CVE enrichment, ServiceNow CMDB sync, agent self-update.

Target scale: up to ~10,000 endpoints. At this size synchronous Postgres writes with
batched upserts are comfortably fine. Do not add a queue or second datastore for MVP.

---

## 3. Architecture (MVP)

```
Endpoint fleet (Linux/Win/macOS)
  agent: collect -> hash cache -> deltas -> spool -> outbound mTLS client
        |
        v  (outbound only, mTLS, deltas)
Go API / ingestion service  (stateless, behind LB)
  endpoints: /enroll /ingest /checkin /heartbeat
        |
        v  (synchronous, idempotent writes)
PostgreSQL  (catalog + current state + commands + audit)
        ^
        |  (queries / REST)
React SPA  (dark theme: reports + fleet management)
```

The data path is strictly one-directional up to Postgres. **Agents never accept inbound
connections.** Management works by the agent polling `/checkin` and executing queued
commands. This is the most important property of the design — preserve it.

---

## 4. Tech stack

- **Agent + API:** Go 1.22+. Static builds, `CGO_ENABLED=0` where possible.
- **Cross-platform process/host facts:** `github.com/shirou/gopsutil/v3`.
- **Service wrapper:** `github.com/kardianos/service` (systemd / Windows Service / launchd).
- **Local spool:** `go.etcd.io/bbolt` (embedded key/value, no external dep).
- **OS-native bits:** `golang.org/x/sys/windows` for Windows specifics.
- **API HTTP:** stdlib `net/http` + `chi` router. JSON over HTTP/1.1, gzip request bodies.
- **DB access:** `pgx/v5` (`jackc/pgx`). Plain SQL + `golang-migrate` for migrations. No ORM.
- **Web:** React + TypeScript + Vite. Tailwind CSS + shadcn/ui-style primitives.
- **Web data fetching:** TanStack Query. Routing: React Router.

Keep dependencies minimal, especially in the agent. Every agent dependency is shipped to
every endpoint — justify each one.

---

## 5. Repo structure (monorepo)

```
swinv/
  go.mod                      # module github.com/ORG/swinv
  cmd/
    agent/main.go             # builds the agent binary
    server/main.go            # builds the API/ingestion service
  internal/
    agent/
      collect/                # process + installed collectors, per-OS files
      hashcache/              # exe hash cache keyed on path,size,mtime,inode
      delta/                  # diff last-sent state -> deltas
      spool/                  # bbolt store-and-forward queue
      transport/             # outbound mTLS client, retry/backoff
      service/                # kardianos service lifecycle
      config/                 # agent config + on-disk state
    server/
      api/                    # http handlers: enroll, ingest, checkin, heartbeat
      store/                  # pgx queries, upserts, idempotency
      auth/                   # mTLS verification, web auth + RBAC
    proto/                    # shared wire types (Go structs + JSON tags) — SINGLE SOURCE OF TRUTH
  migrations/                 # golang-migrate SQL files
  web/                        # React/Vite app (see §10)
  deploy/                     # Dockerfiles, compose for local dev
  CLAUDE.md
```

`internal/proto` holds the wire contract structs. Both agent and server import them.
The web app gets a matching TypeScript types file generated or hand-mirrored from it.

---

## 6. Wire contract (the linchpin — design carefully, change rarely)

All payloads carry version + identity fields so the protocol can evolve without breaking
old agents. Every ingest is **idempotent** so spool replays after an outage never
double-count.

### Common envelope (every request body)
```jsonc
{
  "schema_version": 1,        // bump only on breaking wire changes
  "agent_version": "0.1.0",
  "agent_id": "uuid",         // omitted on /enroll
  "sent_at": "RFC3339"
}
```

### POST /v1/enroll  (no mTLS yet; bearer enrollment token)
Request:
```jsonc
{
  "enrollment_token": "string",
  "agent_version": "0.1.0",
  "host_facts": {
    "hostname": "string",
    "fqdn": "string",
    "os": "linux|windows|darwin",
    "os_version": "string",
    "serial_number": "string|null",   // hardware serial if available
    "mac_addresses": ["aa:bb:.."],    // sorted, deduped
    "primary_ip": "string|null"
  },
  "csr_pem": "string"          // agent generates keypair, sends CSR
}
```
Response: `{ "agent_id": "uuid", "client_cert_pem": "string", "ca_pem": "string" }`

Server resolves/creates the **host** via the correlation waterfall (§7), creates an
**agent** row bound to that host, signs the CSR, returns the client cert. All later calls
use mTLS with that cert.

### POST /v1/ingest  (mTLS)
```jsonc
{
  "...envelope...": "",
  "batch_id": "uuid",          // agent-generated; server dedupes on (agent_id, batch_id)
  "collected_at": "RFC3339",
  "full_snapshot": false,      // true = authoritative full state for that source
  "running": [ SoftwareDelta ],
  "installed": [ SoftwareDelta ]
}
```
`SoftwareDelta`:
```jsonc
{
  "op": "upsert|remove",
  "source": "running|installed",
  "name": "string",
  "publisher": "string|null",
  "version": "string|null",
  "sha256": "hex|null",        // present for running exes; often null for installed
  "signed": true,
  "signer": "string|null",
  "arch": "x64|arm64|...|null",
  // host-specific observation fields:
  "exe_path": "string|null",       // running
  "install_location": "string|null", // installed
  "owning_user": "string|null"     // running
}
```
Response: `{ "accepted": true, "next_full_snapshot_after": "RFC3339|null" }`

Idempotency rule: if `(agent_id, batch_id)` was already processed, return success without
re-applying. A `full_snapshot:true` payload replaces current state for that `source`
(reconcile: remove host_software rows for that source not present in the snapshot).

### POST /v1/checkin  (mTLS) — the management channel
Request: envelope + `{ "config_version": 7 }`.
Response:
```jsonc
{
  "commands": [ { "id":"uuid", "type":"config_update|scan_now|decommission", "payload":{} } ],
  "config": { "...": "" },     // present if server's config_version > agent's
  "config_version": 8
}
```
Agent executes commands, then POSTs acks (reuse checkin with `acked_command_ids`, or a
small `/v1/ack`). Keep commands idempotent by `id`.

### POST /v1/heartbeat  (mTLS)
Envelope + `{ "metrics": { "rss_bytes":N, "cpu_pct":N, "uptime_s":N } }` -> `{ "ok": true }`.
Lightweight liveness + agent resource telemetry. May be merged into checkin later.

**Versioning discipline:** never repurpose a field. Add new fields as optional. Bump
`schema_version` only for breaking changes and keep the server able to read the previous
version during rollout.

---

## 7. PostgreSQL schema

Catalog-centric: **software identity is stored once** in `software_catalog`; hosts
reference it. Hosts and agents are separate so an agent can be reinstalled without losing
host identity (this is also what a future CMDB sync correlates on).

```sql
-- hosts: durable machine identity
CREATE TABLE hosts (
  id            uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  hostname      text NOT NULL,
  fqdn          text,
  os            text NOT NULL,           -- linux|windows|darwin
  os_version    text,
  serial_number text,
  mac_addresses jsonb NOT NULL DEFAULT '[]',
  primary_ip    text,
  first_seen    timestamptz NOT NULL DEFAULT now(),
  last_seen     timestamptz NOT NULL DEFAULT now()
);
-- correlation helpers (waterfall: serial -> fqdn -> mac -> hostname)
CREATE UNIQUE INDEX hosts_serial_uq ON hosts (serial_number) WHERE serial_number IS NOT NULL;
CREATE INDEX hosts_fqdn_idx ON hosts (lower(fqdn));

-- agents: an install of the agent on a host
CREATE TABLE agents (
  id                uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  host_id           uuid NOT NULL REFERENCES hosts(id),
  agent_version     text NOT NULL,
  status            text NOT NULL DEFAULT 'active', -- active|decommissioned
  cert_fingerprint  text UNIQUE,                    -- mTLS client cert SHA-256
  config            jsonb NOT NULL DEFAULT '{}',
  config_version    int  NOT NULL DEFAULT 1,
  enrolled_at       timestamptz NOT NULL DEFAULT now(),
  last_checkin      timestamptz,
  last_heartbeat    timestamptz,
  last_metrics      jsonb
);
CREATE INDEX agents_host_idx ON agents (host_id);

-- software_catalog: canonical software identity, deduped across the whole fleet
CREATE TABLE software_catalog (
  id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  source     text NOT NULL,            -- running|installed|both
  name       text NOT NULL,
  publisher  text,
  version    text,
  sha256     text,                     -- running exes; null for installed-only
  signed     boolean,
  signer     text,
  arch       text,
  first_seen timestamptz NOT NULL DEFAULT now(),
  last_seen  timestamptz NOT NULL DEFAULT now()
);
-- dedupe keys: running exes by hash; installed packages by name+publisher+version
CREATE UNIQUE INDEX catalog_sha_uq ON software_catalog (sha256) WHERE sha256 IS NOT NULL;
CREATE UNIQUE INDEX catalog_pkg_uq ON software_catalog (name, coalesce(publisher,''), coalesce(version,''))
  WHERE sha256 IS NULL;

-- host_software: current state — what is present on each host right now
CREATE TABLE host_software (
  host_id          uuid NOT NULL REFERENCES hosts(id),
  catalog_id       uuid NOT NULL REFERENCES software_catalog(id),
  source           text NOT NULL,       -- running|installed
  exe_path         text,
  install_location text,
  owning_user      text,
  is_running       boolean NOT NULL DEFAULT false,
  first_seen       timestamptz NOT NULL DEFAULT now(),
  last_seen        timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (host_id, catalog_id, source)
);
CREATE INDEX host_software_catalog_idx ON host_software (catalog_id);

-- commands: the outbound management queue agents poll
CREATE TABLE commands (
  id           uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  agent_id     uuid NOT NULL REFERENCES agents(id),
  type         text NOT NULL,           -- config_update|scan_now|decommission
  payload      jsonb NOT NULL DEFAULT '{}',
  status       text NOT NULL DEFAULT 'pending', -- pending|acked|done|failed
  created_at   timestamptz NOT NULL DEFAULT now(),
  acked_at     timestamptz,
  completed_at timestamptz,
  result       jsonb
);
CREATE INDEX commands_agent_pending_idx ON commands (agent_id) WHERE status = 'pending';

-- ingest_batches: idempotency ledger for /ingest
CREATE TABLE ingest_batches (
  agent_id    uuid NOT NULL REFERENCES agents(id),
  batch_id    uuid NOT NULL,
  received_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (agent_id, batch_id)
);

-- audit_log: who did what in the web app
CREATE TABLE audit_log (
  id        bigserial PRIMARY KEY,
  actor     text NOT NULL,
  action    text NOT NULL,
  target    text,
  detail    jsonb NOT NULL DEFAULT '{}',
  at        timestamptz NOT NULL DEFAULT now()
);
```

Notes:
- The `source` discriminator lets reports separate "installed but never seen running"
  (the bulk of SAM/vuln surface) from "running." A piece of software can be both — set
  catalog `source='both'` when it's seen via both paths.
- Do not store volatile PID-level process detail in current-state tables. `is_running`
  + `last_seen` is enough for MVP inventory.
- An optional append-only `observations` table for "what ran when" history is deferred;
  the schema above does not require it.

---

## 8. Agent design

The agent is the bulk of the work and the part most worth getting right.

### 8.1 Two collection loops
- **Process loop** (frequent, e.g. every 5 min): enumerate running processes; capture
  pid, ppid, exe path, cmdline, owning user, start time; resolve exe metadata (hash,
  version, signature) via the hash cache.
- **Installed loop** (slow, e.g. every few hours): enumerate installed packages.
  Installed inventory barely changes, so a slow sweep + deltas is plenty.

Both loops feed the same diff → spool → transport pipeline. Cadence comes from config and
is overridable via a `config_update` command.

### 8.2 Per-OS collection paths (use the cheap routes)
**Running processes** — `gopsutil` for v1 across all three OSes. Drop to native paths
only if profiling shows it's needed.

**Installed software:**
- **Linux:** read package DBs directly — dpkg status file (`/var/lib/dpkg/status`) and the
  rpm DB. Do NOT shell out to `dpkg -l` / `rpm -qa` in a loop. Optionally also scan
  `/opt`, `/usr/local`, Snap/Flatpak for non-packaged apps.
- **Windows:** read the registry Uninstall keys — 64-bit
  `HKLM\SOFTWARE\Microsoft\Windows\CurrentVersion\Uninstall`, 32-bit
  `...\WOW6432Node\...`, and per-user `HKCU`. **Never** use `Win32_Product` via WMI — it
  is slow and triggers MSI self-repair on every enumeration.
- **macOS:** scan `Info.plist` under `/Applications` and `~/Applications` for bundle id,
  name, version. Avoid running `system_profiler` on an interval; acceptable only as an
  occasional fallback.

**Signatures:** Windows `WinVerifyTrust`; macOS `codesign`/Security framework; Linux
package signedness where available. Capture `signed` + `signer` — "unsigned binaries" is
a core report.

### 8.3 Footprint controls (define the agent's character — keep all three)
1. **Hash cache.** Hashing exes is the dominant cost. Cache `sha256` keyed on
   `(path, size, mtime, inode/file-id)`; only rehash when the binary changes. Persist the
   cache (bbolt) so a restart doesn't rehash the world.
2. **Deltas.** Keep last-sent state; diff each scan and send only `upsert`/`remove`
   changes. Send a `full_snapshot` on first run after enroll and on server request
   (`next_full_snapshot_after`) to self-heal drift.
3. **Spool.** Write outbound batches to a bbolt-backed, size-capped on-disk queue. The
   transport drains the spool; data survives server outages and replays on reconnect.
   Because ingest is idempotent on `batch_id`, replays are safe.

Also: cap hashing concurrency with a small worker pool, lower thread priority, and set
`GOMEMLIMIT` to hard-cap RSS. Sub-minute polling buys nothing for inventory — don't.

### 8.4 Service + identity
- Wrap lifecycle with `kardianos/service` (install/start/stop/uninstall on all three).
- On first run: generate keypair, build host facts, call `/enroll` with a CSR + an
  enrollment token (from install config), store the returned client cert + CA. All later
  traffic is mTLS.
- Persist on-disk state (agent_id, certs, hash cache, last-sent state, spool) under an
  OS-appropriate data dir. Lock it down with restrictive permissions.

### 8.5 Agent config (pushed via command)
`{ process_interval_s, installed_interval_s, spool_max_bytes, hash_concurrency,
mem_limit_bytes, server_url }`. Bump `config_version` on change; agent applies on next
checkin.

---

## 9. API / ingestion service

Stateless Go service; scale horizontally behind a load balancer.

- **Endpoints:** `/v1/enroll`, `/v1/ingest`, `/v1/checkin`, `/v1/heartbeat` (§6), plus the
  web-facing read/management API under `/api/...` (§10).
- **Auth:** agent endpoints require mTLS (verify client cert against the CA; map cert
  fingerprint → agent). `/enroll` accepts a bearer enrollment token instead.
- **Idempotency:** check `ingest_batches` before applying; insert `(agent_id, batch_id)`
  in the same transaction as the upserts.
- **Upserts:** resolve/insert `software_catalog` rows (dedupe per §7 keys), then upsert
  `host_software`. Use `INSERT ... ON CONFLICT ... DO UPDATE`. Batch within a transaction.
- **Full snapshot reconcile:** for `full_snapshot:true`, within the txn remove
  `host_software` rows for that `(host, source)` not present in the payload.
- **Writes are synchronous.** No queue. Keep handlers fast; push heavy work into the same
  txn rather than fanning out.

Config: `DATABASE_URL`, `LISTEN_ADDR`, `TLS_CERT/KEY`, `CA_CERT/KEY` (for signing),
`ENROLLMENT_TOKEN`, `WEB_JWT_SECRET`. No secrets in code.

---

## 10. Web app (React, dark theme)

Vite + React + TypeScript. SPA talks straight to the API (no BFF for MVP).

### 10.1 Look & feel
- **True-black background** (`#000`) for the app shell; **near-black elevated surfaces**
  (e.g. `#0a0a0a`/`#111`) for cards, tables, and panels so depth reads without pure-black-
  on-pure-black flatness.
- One accent color used sparingly (links, primary buttons, active nav, key metrics).
- High-contrast text on data-dense tables — these are report tables and must stay readable.
- Tailwind + shadcn/ui-style primitives (Button, Table, Card, Dialog, Badge, Input,
  Select, Tabs). Consistent spacing scale. No heavy theming beyond the dark palette.
- Sentence case, no ALL CAPS. Tabular numerals for counts/versions.

### 10.2 Reports (the point of the product)
- **Host → software:** all software on a given host, filter by source (running/installed).
- **Software → host:** who has a given package/version ("who is running OpenSSL 3.0.x").
- **Version sprawl:** per software name, the spread of versions across the fleet.
- **Unsigned / unapproved binaries:** everything with `signed=false` or unknown signer.
- **Installed-but-never-running:** dormant install surface (uses the `source` split).
- **Agent health:** last checkin/heartbeat, version, RSS/CPU, stale agents.

Every report: server-side filtering + pagination (10k hosts × many packages is large).
Export to CSV.

### 10.3 Fleet management
- List agents with status, host, version, last seen.
- Push config (creates a `config_update` command; bumps config_version).
- Request on-demand scan (`scan_now` command).
- Decommission an agent (`decommission` command + mark status).
- Every mutating action writes `audit_log`.

### 10.4 Auth + RBAC
- Login → JWT. Two roles for MVP: **viewer** (read reports) and **admin** (reports +
  fleet management). Enforce on the server, not just the UI.

---

## 11. Build order (vertical slices — each runnable end to end)

**Phase 0 — Skeleton.** Monorepo, `go.mod`, migrations tooling, local Postgres via
compose, empty server that boots, empty agent that prints version. Define `internal/proto`
wire types first.

**Phase 1 — Contract + ingestion.** Migrations for all §7 tables. Implement `/v1/ingest`
(no mTLS yet — temporary bearer) with catalog dedupe, host_software upsert, idempotency,
and full-snapshot reconcile. Write a throwaway script that POSTs fake running/installed
JSON. **Checkpoint:** fake data lands correctly; replaying a batch is a no-op.

**Phase 2 — Real agent collection.** Process loop + installed loop, per-OS paths, hash
cache, delta engine, bbolt spool, transport with retry. Wire to `/v1/ingest`.
**Checkpoint:** a real host reports running + installed software; killing the server then
restarting it loses nothing (spool replays).

**Phase 3 — Reports.** Read API + React dark-theme app with the §10.2 reports.
**Checkpoint:** can answer "who runs X version Y" and "show unsigned binaries on host Z."

**Phase 4 — Fleet management.** `commands` table + `/v1/checkin` polling + config push +
scan_now + decommission, with the management UI and audit log.
**Checkpoint:** push a config change from the UI; agent picks it up on next checkin.

**Phase 5 — Security hardening.** Enrollment + CSR signing + mTLS on all agent endpoints;
JWT auth + RBAC on the web API; lock down on-disk agent state. **Checkpoint:** a fresh
agent enrolls, gets a cert, and all traffic is mTLS; web roles are enforced server-side.

Do phases in order. Do not start a phase until the previous checkpoint passes.

---

## 12. Deferred features (and why each is safe to add later)

- **Durable queue / ClickHouse:** additive behind the same `/v1/ingest` contract; the
  catalog schema doesn't change. Add when past ~10k endpoints or when you need long-range
  history analytics.
- **BFF:** the SPA already calls a versioned `/api`; a BFF can slot in front later.
- **CVE enrichment:** a join table on `software_catalog`; no agent or wire change.
- **ServiceNow CMDB sync:** the host/agent split and correlation keys in §7 already exist
  precisely so this can be added without a migration. Pull enriches host rows; push uses
  the IRE.
- **Agent self-update:** signed-binary pull + verify-before-swap; new command type, no
  schema change.

Because of the versioned wire contract (§6) and the catalog-centric schema (§7), none of
these require breaking changes.

---

## 13. Guardrails for Claude Code

- Treat `internal/proto` (§6) and the schema (§7) as the contract. Changing either is a
  deliberate, reviewed act — not a casual edit.
- Keep agent dependencies minimal; justify every new one. The agent ships to every host.
- Never make the agent listen for inbound connections. Management is poll-only.
- Always parameterize SQL (pgx). Never build SQL by string concatenation.
- mTLS for all agent traffic except `/enroll`. Verify client certs server-side.
- Enforce RBAC on the server, never trust the client.
- No secrets in code or committed config; use env vars.
- Write tests at each phase: idempotency (replay a batch), delta correctness (add/remove/
  change), full-snapshot reconcile, and per-OS installed parsing against sample fixtures.
- Prefer clarity over cleverness in the agent — it runs unattended on other people's
  machines.
