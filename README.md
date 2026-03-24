# MuninnDB Lite

**Cognitive memory for AI agents** — single binary, MCP-only, zero configuration.

Lightweight fork of [MuninnDB](https://github.com/scrypster/muninndb). Same cognitive engine. Stripped down to MCP stdio for embedding into any AI project.

[![CI](https://github.com/Aperrix/muninndb-lite/actions/workflows/ci.yml/badge.svg)](https://github.com/Aperrix/muninndb-lite/actions/workflows/ci.yml)
[![Release](https://img.shields.io/github/v/release/Aperrix/muninndb-lite)](https://github.com/Aperrix/muninndb-lite/releases/latest)
[![License](https://img.shields.io/badge/license-BSL%201.1-blue)](LICENSE)
[![Go](https://img.shields.io/badge/go-1.25%2B-00ADD8)](https://go.dev)

> **Prerequisites:** None. Single binary, zero dependencies.
> To uninstall: `rm $(which muninndb-lite)` and delete `~/.muninn`.

---

## Install

**macOS / Linux:**

```bash
curl -fsSL https://raw.githubusercontent.com/Aperrix/muninndb-lite/develop/install.sh | sh
```

Then add to your AI tool:

```bash
# Claude Code
claude mcp add --transport stdio muninn -- muninndb-lite mcp
```

That's it. On first call, the database initializes automatically in `~/.muninn/data`.

<details>
<summary>Other MCP clients</summary>

Add to your config file:

```json
{
  "mcpServers": {
    "muninn": {
      "command": "muninndb-lite",
      "args": ["mcp"],
      "env": {}
    }
  }
}
```

| Client | Config file |
|---|---|
| Claude Desktop | `~/Library/Application Support/Claude/claude_desktop_config.json` (macOS) |
| Cursor | `~/.cursor/mcp.json` |
| Windsurf | `~/.codeium/windsurf/mcp_config.json` |
| VS Code / Copilot | `.vscode/mcp.json` |
| OpenClaw | `~/.openclaw/openclaw.json` |
| OpenCode | `~/.config/opencode/opencode.json` |

</details>

<details>
<summary>Manual download</summary>

Binaries available at [GitHub Releases](https://github.com/Aperrix/muninndb-lite/releases/latest):

| Platform | Binary |
|---|---|
| Linux amd64 | `muninndb-lite-linux-amd64` |
| Linux arm64 | `muninndb-lite-linux-arm64` |
| macOS Apple Silicon | `muninndb-lite-darwin-arm64` |
| macOS Intel | `muninndb-lite-darwin-amd64` |
| Windows | `muninndb-lite-windows-amd64.exe` |

</details>

<details>
<summary>Build from source</summary>

```bash
git clone https://github.com/Aperrix/muninndb-lite.git
cd muninndb-lite
go build -o muninndb-lite ./cmd/muninn/
```

</details>

---

## Why This Fork Exists

[MuninnDB](https://github.com/scrypster/muninndb) is a full-featured cognitive memory database with REST, gRPC, MCP, a web UI, multi-node clustering, built-in ONNX embeddings, and SDKs in six languages. It is designed to run as a standalone server.

That's more than most AI agents need.

MuninnDB Lite exists for a different use case: **embedding cognitive memory directly into an AI tool's MCP configuration**. One line in your config, one subprocess, no server to manage.

### What was removed

| Removed | Why |
|---|---|
| REST API, gRPC, MBP protocols | MCP stdio is the only transport needed for agent integration |
| Web UI (dashboard, graph visualizer) | The AI agent is the interface |
| Multi-node clustering (Raft consensus) | Single-node is sufficient for local agent memory |
| Bundled ONNX embedder | Heavy native dependency; Ollama or API providers cover this |
| SDKs (Go, Python, Node, PHP, Kotlin, Swift) | They target REST/gRPC which are removed; MCP is the access layer |
| Admin CLI (cluster, upgrade, REPL) | Not needed for embedded usage |

### What is identical

The cognitive engine, storage layer, and MCP tools are **the same code** as MuninnDB. This is not a reimplementation — it is MuninnDB with fewer entry points. Specifically:

| Kept (identical to MuninnDB) | Documentation |
|---|---|
| Cognitive primitives (Ebbinghaus decay, Hebbian learning, Bayesian confidence, predictive activation) | [Cognitive Primitives](https://github.com/scrypster/muninndb/blob/develop/docs/cognitive-primitives.md) |
| 6-phase activation pipeline (BM25 + vector + decay + Hebbian + graph traversal + ACT-R/CGDN scoring) | [Retrieval Design](https://github.com/scrypster/muninndb/blob/develop/docs/retrieval-design.md) |
| Pebble KV storage (engrams, entities, associations, cache, archive, WAL) | [Architecture](https://github.com/scrypster/muninndb/blob/develop/docs/architecture.md) |
| 36 MCP tools | [MuninnDB Docs](https://muninndb.com/docs) |
| Entity knowledge graph (extraction, relationships, clusters, timeline) | [Entity Graph](https://github.com/scrypster/muninndb/blob/develop/docs/entity-graph.md) |
| Hierarchical memory (trees, parent-child) | [Hierarchical Memory](https://github.com/scrypster/muninndb/blob/develop/docs/hierarchical-memory.md) |
| Vault isolation (multi-tenant, per-vault cognitive config) | [Auth & Vaults](https://github.com/scrypster/muninndb/blob/develop/docs/auth.md) |
| Embedding providers (Ollama, OpenAI, Voyage, Cohere, Google, Jina, Mistral) | [Plugins](https://github.com/scrypster/muninndb/blob/develop/docs/plugins.md) |
| LLM enrichment (entity extraction, summarization, classification) | [Plugins](https://github.com/scrypster/muninndb/blob/develop/docs/plugins.md) |
| Contradiction detection, consolidation, deduplication | [Feature Reference](https://github.com/scrypster/muninndb/blob/develop/docs/feature-reference.md) |
| Soft-delete, restore, provenance tracking | [Feature Reference](https://github.com/scrypster/muninndb/blob/develop/docs/feature-reference.md) |

For deep documentation on any of these features, refer to the [MuninnDB docs](https://github.com/scrypster/muninndb/tree/develop/docs).

### How the fork stays in sync

MuninnDB Lite is a minimal-diff fork. The `internal/` packages are kept intact — only the entry point (`cmd/`) is modified and isolated packages with no wired imports are removed. Upstream changes merge cleanly. Versioning follows upstream with a `-lite` suffix (e.g. `v0.4.1-alpha-lite`).

---

## How It Works

Your AI agent stores and retrieves memories through 36 MCP tools. Memories are not just stored — they are **cognitively processed**:

```
Your AI Agent                    MuninnDB Lite
     |                                |
     |-- muninn_remember ------------>|  Store memory + index
     |                                |  (BM25 full-text + optional vectors)
     |                                |  Initialize cognitive scores
     |                                |
     |-- muninn_recall -------------->|  BM25 search + cognitive scoring:
     |                                |    - Ebbinghaus decay (recent = stronger)
     |                                |    - Hebbian learning (co-activated = linked)
     |                                |    - Bayesian confidence (confirmed = trusted)
     |                                |    - ACT-R temporal weighting
     |<-- ranked results -------------|
     |                                |
     |-- muninn_link ---------------->|  Create associations between memories
     |-- muninn_contradictions ------>|  Detect conflicting memories
     |-- muninn_traverse ------------>|  Walk the knowledge graph
```

Memories that are used together get linked automatically. Memories that aren't accessed fade naturally. Contradictions are detected. The database evolves while the agent works.

For a deep dive into the cognitive mechanics, see [How Memory Works](https://github.com/scrypster/muninndb/blob/develop/docs/how-memory-works.md) in the MuninnDB docs.

---

## MCP Tools

On first connect, call `muninn_guide` — the database returns usage instructions adapted to its current configuration.

All 36 tools are identical to MuninnDB. For detailed parameter documentation, see the [MuninnDB MCP reference](https://muninndb.com/docs).

### Core

| Tool | Description |
|---|---|
| `muninn_remember` | Store a memory (concept, content, tags, entities, relationships) |
| `muninn_remember_batch` | Store up to 50 memories in one call |
| `muninn_recall` | Retrieve memories by context (cognitive activation) |
| `muninn_read` | Read a specific memory by ID |
| `muninn_forget` | Soft-delete a memory |
| `muninn_evolve` | Update an existing memory |
| `muninn_link` | Create a typed relationship between two memories |
| `muninn_guide` | Get usage instructions for the current vault configuration |
| `muninn_status` | Server and vault status |
| `muninn_where_left_off` | Resume where the last session ended |

### Cognitive

| Tool | Description |
|---|---|
| `muninn_contradictions` | Detect contradictory memories |
| `muninn_consolidate` | Merge redundant memories |
| `muninn_explain` | Explain why a memory was returned |
| `muninn_decide` | Decision support based on stored memories |
| `muninn_feedback` | Provide feedback on recall quality |

### Knowledge Graph

| Tool | Description |
|---|---|
| `muninn_entities` | List known entities |
| `muninn_entity` | Entity details |
| `muninn_entity_clusters` | Clusters of related entities |
| `muninn_entity_state` | Set lifecycle state of an entity (active, deprecated, merged, resolved) |
| `muninn_entity_state_batch` | Batch update lifecycle state for multiple entities (max 50) |
| `muninn_entity_timeline` | Entity timeline |
| `muninn_find_by_entity` | Find memories mentioning an entity |
| `muninn_merge_entity` | Merge duplicate entities |
| `muninn_similar_entities` | Find similar entities |
| `muninn_traverse` | Walk the relationship graph |
| `muninn_export_graph` | Export the knowledge graph |

### Hierarchy & Maintenance

| Tool | Description |
|---|---|
| `muninn_remember_tree` | Store a hierarchy of memories |
| `muninn_recall_tree` | Retrieve a memory tree |
| `muninn_add_child` | Add a child to a hierarchical memory |
| `muninn_session` | Session management (vault pinning) |
| `muninn_state` | Detailed memory state (scores, metadata) |
| `muninn_list_deleted` | List soft-deleted memories |
| `muninn_restore` | Restore a deleted memory |
| `muninn_replay_enrichment` | Replay enrichment on a memory |
| `muninn_retry_enrich` | Retry failed enrichment |
| `muninn_provenance` | Memory operation history |

---

## Inline Enrichment

Your AI agent can provide structured data directly in `muninn_remember`, bypassing any need for server-side LLM enrichment:

```json
{
  "content": "Chose PostgreSQL for persistence over MongoDB",
  "concept": "database choice",
  "summary": "Evaluated databases; PostgreSQL selected for ACID compliance",
  "entities": [
    {"name": "PostgreSQL", "type": "database"},
    {"name": "MongoDB", "type": "database"}
  ],
  "entity_relationships": [
    {"from_entity": "PostgreSQL", "to_entity": "MongoDB", "rel_type": "chosen_over", "weight": 0.9}
  ]
}
```

When `entities` and `summary` are provided inline, no server-side LLM call is needed. The agent structures the data, MuninnDB Lite stores it with full cognitive processing.

This is the recommended approach for MuninnDB Lite: the AI agent enriches at write time, no API key required on the server.

### Client-provided embeddings

Agents that compute their own embeddings can pass them directly via the `embedding` parameter on `muninn_remember`, `muninn_recall`, `muninn_explain`, `muninn_evolve`, and `muninn_add_child`. When provided, the server skips its own embedding step and uses the vector directly. The dimension must match the vault's existing embedding dimension.

---

## Optional: Embedding Providers

By default, MuninnDB Lite uses BM25 full-text search combined with cognitive scoring. This works well for most use cases.

For improved semantic recall (especially above ~500 memories), configure an embedding provider via environment variables:

| Provider | Env var | Notes |
|---|---|---|
| Ollama | `MUNINN_OLLAMA_URL=ollama://localhost:11434/nomic-embed-text` | Local, free |
| OpenAI | `MUNINN_OPENAI_KEY=sk-...` | text-embedding-3-small |
| Voyage AI | `MUNINN_VOYAGE_KEY=pa-...` | Top MTEB benchmarks |
| Cohere | `MUNINN_COHERE_KEY=...` | embed-v4 |
| Google | `MUNINN_GOOGLE_KEY=...` | Gemini embeddings |
| Jina | `MUNINN_JINA_KEY=...` | jina-embeddings-v3 |
| Mistral | `MUNINN_MISTRAL_KEY=...` | mistral-embed |

```json
{
  "mcpServers": {
    "muninn": {
      "command": "muninndb-lite",
      "args": ["mcp"],
      "env": {
        "MUNINN_OLLAMA_URL": "ollama://localhost:11434/nomic-embed-text"
      }
    }
  }
}
```

When an embedding provider is configured, MuninnDB Lite combines BM25 + vector similarity + cognitive scoring for retrieval. When none is configured, BM25 + cognitive scoring is used automatically.

For details on embedding providers, see [Plugins](https://github.com/scrypster/muninndb/blob/develop/docs/plugins.md) in the MuninnDB docs.

---

## Optional: LLM Enrichment

Server-side enrichment extracts entities, relationships, and summaries automatically in the background. This is optional — inline enrichment from the agent is the recommended approach.

| Provider | Configuration |
|---|---|
| Ollama | `MUNINN_ENRICH_URL=ollama://localhost:11434/llama3` |
| OpenAI-compatible | `MUNINN_ENRICH_URL=openai://gpt-4o-mini` + `MUNINN_ENRICH_API_KEY` |
| Anthropic | `MUNINN_ENRICH_URL=anthropic://claude-haiku-4-5-20251001` + `MUNINN_ANTHROPIC_KEY` |
| Google Gemini | `MUNINN_ENRICH_URL=google://gemini-1.5-flash` + `MUNINN_GOOGLE_KEY` |

For details on enrichment configuration, see [Plugins](https://github.com/scrypster/muninndb/blob/develop/docs/plugins.md) in the MuninnDB docs.

---

## Configuration Reference

| Env var | Default | Description |
|---|---|---|
| `MUNINNDB_DATA` | `~/.muninn/data` | Data directory |
| **Embedding providers** | | |
| `MUNINN_OLLAMA_URL` | *(none)* | Ollama embedding endpoint |
| `MUNINN_OPENAI_KEY` | *(none)* | OpenAI API key (embeddings) |
| `MUNINN_OPENAI_URL` | *(none)* | OpenAI base URL override (custom endpoints) |
| `MUNINN_VOYAGE_KEY` | *(none)* | Voyage AI API key |
| `MUNINN_COHERE_KEY` | *(none)* | Cohere API key |
| `MUNINN_GOOGLE_KEY` | *(none)* | Google API key (Gemini embeddings + enrichment) |
| `MUNINN_JINA_KEY` | *(none)* | Jina API key |
| `MUNINN_MISTRAL_KEY` | *(none)* | Mistral API key |
| **LLM enrichment** | | |
| `MUNINN_ENRICH_URL` | *(none)* | LLM enrichment provider URL |
| `MUNINN_ENRICH_API_KEY` | *(none)* | API key for enrichment provider |
| `MUNINN_ANTHROPIC_KEY` | *(none)* | Anthropic API key (enrichment fallback) |
| `MUNINN_ENRICH_TIMEOUT` | *(none)* | Per-engram LLM timeout for replay_enrichment (e.g. `60s`, `2m`) |
| **Auth & security** | | |
| `MUNINN_MCP_TOKEN` | *(none)* | Bearer token for MCP auth (alternative to `--mcp-token` flag) |
| `MUNINN_TLS_CERT` | *(none)* | Path to TLS certificate file (PEM) |
| `MUNINN_TLS_KEY` | *(none)* | Path to TLS private key file (PEM) |
| **Tuning** | | |
| `MUNINN_MEM_LIMIT_GB` | `4` | Go memory limit in GB |
| `MUNINN_GC_PERCENT` | `200` | Go GC target percentage |
| `MUNINN_HNSW_WARN_THRESHOLD_MB` | *(none)* | Warn when HNSW in-memory vectors exceed N MB |
| `MUNINN_HNSW_MAX_MB` | *(none)* | Skip HNSW insert when memory exceeds N MB |

For the full configuration reference including vault-level cognitive tuning (plasticity presets, decay rates, Hebbian weights, ACT-R parameters), see [Feature Reference](https://github.com/scrypster/muninndb/blob/develop/docs/feature-reference.md) in the MuninnDB docs.

---

## License

MuninnDB Lite inherits the **Business Source License 1.1** (BSL 1.1) from MuninnDB.

- Free for individuals, hobbyists, researchers, and open-source projects.
- Free for small organizations (<50 employees **and** <$5M revenue).
- Free for all internal use.
- Automatically becomes Apache 2.0 on February 26, 2030.

Full terms: [LICENSE](LICENSE).

---

*Lightweight fork of [MuninnDB](https://github.com/scrypster/muninndb) by [MJ Bonanno](https://scrypster.com).*
*Named after Muninn, one of Odin's ravens, whose name means "memory" in Old Norse.*
