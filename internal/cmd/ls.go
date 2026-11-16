package cmd

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

func RunLs(args []string) int {
	fs := flag.NewFlagSet("ls", flag.ExitOnError)
	var agentFilter string
	var targetDir string
	var jsonOutput bool

	fs.StringVar(&agentFilter, "a", "", "Filter by specific agent (short)")
	fs.StringVar(&agentFilter, "agent", "", "Filter by specific agent (claude, agy, gemini, omp, pi)")
	fs.StringVar(&targetDir, "d", "", "Target directory (short)")
	fs.StringVar(&targetDir, "dir", "", "Target directory (default: current working directory)")
	fs.BoolVar(&jsonOutput, "json", false, "Output as JSON")

	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, "aim ls - List local AI agent sessions tied to a folder\n\n")
		fmt.Fprintf(os.Stderr, "Usage:\n")
		fmt.Fprintf(os.Stderr, "  aim ls [flags] [path]\n\n")
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

	if jsonOutput {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(sessions); err != nil {
			fmt.Fprintf(os.Stderr, "Error encoding JSON: %v\n", err)
			return 1
		}
		return 0
	}

	if len(sessions) == 0 {
		return 0
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
	fmt.Fprintf(w, "AGENT\tID\tUPDATED\tTITLE\n")
	for _, s := range sessions {
		relTime := picker.FormatRelativeTime(s.LastUpdated)
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", s.Agent, s.ID, relTime, s.Title)
	}
	w.Flush()
	return 0
}
