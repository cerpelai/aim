package main

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

func main() {
	var agentFilter string
	var targetDir string

	flag.StringVar(&agentFilter, "a", "", "Filter by specific agent (short)")
	flag.StringVar(&agentFilter, "agent", "", "Filter by specific agent (claude, agy, gemini, omp, pi)")
	flag.StringVar(&targetDir, "d", "", "Target directory (short)")
	flag.StringVar(&targetDir, "dir", "", "Target directory (default: current working directory)")

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "aic - Interactive session chooser & resume tool for local AI agents\n\n")
		fmt.Fprintf(os.Stderr, "Usage:\n")
		fmt.Fprintf(os.Stderr, "  aic [flags] [path]\n\n")
		fmt.Fprintf(os.Stderr, "Flags:\n")
		flag.PrintDefaults()
	}

	flag.Parse()

	if targetDir == "" {
		if flag.NArg() > 0 {
			targetDir = flag.Arg(0)
		} else {
			cwd, err := os.Getwd()
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error getting current working directory: %v\n", err)
				os.Exit(1)
			}
			targetDir = cwd
		}
	}

	targetDir = agent.NormalizePath(targetDir)

	f := finder.NewDefaultFinder()
	sessions, err := f.FindAll(targetDir, agentFilter)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error finding sessions: %v\n", err)
		os.Exit(1)
	}

	if len(sessions) == 0 {
		fmt.Fprintf(os.Stderr, "No agent sessions found for %s\n", targetDir)
		os.Exit(1)
	}

	selected, err := picker.SelectSession(sessions)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}

	provider := f.ProviderByName(selected.Agent)
	if provider == nil {
		fmt.Fprintf(os.Stderr, "Provider %s not found\n", selected.Agent)
		os.Exit(1)
	}

	bin, args, dir := provider.ResumeCommand(*selected)
	binPath, err := exec.LookPath(bin)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Agent executable %s not found in PATH: %v\n", bin, err)
		os.Exit(1)
	}

	execArgs := append([]string{bin}, args...)

	if dir != "" {
		if err := os.Chdir(dir); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to change directory to %s: %v\n", dir, err)
		}
	}

	// Replace current process
	err = syscall.Exec(binPath, execArgs, os.Environ())
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to exec %s: %v\n", bin, err)
		os.Exit(1)
	}
}
