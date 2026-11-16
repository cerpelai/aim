package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/cerpelai/aim/internal/agent"
	"github.com/cerpelai/aim/internal/mover"
)

func main() {
	var dryRun bool
	var force bool
	var moveFilesystem bool
	var moveTrust bool

	flag.BoolVar(&dryRun, "n", false, "Dry run: simulate updates without modifying files")
	flag.BoolVar(&dryRun, "dry-run", false, "Dry run: simulate updates without modifying files")
	flag.BoolVar(&force, "f", false, "Force: apply changes without interactive confirmation prompt")
	flag.BoolVar(&force, "force", false, "Force: apply changes without interactive confirmation prompt")
	flag.BoolVar(&moveFilesystem, "fs", false, "Also move/rename the directory on the filesystem (os.Rename)")
	flag.BoolVar(&moveFilesystem, "filesystem", false, "Also move/rename the directory on the filesystem (os.Rename)")
	flag.BoolVar(&moveTrust, "trust", false, "Also migrate agent directory trust / permissions (disabled by default)")

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "aimv - Move / update AI agent session references when a directory moves\n\n")
		fmt.Fprintf(os.Stderr, "Usage:\n")
		fmt.Fprintf(os.Stderr, "  aimv [flags] <old_path> <new_path>\n\n")
		fmt.Fprintf(os.Stderr, "Flags:\n")
		flag.PrintDefaults()
	}

	flag.Parse()

	if flag.NArg() < 2 {
		flag.Usage()
		os.Exit(1)
	}

	oldPath := agent.NormalizePath(flag.Arg(0))
	newPath := agent.NormalizePath(flag.Arg(1))

	fmt.Printf("Checking AI agent session references:\n  from: %s\n  to:   %s\n\n", oldPath, newPath)

	// Step 1: Always preview first
	reports, err := mover.MoveSessionReferences(oldPath, newPath, moveTrust, true)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	if len(reports) == 0 {
		fmt.Println("No agent session references found for the specified old directory.")
		if !moveFilesystem {
			os.Exit(0)
		}
	} else {
		fmt.Println("Planned updates:")
		for _, r := range reports {
			fmt.Printf("[%s]\n", r.Agent)
			for _, d := range r.Details {
				fmt.Printf("  • %s\n", d)
			}
			fmt.Println()
		}
	}

	// Always display options status (enabled/disabled)
	fmt.Println("Options:")
	if moveFilesystem {
		fmt.Printf("  • Filesystem move (--fs):      ENABLED (rename %s -> %s)\n", oldPath, newPath)
	} else {
		fmt.Println("  • Filesystem move (--fs):      DISABLED (session references only)")
	}

	if moveTrust {
		fmt.Println("  • Migrate trust   (--trust):   ENABLED (transfer trusted directories/permissions)")
	} else {
		fmt.Println("  • Migrate trust   (--trust):   DISABLED (will not touch agent trust configs)")
	}
	fmt.Println()

	if dryRun {
		fmt.Println("Dry run mode (-n): No changes made.")
		return
	}

	// Step 2: Confirmation prompt unless -f
	if !force {
		fmt.Print("Proceed with updates? [y/N]: ")
		reader := bufio.NewReader(os.Stdin)
		input, err := reader.ReadString('\n')
		if err != nil {
			fmt.Println("\nAborted.")
			os.Exit(1)
		}
		ans := strings.ToLower(strings.TrimSpace(input))
		if ans != "y" && ans != "yes" {
			fmt.Println("Aborted. No changes made.")
			return
		}
	}

	// Step 3: Apply changes
	fmt.Println("\nApplying updates...")
	_, err = mover.MoveSessionReferences(oldPath, newPath, moveTrust, false)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error during migration: %v\n", err)
		os.Exit(1)
	}

	if moveFilesystem {
		if _, err := os.Stat(oldPath); err == nil {
			if err := os.Rename(oldPath, newPath); err != nil {
				fmt.Fprintf(os.Stderr, "Failed to rename folder on disk: %v\n", err)
				os.Exit(1)
			}
			fmt.Printf("Directory renamed on disk: %s -> %s\n", oldPath, newPath)
		}
	}

	fmt.Println("Done.")
}
