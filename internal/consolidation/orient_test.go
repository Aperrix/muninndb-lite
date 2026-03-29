package consolidation

import (
	"context"
	"testing"

	"github.com/scrypster/muninndb/internal/storage"
)

func TestOrient_EmptyVault(t *testing.T) {
	store, _, cleanup := testStoreWithDB(t)
	defer cleanup()

	ctx := context.Background()
	vault := "empty_vault"
	wsPrefix := store.ResolveVaultPrefix(vault)

	mock := &mockEngineInterface{store: store}
	w := &Worker{Engine: mock}

	summary, err := w.runPhase0Orient(ctx, store, wsPrefix, vault)
	if err != nil {
		t.Fatal(err)
	}
	if summary.EngramCount != 0 {
		t.Errorf("EngramCount = %d, want 0", summary.EngramCount)
	}
	if summary.IsLegal {
		t.Error("IsLegal should be false for vault 'empty_vault'")
	}
}

func TestOrient_WithEngrams(t *testing.T) {
	store, db, cleanup := testStoreWithDB(t)
	defer cleanup()

	ctx := context.Background()
	vault := "orient_test"
	wsPrefix := store.ResolveVaultPrefix(vault)

	embed := []float32{1, 0, 0}
	writeEngramWithEmbedding(t, ctx, store, db, wsPrefix, &storage.Engram{
		Concept: "a", Content: "content a", Confidence: 0.8, Relevance: 0.6,
		Stability: 20, Embedding: embed,
	})
	writeEngramWithEmbedding(t, ctx, store, db, wsPrefix, &storage.Engram{
		Concept: "b", Content: "content b", Confidence: 0.9, Relevance: 0.4,
		Stability: 10, Embedding: embed,
	})

	mock := &mockEngineInterface{store: store}
	w := &Worker{Engine: mock}

	summary, err := w.runPhase0Orient(ctx, store, wsPrefix, vault)
	if err != nil {
		t.Fatal(err)
	}
	if summary.EngramCount != 2 {
		t.Errorf("EngramCount = %d, want 2", summary.EngramCount)
	}
	if summary.WithEmbed != 2 {
		t.Errorf("WithEmbed = %d, want 2", summary.WithEmbed)
	}
	if summary.AvgRelevance < 0.4 || summary.AvgRelevance > 0.6 {
		t.Errorf("AvgRelevance = %f, want ~0.5", summary.AvgRelevance)
	}
	if summary.AvgStability < 14 || summary.AvgStability > 16 {
		t.Errorf("AvgStability = %f, want ~15", summary.AvgStability)
	}
}

func TestIsLegalVault(t *testing.T) {
	tests := []struct {
		vault string
		want  bool
	}{
		{"legal", true},
		{"Legal", true},
		{"LEGAL", true},
		{"legal:contracts", true},
		{"legal/docs", true},
		{"Legal:NDA", true},
		// These should NOT match anymore:
		{"legal-docs", false},
		{"my_legal", false},
		{"paralegal", false},
		{"illegal", false},
		// Normal vaults:
		{"work", false},
		{"default", false},
		{"personal", false},
		{"projects", false},
	}
	for _, tt := range tests {
		t.Run(tt.vault, func(t *testing.T) {
			if got := isLegalVault(tt.vault); got != tt.want {
				t.Errorf("isLegalVault(%q) = %v, want %v", tt.vault, got, tt.want)
			}
		})
	}
}

func TestDedup_ConfigurableThreshold(t *testing.T) {
	store, db, cleanup := testStoreWithDB(t)
	defer cleanup()

	ctx := context.Background()
	vault := "threshold_test"
	wsPrefix := store.ResolveVaultPrefix(vault)

	// Two embeddings with cosine similarity ~0.90 (between 0.85 and 0.95).
	embedA := []float32{1, 0, 0, 0}
	embedB := []float32{0.9, 0.45, 0, 0}

	writeEngramWithEmbedding(t, ctx, store, db, wsPrefix, &storage.Engram{
		Concept: "a", Content: "content a", Confidence: 0.9, Relevance: 0.9,
		Stability: 30, Embedding: embedA,
	})
	writeEngramWithEmbedding(t, ctx, store, db, wsPrefix, &storage.Engram{
		Concept: "b", Content: "content b", Confidence: 0.5, Relevance: 0.5,
		Stability: 30, Embedding: embedB,
	})

	// Verify similarity is in the expected range.
	sim := cosineSimilarity(embedA, embedB)
	if sim < 0.85 || sim > 0.95 {
		t.Fatalf("test embeddings have cosine similarity %f, expected 0.85-0.95", sim)
	}

	// At default threshold (0.95), these should NOT cluster.
	mock := &mockEngineInterface{store: store}
	w := &Worker{Engine: mock, MaxDedup: 100, MaxTransitive: 100}
	report := &ConsolidationReport{}

	if err := w.runPhase2Dedup(ctx, store, wsPrefix, report, vault); err != nil {
		t.Fatal(err)
	}
	if report.DedupClusters != 0 {
		t.Errorf("default threshold: DedupClusters = %d, want 0", report.DedupClusters)
	}

	// At threshold 0.85, these SHOULD cluster and merge.
	store2, db2, cleanup2 := testStoreWithDB(t)
	defer cleanup2()
	wsPrefix2 := store2.ResolveVaultPrefix(vault)

	writeEngramWithEmbedding(t, ctx, store2, db2, wsPrefix2, &storage.Engram{
		Concept: "a", Content: "content a", Confidence: 0.9, Relevance: 0.9,
		Stability: 30, Embedding: embedA,
	})
	writeEngramWithEmbedding(t, ctx, store2, db2, wsPrefix2, &storage.Engram{
		Concept: "b", Content: "content b", Confidence: 0.5, Relevance: 0.5,
		Stability: 30, Embedding: embedB,
	})

	mock2 := &mockEngineInterface{store: store2}
	w2 := &Worker{Engine: mock2, MaxDedup: 100, MaxTransitive: 100, DedupThreshold: 0.85}
	report2 := &ConsolidationReport{}

	if err := w2.runPhase2Dedup(ctx, store2, wsPrefix2, report2, vault); err != nil {
		t.Fatal(err)
	}
	if report2.DedupClusters != 1 {
		t.Errorf("0.85 threshold: DedupClusters = %d, want 1", report2.DedupClusters)
	}
}
