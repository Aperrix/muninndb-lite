package main

import (
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"sync/atomic"
	"time"
)

// headlessMode is set by the --daemon flag handler in main.go.
// When true, runMCPStandalone skips the stdio proxy and instead
// exposes the MCP server on the well-known port with idle timeout.
var headlessMode bool

// mcpPort returns the MCP port to use, checking MUNINN_MCP_PORT env
// var first, then falling back to defaultMCPPort (8750).
// This mirrors upstream's --mcp-addr flag behavior.
func mcpPort() string {
	if p := os.Getenv("MUNINN_MCP_PORT"); p != "" {
		return p
	}
	return defaultMCPPort
}

// idleTimeout is how long the headless daemon waits without any MCP
// request before shutting itself down.
const idleTimeout = 5 * time.Minute

// isEngineReachable probes the MCP health endpoint on the given port.
func isEngineReachable(port string) bool {
	c := &http.Client{Timeout: 500 * time.Millisecond}
	resp, err := c.Get("http://127.0.0.1:" + port + "/mcp")
	if err != nil {
		return false
	}
	resp.Body.Close()
	return resp.StatusCode < 500
}

// forkDaemon launches muninndb-lite --daemon as a detached process.
// The daemon starts the engine on the well-known MCP port and serves
// until idle timeout or SIGTERM.
func forkDaemon() error {
	// Build args: --daemon + forward all original args except "mcp" subcommand.
	args := []string{"--daemon"}
	for _, a := range os.Args[1:] {
		if a == "mcp" {
			continue
		}
		args = append(args, a)
	}

	cmd := exec.Command(os.Args[0], args...)
	cmd.SysProcAttr = daemonSysProcAttr()
	daemonExtraSetup(cmd)
	cmd.Stdin = nil
	cmd.Stdout = nil

	logPath := filepath.Join(defaultDataDir(), "daemon.log")
	if lf, err := os.OpenFile(logPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600); err == nil {
		cmd.Stderr = lf
		// lf is intentionally NOT closed here — the child inherits the fd.
		// The OS closes it when the child exits.
	}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("fork daemon: %w", err)
	}

	// Write PID file so the daemon can be stopped if needed.
	pidPath := filepath.Join(defaultDataDir(), "muninn-lite.pid")
	if err := writePID(pidPath, cmd.Process.Pid); err != nil {
		slog.Warn("failed to write daemon PID file", "err", err)
	}

	// Detach — don't wait for child.
	cmd.Process.Release()
	return nil
}

// waitForHealth polls the MCP endpoint until it responds or timeout.
func waitForHealth(port string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if isEngineReachable(port) {
			return nil
		}
		time.Sleep(200 * time.Millisecond)
	}
	return fmt.Errorf("daemon did not become healthy within %s", timeout)
}

// runHeadless exposes the internal MCP server on the well-known port
// via a reverse proxy that tracks request timestamps for idle detection.
// It blocks until the idle timeout is reached, then calls cancel to
// trigger graceful shutdown of the engine.
func runHeadless(internalPort int, cancel func()) {
	publicAddr := "127.0.0.1:" + mcpPort()

	// Bind the well-known port first — fail fast if another daemon is running.
	ln, err := net.Listen("tcp", publicAddr)
	if err != nil {
		slog.Error("headless: failed to bind well-known port (another daemon running?)",
			"addr", publicAddr, "err", err)
		os.Exit(1)
	}

	// Reverse proxy to the internal MCP server.
	target, _ := url.Parse(fmt.Sprintf("http://127.0.0.1:%d", internalPort))
	proxy := httputil.NewSingleHostReverseProxy(target)

	// Track last request timestamp for idle detection.
	var lastActivity atomic.Int64
	lastActivity.Store(time.Now().Unix())

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		lastActivity.Store(time.Now().Unix())
		proxy.ServeHTTP(w, r)
	})

	srv := &http.Server{Handler: handler}
	go func() {
		slog.Info("headless daemon listening", "addr", publicAddr)
		if err := srv.Serve(ln); err != nil && err != http.ErrServerClosed {
			slog.Error("headless: serve error", "err", err)
		}
	}()

	// Idle watchdog — shut down when no MCP requests for idleTimeout.
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	for range ticker.C {
		idle := time.Since(time.Unix(lastActivity.Load(), 0))
		if idle > idleTimeout {
			slog.Info("headless daemon: idle timeout, shutting down",
				"idle", idle.Round(time.Second))
			cancel()
			return
		}
	}
}
