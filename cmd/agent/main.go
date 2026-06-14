package main

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"github.com/featherpoint/swinv/internal/agent/collect"
	"github.com/featherpoint/swinv/internal/agent/config"
	"github.com/featherpoint/swinv/internal/agent/delta"
	"github.com/featherpoint/swinv/internal/agent/enroll"
	"github.com/featherpoint/swinv/internal/agent/hashcache"
	"github.com/featherpoint/swinv/internal/agent/spool"
	"github.com/featherpoint/swinv/internal/agent/transport"
	"github.com/featherpoint/swinv/internal/proto"
	"github.com/google/uuid"
	"github.com/kardianos/service"
)

type program struct {
	cfg     *config.Config
	state   *config.State
	dataDir string
	stop    chan struct{}
}

func (p *program) Start(s service.Service) error {
	p.stop = make(chan struct{})
	go p.run()
	return nil
}

func (p *program) run() {
	log.Printf("swinv-agent %s started on %s/%s", config.AgentVersion, runtime.GOOS, runtime.GOARCH)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() { <-p.stop; cancel() }()

	// First run: enroll if we don't have a cert yet
	if !enroll.IsEnrolled(p.dataDir) {
		if err := p.doEnroll(ctx); err != nil {
			log.Fatalf("enrollment failed: %v", err)
		}
	}

	// Build mTLS transport
	tlsCfg, err := enroll.TLSConfig(p.dataDir)
	if err != nil {
		log.Fatalf("tls config: %v", err)
	}
	client, err := transport.New(transport.Config{
		BaseURL: p.cfg.ServerURL,
		AgentID: p.state.AgentID,
		TLSConfig: tlsCfg,
	})
	if err != nil {
		log.Fatalf("transport: %v", err)
	}

	cache, err := hashcache.Open(filepath.Join(p.dataDir, "hashcache.db"))
	if err != nil {
		log.Fatalf("hashcache: %v", err)
	}
	defer cache.Close()

	sp, err := spool.Open(p.dataDir, p.cfg.SpoolMaxBytes)
	if err != nil {
		log.Fatalf("spool: %v", err)
	}
	defer sp.Close()

	procState := delta.State{}
	instState := delta.State{}

	processTick := time.NewTicker(time.Duration(p.cfg.ProcessIntervalS) * time.Second)
	installedTick := time.NewTicker(time.Duration(p.cfg.InstalledIntervalS) * time.Second)
	checkinTick := time.NewTicker(60 * time.Second)
	heartbeatTick := time.NewTicker(30 * time.Second)
	drainTick := time.NewTicker(10 * time.Second)

	// Initial full snapshots on startup
	p.collectAndSpool(ctx, cache, sp, &procState, &instState, true)

	for {
		select {
		case <-p.stop:
			return
		case <-processTick.C:
			p.collectAndSpool(ctx, cache, sp, &procState, nil, false)
		case <-installedTick.C:
			p.collectAndSpool(ctx, cache, sp, nil, &instState, false)
		case <-drainTick.C:
			drainSpool(ctx, sp, client)
		case <-checkinTick.C:
			p.doCheckin(ctx, client)
		case <-heartbeatTick.C:
			p.doHeartbeat(ctx, client)
		}
	}
}

// doEnroll performs first-run enrollment: generates keypair, sends CSR, stores cert.
func (p *program) doEnroll(ctx context.Context) error {
	facts := collect.HostFacts()
	log.Printf("enrolling host %s (%s)…", facts.Hostname, facts.OS)

	csrPEM, err := enroll.GenerateKeyAndCSR(p.dataDir, facts.Hostname)
	if err != nil {
		return fmt.Errorf("generate CSR: %w", err)
	}

	req := proto.EnrollRequest{
		EnrollmentToken: p.cfg.EnrollmentToken,
		AgentVersion:    config.AgentVersion,
		HostFacts:       facts,
		CSRPEM:          csrPEM,
	}

	// /enroll uses plain HTTPS (no client cert yet) — skip client cert verification
	// for the enrollment call only; all subsequent calls use full mTLS.
	plainClient := &http.Client{
		Timeout: 30 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: p.cfg.InsecureSkipVerify}, //nolint:gosec
		},
	}

	body, _ := json.Marshal(req)
	httpReq, err := http.NewRequestWithContext(ctx, "POST", p.cfg.ServerURL+"/v1/enroll", bytes.NewReader(body))
	if err != nil {
		return err
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := plainClient.Do(httpReq)
	if err != nil {
		return fmt.Errorf("POST /v1/enroll: %w", err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("enroll %d: %s", resp.StatusCode, raw)
	}

	var er proto.EnrollResponse
	if err := json.Unmarshal(raw, &er); err != nil {
		return fmt.Errorf("parse enroll response: %w", err)
	}

	if err := enroll.StoreCerts(p.dataDir, er.ClientCertPEM, er.CAPEM); err != nil {
		return fmt.Errorf("store certs: %w", err)
	}

	p.state.AgentID = er.AgentID
	if err := config.SaveState(p.dataDir, p.state); err != nil {
		return fmt.Errorf("save state: %w", err)
	}

	log.Printf("enrolled: agent_id=%s", er.AgentID)
	return nil
}

func (p *program) collectAndSpool(
	ctx context.Context,
	cache *hashcache.Cache,
	sp *spool.Spool,
	procState *delta.State,
	instState *delta.State,
	full bool,
) {
	var running, installed []proto.SoftwareDelta

	if procState != nil {
		cur := collect.RunningProcesses(ctx, cache, p.cfg.HashConcurrency)
		d, next := delta.Diff(*procState, cur)
		*procState = next
		running = d
	}
	if instState != nil {
		cur := collect.InstalledSoftware()
		d, next := delta.Diff(*instState, cur)
		*instState = next
		installed = d
	}

	if len(running) == 0 && len(installed) == 0 && !full {
		return
	}

	batch := proto.IngestRequest{
		Envelope: proto.Envelope{
			SchemaVersion: proto.SchemaVersion,
			AgentVersion:  config.AgentVersion,
			AgentID:       p.state.AgentID,
			SentAt:        time.Now().UTC().Format(time.RFC3339),
		},
		BatchID:      uuid.New().String(),
		CollectedAt:  time.Now(),
		FullSnapshot: full,
		Running:      running,
		Installed:    installed,
	}
	if err := sp.Push(batch); err != nil {
		log.Printf("spool push: %v", err)
	}
}

func drainSpool(ctx context.Context, sp *spool.Spool, client *transport.Client) {
	for sp.Len() > 0 {
		var batch proto.IngestRequest
		key, err := sp.Peek(&batch)
		if err != nil {
			break
		}
		var resp proto.IngestResponse
		if err := client.Post(ctx, "/v1/ingest", batch, &resp); err != nil {
			log.Printf("ingest: %v", err)
			break
		}
		_ = sp.Ack(key)
	}
}

func (p *program) doCheckin(ctx context.Context, client *transport.Client) {
	req := proto.CheckinRequest{
		Envelope: proto.Envelope{
			SchemaVersion: proto.SchemaVersion,
			AgentVersion:  config.AgentVersion,
			AgentID:       p.state.AgentID,
			SentAt:        time.Now().UTC().Format(time.RFC3339),
		},
		ConfigVersion: p.state.ConfigVersion,
	}
	var resp proto.CheckinResponse
	if err := client.Post(ctx, "/v1/checkin", req, &resp); err != nil {
		log.Printf("checkin: %v", err)
		return
	}
	if resp.ConfigVersion > p.state.ConfigVersion {
		p.state.ConfigVersion = resp.ConfigVersion
		_ = config.SaveState(p.dataDir, p.state)
		log.Printf("config updated to version %d", resp.ConfigVersion)
	}
}

func (p *program) doHeartbeat(ctx context.Context, client *transport.Client) {
	req := proto.HeartbeatRequest{
		Envelope: proto.Envelope{
			SchemaVersion: proto.SchemaVersion,
			AgentVersion:  config.AgentVersion,
			AgentID:       p.state.AgentID,
			SentAt:        time.Now().UTC().Format(time.RFC3339),
		},
		Metrics: currentMetrics(),
	}
	var resp proto.HeartbeatResponse
	_ = client.Post(ctx, "/v1/heartbeat", req, &resp)
}

func (p *program) Stop(s service.Service) error {
	close(p.stop)
	return nil
}

func main() {
	if len(os.Args) > 1 && os.Args[1] == "version" {
		fmt.Printf("swinv-agent %s\n", config.AgentVersion)
		return
	}

	dataDir := config.DataDir()
	if err := os.MkdirAll(dataDir, 0700); err != nil {
		log.Fatalf("mkdir dataDir: %v", err)
	}
	// Ensure data dir is not readable by other users
	if err := os.Chmod(dataDir, 0700); err != nil {
		log.Printf("chmod dataDir: %v", err)
	}

	cfg, err := config.LoadConfig(dataDir)
	if err != nil {
		log.Fatalf("config: %v", err)
	}
	state, err := config.LoadState(dataDir)
	if err != nil {
		log.Fatalf("state: %v", err)
	}

	svcConfig := &service.Config{
		Name:        "swinv-agent",
		DisplayName: "Software Inventory Agent",
		Description: "Collects running processes and installed software for fleet inventory.",
	}

	prg := &program{cfg: cfg, state: state, dataDir: dataDir}
	svc, err := service.New(prg, svcConfig)
	if err != nil {
		log.Fatalf("service.New: %v", err)
	}

	if len(os.Args) > 1 {
		if err := service.Control(svc, os.Args[1]); err != nil {
			fmt.Fprintf(os.Stderr, "service control %q: %v\n", os.Args[1], err)
			os.Exit(1)
		}
		return
	}

	if err := svc.Run(); err != nil {
		log.Fatalf("service run: %v", err)
	}
}
