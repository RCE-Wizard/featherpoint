package main

import (
	"os"
	"runtime"
	"time"

	"github.com/featherpoint/swinv/internal/proto"
)

var startTime = time.Now()

func currentMetrics() proto.AgentMetrics {
	var ms runtime.MemStats
	runtime.ReadMemStats(&ms)
	return proto.AgentMetrics{
		RSSBytes: int64(ms.Sys),
		CPUPct:   0, // gopsutil self-process CPU added in Phase 2 tuning
		UptimeS:  int64(time.Since(startTime).Seconds()),
	}
}

// ensure os imported (used in main.go)
var _ = os.Getenv
