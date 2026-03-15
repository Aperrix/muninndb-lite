package main

import (
	"fmt"
	"os"
)

func main() {
	if len(os.Args) < 2 {
		// No args: run MCP standalone (engine + stdio proxy in one process)
		runMCPStandalone()
		return
	}

	sub := parseSubcommand(os.Args[1:])

	rest := os.Args[2:]

	switch sub {
	case "":
		runMCPStandalone()
	case "mcp":
		runMCPStandalone()
	case "exec":
		runExec(rest)
	case "exec:remember", "exec:recall", "exec:read", "exec:forget":
		// parseSubcommand joins "exec remember" as "exec:remember" — pass
		// the operation name back so runExec can parse it.
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
