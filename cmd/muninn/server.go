package main

import (
	"context"
	"crypto/tls"
	"flag"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	neturl "net/url"
	"os"
	"os/signal"
	"path/filepath"
	"runtime/debug"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/scrypster/muninndb/internal/auth"
	"github.com/scrypster/muninndb/internal/cognitive"
	plugincfg "github.com/scrypster/muninndb/internal/config"
	"github.com/scrypster/muninndb/internal/engine"
	"github.com/scrypster/muninndb/internal/engine/activation"
	"github.com/scrypster/muninndb/internal/engine/trigger"
	"github.com/scrypster/muninndb/internal/index/fts"
	hnswpkg "github.com/scrypster/muninndb/internal/index/hnsw"
	"github.com/scrypster/muninndb/internal/logging"
	"github.com/scrypster/muninndb/internal/mcp"
	"github.com/scrypster/muninndb/internal/metrics/latency"
	"github.com/scrypster/muninndb/internal/plugin"
	embedpkg "github.com/scrypster/muninndb/internal/plugin/embed"
	enrichpkg "github.com/scrypster/muninndb/internal/plugin/enrich"
	"github.com/scrypster/muninndb/internal/replication"
	"github.com/scrypster/muninndb/internal/storage"
	"github.com/scrypster/muninndb/internal/storage/migrate"
	"github.com/scrypster/muninndb/internal/wal"
)

const defaultMCPPort = "8750"
const defaultOpenAIEmbedProviderURL = "openai://text-embedding-3-small"

// resolveEmbedInfo reads env vars and the saved plugin config to determine the
// active embed provider + model without side-effects (no network calls).
func resolveEmbedInfo(cfg plugincfg.PluginConfig) embedInfo {
	openAIOverride := strings.TrimSpace(os.Getenv("MUNINN_OPENAI_URL"))
	openAIOverrideValid := true
	if openAIOverride != "" {
		if _, err := resolveOpenAIEmbedProviderURL(openAIOverride); err != nil {
			openAIOverrideValid = false
		}
	}

	if rawURL := os.Getenv("MUNINN_OLLAMA_URL"); rawURL != "" {
		if provCfg, err := plugin.ParseProviderURL(rawURL); err == nil {
			return embedInfo{Provider: "ollama", Model: provCfg.Model}
		}
		return embedInfo{Provider: "ollama", Model: ""}
	}
	if os.Getenv("MUNINN_OPENAI_KEY") != "" {
		if openAIOverrideValid {
			if providerURL, err := resolveOpenAIEmbedProviderURL(openAIOverride); err == nil {
				if provCfg, parseErr := plugin.ParseProviderURL(providerURL); parseErr == nil {
					return embedInfo{Provider: "openai", Model: provCfg.Model}
				}
			}
			return embedInfo{Provider: "openai", Model: "text-embedding-3-small"}
		}
	}
	if os.Getenv("MUNINN_VOYAGE_KEY") != "" {
		return embedInfo{Provider: "voyage", Model: "voyage-3"}
	}
	if os.Getenv("MUNINN_COHERE_KEY") != "" {
		return embedInfo{Provider: "cohere", Model: "embed-v4"}
	}
	if os.Getenv("MUNINN_GOOGLE_KEY") != "" {
		return embedInfo{Provider: "google", Model: "text-embedding-004"}
	}
	if os.Getenv("MUNINN_JINA_KEY") != "" {
		return embedInfo{Provider: "jina", Model: "jina-embeddings-v3"}
	}
	if os.Getenv("MUNINN_MISTRAL_KEY") != "" {
		return embedInfo{Provider: "mistral", Model: "mistral-embed"}
	}
	switch cfg.EmbedProvider {
	case "ollama":
		if cfg.EmbedURL != "" {
			if provCfg, err := plugin.ParseProviderURL(cfg.EmbedURL); err == nil {
				return embedInfo{Provider: "ollama", Model: provCfg.Model}
			}
			return embedInfo{Provider: "ollama", Model: ""}
		}
	case "openai":
		if !openAIOverrideValid {
			break
		}
		openaiSource := cfg.EmbedURL
		if openAIOverride != "" {
			openaiSource = openAIOverride
		}
		if providerURL, err := resolveOpenAIEmbedProviderURL(openaiSource); err == nil {
			if provCfg, parseErr := plugin.ParseProviderURL(providerURL); parseErr == nil {
				return embedInfo{Provider: "openai", Model: provCfg.Model}
			}
			return embedInfo{Provider: "openai", Model: "text-embedding-3-small"}
		}
		if strings.TrimSpace(openaiSource) == "" {
			return embedInfo{Provider: "openai", Model: "text-embedding-3-small"}
		}
	case "voyage":
		return embedInfo{Provider: "voyage", Model: "voyage-3"}
	case "cohere":
		return embedInfo{Provider: "cohere", Model: "embed-v4"}
	case "google":
		return embedInfo{Provider: "google", Model: "text-embedding-004"}
	case "jina":
		return embedInfo{Provider: "jina", Model: "jina-embeddings-v3"}
	case "mistral":
		return embedInfo{Provider: "mistral", Model: "mistral-embed"}
	case "none":
		return embedInfo{Provider: "none", Model: ""}
	}
	return embedInfo{Provider: "none", Model: ""}
}

// embedInfo holds provider and model info for the status display.
// This replaces the rest.EmbedInfo dependency.
type embedInfo struct {
	Provider             string
	Model                string
	HardwareAccelerated  *bool
}

// injectOpenAIBaseURL injects openAIOverride (the value of MUNINN_OPENAI_URL) as
// a base_url query param into an openai:// enrich URL, mirroring how the embed
// provider handles the same env var. No-ops when:
//   - enrichURL is not an openai:// URL
//   - enrichURL already has an explicit base_url param
//   - openAIOverride is empty or resolves to the default api.openai.com
func injectOpenAIBaseURL(enrichURL, openAIOverride string) string {
	if !strings.HasPrefix(strings.ToLower(enrichURL), "openai://") {
		return enrichURL
	}
	parsed, err := neturl.Parse(enrichURL)
	if err != nil || parsed.Query().Get("base_url") != "" {
		return enrichURL
	}
	if openAIOverride == "" {
		return enrichURL
	}
	// If MUNINN_OPENAI_URL is itself an openai:// URL, extract its base_url param.
	// If it's a plain http(s) URL, use it directly as the base URL.
	baseURL := openAIOverride
	if strings.HasPrefix(strings.ToLower(openAIOverride), "openai://") {
		p, err := neturl.Parse(openAIOverride)
		if err != nil {
			return enrichURL
		}
		b := p.Query().Get("base_url")
		if b == "" {
			return enrichURL // openai:// with no base_url = default api.openai.com, nothing to inject
		}
		baseURL = b
	}
	q := parsed.Query()
	q.Set("base_url", baseURL)
	parsed.RawQuery = q.Encode()
	return parsed.String()
}

// resolveOpenAIEmbedProviderURL resolves an OpenAI embed URL override into a
// provider URL that ParseProviderURL can handle.
func resolveOpenAIEmbedProviderURL(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return defaultOpenAIEmbedProviderURL, nil
	}
	if strings.HasPrefix(strings.ToLower(raw), "openai://") {
		if _, err := plugin.ParseProviderURL(raw); err != nil {
			return "", err
		}
		return raw, nil
	}
	providerURL := defaultOpenAIEmbedProviderURL + "?base_url=" + neturl.QueryEscape(raw)
	if _, err := plugin.ParseProviderURL(providerURL); err != nil {
		return "", err
	}
	return providerURL, nil
}

func sanitizeProviderURLForLog(providerURL string) string {
	parsed, err := neturl.Parse(providerURL)
	if err != nil {
		return providerURL
	}
	if strings.EqualFold(parsed.Scheme, "openai") {
		parsed.RawQuery = ""
		return parsed.String()
	}
	return providerURL
}

func openAIEmbedLogAttrs(providerURL string) []any {
	provCfg, err := plugin.ParseProviderURL(providerURL)
	if err != nil {
		return []any{"url", sanitizeProviderURLForLog(providerURL)}
	}
	return []any{"model", provCfg.Model, "custom_base_url", provCfg.BaseURL != "https://api.openai.com"}
}

// buildEmbedder constructs an embedder. Priority (highest to lowest):
//  1. Environment variables
//  2. Saved plugin_config.json
//  3. Bundled local ONNX model
//  4. Noop
func buildEmbedder(ctx context.Context, cfg plugincfg.PluginConfig, dataDir string) (activation.Embedder, plugin.EmbedPlugin, error) {
	const (
		ollamaURL  = "MUNINN_OLLAMA_URL"
		openaiKey  = "MUNINN_OPENAI_KEY"
		openaiURL  = "MUNINN_OPENAI_URL"
		voyageKey  = "MUNINN_VOYAGE_KEY"
		cohereKey  = "MUNINN_COHERE_KEY"
		googleKey  = "MUNINN_GOOGLE_KEY"
		jinaKey    = "MUNINN_JINA_KEY"
		mistralKey = "MUNINN_MISTRAL_KEY"
		localEmbed = "MUNINN_LOCAL_EMBED"
	)

	openAIEnvOverride := strings.TrimSpace(os.Getenv(openaiURL))
	openAIEnvOverrideInvalid := false
	if openAIEnvOverride != "" {
		if _, err := resolveOpenAIEmbedProviderURL(openAIEnvOverride); err != nil {
			openAIEnvOverrideInvalid = true
			slog.Warn("invalid OpenAI URL override detected, OpenAI embedder disabled", "env_var", openaiURL, "error", err)
		}
	}

	tryEmbedService := func(providerURL string, pcfg plugin.PluginConfig) *embedpkg.EmbedService {
		logURL := sanitizeProviderURLForLog(providerURL)
		svc, err := embedpkg.NewEmbedService(providerURL)
		if err != nil {
			slog.Warn("embedder service creation failed", "url", logURL, "error", err)
			return nil
		}
		if err := svc.Init(ctx, pcfg); err != nil {
			slog.Warn("embedder init failed, trying next provider", "url", logURL, "error", err)
			_ = svc.Close()
			return nil
		}
		return svc
	}

	if url := os.Getenv(ollamaURL); url != "" {
		slog.Info("initializing Ollama embedder", "url", url)
		if svc := tryEmbedService(url, plugin.PluginConfig{}); svc != nil {
			return embedpkg.NewEmbedServiceAdapter(svc), svc, nil
		}
	}

	if key := os.Getenv(openaiKey); key != "" {
		if !openAIEnvOverrideInvalid {
			openaiProviderURL, err := resolveOpenAIEmbedProviderURL(openAIEnvOverride)
			if err != nil {
				slog.Warn("failed to resolve OpenAI provider URL, skipping OpenAI embedder", "error", err)
			} else {
				slog.Info("initializing OpenAI embedder", openAIEmbedLogAttrs(openaiProviderURL)...)
				if svc := tryEmbedService(openaiProviderURL, plugin.PluginConfig{APIKey: key}); svc != nil {
					return embedpkg.NewEmbedServiceAdapter(svc), svc, nil
				}
			}
		}
	}

	if key := os.Getenv(voyageKey); key != "" {
		slog.Info("initializing Voyage embedder")
		if svc := tryEmbedService("voyage://voyage-3", plugin.PluginConfig{APIKey: key}); svc != nil {
			return embedpkg.NewEmbedServiceAdapter(svc), svc, nil
		}
	}

	if key := os.Getenv(cohereKey); key != "" {
		slog.Info("initializing Cohere embedder")
		if svc := tryEmbedService("cohere://embed-v4", plugin.PluginConfig{APIKey: key}); svc != nil {
			return embedpkg.NewEmbedServiceAdapter(svc), svc, nil
		}
	}

	if key := os.Getenv(googleKey); key != "" {
		slog.Info("initializing Google embedder")
		if svc := tryEmbedService("google://text-embedding-004", plugin.PluginConfig{APIKey: key}); svc != nil {
			return embedpkg.NewEmbedServiceAdapter(svc), svc, nil
		}
	}

	if key := os.Getenv(jinaKey); key != "" {
		slog.Info("initializing Jina embedder")
		if svc := tryEmbedService("jina://jina-embeddings-v3", plugin.PluginConfig{APIKey: key}); svc != nil {
			return embedpkg.NewEmbedServiceAdapter(svc), svc, nil
		}
	}

	if key := os.Getenv(mistralKey); key != "" {
		slog.Info("initializing Mistral embedder")
		if svc := tryEmbedService("mistral://mistral-embed", plugin.PluginConfig{APIKey: key}); svc != nil {
			return embedpkg.NewEmbedServiceAdapter(svc), svc, nil
		}
	}

	// 2. Saved config fallback
	if cfg.EmbedProvider != "" && cfg.EmbedProvider != "none" {
		switch cfg.EmbedProvider {
		case "local":
			if os.Getenv(localEmbed) != "0" && embedpkg.LocalAvailable() {
				slog.Info("initializing bundled local ONNX embedder from saved config", "data_dir", dataDir)
				if svc := tryEmbedService("local://all-MiniLM-L6-v2", plugin.PluginConfig{DataDir: dataDir}); svc != nil {
					return embedpkg.NewEmbedServiceAdapter(svc), svc, nil
				}
				slog.Warn("bundled local embedder init failed (saved config), falling back")
			}
		case "ollama":
			if cfg.EmbedURL != "" {
				slog.Info("initializing Ollama embedder from saved config", "url", cfg.EmbedURL)
				if svc := tryEmbedService(cfg.EmbedURL, plugin.PluginConfig{}); svc != nil {
					return embedpkg.NewEmbedServiceAdapter(svc), svc, nil
				}
			}
		case "openai":
			if openAIEnvOverrideInvalid {
				slog.Warn("invalid OpenAI URL override is set, skipping OpenAI embedder from saved config", "env_var", openaiURL)
				break
			}
			openaiSource := cfg.EmbedURL
			if openAIEnvOverride != "" {
				openaiSource = openAIEnvOverride
			}
			openaiProviderURL, err := resolveOpenAIEmbedProviderURL(openaiSource)
			if err != nil {
				if strings.TrimSpace(openaiSource) != "" {
					slog.Warn("invalid OpenAI URL in saved config, skipping OpenAI embedder", "error", err)
				} else {
					slog.Warn("failed to resolve OpenAI provider URL from saved config, skipping OpenAI embedder", "error", err)
				}
				break
			}
			slog.Info("initializing OpenAI embedder from saved config", openAIEmbedLogAttrs(openaiProviderURL)...)
			if svc := tryEmbedService(openaiProviderURL, plugin.PluginConfig{APIKey: cfg.EmbedAPIKey}); svc != nil {
				return embedpkg.NewEmbedServiceAdapter(svc), svc, nil
			}
		case "voyage":
			slog.Info("initializing Voyage embedder from saved config")
			if svc := tryEmbedService("voyage://voyage-3", plugin.PluginConfig{APIKey: cfg.EmbedAPIKey}); svc != nil {
				return embedpkg.NewEmbedServiceAdapter(svc), svc, nil
			}
		case "cohere":
			slog.Info("initializing Cohere embedder from saved config")
			if svc := tryEmbedService("cohere://embed-v4", plugin.PluginConfig{APIKey: cfg.EmbedAPIKey}); svc != nil {
				return embedpkg.NewEmbedServiceAdapter(svc), svc, nil
			}
		case "google":
			slog.Info("initializing Google embedder from saved config")
			if svc := tryEmbedService("google://text-embedding-004", plugin.PluginConfig{APIKey: cfg.EmbedAPIKey}); svc != nil {
				return embedpkg.NewEmbedServiceAdapter(svc), svc, nil
			}
		case "jina":
			slog.Info("initializing Jina embedder from saved config")
			if svc := tryEmbedService("jina://jina-embeddings-v3", plugin.PluginConfig{APIKey: cfg.EmbedAPIKey}); svc != nil {
				return embedpkg.NewEmbedServiceAdapter(svc), svc, nil
			}
		case "mistral":
			slog.Info("initializing Mistral embedder from saved config")
			if svc := tryEmbedService("mistral://mistral-embed", plugin.PluginConfig{APIKey: cfg.EmbedAPIKey}); svc != nil {
				return embedpkg.NewEmbedServiceAdapter(svc), svc, nil
			}
		}
	}

	// Local ONNX embedder is not available in muninndb-lite (assets removed).

	slog.Warn("no embedder configured, semantic similarity disabled")
	return activation.NewNoopEmbedder(), nil, nil
}

// buildEnricher constructs an EnrichService from environment variables.
// Reads MUNINN_ENRICH_URL to select provider and model. Supported schemes:
//
//	ollama://localhost:11434/llama3.2          (local, no key required)
//	openai://gpt-4o-mini                       (MUNINN_ENRICH_API_KEY required)
//	anthropic://claude-haiku-4-5-20251001      (MUNINN_ANTHROPIC_KEY or MUNINN_ENRICH_API_KEY)
//	google://gemini-1.5-flash                  (MUNINN_GOOGLE_KEY or MUNINN_ENRICH_API_KEY)
//
// Returns nil without error if MUNINN_ENRICH_URL is not set — LLM enrichment
// is optional. Logs a warning on init failure so the server starts without
// enrichment rather than refusing to start.
//
// Priority:
//  1. MUNINN_ENRICH_URL env var
//  2. Saved plugin_config.json
func buildEnricher(ctx context.Context, cfg plugincfg.PluginConfig) plugin.EnrichPlugin {
	enrichURL := os.Getenv("MUNINN_ENRICH_URL")

	if enrichURL == "" && cfg.EnrichURL != "" {
		enrichURL = cfg.EnrichURL
	}

	if enrichURL == "" {
		slog.Info("no enrich plugin configured, LLM enrichment disabled")
		return nil
	}

	enrichURL = injectOpenAIBaseURL(enrichURL, strings.TrimSpace(os.Getenv("MUNINN_OPENAI_URL")))
	slog.Info("initializing enrich plugin", "url", enrichURL)
	svc, err := enrichpkg.NewEnrichService(enrichURL)
	if err != nil {
		slog.Warn("enrich plugin URL parse failed, LLM enrichment disabled", "err", err)
		return nil
	}

	apiKey := os.Getenv("MUNINN_ENRICH_API_KEY")
	if apiKey == "" {
		apiKey = os.Getenv("MUNINN_ANTHROPIC_KEY")
	}
	if apiKey == "" {
		apiKey = os.Getenv("MUNINN_GOOGLE_KEY")
	}
	if apiKey == "" {
		apiKey = cfg.EnrichAPIKey // saved config fallback
	}
	if err := svc.Init(ctx, plugin.PluginConfig{APIKey: apiKey}); err != nil {
		slog.Warn("enrich plugin init failed (LLM provider may be down), LLM enrichment disabled", "err", err)
		return nil
	}

	slog.Info("enrich plugin initialized", "url", enrichURL)
	return svc
}

// applyMemoryLimits sets GOMEMLIMIT and GOGC for the server process.
func applyMemoryLimits() {
	const defaultMemGB = 4
	const defaultGCPercent = 200

	memGB := defaultMemGB
	if s := os.Getenv("MUNINN_MEM_LIMIT_GB"); s != "" {
		if n, err := strconv.Atoi(s); err == nil && n > 0 {
			memGB = n
		}
	}

	gcPct := defaultGCPercent
	if s := os.Getenv("MUNINN_GC_PERCENT"); s != "" {
		if n, err := strconv.Atoi(s); err == nil && n > 0 {
			gcPct = n
		}
	}

	debug.SetMemoryLimit(int64(memGB) * 1024 * 1024 * 1024)
	debug.SetGCPercent(gcPct)
	slog.Info("memory limits applied",
		"mem_limit_gb", memGB,
		"gc_percent", gcPct,
	)
}

// runStartupMigrations runs all idempotent storage migrations on startup.
func runStartupMigrations(ctx context.Context, store *storage.PebbleStore) {
	names, err := store.ListVaultNames()
	if err != nil {
		slog.Warn("startup migration: failed to list vault names", "err", err)
		return
	}
	for _, name := range names {
		prefix := store.ResolveVaultPrefix(name)
		if err := store.MigrateBuckets(ctx, prefix); err != nil {
			slog.Warn("startup migration: MigrateBuckets failed", "vault", name, "err", err)
		}
	}
	slog.Info("startup migration complete", "vaults", len(names))
}

// parseCORSOrigins splits a comma-separated MUNINN_CORS_ORIGINS env var into a slice.
func parseCORSOrigins(env string) []string {
	if env == "" {
		return nil
	}
	var origins []string
	for _, o := range strings.Split(env, ",") {
		if o = strings.TrimSpace(o); o != "" {
			origins = append(origins, o)
		}
	}
	return origins
}

// validateServerFlags checks that each addr is a valid host:port pair.
func validateServerFlags(addrs ...string) error {
	for _, addr := range addrs {
		host, portStr, err := net.SplitHostPort(addr)
		if err != nil {
			return fmt.Errorf("invalid address %q: %w", addr, err)
		}
		_ = host
		port, err := strconv.Atoi(portStr)
		if err != nil || port < 1 || port > 65535 {
			return fmt.Errorf("invalid port in address %q: port must be 1-65535", addr)
		}
	}
	return nil
}

// parseListenHost extracts the --listen-host value from args, falling back to
// envVal and then "127.0.0.1".
func parseListenHost(args []string, envVal string) string {
	host := "127.0.0.1"
	if envVal != "" {
		host = envVal
	}
	for i, arg := range args {
		if (arg == "--listen-host" || arg == "-listen-host") && i+1 < len(args) {
			host = args[i+1]
			break
		}
		if after, ok := strings.CutPrefix(arg, "--listen-host="); ok {
			host = after
			break
		}
		if after, ok := strings.CutPrefix(arg, "-listen-host="); ok {
			host = after
			break
		}
	}
	return host
}

// runMCPStandalone starts the full engine inline and runs the MCP server +
// stdio proxy in a single process. This is the only entry point for
// muninndb-lite — no daemon, no REST/gRPC/MBP/UI.
func runMCPStandalone() {
	loadEnvFile()

	applyMemoryLimits()

	// Flags
	fs := flag.NewFlagSet("muninndb-lite", flag.ExitOnError)
	dataDir := fs.String("data", defaultDataDir(), "data directory")
	mcpToken := fs.String("mcp-token", "", "Bearer token for MCP auth (reads MUNINN_MCP_TOKEN env or ~/.muninn/mcp.token if empty)")
	tlsCert := fs.String("tls-cert", "", "Path to TLS certificate file (PEM)")
	tlsKey := fs.String("tls-key", "", "Path to TLS private key file (PEM)")
	var logLevelStr string
	fs.StringVar(&logLevelStr, "log-level", "info", "Log level: debug, info, warn, error")
	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: muninndb-lite [mcp] [flags]\n\n")
		fmt.Fprintf(os.Stderr, "Starts the MuninnDB engine and MCP stdio proxy in a single process.\n\n")
		fs.PrintDefaults()
	}

	// Strip the "mcp" subcommand before parsing flags so that
	// "muninndb-lite mcp --data /foo" works correctly.
	args := os.Args[1:]
	if len(args) > 0 && args[0] == "mcp" {
		args = args[1:]
	}
	fs.Parse(args)

	// MCP token resolution order (highest to lowest priority):
	//   1. --mcp-token flag  — explicit override for tests / container entrypoints
	//   2. MUNINN_MCP_TOKEN env var — preferred for Docker / docker-compose deployments
	//   3. ~/.muninn/mcp.token file — keeps the token out of `ps` output on bare-metal
	if *mcpToken == "" {
		*mcpToken = os.Getenv("MUNINN_MCP_TOKEN")
	}
	if *mcpToken == "" {
		*mcpToken = readTokenFile()
	}

	// TLS env fallbacks
	if *tlsCert == "" {
		*tlsCert = os.Getenv("MUNINN_TLS_CERT")
	}
	if *tlsKey == "" {
		*tlsKey = os.Getenv("MUNINN_TLS_KEY")
	}

	if (*tlsCert == "") != (*tlsKey == "") {
		slog.Error("tls: --tls-cert and --tls-key must both be set (or neither)")
		os.Exit(1)
	}

	var clientTLS *tls.Config
	if *tlsCert != "" {
		cert, err := tls.LoadX509KeyPair(*tlsCert, *tlsKey)
		if err != nil {
			slog.Error("tls: failed to load certificate", "cert", *tlsCert, "err", err)
			os.Exit(1)
		}
		clientTLS = &tls.Config{
			Certificates: []tls.Certificate{cert},
			MinVersion:   tls.VersionTLS12,
		}
		slog.Info("tls: TLS enabled", "cert", *tlsCert)
	}

	var logLevel slog.Level
	if err := logLevel.UnmarshalText([]byte(logLevelStr)); err != nil {
		fmt.Fprintf(os.Stderr, "invalid --log-level %q: must be debug, info, warn, or error\n", logLevelStr)
		os.Exit(1)
	}
	ring := logging.NewRingBuffer(1000, nil)
	baseHandler := slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: logLevel})
	slog.SetDefault(slog.New(logging.NewRingHandler(baseHandler, ring)))

	// Open Pebble
	dbPath := filepath.Join(*dataDir, "pebble")
	if err := os.MkdirAll(dbPath, 0700); err != nil {
		slog.Error("create data dir", "err", err)
		os.Exit(1)
	}

	testFile := filepath.Join(dbPath, ".write-test")
	if err := os.WriteFile(testFile, []byte("ok"), 0600); err != nil {
		slog.Error("data directory is not writable", "path", dbPath, "err", err)
		os.Exit(1)
	}
	os.Remove(testFile)

	db, err := storage.OpenPebble(dbPath, storage.DefaultOptions())
	if err != nil {
		slog.Error("open pebble", "err", err)
		os.Exit(1)
	}

	if err := replication.CheckAndSetSchemaVersion(db); err != nil {
		slog.Error("schema version check", "err", err)
		os.Exit(1)
	}

	// Run versioned schema migrations
	migRunner := migrate.NewRunner(db)
	migRunner.Register(migrate.Migration{
		Version:     1,
		Description: "backfill embed_dim in ERF records for existing embeddings",
		Up:          migrate.BackfillEmbedDim,
	})
	migRunner.Register(migrate.Migration{
		Version:     2,
		Description: "backfill relationship entity index (0x26) for GetEntityAggregate optimisation",
		Up:          migrate.BackfillRelEntityIndex,
	})
	if applied, err := migRunner.Run(); err != nil {
		slog.Error("migration failed", "err", err)
		db.Close()
		os.Exit(1)
	} else if applied > 0 {
		slog.Info("migrations applied", "count", applied)
	}

	authStore := auth.NewStore(db)
	secretPath := filepath.Join(*dataDir, "auth_secret")
	if _, err := auth.Bootstrap(authStore, secretPath); err != nil {
		slog.Error("auth bootstrap failed", "err", err)
		os.Exit(1)
	}

	// Open WAL
	walPath := filepath.Join(*dataDir, "wal")
	mol, err := wal.Open(walPath)
	if err != nil {
		slog.Error("open wal", "err", err)
		os.Exit(1)
	}
	defer mol.Close()

	// Recover WAL
	lastSeq := wal.LoadLastSeq(db)
	var replayedCount int
	err = mol.Recover(db, func(e *wal.MOLEntry) error {
		if e.SeqNum <= lastSeq {
			return nil
		}
		replayedCount++
		return nil
	})
	if err != nil {
		slog.Error("recover wal", "err", err)
		os.Exit(1)
	}
	if replayedCount > 0 {
		slog.Info("wal recovery", "replayed_entries", replayedCount, "last_committed_seq", lastSeq)
	}

	// Build storage layer
	store := storage.NewPebbleStore(db, storage.PebbleStoreConfig{CacheSize: 10000})

	runStartupMigrations(context.Background(), store)

	// Create GroupCommitter
	gc := wal.NewGroupCommitter(mol, db)
	store.SetWAL(mol, gc)

	// Build indexes
	ftsIndex := fts.New(db)

	// Load saved plugin config
	savedPluginCfg, err := plugincfg.LoadPluginConfig(*dataDir)
	if err != nil {
		slog.Warn("failed to load plugin config, using defaults", "err", err)
	}

	// Build embedder
	initCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	embedder, embedPlugin, err := buildEmbedder(initCtx, savedPluginCfg, *dataDir)
	cancel()
	if err != nil {
		slog.Error("embedder build failed", "err", err)
		os.Exit(1)
	}

	// Build enrich plugin
	enrichCtx, enrichCancel := context.WithTimeout(context.Background(), 30*time.Second)
	enrichPlugin := buildEnricher(enrichCtx, savedPluginCfg)
	enrichCancel()

	// Build HNSW registry
	hnswRegistry := hnswpkg.NewRegistry(db)

	if warnMBStr := os.Getenv("MUNINN_HNSW_WARN_THRESHOLD_MB"); warnMBStr != "" {
		if warnMB, err := strconv.ParseInt(warnMBStr, 10, 64); err != nil || warnMB <= 0 {
			slog.Warn("invalid MUNINN_HNSW_WARN_THRESHOLD_MB, ignoring", "value", warnMBStr)
		} else {
			hnswRegistry.SetWarnThresholdBytes(warnMB << 20)
			slog.Info("hnsw: memory warn threshold configured", "warn_threshold_mb", warnMB)
		}
	}
	if maxMBStr := os.Getenv("MUNINN_HNSW_MAX_MB"); maxMBStr != "" {
		if maxMB, err := strconv.ParseInt(maxMBStr, 10, 64); err != nil || maxMB <= 0 {
			slog.Warn("invalid MUNINN_HNSW_MAX_MB, ignoring", "value", maxMBStr)
		} else {
			hnswRegistry.SetMaxBytes(maxMB << 20)
			slog.Info("hnsw: hard memory limit configured", "max_mb", maxMB)
		}
	}

	// Build activation engine
	actEngine := activation.New(store, activation.NewFTSAdapter(ftsIndex), activation.NewHNSWAdapter(hnswRegistry), embedder)

	// Build trigger system
	trigSystem := trigger.New(store, trigger.NewFTSAdapter(ftsIndex), trigger.NewHNSWAdapter(hnswRegistry), embedder)

	// Signal handling context
	ctx, ctxCancel := context.WithCancel(context.Background())

	// Create cognitive workers
	hebbianWorkerImpl := cognitive.NewHebbianWorker(cognitive.NewHebbianStoreAdapter(store))
	contradictWorkerImpl := cognitive.NewContradictWorker(cognitive.NewContradictStoreAdapter(store))
	confidenceWorkerImpl := cognitive.NewConfidenceWorker(cognitive.NewConfidenceStoreAdapter(store))

	transitionWorkerImpl := cognitive.NewTransitionWorker(ctx, store.TransitionCache())
	actEngine.SetTransitionStore(store.TransitionCache())

	// Build engine
	eng := engine.NewEngine(engine.EngineConfig{
		Store:            store,
		AuthStore:        authStore,
		FTSIndex:         ftsIndex,
		ActivationEngine: actEngine,
		TriggerSystem:    trigSystem,
		HebbianWorker:    hebbianWorkerImpl,
		ContradictWorker: contradictWorkerImpl.Worker,
		ConfidenceWorker: confidenceWorkerImpl.Worker,
		Embedder:         embedder,
		HNSWRegistry:     hnswRegistry,
	})

	eng.SetTransitionWorker(transitionWorkerImpl)

	latTracker := latency.New()
	eng.SetLatencyTracker(latTracker)

	// Build plugin registry
	pluginRegistry := plugin.NewRegistry()
	if embedPlugin != nil {
		if err := pluginRegistry.Register(embedPlugin); err != nil {
			slog.Warn("failed to register embed plugin in registry", "err", err)
		}
	}
	if enrichPlugin != nil {
		if err := pluginRegistry.Register(enrichPlugin); err != nil {
			slog.Warn("failed to register enrich plugin in registry", "err", err)
		}
		eng.SetEnrichPlugin(enrichPlugin)
		if timeoutStr := os.Getenv("MUNINN_ENRICH_TIMEOUT"); timeoutStr != "" {
			if d, err := time.ParseDuration(timeoutStr); err == nil && d > 0 {
				eng.SetReplayEnrichTimeout(d)
				slog.Info("replay enrichment per-engram timeout configured", "timeout", d)
			} else if err != nil {
				slog.Warn("MUNINN_ENRICH_TIMEOUT invalid, ignoring", "value", timeoutStr, "err", err)
			}
		}
		if es, ok := enrichPlugin.(interface {
			SetBreakerStateChangeHook(interface {
				SetHealthy(name string, healthy bool)
				SetUnhealthy(name string, err error)
			})
		}); ok {
			es.SetBreakerStateChangeHook(pluginRegistry)
		}
	}

	// Shared plugin store
	pStore := plugin.NewStoreAdapter(store, hnswRegistry)

	// Build MCP server on ephemeral port
	mcpAdapter := mcp.NewEngineAdapter(eng, enrichPlugin, pStore)

	// Acquire an ephemeral port: listen on :0, capture the port, close the
	// listener, then tell the MCP server to bind to that exact port.
	// The race window is negligible on localhost single-process.
	ephemeralLn, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		slog.Error("failed to acquire ephemeral port", "err", err)
		os.Exit(1)
	}
	ephemeralPort := ephemeralLn.Addr().(*net.TCPAddr).Port
	ephemeralLn.Close()

	mcpAddr := fmt.Sprintf("127.0.0.1:%d", ephemeralPort)
	mcpServer := mcp.New(mcpAddr, mcpAdapter, *mcpToken, authStore, clientTLS)

	// Signal handling
	sigCh := make(chan os.Signal, 2)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigCh
		slog.Info("shutdown signal received")
		ctxCancel()
		<-sigCh
		slog.Error("second signal received — forcing immediate exit")
		os.Exit(1)
	}()

	// Start GroupCommitter
	go gc.Run(ctx)

	// Start trigger system
	trigSystem.Start(ctx)

	// Start cognitive workers
	go contradictWorkerImpl.Worker.Run(ctx)
	go confidenceWorkerImpl.Worker.Run(ctx)

	// Start retroactive embed processor
	var retroProcessor *plugin.RetroactiveProcessor
	if embedPlugin != nil {
		retroProcessor = plugin.NewRetroactiveProcessor(pStore, embedPlugin, plugin.DigestEmbed)
		retroProcessor.Start(ctx)
		slog.Info("retroactive embed processor started")
	}

	// Start retroactive enrich processor
	var enrichProcessor *plugin.RetroactiveProcessor
	if enrichPlugin != nil {
		enrichProcessor = plugin.NewRetroactiveProcessor(pStore, enrichPlugin, plugin.DigestEnrich)
		enrichProcessor.Start(ctx)
		slog.Info("retroactive enrich processor started")
	}

	// Wire engine -> processors
	switch {
	case retroProcessor != nil && enrichProcessor != nil:
		embedNotify := retroProcessor.Notify
		enrichNotify := enrichProcessor.Notify
		eng.SetOnWrite(func() { embedNotify(); enrichNotify() })
	case retroProcessor != nil:
		eng.SetOnWrite(retroProcessor.Notify)
	case enrichProcessor != nil:
		eng.SetOnWrite(enrichProcessor.Notify)
	}

	var obsProcs []*plugin.RetroactiveProcessor
	if retroProcessor != nil {
		obsProcs = append(obsProcs, retroProcessor)
	}
	if enrichProcessor != nil {
		obsProcs = append(obsProcs, enrichProcessor)
	}
	eng.SetRetroactiveProcessors(obsProcs...)

	// Start MCP server in a goroutine
	errCh := make(chan error, 1)
	go func() {
		slog.Info("mcp listening", "addr", mcpAddr)
		if err := mcpServer.Serve(); err != nil && err != http.ErrServerClosed {
			errCh <- err
		}
	}()

	// Brief pause to let MCP server bind or fail fast.
	select {
	case err := <-errCh:
		slog.Error("mcp server failed to start", "err", err)
		ctxCancel()
		os.Exit(1)
	case <-time.After(100 * time.Millisecond):
		// Server likely started successfully.
	}

	if headlessMode {
		slog.Info("muninndb-lite started (headless daemon mode)", "mcp_internal", mcpAddr)
		// Headless: expose on :defaultMCPPort via reverse proxy + idle watchdog.
		// Blocks until idle timeout or SIGTERM.
		runHeadless(ephemeralPort, ctxCancel)
		slog.Info("headless daemon stopping")
	} else {
		slog.Info("muninndb-lite started (MCP stdio mode)", "mcp_addr", mcpAddr)
		// Set the proxy URL to point at our ephemeral MCP server and run the
		// stdio proxy on the main goroutine. When stdin closes (EOF), the proxy
		// returns and we trigger graceful shutdown.
		mcpProxyURL = fmt.Sprintf("http://127.0.0.1:%d/mcp", ephemeralPort)
		runMCPStdioWith(os.Stdin, os.Stdout)
		slog.Info("stdin closed, shutting down")
	}
	ctxCancel()

	shutdownDone := make(chan struct{})
	go func() {
		defer close(shutdownDone)
		if retroProcessor != nil {
			retroProcessor.Stop()
		}
		if enrichProcessor != nil {
			enrichProcessor.Stop()
		}
		if enrichPlugin != nil {
			if closer, ok := enrichPlugin.(interface{ Close() error }); ok {
				_ = closer.Close()
			}
		}
		netShutCtx, netShutCancel := context.WithTimeout(context.Background(), 25*time.Second)
		defer netShutCancel()
		if err := mcpServer.Shutdown(netShutCtx); err != nil {
			slog.Error("mcp shutdown error", "err", err)
		}
		eng.Stop()
		hebbianWorkerImpl.Stop()
		transitionWorkerImpl.Stop()
		if err := store.Close(); err != nil {
			slog.Error("store close error", "err", err)
		}
		gc.Stop()
	}()
	select {
	case <-shutdownDone:
		slog.Info("shutdown complete")
	case <-time.After(30 * time.Second):
		slog.Error("shutdown timed out after 30s; forcing exit")
		os.Exit(1)
	}
}
