// Package collect enumerates running processes and installed software.
package collect

import (
	"context"
	"log"
	"path/filepath"

	"github.com/featherpoint/swinv/internal/agent/hashcache"
	"github.com/featherpoint/swinv/internal/proto"
	gops "github.com/shirou/gopsutil/v3/process"
	"golang.org/x/sync/errgroup"
)

// RunningProcesses returns a SoftwareDelta slice for all currently running executables.
// Hashing is done in parallel via a worker pool bounded by concurrency.
func RunningProcesses(ctx context.Context, cache *hashcache.Cache, concurrency int) []proto.SoftwareDelta {
	procs, err := gops.ProcessesWithContext(ctx)
	if err != nil {
		log.Printf("collect/processes: %v", err)
		return nil
	}

	type result struct {
		delta proto.SoftwareDelta
	}

	sem := make(chan struct{}, concurrency)
	resultCh := make(chan result, len(procs))

	var g errgroup.Group
	for _, p := range procs {
		p := p
		g.Go(func() error {
			sem <- struct{}{}
			defer func() { <-sem }()

			exe, err := p.ExeWithContext(ctx)
			if err != nil || exe == "" {
				return nil
			}

			name := filepath.Base(exe)
			username, _ := p.UsernameWithContext(ctx)

			hash := cache.Get(exe)

			d := proto.SoftwareDelta{
				Op:         "upsert",
				Source:     "running",
				Name:       name,
				ExePath:    &exe,
				OwningUser: strPtr(username),
			}
			if hash != "" {
				d.SHA256 = &hash
			}

			// Per-OS signature enrichment
			signed, signer := signatureOf(exe)
			d.Signed = signed
			if signer != "" {
				d.Signer = &signer
			}

			resultCh <- result{d}
			return nil
		})
	}
	_ = g.Wait()
	close(resultCh)

	var out []proto.SoftwareDelta
	seen := map[string]bool{} // dedupe by exe path
	for r := range resultCh {
		key := ""
		if r.delta.ExePath != nil {
			key = *r.delta.ExePath
		}
		if key != "" && seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, r.delta)
	}
	return out
}

func strPtr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
