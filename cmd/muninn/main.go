package main

import (
	"fmt"
	"log/slog"
	"os"
	"time"
)

func main() {
	if len(os.Args) < 2 {
		startOrProxy()
		return
	}

	// --daemon flag: run engine headless (forked by forkDaemon).
	// Mirrors upstream's --daemon → runServer() pattern, but calls
	// runMCPStandalone() since runServer() depends on REST/gRPC/UI
	// packages that are not present in muninndb-lite.
	for i, arg := range os.Args[1:] {
		if arg == "--daemon" {
			os.Args = append(os.Args[:i+1], os.Args[i+2:]...)
			headlessMode = true
			runMCPStandalone()
			return
		}
	}

	sub := parseSubcommand(os.Args[1:])

	// If the "subcommand" is actually a flag (e.g. --data), treat as no subcommand.
	if len(sub) > 0 && sub[0] == '-' {
		startOrProxy()
		return
	}

	rest := os.Args[2:]

	switch sub {
	case "":
		startOrProxy()
	case "mcp":
		startOrProxy()
	case "exec":
		runExec(rest)
	case "exec:remember", "exec:recall", "exec:read", "exec:forget":
		runExec(os.Args[2:])
	case "help", "--help", "-h":
		printHelp()
	case "version", "--version":
		fmt.Println(muninnVersion())
	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %q\n\n", sub)
		fmt.Fprintln(os.Stderr, "muninndb-lite supports: mcp, exec, version, help")
		fmt.Fprintln(os.Stderr, "Run 'muninndb-lite help' to see all commands.")
		os.Exit(1)
	}
}

// startOrProxy connects to an existing daemon or forks one, then proxies.
func startOrProxy() {
	port := mcpPort()

	// Set proxy URL to the daemon port (overrides default from mcp_stdio.go).
	mcpProxyURL = "http://127.0.0.1:" + port + "/mcp"

	if isEngineReachable(port) {
		// Daemon already running — proxy to it.
		runMCPStdio()
		return
	}

	// No daemon — fork one and wait for it to be ready.
	if err := forkDaemon(); err != nil {
		slog.Error("failed to start daemon", "err", err)
		os.Exit(1)
	}

	if err := waitForHealth(port, 10*time.Second); err != nil {
		slog.Error("daemon failed to start", "err", err)
		os.Exit(1)
	}

	// Proxy to the daemon (same as upstream "muninn mcp").
	runMCPStdio()
}
