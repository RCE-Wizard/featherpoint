package delta_test

import (
	"testing"

	"github.com/featherpoint/swinv/internal/agent/delta"
	"github.com/featherpoint/swinv/internal/proto"
)

func strPtr(s string) *string { return &s }

func TestDiffAddRemoveChange(t *testing.T) {
	last := delta.State{}

	current := []proto.SoftwareDelta{
		{Source: "installed", Name: "curl", Publisher: strPtr("curl"), Version: strPtr("7.88")},
		{Source: "installed", Name: "openssl", Publisher: strPtr("openssl"), Version: strPtr("3.0")},
	}

	deltas, next := delta.Diff(last, current)

	if len(deltas) != 2 {
		t.Fatalf("expected 2 upserts, got %d", len(deltas))
	}
	for _, d := range deltas {
		if d.Op != "upsert" {
			t.Errorf("expected upsert, got %s", d.Op)
		}
	}

	// Remove one
	current2 := []proto.SoftwareDelta{
		{Source: "installed", Name: "curl", Publisher: strPtr("curl"), Version: strPtr("7.88")},
	}
	deltas2, _ := delta.Diff(next, current2)
	if len(deltas2) != 1 || deltas2[0].Op != "remove" || deltas2[0].Name != "openssl" {
		t.Fatalf("expected remove of openssl, got %+v", deltas2)
	}

	// Version change: old version is removed, new version is upserted (different catalog keys)
	current3 := []proto.SoftwareDelta{
		{Source: "installed", Name: "curl", Publisher: strPtr("curl"), Version: strPtr("8.0")},
	}
	deltas3, _ := delta.Diff(delta.State{
		{Source: "installed", Name: "curl", Publisher: "curl", Version: "7.88"}: current2[0],
	}, current3)
	if len(deltas3) != 2 {
		t.Fatalf("expected remove+upsert on version change, got %+v", deltas3)
	}
	ops := map[string]bool{}
	for _, d := range deltas3 {
		ops[d.Op] = true
	}
	if !ops["upsert"] || !ops["remove"] {
		t.Fatalf("expected upsert and remove ops, got %+v", deltas3)
	}
}

func TestDiffNoop(t *testing.T) {
	item := proto.SoftwareDelta{Source: "installed", Name: "vim", Publisher: strPtr("vim.org"), Version: strPtr("9.0"), Signed: true}
	_, state := delta.Diff(delta.State{}, []proto.SoftwareDelta{item})

	deltas, _ := delta.Diff(state, []proto.SoftwareDelta{item})
	if len(deltas) != 0 {
		t.Fatalf("expected no deltas on identical state, got %d", len(deltas))
	}
}
