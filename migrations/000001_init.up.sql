-- hosts: durable machine identity
CREATE TABLE hosts (
  id            uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  hostname      text NOT NULL,
  fqdn          text,
  os            text NOT NULL,
  os_version    text,
  serial_number text,
  mac_addresses jsonb NOT NULL DEFAULT '[]',
  primary_ip    text,
  first_seen    timestamptz NOT NULL DEFAULT now(),
  last_seen     timestamptz NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX hosts_serial_uq ON hosts (serial_number) WHERE serial_number IS NOT NULL;
CREATE INDEX hosts_fqdn_idx ON hosts (lower(fqdn));

-- agents: an install of the agent on a host
CREATE TABLE agents (
  id               uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  host_id          uuid NOT NULL REFERENCES hosts(id),
  agent_version    text NOT NULL,
  status           text NOT NULL DEFAULT 'active',
  cert_fingerprint text UNIQUE,
  config           jsonb NOT NULL DEFAULT '{}',
  config_version   int  NOT NULL DEFAULT 1,
  enrolled_at      timestamptz NOT NULL DEFAULT now(),
  last_checkin     timestamptz,
  last_heartbeat   timestamptz,
  last_metrics     jsonb
);
CREATE INDEX agents_host_idx ON agents (host_id);

-- software_catalog: canonical software identity, deduped across the fleet
CREATE TABLE software_catalog (
  id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  source     text NOT NULL,
  name       text NOT NULL,
  publisher  text,
  version    text,
  sha256     text,
  signed     boolean,
  signer     text,
  arch       text,
  first_seen timestamptz NOT NULL DEFAULT now(),
  last_seen  timestamptz NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX catalog_sha_uq ON software_catalog (sha256) WHERE sha256 IS NOT NULL;
CREATE UNIQUE INDEX catalog_pkg_uq ON software_catalog (name, coalesce(publisher,''), coalesce(version,''))
  WHERE sha256 IS NULL;

-- host_software: current state
CREATE TABLE host_software (
  host_id          uuid NOT NULL REFERENCES hosts(id),
  catalog_id       uuid NOT NULL REFERENCES software_catalog(id),
  source           text NOT NULL,
  exe_path         text,
  install_location text,
  owning_user      text,
  is_running       boolean NOT NULL DEFAULT false,
  first_seen       timestamptz NOT NULL DEFAULT now(),
  last_seen        timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (host_id, catalog_id, source)
);
CREATE INDEX host_software_catalog_idx ON host_software (catalog_id);

-- commands: management queue agents poll
CREATE TABLE commands (
  id           uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  agent_id     uuid NOT NULL REFERENCES agents(id),
  type         text NOT NULL,
  payload      jsonb NOT NULL DEFAULT '{}',
  status       text NOT NULL DEFAULT 'pending',
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

-- audit_log: web app mutation trail
CREATE TABLE audit_log (
  id     bigserial PRIMARY KEY,
  actor  text NOT NULL,
  action text NOT NULL,
  target text,
  detail jsonb NOT NULL DEFAULT '{}',
  at     timestamptz NOT NULL DEFAULT now()
);

-- enrollment_tokens: pre-shared tokens for agent enrollment
CREATE TABLE enrollment_tokens (
  token      text PRIMARY KEY,
  label      text,
  created_at timestamptz NOT NULL DEFAULT now(),
  expires_at timestamptz,
  used_count int NOT NULL DEFAULT 0
);

-- web_users: human users of the web app
CREATE TABLE web_users (
  id            uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  username      text UNIQUE NOT NULL,
  password_hash text NOT NULL,
  role          text NOT NULL DEFAULT 'viewer',
  created_at    timestamptz NOT NULL DEFAULT now()
);
