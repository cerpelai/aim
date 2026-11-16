package cmd

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"syscall"

	"github.com/cerpelai/aim/internal/agent"
	"github.com/cerpelai/aim/internal/finder"
	"github.com/cerpelai/aim/internal/picker"
)

func RunContinue(args []string) int {
	fs := flag.NewFlagSet("continue", flag.ExitOnError)
	var agentFilter string
	var targetDir string

	fs.StringVar(&agentFilter, "a", "", "Filter by specific agent (short)")
	fs.StringVar(&agentFilter, "agent", "", "Filter by specific agent (claude, agy, gemini, omp, pi)")
	fs.StringVar(&targetDir, "d", "", "Target directory (short)")
	fs.StringVar(&targetDir, "dir", "", "Target directory (default: current working directory)")

	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, "aim c (or continue) - Select and resume AI agent session interactively\n\n")
		fmt.Fprintf(os.Stderr, "Usage:\n")
		fmt.Fprintf(os.Stderr, "  aim c [flags] [path]\n")
		fmt.Fprintf(os.Stderr, "  aim continue [flags] [path]\n\n")
		fmt.Fprintf(os.Stderr, "Flags:\n")
		fs.PrintDefaults()
	}

	if err := fs.Parse(args); err != nil {
		return 1
	}

	if targetDir == "" {
		if fs.NArg() > 0 {
			targetDir = fs.Arg(0)
		} else {
			cwd, err := os.Getwd()
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error getting current working directory: %v\n", err)
				return 1
			}
			targetDir = cwd
		}
	}

	targetDir = agent.NormalizePath(targetDir)

	f := finder.NewDefaultFinder()
	sessions, err := f.FindAll(targetDir, agentFilter)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error finding sessions: %v\n", err)
		return 1
	}

	if len(sessions) == 0 {
		fmt.Fprintf(os.Stderr, "No agent sessions found for %s\n", targetDir)
		return 1
	}

	selected, err := picker.SelectSession(sessions)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		return 1
	}

	provider := f.ProviderByName(selected.Agent)
	if provider == nil {
		fmt.Fprintf(os.Stderr, "Provider %s not found\n", selected.Agent)
		return 1
	}

	bin, resumeArgs, dir := provider.ResumeCommand(*selected)
	binPath, err := exec.LookPath(bin)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Agent executable %s not found in PATH: %v\n", bin, err)
		return 1
	}

	execArgs := append([]string{bin}, resumeArgs...)

	if dir != "" {
		if err := os.Chdir(dir); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to change directory to %s: %v\n", dir, err)
		}
	}

	// Replace current process
	err = syscall.Exec(binPath, execArgs, os.Environ())
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to exec %s: %v\n", bin, err)
		return 1
	}
	return 0
}
