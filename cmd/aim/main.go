package main

import (
	"fmt"
	"os"

	"github.com/cerpelai/aim/internal/cmd"
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(0)
	}

	subcommand := os.Args[1]
	args := os.Args[2:]

	switch subcommand {
	case "ls", "list":
		os.Exit(cmd.RunLs(args))
	case "c", "continue", "open":
		os.Exit(cmd.RunContinue(args))
	case "mv", "move":
		os.Exit(cmd.RunMv(args))
	case "help", "-h", "--help":
		printUsage()
		os.Exit(0)
	default:
		fmt.Fprintf(os.Stderr, "aim: unknown command '%s'\n\n", subcommand)
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Fprintf(os.Stderr, "aim - AI Agent Session Manager\n\n")
	fmt.Fprintf(os.Stderr, "Usage:\n")
	fmt.Fprintf(os.Stderr, "  aim <command> [flags] [arguments]\n\n")
	fmt.Fprintf(os.Stderr, "Available Commands:\n")
	fmt.Fprintf(os.Stderr, "  ls, list           List local AI agent sessions for a folder\n")
	fmt.Fprintf(os.Stderr, "  c, continue, open  Interactively pick and resume an agent session\n")
	fmt.Fprintf(os.Stderr, "  mv, move           Rebind sessions across agents when a folder moves\n")
	fmt.Fprintf(os.Stderr, "  help               Show help for aim\n\n")
	fmt.Fprintf(os.Stderr, "Run 'aim <command> --help' for details on any command.\n")
}
