// Package delta computes the difference between last-sent software state and current state.
package delta

import (
	"github.com/featherpoint/swinv/internal/proto"
)

// Key uniquely identifies a software observation within a source.
type Key struct {
	Source string
	Name   string
	// For running exes, SHA256 is the identity; for installed, it's name+publisher+version.
	Publisher string
	Version   string
	SHA256    string
}

// State is the last-sent collection state, keyed by software identity.
type State map[Key]proto.SoftwareDelta

func keyOf(d proto.SoftwareDelta) Key {
	k := Key{Source: d.Source, Name: d.Name}
	if d.SHA256 != nil {
		k.SHA256 = *d.SHA256
	}
	if d.Publisher != nil {
		k.Publisher = *d.Publisher
	}
	if d.Version != nil {
		k.Version = *d.Version
	}
	return k
}

// Diff computes the deltas between last and current state.
// Returns (deltas, newState).
func Diff(last State, current []proto.SoftwareDelta) ([]proto.SoftwareDelta, State) {
	next := make(State, len(current))
	var deltas []proto.SoftwareDelta

	for _, d := range current {
		k := keyOf(d)
		next[k] = d
		prev, exists := last[k]
		if !exists {
			upsert := d
			upsert.Op = "upsert"
			deltas = append(deltas, upsert)
			continue
		}
		// Emit upsert only if something observable changed
		if observableChange(prev, d) {
			upsert := d
			upsert.Op = "upsert"
			deltas = append(deltas, upsert)
		}
	}

	// Anything in last but not in current → remove
	for k, prev := range last {
		if _, ok := next[k]; !ok {
			remove := prev
			remove.Op = "remove"
			deltas = append(deltas, remove)
		}
	}

	return deltas, next
}

func observableChange(a, b proto.SoftwareDelta) bool {
	if strVal(a.Version) != strVal(b.Version) {
		return true
	}
	if strVal(a.ExePath) != strVal(b.ExePath) {
		return true
	}
	if strVal(a.OwningUser) != strVal(b.OwningUser) {
		return true
	}
	if a.Signed != b.Signed || strVal(a.Signer) != strVal(b.Signer) {
		return true
	}
	return false
}

func strVal(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
