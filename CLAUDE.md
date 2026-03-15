# CLAUDE.md — muninndb-lite

## What is this project

muninndb-lite is a **minimal-diff fork** of [MuninnDB](https://github.com/scrypster/muninndb), stripped to MCP stdio transport only. Same cognitive engine, same 36 MCP tools, same storage layer. No REST, gRPC, web UI, clustering, or bundled ONNX embedder.

Target: embedded cognitive memory for any AI agent (LLM-agnostic) via MCP.

## Architecture decisions

### Minimal-diff strategy

The fork modifies only `cmd/muninn/` (entry point + wiring). All `internal/` packages that compile are kept intact — even those not wired at runtime (mbp, replication, metrics). This ensures upstream merges are clean.

**Do NOT modify files in `internal/`** unless absolutely necessary. If you must, document why.

The only exception is `internal/plugin/embed/local_stub.go` — a stub replacing the deleted ONNX local embedder files.

### Why this works

MuninnDB's architecture is well-decoupled:
- All transports are wired via interfaces, in separable blocks in `server.go`
- The engine does NOT depend on transports (coupling is inverted: transports import engine)
- Replication is opt-in via `SetCoordinator()` — not calling it = no cluster
- MCP auth is in `mcp/context.go`, independent of the `auth/` package
- Vault isolation is at the storage layer (Pebble key prefixes), not in auth

### Binary size

The Go linker does **dead code elimination**. Only code reachable from `main()` is included in the binary. Packages present in the source tree but never called (REST, gRPC, replication, etc.) are excluded from the compiled binary. The binary is the same size as if those files were deleted.

### Module path

The Go module path is `github.com/scrypster/muninndb` (upstream). **Do NOT change it.** Changing it would break all internal imports and make upstream merges impossible. This means `go install` would install upstream MuninnDB, not our fork — distribution is via compiled binaries (GitHub Releases) or `git clone` + `go build`.

### Entry point

`muninndb-lite mcp` (or just `muninndb-lite`) starts:
1. The full engine inline (Pebble, WAL, FTS, HNSW, auth, cognitive workers, plugins)
2. An MCP HTTP server on an **ephemeral port** (127.0.0.1:0)
3. A stdio proxy loop bridging stdin/stdout to that internal HTTP server
4. Graceful shutdown on stdin EOF or SIGINT/SIGTERM

There is **no daemon mode**, no `start`/`stop` commands, no persistent server. The process lives and dies with the MCP client.

## Recall pipeline

```
LLM Client (any)              muninndb-lite
     |                              |
     |-- muninn_recall ------------>|  1. BM25 full-text search (always)
     |                              |  2. HNSW vector search (if embeddings configured)
     |                              |  3. Cognitive scoring:
     |                              |     - Ebbinghaus decay (recent/frequent = stronger)
     |                              |     - Hebbian learning (co-activated = linked)
     |                              |     - Bayesian confidence (confirmed = trusted)
     |                              |     - ACT-R / CGDN composite scoring
     |<-- ranked results -----------|  4. Return top-N
```

When no embedding provider is configured, the engine auto-adjusts weights: `SemanticSimilarity` drops to 0, `FullTextRelevance` is rescaled proportionally. No code change needed — the fallback is native.

## Embedding providers (optional)

Detection is implemented in `cmd/muninn/server.go` (`buildEmbedder()`). Priority chain (first match wins):

1. `MUNINN_OLLAMA_URL` → Ollama (local, free)
2. `MUNINN_OPENAI_KEY` → OpenAI (text-embedding-3-small)
3. `MUNINN_VOYAGE_KEY` → Voyage AI
4. `MUNINN_COHERE_KEY` → Cohere
5. `MUNINN_GOOGLE_KEY` → Google Gemini
6. `MUNINN_JINA_KEY` → Jina
7. `MUNINN_MISTRAL_KEY` → Mistral
8. Saved config (`~/.muninn/.../plugin_config.json`) — fallback
9. Noop embedder — zero vectors, BM25-only mode

## LLM enrichment (optional)

The LLM client can provide entities, relationships, and summaries **inline** in `muninn_remember` — no server-side LLM needed:

```json
{
  "content": "...", "concept": "...",
  "summary": "...",
  "entities": [{"name": "PostgreSQL", "type": "database"}],
  "entity_relationships": [{"from_entity": "A", "to_entity": "B", "rel_type": "uses"}]
}
```

When inline data is provided, the server-side enrichment pipeline skips those stages. Limits: max 20 entities, 30 relationships, 30 entity_relationships per call. Valid entity types: person, organization, location, concept, technology, project, tool, database, service, framework, language, product, event.

Server-side enrichment (optional, requires API key or local LLM):
- `MUNINN_ENRICH_URL=ollama://host:port/model` — local, free
- `MUNINN_ENRICH_URL=openai://host:port/model` + key
- `MUNINN_ENRICH_URL=anthropic://model` + `MUNINN_ANTHROPIC_KEY`

## Packages

### Wired (active at runtime)

- `internal/mcp/` — MCP server, 36 tools, JSON-RPC 2.0
- `internal/engine/` — cognitive engine, activation pipeline (ACT-R/CGDN), triggers
- `internal/storage/` — Pebble KV store, engrams, entities, associations, cache
- `internal/cognitive/` — Ebbinghaus decay, Hebbian learning, Bayesian confidence, contradiction
- `internal/index/fts/` — BM25 full-text search (k1=1.2, b=0.75, field weighting)
- `internal/index/hnsw/` — HNSW vector similarity
- `internal/index/adjacency/` — entity relationship graph
- `internal/plugin/embed/` — 7 HTTP embedding providers
- `internal/plugin/enrich/` — LLM enrichment pipeline (4 stages: entities, relationships, classification, summary)
- `internal/auth/` — vault isolation, plasticity config (cognitive tuning per vault, 4 presets)
- `internal/consolidation/` — deduplication, transitive merge
- `internal/wal/` — write-ahead logging
- `internal/config/` — configuration management

### Present but not wired (kept because wired packages import them)

- `internal/transport/mbp/` — shared types (`WriteRequest`, `ActivateRequest`, etc.) used by engine and MCP
- `internal/replication/` — imported by engine for `CheckAndSetSchemaVersion()` and `CognitiveForwarder` interface
- `internal/metrics/` — imported by engine and fts

### Deleted (no wired package imports them)

Protected by `.gitattributes` merge=ours:
- `internal/transport/rest/`, `internal/transport/grpc/`, `internal/ui/`
- `web/`, `sdk/`, `proto/`, `docs/`
- `cmd/bench/`, `cmd/diag/`, `cmd/eval-semantic/`
- `internal/plugin/embed/local*.go` (ONNX Runtime)
- `Dockerfile`, `docker-compose.yml`, `.goreleaser.yml`, `install.sh`, `install.ps1`
- `CONTRIBUTING.md`, `CHANGELOG.md`, `CLA.md`, `NOTICE`, `contrib/`

## How to work on this project

### Build & test

```bash
make build    # produces ./muninndb-lite
make test     # go test -race ./cmd/muninn/... ./internal/...
make vet      # go vet
```

### Upstream sync

```bash
git remote add upstream https://github.com/scrypster/muninndb.git
git fetch upstream
git merge upstream/develop
# Conflicts expected only in: cmd/muninn/server.go, cmd/muninn/main.go, go.mod
# .gitattributes merge=ours handles deleted files automatically
```

### What to change vs what to leave alone

| Area | Rule |
|---|---|
| `cmd/muninn/main.go` | Our entry point — modify freely |
| `cmd/muninn/server.go` | Our wiring — modify freely (contains `runMCPStandalone()`) |
| `cmd/muninn/help.go` | Our help text — modify freely |
| `cmd/muninn/*.go` (other) | Dead code from upstream — do NOT delete (merge compatibility), do NOT modify |
| `internal/` | Do NOT modify — upstream code, must merge cleanly |
| `.github/workflows/` | Our CI/release — modify freely |
| `README.md`, `Makefile` | Ours — modify freely |

### Key files

| File | Role |
|---|---|
| `cmd/muninn/server.go` | `runMCPStandalone()` — engine init + MCP server + stdio proxy |
| `cmd/muninn/main.go` | Entry point — dispatches `mcp`, `version`, `help` |
| `cmd/muninn/mcp_stdio.go` | `runMCPStdioWith()` — stdio-to-HTTP proxy loop (upstream, unmodified) |
| `cmd/muninn/help.go` | CLI help text |
| `internal/plugin/embed/local_stub.go` | Stub for removed ONNX embedder |
| `.gitattributes` | merge=ours rules for all deleted paths |

## Conventions

- Commit format: `{type}({scope}): {description}`
- Binary name: `muninndb-lite`
- MCP tool prefix: `muninn_` (unchanged from upstream for compatibility)
- Default data directory: `~/.muninn/data`
- Distribution: GitHub Releases (compiled binaries) or `git clone` + `go build`
- Versioning: follows upstream tags with `-lite` suffix (e.g. upstream `v0.4.1-alpha` → `v0.4.1-alpha-lite`). Release workflow triggers on `v*-lite` tags only.

## License

BSL 1.1 (inherited from MuninnDB). Free for individuals, orgs <50 employees and <$5M revenue, internal use. Becomes Apache 2.0 in February 2030. Author (MJ Bonanno / Scrypster) is aware of this fork.
