package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/cerpelai/aim/internal/agent"
	"github.com/cerpelai/aim/internal/finder"
	"github.com/cerpelai/aim/internal/picker"
)

func main() {
	var agentFilter string
	var targetDir string
	var jsonOutput bool

	flag.StringVar(&agentFilter, "a", "", "Filter by specific agent (short)")
	flag.StringVar(&agentFilter, "agent", "", "Filter by specific agent (claude, agy, gemini, omp, pi)")
	flag.StringVar(&targetDir, "d", "", "Target directory (short)")
	flag.StringVar(&targetDir, "dir", "", "Target directory (default: current working directory)")
	flag.BoolVar(&jsonOutput, "json", false, "Output as JSON")

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "ails - List local AI agent sessions tied to a folder\n\n")
		fmt.Fprintf(os.Stderr, "Usage:\n")
		fmt.Fprintf(os.Stderr, "  ails [flags] [path]\n\n")
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

	if jsonOutput {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(sessions); err != nil {
			fmt.Fprintf(os.Stderr, "Error encoding JSON: %v\n", err)
			os.Exit(1)
		}
		return
	}

	if len(sessions) == 0 {
		return
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
	fmt.Fprintf(w, "AGENT\tID\tUPDATED\tTITLE\n")
	for _, s := range sessions {
		relTime := picker.FormatRelativeTime(s.LastUpdated)
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", s.Agent, s.ID, relTime, s.Title)
	}
	w.Flush()
}
