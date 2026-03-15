package embed

import (
	"context"
	"fmt"
)

// LocalProvider is a stub for the removed ONNX local embedder.
// The local embed assets have been removed from muninndb-lite.
type LocalProvider struct{}

func (p *LocalProvider) Name() string { return "local" }
func (p *LocalProvider) Init(_ context.Context, _ ProviderHTTPConfig) (int, error) {
	return 0, fmt.Errorf("local ONNX embedder is not available in muninndb-lite")
}
func (p *LocalProvider) EmbedBatch(_ context.Context, _ []string) ([]float32, error) {
	return nil, fmt.Errorf("local ONNX embedder is not available in muninndb-lite")
}
func (p *LocalProvider) MaxBatchSize() int { return 1 }
func (p *LocalProvider) Close() error      { return nil }

// LocalAvailable returns false — the local ONNX embedder is not available in muninndb-lite.
func LocalAvailable() bool { return false }
