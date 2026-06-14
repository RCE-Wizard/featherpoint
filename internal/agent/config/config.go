package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
)

const AgentVersion = "0.1.0"

// Config is the agent's runtime configuration.
type Config struct {
	ServerURL          string `json:"server_url"`
	EnrollmentToken    string `json:"enrollment_token"`
	ProcessIntervalS   int    `json:"process_interval_s"`
	InstalledIntervalS int    `json:"installed_interval_s"`
	SpoolMaxBytes      int64  `json:"spool_max_bytes"`
	HashConcurrency    int    `json:"hash_concurrency"`
	MemLimitBytes      int64  `json:"mem_limit_bytes"`
}

// State is the agent's persisted identity state.
type State struct {
	AgentID       string `json:"agent_id"`
	ConfigVersion int    `json:"config_version"`
}

func Defaults() Config {
	return Config{
		ServerURL:          "https://localhost:8080",
		ProcessIntervalS:   300,  // 5 min
		InstalledIntervalS: 3600, // 1 hr
		SpoolMaxBytes:      64 * 1024 * 1024, // 64 MB
		HashConcurrency:    4,
		MemLimitBytes:      64 * 1024 * 1024,
	}
}

// DataDir returns the OS-appropriate data directory for the agent.
func DataDir() string {
	switch runtime.GOOS {
	case "windows":
		return filepath.Join(os.Getenv("ProgramData"), "featherpoint", "swinv-agent")
	case "darwin":
		return "/Library/Application Support/featherpoint/swinv-agent"
	default:
		return "/var/lib/swinv-agent"
	}
}

func LoadState(dataDir string) (*State, error) {
	path := filepath.Join(dataDir, "state.json")
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return &State{}, nil
	}
	if err != nil {
		return nil, err
	}
	var s State
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, err
	}
	return &s, nil
}

func SaveState(dataDir string, s *State) error {
	if err := os.MkdirAll(dataDir, 0700); err != nil {
		return err
	}
	data, err := json.Marshal(s)
	if err != nil {
		return err
	}
	path := filepath.Join(dataDir, "state.json")
	return os.WriteFile(path, data, 0600)
}

func LoadConfig(dataDir string) (*Config, error) {
	path := filepath.Join(dataDir, "config.json")
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		c := Defaults()
		return &c, nil
	}
	if err != nil {
		return nil, err
	}
	var c Config
	if err := json.Unmarshal(data, &c); err != nil {
		return nil, err
	}
	return &c, nil
}
