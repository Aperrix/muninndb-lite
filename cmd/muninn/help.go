package main

import (
	"fmt"
	"os"

	"golang.org/x/term"
)

// wantsHelp returns true if args contain -h, --help, or help.
func wantsHelp(args []string) bool {
	for _, a := range args {
		if a == "-h" || a == "--help" || a == "help" {
			return true
		}
	}
	return false
}

// printSubcommandUsage prints a consistently-formatted subcommand help block.
func printSubcommandUsage(name, summary, usage string, flags [][2]string, examples []string) {
	isTTY := term.IsTerminal(int(os.Stdout.Fd()))
	bold := func(s string) string {
		if isTTY {
			return "\033[1m" + s + "\033[0m"
		}
		return s
	}
	dim := func(s string) string {
		if isTTY {
			return "\033[2m" + s + "\033[0m"
		}
		return s
	}

	fmt.Println()
	fmt.Printf("%s — %s\n", bold("muninndb-lite "+name), summary)
	fmt.Println()
	fmt.Printf("  Usage: %s\n", usage)

	if len(flags) > 0 {
		fmt.Println()
		fmt.Println(bold("  Flags:"))
		for _, f := range flags {
			fmt.Printf("    %-24s %s\n", f[0], f[1])
		}
	}

	if len(examples) > 0 {
		fmt.Println()
		fmt.Println(bold("  Examples:"))
		for _, e := range examples {
			fmt.Printf("    %s\n", dim(e))
		}
	}
	fmt.Println()
}

// subcommandHelp maps subcommand names to their help printers.
var subcommandHelp = map[string]func(){
	"mcp": func() {
		printSubcommandUsage("mcp", "start engine + MCP stdio proxy", "muninndb-lite [mcp] [flags]",
			[][2]string{
				{"--data <dir>", "Data directory (default: ~/.muninn/data)"},
				{"--mcp-token <tok>", "Bearer token for MCP auth (reads ~/.muninn/mcp.token if empty)"},
				{"--tls-cert <path>", "Path to TLS certificate file (PEM)"},
				{"--tls-key <path>", "Path to TLS private key file (PEM)"},
				{"--log-level <level>", "Log level: debug, info, warn, error (default: info)"},
			},
			[]string{
				"muninndb-lite",
				"muninndb-lite mcp",
				"muninndb-lite --data /var/lib/muninn/data",
			})
	},
	"version": func() {
		printSubcommandUsage("version", "print version", "muninndb-lite version", nil, nil)
	},
}

func printHelp() {
	isTTY := term.IsTerminal(int(os.Stdout.Fd()))

	bold := func(s string) string {
		if isTTY {
			return "\033[1m" + s + "\033[0m"
		}
		return s
	}
	dim := func(s string) string {
		if isTTY {
			return "\033[2m" + s + "\033[0m"
		}
		return s
	}
	cyan := func(s string) string {
		if isTTY {
			return "\033[36m" + s + "\033[0m"
		}
		return s
	}

	fmt.Println()
	fmt.Println(bold("muninndb-lite") + " — MCP-only cognitive memory database")
	fmt.Println()
	fmt.Println("  A minimal fork of MuninnDB that runs as a single-process MCP stdio server.")
	fmt.Println("  No daemon, no REST API, no gRPC, no web UI — just the engine + MCP over stdio.")
	fmt.Println()

	fmt.Println(bold("USAGE"))
	fmt.Println()
	fmt.Printf("  %s       %s\n", cyan("muninndb-lite"), dim("# start engine + MCP stdio proxy"))
	fmt.Printf("  %s   %s\n", cyan("muninndb-lite mcp"), dim("# same as above (explicit)"))
	fmt.Println()

	fmt.Println(bold("COMMANDS"))
	fmt.Println()
	fmt.Printf("  %-36s %s\n", cyan("muninndb-lite [mcp]"), "Start engine and MCP stdio proxy (default)")
	fmt.Printf("  %-36s %s\n", cyan("muninndb-lite version"), "Print version")
	fmt.Printf("  %-36s %s\n", cyan("muninndb-lite help"), "Show this message")
	fmt.Println()

	fmt.Println(bold("FLAGS"))
	fmt.Println()
	fmt.Printf("  %-28s %s\n", "--data <dir>", "Data directory (default: ~/.muninn/data)")
	fmt.Printf("  %-28s %s\n", "--mcp-token <tok>", "MCP bearer token for AI tool auth")
	fmt.Printf("  %-28s %s\n", "--log-level <level>", "Log level: debug, info, warn, error")
	fmt.Printf("  %-28s %s\n", "--tls-cert <path>", "TLS certificate file (PEM)")
	fmt.Printf("  %-28s %s\n", "--tls-key <path>", "TLS private key file (PEM)")
	fmt.Println()

	fmt.Println(bold("EMBEDDERS") + dim("  (optional — enable semantic similarity search)"))
	fmt.Println()
	fmt.Printf("  %-28s %s\n", "MUNINN_OLLAMA_URL", "Local Ollama embed model (e.g. ollama://localhost:11434/nomic-embed-text)")
	fmt.Printf("  %-28s %s\n", "MUNINN_OPENAI_KEY", "OpenAI embeddings API key (text-embedding-3-small, 1536d)")
	fmt.Printf("  %-28s %s\n", "MUNINN_OPENAI_URL", "Optional OpenAI base URL or provider URL override")
	fmt.Printf("  %-28s %s\n", "MUNINN_VOYAGE_KEY", "Voyage AI embeddings API key (voyage-3, 1024d)")
	fmt.Printf("  %-28s %s\n", "MUNINN_COHERE_KEY", "Cohere embeddings API key (embed-v4, 1024d)")
	fmt.Printf("  %-28s %s\n", "MUNINN_GOOGLE_KEY", "Google Gemini embeddings API key (text-embedding-004, 768d)")
	fmt.Printf("  %-28s %s\n", "MUNINN_JINA_KEY", "Jina embeddings API key (jina-embeddings-v3, 1024d)")
	fmt.Printf("  %-28s %s\n", "MUNINN_MISTRAL_KEY", "Mistral embeddings API key (mistral-embed, 1024d)")
	fmt.Println()

	fmt.Println(bold("LLM ENRICHMENT") + dim("  (optional — auto-extract entities, relationships, summaries)"))
	fmt.Println()
	fmt.Println("  Set MUNINN_ENRICH_URL to enable background LLM enrichment on every new memory.")
	fmt.Println()
	fmt.Printf("  %-28s %s\n", "Ollama (local, no key):", "MUNINN_ENRICH_URL=ollama://localhost:11434/llama3.2")
	fmt.Printf("  %-28s %s\n", "OpenAI:", "MUNINN_ENRICH_URL=openai://gpt-4o-mini")
	fmt.Printf("  %-28s %s\n", "Anthropic:", "MUNINN_ENRICH_URL=anthropic://claude-haiku-4-5-20251001")
	fmt.Println()
}
