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

	switch sub {
	case "":
		runMCPStandalone()
	case "mcp":
		runMCPStandalone()
	case "help", "--help", "-h":
		printHelp()
	case "version", "--version":
		fmt.Println(muninnVersion())
	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %q\n\n", sub)
		fmt.Fprintln(os.Stderr, "muninndb-lite only supports: mcp, version, help")
		fmt.Fprintln(os.Stderr, "Run 'muninndb-lite help' to see all commands.")
		os.Exit(1)
	}
}
