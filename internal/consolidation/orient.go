package consolidation

import (
	"context"
	"strings"

	"github.com/scrypster/muninndb/internal/storage"
)

// VaultSummary is the read-only output of Phase 0 (Orient).
type VaultSummary struct {
	Vault        string
	EngramCount  int
	WithEmbed    int
	AvgRelevance float32
	AvgStability float32
	IsLegal      bool
}

// runPhase0Orient scans the vault to produce a VaultSummary.
// Pure read-only -- no mutations even when DryRun is false.
func (w *Worker) runPhase0Orient(ctx context.Context, store *storage.PebbleStore, wsPrefix [8]byte, vault string) (*VaultSummary, error) {
	allIDs, err := scanAllEngramIDs(ctx, store, wsPrefix)
	if err != nil {
		return nil, err
	}

	summary := &VaultSummary{
		Vault:       vault,
		EngramCount: len(allIDs),
		IsLegal:     isLegalVault(vault),
	}

	if len(allIDs) == 0 {
		return summary, nil
	}

	engrams, err := store.GetEngrams(ctx, wsPrefix, allIDs)
	if err != nil {
		return nil, err
	}

	var totalRelevance, totalStability float64
	var count int
	for _, eng := range engrams {
		if eng == nil {
			continue
		}
		count++
		totalRelevance += float64(eng.Relevance)
		totalStability += float64(eng.Stability)

		embed := eng.Embedding
		if len(embed) == 0 {
			if loaded, err := store.GetEmbedding(ctx, wsPrefix, eng.ID); err == nil && len(loaded) > 0 {
				embed = loaded
			}
		}
		if len(embed) > 0 {
			summary.WithEmbed++
		}
	}

	if count > 0 {
		summary.AvgRelevance = float32(totalRelevance / float64(count))
		summary.AvgStability = float32(totalStability / float64(count))
	}

	return summary, nil
}

// isLegalVault returns true if the vault is the "legal" vault or uses the legal prefix convention.
// Matches: "legal", "legal:contracts", "legal/docs" — but NOT "paralegal" or "illegal".
func isLegalVault(vault string) bool {
	v := strings.ToLower(vault)
	return v == "legal" || strings.HasPrefix(v, "legal:") || strings.HasPrefix(v, "legal/")
}
