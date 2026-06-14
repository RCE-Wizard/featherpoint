// Package proto defines the shared wire contract between agent and server.
// Treat this file as a contract — changes require deliberate review.
package proto

import "time"

const SchemaVersion = 1

// Envelope is embedded in every request body.
type Envelope struct {
	SchemaVersion int    `json:"schema_version"`
	AgentVersion  string `json:"agent_version"`
	AgentID       string `json:"agent_id,omitempty"` // omitted on /enroll
	SentAt        string `json:"sent_at"`            // RFC3339
}

// HostFacts describes the physical/virtual machine.
type HostFacts struct {
	Hostname     string   `json:"hostname"`
	FQDN         string   `json:"fqdn"`
	OS           string   `json:"os"` // linux|windows|darwin
	OSVersion    string   `json:"os_version"`
	SerialNumber *string  `json:"serial_number"`
	MACAddresses []string `json:"mac_addresses"`
	PrimaryIP    *string  `json:"primary_ip"`
}

// EnrollRequest is the body for POST /v1/enroll.
type EnrollRequest struct {
	EnrollmentToken string    `json:"enrollment_token"`
	AgentVersion    string    `json:"agent_version"`
	HostFacts       HostFacts `json:"host_facts"`
	CSRPEM          string    `json:"csr_pem"`
}

// EnrollResponse is the response from POST /v1/enroll.
type EnrollResponse struct {
	AgentID       string `json:"agent_id"`
	ClientCertPEM string `json:"client_cert_pem"`
	CAPEM         string `json:"ca_pem"`
}

// SoftwareDelta is a single change in a batch.
type SoftwareDelta struct {
	Op              string  `json:"op"`              // upsert|remove
	Source          string  `json:"source"`          // running|installed
	Name            string  `json:"name"`
	Publisher       *string `json:"publisher"`
	Version         *string `json:"version"`
	SHA256          *string `json:"sha256"`
	Signed          bool    `json:"signed"`
	Signer          *string `json:"signer"`
	Arch            *string `json:"arch"`
	ExePath         *string `json:"exe_path"`
	InstallLocation *string `json:"install_location"`
	OwningUser      *string `json:"owning_user"`
}

// IngestRequest is the body for POST /v1/ingest.
type IngestRequest struct {
	Envelope
	BatchID      string          `json:"batch_id"`
	CollectedAt  time.Time       `json:"collected_at"`
	FullSnapshot bool            `json:"full_snapshot"`
	Running      []SoftwareDelta `json:"running"`
	Installed    []SoftwareDelta `json:"installed"`
}

// IngestResponse is the response from POST /v1/ingest.
type IngestResponse struct {
	Accepted              bool    `json:"accepted"`
	NextFullSnapshotAfter *string `json:"next_full_snapshot_after"` // RFC3339|null
}

// CheckinRequest is the body for POST /v1/checkin.
type CheckinRequest struct {
	Envelope
	ConfigVersion   int      `json:"config_version"`
	AckedCommandIDs []string `json:"acked_command_ids,omitempty"`
}

// Command is a management instruction from server to agent.
type Command struct {
	ID      string                 `json:"id"`
	Type    string                 `json:"type"` // config_update|scan_now|decommission
	Payload map[string]interface{} `json:"payload"`
}

// AgentConfig is the agent's runtime configuration.
type AgentConfig struct {
	ProcessIntervalS   int    `json:"process_interval_s"`
	InstalledIntervalS int    `json:"installed_interval_s"`
	SpoolMaxBytes      int64  `json:"spool_max_bytes"`
	HashConcurrency    int    `json:"hash_concurrency"`
	MemLimitBytes      int64  `json:"mem_limit_bytes"`
	ServerURL          string `json:"server_url"`
}

// CheckinResponse is the response from POST /v1/checkin.
type CheckinResponse struct {
	Commands      []Command   `json:"commands"`
	Config        AgentConfig `json:"config,omitempty"`
	ConfigVersion int         `json:"config_version"`
}

// HeartbeatRequest is the body for POST /v1/heartbeat.
type HeartbeatRequest struct {
	Envelope
	Metrics AgentMetrics `json:"metrics"`
}

// AgentMetrics holds resource usage telemetry from the agent.
type AgentMetrics struct {
	RSSBytes int64   `json:"rss_bytes"`
	CPUPct   float64 `json:"cpu_pct"`
	UptimeS  int64   `json:"uptime_s"`
}

// HeartbeatResponse is the response from POST /v1/heartbeat.
type HeartbeatResponse struct {
	OK bool `json:"ok"`
}
