package consolidation

import (
	"context"
	"testing"

	"github.com/scrypster/muninndb/internal/storage"
)

func TestDreamOnce_DryRun_NoMutations(t *testing.T) {
	store, db, cleanup := testStoreWithDB(t)
	defer cleanup()

	ctx := context.Background()
	vault := "dream_dry"
	wsPrefix := store.ResolveVaultPrefix(vault)

	embed := []float32{1, 0, 0}
	id := writeEngramWithEmbedding(t, ctx, store, db, wsPrefix, &storage.Engram{
		Concept: "test", Content: "some content", Confidence: 0.8, Relevance: 0.6,
		Stability: 20, Embedding: embed,
	})

	mock := &mockEngineInterface{store: store}
	w := NewWorker(mock)

	report, err := w.DreamOnce(ctx, DreamOpts{DryRun: true, Force: true, Scope: vault})
	if err != nil {
		t.Fatal(err)
	}

	if len(report.Reports) != 1 {
		t.Fatalf("expected 1 vault report, got %d", len(report.Reports))
	}
	if !report.Reports[0].DryRun {
		t.Error("expected DryRun=true in report")
	}

	// Verify engram is untouched.
	eng, err := store.GetEngram(ctx, wsPrefix, id)
	if err != nil {
		t.Fatal(err)
	}
	if eng.State == storage.StateArchived {
		t.Error("engram should not be archived in dry-run mode")
	}
}

func TestDreamOnce_LegalVaultSkipped(t *testing.T) {
	store, db, cleanup := testStoreWithDB(t)
	defer cleanup()

	ctx := context.Background()
	vault := "legal/docs"
	wsPrefix := store.ResolveVaultPrefix(vault)

	embed := []float32{1, 0, 0}
	writeEngramWithEmbedding(t, ctx, store, db, wsPrefix, &storage.Engram{
		Concept: "contract", Content: "confidential agreement", Confidence: 0.9, Relevance: 0.9,
		Stability: 30, Embedding: embed,
	})

	mock := &mockEngineInterface{store: store}
	w := NewWorker(mock)

	report, err := w.DreamOnce(ctx, DreamOpts{Force: true, Scope: vault})
	if err != nil {
		t.Fatal(err)
	}

	if len(report.Skipped) != 1 || report.Skipped[0] != "legal/docs" {
		t.Errorf("expected legal/docs in Skipped, got %v", report.Skipped)
	}

	if len(report.Reports) != 1 {
		t.Fatalf("expected 1 report, got %d", len(report.Reports))
	}
	r := report.Reports[0]
	if r.LegalSkipped == 0 {
		t.Error("expected LegalSkipped > 0")
	}
	if r.MergedEngrams != 0 {
		t.Error("legal vault should have 0 merged engrams")
	}
}

func TestDreamOnce_ScopeFilter(t *testing.T) {
	store, db, cleanup := testStoreWithDB(t)
	defer cleanup()

	ctx := context.Background()

	for _, vault := range []string{"work", "personal"} {
		wsPrefix := store.ResolveVaultPrefix(vault)
		writeEngramWithEmbedding(t, ctx, store, db, wsPrefix, &storage.Engram{
			Concept: "test", Content: "content", Confidence: 0.5, Relevance: 0.5,
			Stability: 20, Embedding: []float32{1, 0, 0},
		})
	}

	mock := &mockEngineInterface{store: store}
	w := NewWorker(mock)

	report, err := w.DreamOnce(ctx, DreamOpts{Force: true, Scope: "work"})
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Reports) != 1 {
		t.Fatalf("expected 1 vault report with scope, got %d", len(report.Reports))
	}
	if report.Reports[0].Vault != "work" {
		t.Errorf("expected vault 'work', got %q", report.Reports[0].Vault)
	}
}

func TestDreamOnce_EmptyVault(t *testing.T) {
	store, _, cleanup := testStoreWithDB(t)
	defer cleanup()

	ctx := context.Background()
	mock := &mockEngineInterface{store: store}
	w := NewWorker(mock)

	report, err := w.DreamOnce(ctx, DreamOpts{Force: true, Scope: "empty"})
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Reports) != 1 {
		t.Fatalf("expected 1 report, got %d", len(report.Reports))
	}
	if report.Reports[0].Orient == nil {
		t.Error("expected orient summary even for empty vault")
	}
}
