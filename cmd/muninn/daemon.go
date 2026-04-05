package main

import (
	"fmt"
	"io"
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

const (
	// defaultIdleTimeout is the grace period after the last keepalive before
	// the daemon shuts down. Override with MUNINN_IDLE_TIMEOUT env var.
	defaultIdleTimeout = 5 * time.Minute

	// keepaliveInterval is how often each proxy pings the daemon.
	keepaliveInterval = 2 * time.Minute

	// pidFileName is the PID file for the lite daemon.
	// Separate from upstream's "muninn.pid" to avoid conflict.
	pidFileName = "muninn-lite.pid"
)

// mcpPort returns the well-known MCP port, checking MUNINN_MCP_PORT env
// var first. Mirrors upstream's --mcp-addr convention.
func mcpPort() string {
	if p := os.Getenv("MUNINN_MCP_PORT"); p != "" {
		return p
	}
	return defaultMCPPort
}

// getIdleTimeout returns the daemon idle timeout.
// MUNINN_IDLE_TIMEOUT accepts Go duration strings ("1h", "30m") or "0" to disable.
func getIdleTimeout() time.Duration {
	if s := os.Getenv("MUNINN_IDLE_TIMEOUT"); s != "" {
		if s == "0" || s == "off" {
			return 0
		}
		if d, err := time.ParseDuration(s); err == nil && d > 0 {
			return d
		}
	}
	return defaultIdleTimeout
}

// isLiteDaemonRunning checks the lite-specific PID file and verifies the
// process is alive. Cleans up stale PID files. Separate from upstream's
// isDaemonRunning() (in upgrade.go) which checks "muninn.pid".
func isLiteDaemonRunning() bool {
	pidPath := filepath.Join(defaultDataDir(), pidFileName)
	pid, err := readPID(pidPath)
	if err != nil {
		return false
	}
	if isProcessRunning(pid) {
		return true
	}
	os.Remove(pidPath)
	return false
}

// forkDaemon launches muninndb-lite --daemon as a detached background process.
// Uses upstream helpers: daemonSysProcAttr(), daemonExtraSetup(), logFilePath(),
// writePID().
func forkDaemon() error {
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

	lf, err := os.OpenFile(logFilePath(), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
	if err == nil {
		cmd.Stderr = lf
	}

	if err := cmd.Start(); err != nil {
		if lf != nil {
			lf.Close()
		}
		return fmt.Errorf("fork daemon: %w", err)
	}

	// Close parent's copy of log file — child has inherited the fd.
	if lf != nil {
		lf.Close()
	}

	pidPath := filepath.Join(defaultDataDir(), pidFileName)
	if err := writePID(pidPath, cmd.Process.Pid); err != nil {
		slog.Warn("failed to write daemon PID file", "err", err)
	}

	cmd.Process.Release()
	return nil
}

// waitForHealth polls the MCP health endpoint until it responds or timeout.
// Pattern matches upstream's runStart() health loop.
func waitForHealth(port string, timeout time.Duration) error {
	healthURL := "http://127.0.0.1:" + port + "/mcp/health"
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		time.Sleep(200 * time.Millisecond)
		resp, err := http.Get(healthURL)
		if err != nil {
			continue
		}
		io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
		if resp.StatusCode == 200 {
			return nil
		}
	}
	return fmt.Errorf("daemon did not become healthy within %s", timeout)
}

// startKeepalive pings /mcp/health every 2 minutes to keep the daemon alive.
// Runs until the process exits (session closed).
func startKeepalive(port string) {
	c := &http.Client{Timeout: 2 * time.Second}
	healthURL := "http://127.0.0.1:" + port + "/mcp/health"
	ticker := time.NewTicker(keepaliveInterval)
	defer ticker.Stop()
	for range ticker.C {
		resp, err := c.Get(healthURL)
		if err != nil {
			continue
		}
		resp.Body.Close()
	}
}

// runHeadless exposes the internal MCP server on the well-known port via
// a reverse proxy. Tracks request timestamps for idle detection. Blocks
// until idle timeout or SIGTERM.
func runHeadless(internalPort int, cancel func()) {
	publicAddr := "127.0.0.1:" + mcpPort()
	timeout := getIdleTimeout()

	ln, err := net.Listen("tcp", publicAddr)
	if err != nil {
		slog.Error("headless: port unavailable (another daemon running?)",
			"addr", publicAddr, "err", err)
		os.Exit(1)
	}

	target, _ := url.Parse(fmt.Sprintf("http://127.0.0.1:%d", internalPort))
	proxy := httputil.NewSingleHostReverseProxy(target)

	var lastActivity atomic.Int64
	lastActivity.Store(time.Now().Unix())

	srv := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		lastActivity.Store(time.Now().Unix())
		proxy.ServeHTTP(w, r)
	})}

	go func() {
		if timeout > 0 {
			slog.Info("headless daemon listening", "addr", publicAddr, "idle_timeout", timeout)
		} else {
			slog.Info("headless daemon listening", "addr", publicAddr, "idle_timeout", "disabled")
		}
		if err := srv.Serve(ln); err != nil && err != http.ErrServerClosed {
			slog.Error("headless: serve error", "err", err)
		}
	}()

	if timeout <= 0 {
		select {} // disabled — block until SIGTERM
	}

	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	for range ticker.C {
		idle := time.Since(time.Unix(lastActivity.Load(), 0))
		if idle > timeout {
			slog.Info("headless daemon: idle timeout", "idle", idle.Round(time.Second))
			cancel()
			return
		}
	}
}
