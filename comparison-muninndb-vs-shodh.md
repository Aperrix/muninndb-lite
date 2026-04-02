# MuninnDB vs Shodh Memory — Feature Comparison

Comparative analysis of [MuninnDB](https://github.com/scrypster/muninndb) (v0.4.9-alpha) and [Shodh Memory](https://github.com/varun29ankuS/shodh-memory). Analysis date: 2026-04-02. Revised after upstream code review.

---

## Architecture

| | MuninnDB | Shodh Memory |
|---|---|---|
| **Language** | Go | Rust + TypeScript (MCP bridge) + Python (PyO3) |
| **Storage** | Pebble (LSM, CockroachDB) | RocksDB (9 column families) |
| **Vector index** | Custom HNSW (M=16, Ef=50) | Custom Vamana/DiskANN + SPANN + Product Quantization |
| **Full-text search** | Custom BM25 (k1=1.2, b=0.75, Snowball stemming) | Tantivy (Rust-native Lucene alternative) |
| **Transports** | REST (77 endpoints) + gRPC (protobuf) + MCP stdio | REST (Axum) + MCP stdio (TS bridge) + MCP native (rmcp) + WebSocket/SSE + Zenoh |
| **Embeddings** | **Local ONNX** (bge-small-en-v1.5, 384d) + 7 HTTP providers (Ollama, OpenAI, Voyage, Cohere, Google, Jina, Mistral) | **Local ONNX** (MiniLM-L6-v2, 384d), no external providers |
| **Enrichment** | LLM-based (Ollama, OpenAI, Anthropic, Google), 4 stages | None — local NLP only (POS + YAKE + optional neural NER) |
| **Web UI** | Yes — SPA dashboard with vault management, memory browser, entity graph, live log streaming (SSE) | TUI dashboard (terminal-based) |
| **Clustering** | Yes — Cortex/Lobe (leader/follower) replication with cognitive forwarding, leader election, quorum, mTLS, graceful failover | No |
| **SDKs** | Go, Python (+ LangChain), Node.js, PHP, Kotlin, Swift | Python (PyO3/maturin), npm (MCP bridge) |
| **License** | BSL 1.1 (Apache 2.0 in Feb 2030) | Apache 2.0 |

## Memory Model

| | MuninnDB | Shodh Memory |
|---|---|---|
| **Memory unit** | Engram | Experience |
| **Tiers** | Single-tier with continuous activation (ACT-R/CGDN scoring) | **3-tier**: Working (100 items, in-memory) -> Session (100MB, ephemeral) -> Long-Term (RocksDB) |
| **Promotion** | No promotion — activation level determines retrieval rank continuously | Automatic: importance >= 0.4 + 5min -> Session; >= 0.6 + 1h -> Long-Term |
| **Memory types** | 12 built-in (fact, decision, observation, preference, issue, task, procedure, event, goal, constraint, identity, reference) + free-form labels | 13 types (Observation, Decision, Learning, Error, Discovery, Pattern, Context, Task, CodeEdit, FileAccess, Search, Command, Conversation) + Intention |
| **Binary format** | ERF (Engram Record Format) — custom binary with magic bytes 0x4D554E4E, CRC-16, Zstd compression, v1/v2 versioning | Standard serde serialization to RocksDB |
| **Deduplication** | Write-time novelty detector (Jaccard >= 0.70, creates RelRefines) + consolidation semantic dedup (cosine >= 0.95) | **Content-hash (SHA-256)** at write time + semantic |
| **Soft-delete** | Yes, 7-day recovery window (muninn_restore) | No |
| **Versioning** | Yes (muninn_evolve archives previous version) | No |
| **Vault isolation** | Full vault system with per-vault plasticity presets (default, reference, scratchpad, knowledge-graph), 30+ tunable parameters | Per user_id only (simple prefix isolation) |
| **Emotional metadata** | No | Yes — emotional_valence (-1.0 to 1.0), emotional_arousal (0.0 to 1.0), emotion label |
| **Multimodal** | No | Media references (image, audio, video, document URIs with MIME types) |
| **Robotics fields** | No | 26 spatial/mission fields (geo_location, heading, sensor_data, robot_id, terrain_type, nearby_agents, etc.) |

## Cognitive Science Features

| Feature | MuninnDB | Shodh Memory |
|---|---|---|
| **Decay model** | Ebbinghaus exponential: R = e^(-t/S), floor 5%, stability max 365d, spacing bonus (tanh-based, up to 1.5x with 7-day spacing) | **Hybrid**: exponential < 3 days (fast noise filtering), **power-law** >= 3 days (heavy tail preserves important memories). Based on Wixted 2004. |
| **Hebbian learning** | Co-activation detection, SGD-style weight updates, LR=0.01, batch every 1min, configurable weight decay | Co-activation + **LTP** (Long-Term Potentiation — edges become permanent after threshold co-activations) + **asymmetric 4:1 penalty** (false positives penalized 4x harder) + importance floor 0.05 |
| **Bayesian confidence** | Yes — Welford online variance, confidence stored as x 1e6 for precision | No (simple credibility field without Bayesian updating) |
| **Contradiction** | Dedicated module — precomputed 64x64 boolean matrix, severity scoring (direct negation 1.0, incompatible relations 0.9), pairwise checking | **Memory interference** — retroactive interference (new disrupts old) + proactive interference (old disrupts new), competition mechanics during retrieval. Based on Anderson & Neely 1996. |
| **Activation model** | **ACT-R** (Anderson 1993): B(M) = ln(n+1) - d * ln(age/(n+1)), power-law decay, Hebbian inside softplus. **CGDN** (Cognitive-Gated Divisive Normalization): lateral inhibition, Ebbinghaus gate x Hebbian gate / normalization. Both available, configurable per vault. | Spreading activation (Dijkstra-style best-first graph traversal with edge weights) |
| **Consolidation** | **Dream engine** — 5 phases: Orient (vault stats), Activation Replay, Semantic Dedup (cosine 0.95, dream mode 0.85), Schema Promotion (future), Transitive Inference (A->B->C with weight >= 0.7) | **Memory replay** — hippocampal-style replay (Rasch & Born 2013), high-value memories replayed during consolidation, co-activation strengthens related memories |
| **Feedback** | **Explicit** — muninn_feedback tool, SGD learning loop on scoring weights | **Implicit** — behavioral analysis (entity overlap between surfaced memories and subsequent actions, tool-usage attribution, semantic similarity, negative keyword detection, topic change detection), momentum-based EMA updates |
| **Novelty detection** | Jaccard similarity on 30-term fingerprints, 0.70 threshold, LRU cache (1000/vault, 16 shards) | Not mentioned |
| **Working memory** | Session-scoped attention buffer with exponential halflife decay, max items with LRU eviction, promotion to long-term | 3-tier model with explicit Working Memory tier (100 items, in-memory, recent/highly active) |
| **Episodic memory** | Durable ordered sequences of engram activations (episodes with frames, positions, timestamps, notes) | Episode tracking with bi-temporal entity interactions, episode sources (Message, Document, Event, Observation) |

## Search & Retrieval

| | MuninnDB | Shodh Memory |
|---|---|---|
| **Pipeline** | BM25 + HNSW + decay + time + transition candidates -> **phase3RRF fusion** -> ACT-R scoring (default) or CGDN | BM25 -> Vector (Vamana) -> **RRF fusion** (BM25 0.35 + Vector 0.40 + graph dynamic/density-based) -> cognitive boost |
| **Traversal profiles** | 5 profiles: default (balanced), causal (cause/effect chains), confirmatory (supporting evidence), adversarial (conflicts), structural (hierarchy) | 3 modes: semantic, associative, hybrid |
| **Recall modes** | 4 modes: semantic (high-precision vector), recent (recency-biased), balanced (defaults), deep (4 hops, threshold 0.1) | Same 3: semantic, associative, hybrid |
| **Graph traversal** | BFS with 0.7x hop penalty, max 500 nodes, 20 edges/node, weight-sorted prefix keys, optional entity-link following (0.1 weight) | Dijkstra-style best-first with edge weights, ontological type-aware filtering |
| **Score explain** | Yes (muninn_explain — full breakdown per memory) | No |
| **Proactive surfacing** | **Yes** — trigger system with HNSW sweep every 30s, context-based subscriptions, ActivationPush delivery | **Yes** — push-based in MCP bridge layer, surfaces memories on every non-memory tool call, < 30ms target |
| **BM25 field weights** | Concept 3.0, Tags 2.0, Content 1.0, CreatedBy 0.5 | Not documented (Tantivy-based) |

## Entity & Knowledge Graph

| | MuninnDB | Shodh Memory |
|---|---|---|
| **Entity extraction** | LLM-based (4 providers: Ollama, OpenAI, Anthropic, Google) or client-provided inline | **Local** — Capitalization-based proper noun detection + YAKE keyword extraction (Rust crate) + optional neural NER (ONNX bert-tiny). Zero LLM calls. |
| **Entity types** | 13 types: person, organization, location, concept, technology, project, tool, database, service, framework, language, product, event | 10+ types: Person, Organization, Location, Technology, Concept, Event, Date, Product, Skill, Keyword |
| **Entity lifecycle** | Rich — active, deprecated, merged, resolved. Tools: state, state_batch, similar_entities, merge_entity | Basic — entities, relationships, episodes |
| **Entity timeline** | Yes (muninn_entity_timeline) | Episode-based temporal tracking |
| **Co-occurrence** | Yes (muninn_entity_clusters with min_count, top_n) | Yes (entity_pair_index in RocksDB) |
| **Similarity detection** | Trigram similarity for duplicate detection (muninn_similar_entities) | Embedding similarity with merge threshold |
| **Graph export** | JSON-LD (W3C standard) + GraphML | No |
| **Spreading activation** | Via BFS traversal profiles (causal, confirmatory, adversarial, structural) | Native spreading activation with Dijkstra-style traversal, ontological type-aware boosting |

## MCP Tools

| Category | MuninnDB (36 tools) | Shodh Memory (38-47 tools) |
|---|---|---|
| **Memory CRUD** | remember, remember_batch, recall, read, forget, restore | remember, recall, list_memories, read_memory, forget, reinforce |
| **Proactive** | — | proactive_context, context_summary |
| **Associations** | link, traverse, contradictions | (via graph spreading activation) |
| **Evolution** | evolve, consolidate | — |
| **State management** | state (7 lifecycle states), list_deleted | — |
| **Decision capture** | decide (with rationale, alternatives, evidence) | — |
| **Feedback** | feedback (SGD learning) | (implicit, no explicit tool) |
| **Entity management** | find_by_entity, entity, entities, entity_state, entity_state_batch, entity_clusters, similar_entities, merge_entity, entity_timeline | (basic, via graph_memory module) |
| **Hierarchical** | remember_tree, recall_tree, add_child | (parent_id field on memories) |
| **Enrichment** | retry_enrich, replay_enrichment | — |
| **Export** | export_graph (JSON-LD, GraphML) | — |
| **Diagnostics** | explain, session, status, provenance, guide, where_left_off | memory_stats, verify_index, repair_index, consolidation_report |
| **Todos/GTD** | — | **12 tools**: add_todo, list_todos, update_todo, complete_todo, delete_todo, reorder_todo, list_subtasks, add_todo_comment, list_todo_comments, update_todo_comment, delete_todo_comment, todo_stats |
| **Projects** | — | **4 tools**: add_project, list_projects, archive_project, delete_project |
| **Reminders** | — | **3 tools**: set_reminder, list_reminders, dismiss_reminder |
| **Backup** | — (CLI only) | **5 tools**: backup_create, backup_list, backup_verify, backup_restore, backup_purge |
| **Token tracking** | — | token_status, reset_token_session |
| **MCP prompts** | — | 6 slash commands: quick_recall, session_summary, what_i_know, pending_work, recent_memories, memory_health |

## Infrastructure & Operations

| | MuninnDB | Shodh Memory |
|---|---|---|
| **Clustering** | **Yes** — Cortex/Lobe architecture, cognitive forwarding (Hebbian associations replicated across nodes), leader election, quorum tracking, graceful failover, mTLS, join protocol with one-time tokens | No |
| **Replication** | WAL-backed replication log, per-Lobe streams, Muninn Binary Protocol framing, partition healing with cognitive reconciliation | No |
| **Metrics** | Prometheus (counters, histograms, gauges) — writes, activations, novelty drops, FTS/write/activate/read latency, embed pending | Basic memory_stats |
| **Backup** | Scheduled periodic backups (Pebble checkpoint + WAL + auth), retention policy, point-in-time, offline verification | MCP-exposed: backup_create, backup_list, backup_verify, backup_restore, backup_purge. SHA-256 verification. |
| **Auth** | Admin users (bcrypt/Argon2), API keys (3 modes: full/observe/write), vault-scoped, expiry support, middleware guards | API keys (env-based), dev mode key |
| **Deployment** | Binary, Docker, docker-compose, install scripts (Linux/macOS/Windows) | Binary, Docker, npx, pip, brew tap, cargo install |
| **Web dashboard** | SPA with vault management, memory browser, entity graph visualization, cognitive pipeline editor, live log streaming (SSE), API key management | TUI dashboard (terminal-based monitoring) |
| **CLI** | 14+ subcommands: init, start, stop, restart, status, shell, logs, exec, dream, backup, upgrade, cluster, vault, api-key, admin, completion | Server binary + npx MCP bridge |

## Plasticity System (MuninnDB Exclusive)

MuninnDB offers per-vault cognitive tuning with 4 presets and 30+ tunable parameters:

| Preset | Hebbian | Temporal Decay | Hop Depth | Use Case |
|--------|---------|----------------|-----------|----------|
| `default` | Enabled | Enabled | 2 | General-purpose balanced retrieval |
| `reference` | Enabled | **Disabled** | 3 | Knowledge bases — facts don't decay |
| `scratchpad` | **Disabled** | Enabled | 0 | Temporary notes — recent > frequent |
| `knowledge-graph` | Enabled | Enabled | 4 | Deep entity relationship exploration |

Additional tunable parameters: ACT-R decay exponent, Hebbian scale, semantic/FTS weight balance, relevance floor, temporal halflife, traversal profile, CGDN toggle, predictive activation, max engrams, retention days, association decay/pruning, behavior mode (autonomous/prompted/selective), inline enrichment policy, recall mode.

Shodh has no equivalent — all memories share the same cognitive pipeline configuration.

---

## Features Exclusive to Each

### Only in MuninnDB

| Feature | Detail |
|---|---|
| Vault isolation + plasticity presets | Per-vault cognitive tuning with 4 presets and 30+ parameters |
| Hierarchical tree memory | remember_tree, recall_tree, add_child with ordinal preservation |
| Entity lifecycle management | 9 tools: state, state_batch, similar, merge, timeline, clusters, entity, entities, find_by |
| Dream engine | Multi-phase consolidation: orient, replay, dedup, schema, transitive inference |
| Provenance audit trail | Ordered history of who wrote/changed what, with source types (Human, LLM, Document, Inferred, etc.) |
| MQL query language | ACTIVATE with WHERE predicates (State, Score, Tag, Creator, CreatedAfter, AND/OR) |
| Trigger system | Push subscriptions with context-based delivery, delta thresholds, circuit breaker |
| 5 traversal profiles | causal, confirmatory, adversarial, structural, default |
| Score explain | Full per-memory scoring breakdown (muninn_explain) |
| Soft-delete + restore | 7-day recovery window |
| Memory versioning | muninn_evolve archives old version |
| Export JSON-LD/GraphML | W3C standard entity graph export |
| Transitive inference | A->B->C with peak weight fallback |
| Clustering/replication | Cortex/Lobe with cognitive forwarding |
| Web UI dashboard | SPA with entity graph viz, live logs, vault management |
| gRPC transport | Protobuf-based with mobile SDKs (Kotlin/Swift) |
| Multiple LLM enrichment providers | 4 providers x 4 stages (entities, relationships, classification, summary) |
| 8 embedding providers | Local ONNX + 7 HTTP (Ollama, OpenAI, Voyage, Cohere, Google, Jina, Mistral) |
| Prometheus metrics | Counters, histograms, gauges with HTTP scrape endpoint |
| Novelty detection | Jaccard fingerprinting at write time |
| Extractive brief generation | Top-5 sentence extraction per engram |
| Coherence metrics | Atomic counters: orphans, contradictions, refines, confidence variance |
| ACT-R + CGDN activation | Two cognitive activation models, configurable per vault |

### Only in Shodh Memory

| Feature | Detail |
|---|---|
| **MCP bridge-level proactive surfacing** | Intercepts every non-memory tool call in TS bridge layer (MuninnDB has server-side trigger system instead) |
| **Implicit feedback** | Learns from agent behavior (entity overlap, tool-usage attribution, semantic similarity, repetition/topic detection), no explicit signal needed |
| **Memory replay** | Hippocampal-style consolidation (Rasch & Born 2013), high-value memories replayed with co-activation strengthening |
| **Memory interference** | Retroactive + proactive interference modeling (Anderson & Neely 1996), competition mechanics during retrieval |
| **Hybrid exponential + power-law decay** | Wixted 2004: exponential < 3d (noise filtering), power-law >= 3d (heavy tail preserves old important memories) |
| **LTP (Long-Term Potentiation)** | Hebbian edges become permanent after threshold co-activations, slower decay (0.5x lambda) |
| **GTD task management** | 12 todo tools + 4 project tools + 3 reminder tools = complete Getting Things Done system |
| **Cross-encoder reranking** | Mentioned in Shodh's module header comment but **not implemented** — no actual cross-encoder code exists |
| **RRF (Reciprocal Rank Fusion)** | Score fusion: BM25 (0.35) + Vector (0.40) + Graph (0.25) instead of simple weighted sum |
| **Content-hash dedup at write time** | SHA-256 check prevents exact duplicates before storage |
| **Emotional valence/arousal** | Per-memory sentiment metadata (-1.0 to 1.0 valence, 0.0 to 1.0 arousal) |
| **Robotics integration** | Zenoh/ROS2 transport, 26 spatial/mission fields per memory |
| **A/B testing framework** | Built-in scoring weight experimentation per user |
| **Token tracking** | Context window usage monitoring per session |
| **Prospective memory** | Time-based and context-based future reminders (Intention type) |
| **MIF encrypted export** | AES-256-GCM encrypted memory interchange format |
| **MCP prompts** | 6 slash commands (quick_recall, session_summary, what_i_know, pending_work, recent_memories, memory_health) |
| **Multimodal references** | Image, audio, video, document URIs with MIME types |
| **Zero-LLM entity extraction** | Capitalization heuristics + YAKE (Rust) + optional neural NER (ONNX) — no external API needed |
| **Python bindings** | Native Rust -> Python via PyO3/maturin |
| **3-tier memory model** | Working (in-memory) -> Session (ephemeral) -> Long-Term (persistent) with automatic promotion |

---

## Interesting Shodh Features That Could Benefit MuninnDB

Ranked by potential impact:

### 1. Hybrid Exponential + Power-Law Decay (HIGH IMPACT)

**What:** Replace pure Ebbinghaus exponential decay with Wixted's hybrid model — exponential for t < 3 days (fast noise filtering), power-law for t >= 3 days (heavy tail preserves important memories).

**Why it matters:** Pure exponential decay drops off too aggressively for long-term memories. Power-law decay (R = a * t^(-b)) better matches empirical forgetting curves for memories older than a few days (Wixted & Ebbesen 1991, Wixted 2004). This means old but important memories survive longer without requiring access/reinforcement.

**Implementation complexity:** Low — modify the formula in `internal/cognitive/decay.go`. The transition point (3 days) and power-law exponent are simple constants. Could be a plasticity parameter.

### 2. MCP Bridge-Level Proactive Surfacing (MEDIUM IMPACT — MuninnDB already has server-side equivalent)

**What:** Shodh intercepts every non-memory MCP tool call in the TypeScript bridge layer and appends relevant memories to the response. This is a transport-layer approach vs MuninnDB's server-side trigger system (HNSW sweep every 30s with ActivationPush).

**Why it matters:** MuninnDB's trigger system is more architecturally sound (server-side, configurable thresholds, circuit breakers) but requires explicit subscription setup. Shodh's approach is zero-config — it works immediately without the agent needing to subscribe. The two approaches are complementary, not exclusive.

**Implementation complexity:** Low — could be a thin MCP-layer wrapper that subscribes to the trigger system automatically on session start, or a `muninn_proactive` tool that returns recent ActivationPush results.

### 3. Implicit Feedback (HIGH IMPACT)

**What:** Learn from agent behavior rather than requiring explicit `muninn_feedback` calls. Analyze entity overlap between surfaced memories and subsequent actions, tool-usage attribution, semantic similarity between recalled content and what the agent actually does with it.

**Why it matters:** Most agents never call explicit feedback tools. Implicit signals (did the agent use the information? did it contradict it? did it ignore it?) are richer and more natural. Shodh uses momentum-based EMA updates with type-dependent inertia.

**Implementation complexity:** Medium — MuninnDB already has the SGD feedback loop. Adding behavioral signal extraction would require: (1) tracking which memories were surfaced in a session, (2) analyzing subsequent tool calls for entity/content overlap, (3) feeding composite signals into the existing SGD pipeline.

### 4. Content-Hash Deduplication at Write Time (HIGH IMPACT, LOW EFFORT)

**What:** SHA-256 hash of content checked before storage. Exact duplicate content is rejected immediately.

**Why it matters:** LLM agents frequently store the same information multiple times (retries, conversation loops, rephrased identical content). Catching exact duplicates at write time is O(1) and eliminates noise before it enters the index. MuninnDB currently only deduplicates in the consolidation pipeline (cosine >= 0.95), which runs much later and is more expensive.

**Implementation complexity:** Very low — add a SHA-256 check in the engine Write path. Store content hashes in a Pebble key prefix. Return existing engram ID on duplicate.

### 5. RRF (Reciprocal Rank Fusion) (MEDIUM IMPACT)

**What:** Replace the current weighted sum for combining BM25 + HNSW + graph scores with Reciprocal Rank Fusion: `score = sum(1/(k + rank_i))` where k=60.

**Why it matters:** Weighted sum is sensitive to score scale differences between signals (BM25 scores vs cosine similarity vs graph weights). RRF is rank-based and scale-invariant — it doesn't care about absolute scores, only relative ranking within each signal. This produces more robust fusion, especially when one signal is noisy or missing (e.g., no embeddings configured).

**Implementation complexity:** Low — modify scoring in `internal/engine/activation/`. RRF formula is 3 lines of code. Could coexist with weighted sum as a plasticity parameter.

### 6. Cross-Encoder Reranking (MEDIUM IMPACT)

**What:** After initial recall (top-50), re-score the top candidates with a more accurate model (cross-encoder) that sees query + document jointly. Return top-10.

**Why it matters:** Bi-encoder retrieval (HNSW) is fast but approximate — it embeds query and document independently. Cross-encoder scoring is 10-100x more accurate because it processes the pair jointly, capturing fine-grained interactions. This is the standard pattern in production retrieval systems (retrieve-then-rerank).

**Note:** Shodh Memory lists cross-encoder reranking in a module header comment but has not implemented it. This is a novel feature for both systems.

**Implementation complexity:** Medium — requires either a local ONNX cross-encoder model (upstream MuninnDB has ONNX runtime for bge-small-en-v1.5, but cross-encoder needs a different model/session config) or an LLM API call. Could be an optional plugin. Latency cost: 50-200ms for top-10 reranking.

### 7. LTP (Long-Term Potentiation) on Associations (MEDIUM IMPACT)

**What:** After N co-activations, a Hebbian association becomes "potentiated" — its decay rate drops to 0.5x and it effectively becomes permanent knowledge.

**Why it matters:** Repeatedly co-activated memories represent established knowledge that shouldn't decay. LTP is the biological mechanism for this (Bi & Poo 1998). MuninnDB's Hebbian learning updates weights but doesn't have a potentiation threshold — old but well-established associations can still decay away.

**Implementation complexity:** Low — add a co-activation counter to association metadata in `internal/cognitive/hebbian.go`. When counter exceeds threshold, set a potentiated flag that halves the decay rate.

### 8. Memory Interference Modeling (MEDIUM IMPACT)

**What:** Model how similar memories compete during retrieval. New memories can suppress old similar ones (retroactive interference) and old memories can suppress new similar ones (proactive interference).

**Why it matters:** MuninnDB has contradiction detection (binary: contradicts or not), but interference is a gradient — similar but non-contradictory memories still compete for activation. Modeling this improves ranking when a vault has many related memories on the same topic.

**Implementation complexity:** Medium — requires tracking retrieval competition scores in the activation pipeline. Could be integrated into the CGDN divisive normalization model (which already models lateral inhibition).

### 9. Emotional Metadata (LOW-MEDIUM IMPACT)

**What:** Per-memory emotional valence (-1.0 to 1.0) and arousal (0.0 to 1.0). Emotional memories get retrieval priority.

**Why it matters:** Emotionally significant events (crises, breakthroughs, decisions under pressure) should be recalled more readily. This aligns with the biological phenomenon of emotional memory enhancement. The metadata could be provided by the LLM client or extracted by the enrichment pipeline.

**Implementation complexity:** Low — add two float fields to the engram schema. LLM clients can provide them inline (like entities/relationships today). Scoring boost: multiply activation score by (1 + 0.2 * arousal).

### 10. A/B Testing for Scoring Weights (LOW IMPACT)

**What:** Built-in framework to experiment with different scoring weight configurations per user/vault.

**Why it matters:** Optimal scoring weights vary by use case (coding assistant vs research vs knowledge management). A/B testing allows empirical optimization rather than guessing. MuninnDB's plasticity system already supports per-vault weight tuning — adding systematic experimentation would close the loop.

**Implementation complexity:** Low — the plasticity system already parameterizes weights. Adding experiment tracking (which config was active, what feedback was received) could be a thin layer on top.

---

## Strategic Assessment

**MuninnDB's strengths:** Operational maturity (clustering, metrics, web UI, auth, backup), cognitive depth (ACT-R, CGDN, 5 traversal profiles, dream engine, transitive inference), and entity management richness (9 entity tools, trigram similarity, merge, timeline).

**Shodh's strengths:** Zero-dependency philosophy (local embeddings, local NLP, no LLM calls for storage), proactive intelligence (push-based surfacing, implicit feedback), biological fidelity (power-law decay, LTP, memory interference, hippocampal replay), and breadth (GTD task management, robotics, multimodal).

**Key philosophical difference:** MuninnDB is a *database with cognitive features* — it prioritizes operational concerns (clustering, auth, export, versioning) and gives the LLM client explicit control (36 tools). Shodh is a *cognitive system with database features* — it prioritizes biological plausibility (3-tier model, interference, replay) and autonomy (proactive surfacing, implicit feedback, zero external deps).

The most impactful features to adopt from Shodh (items 1-4 above) all share a common theme: making MuninnDB smarter without requiring more work from the LLM client. Power-law decay preserves important memories automatically. Proactive surfacing recalls without being asked. Implicit feedback learns without explicit signals. Content-hash dedup prevents noise at the source. These improvements align with MuninnDB's trajectory toward autonomous cognitive behavior (dream engine, triggers, auto-association) while requiring minimal architectural changes.
