package main

import (
	"slices"
	"testing"
)

// TestExtractOperation verifies that extractOperation correctly identifies the
// operation name by scanning for known operation tokens, regardless of what
// flags or flag values surround it.
func TestExtractOperation(t *testing.T) {
	tests := []struct {
		name     string
		args     []string
		wantOp   string
		wantRest []string
	}{
		{
			name:     "op only",
			args:     []string{"remember"},
			wantOp:   "remember",
			wantRest: []string{},
		},
		{
			name:     "flag=value before op",
			args:     []string{"--vault=default", "remember"},
			wantOp:   "remember",
			wantRest: []string{"--vault=default"},
		},
		{
			name:     "flag value before op",
			args:     []string{"--vault", "default", "remember"},
			wantOp:   "remember",
			wantRest: []string{"--vault", "default"},
		},
		{
			name:     "op followed by flags",
			args:     []string{"remember", "--concept", "c", "--content", "body"},
			wantOp:   "remember",
			wantRest: []string{"--concept", "c", "--content", "body"},
		},
		{
			// Known-ops scan is unaffected by any flags before the op name.
			name:     "unrecognized flag before value-flag before op",
			args:     []string{"--verbose", "--data-dir", "/tmp", "remember"},
			wantOp:   "remember",
			wantRest: []string{"--verbose", "--data-dir", "/tmp"},
		},
		{
			// Flag value starting with "-" used to fool the old heuristic.
			// Known-ops scan is immune to this entirely.
			name:     "flag value starting with dash before op",
			args:     []string{"--data-dir", "-custom-path", "remember"},
			wantOp:   "remember",
			wantRest: []string{"--data-dir", "-custom-path"},
		},
		{
			name:     "recall op",
			args:     []string{"recall"},
			wantOp:   "recall",
			wantRest: []string{},
		},
		{
			name:     "read op",
			args:     []string{"--vault", "myvault", "read", "--id", "abc"},
			wantOp:   "read",
			wantRest: []string{"--vault", "myvault", "--id", "abc"},
		},
		{
			name:     "forget op",
			args:     []string{"forget", "--id", "abc"},
			wantOp:   "forget",
			wantRest: []string{"--id", "abc"},
		},
		{
			name:     "flag at end no op",
			args:     []string{"--data-dir"},
			wantOp:   "",
			wantRest: []string{"--data-dir"},
		},
		{
			name:     "no args",
			args:     []string{},
			wantOp:   "",
			wantRest: []string{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotOp, gotRest := extractOperation(tt.args)
			if gotOp != tt.wantOp {
				t.Errorf("op: got %q, want %q (args: %v)", gotOp, tt.wantOp, tt.args)
			}
			if gotRest == nil {
				gotRest = []string{}
			}
			if !slices.Equal(gotRest, tt.wantRest) {
				t.Errorf("rest: got %v, want %v", gotRest, tt.wantRest)
			}
		})
	}
}
