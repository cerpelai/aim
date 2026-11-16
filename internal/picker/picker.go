package picker

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/cerpelai/aim/internal/agent"
)

// FormatRelativeTime formats relative time concisely.
func FormatRelativeTime(t time.Time) string {
	if t.IsZero() {
		return "-"
	}
	d := time.Since(t)
	switch {
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		return fmt.Sprintf("%dm ago", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh ago", int(d.Hours()))
	case d < 30*24*time.Hour:
		return fmt.Sprintf("%dd ago", int(d.Hours()/24))
	default:
		return t.Format("2006-01-02")
	}
}

// SelectSession presents an fzf picker (or fallback) to choose a session.
func SelectSession(sessions []agent.Session) (*agent.Session, error) {
	if len(sessions) == 0 {
		return nil, fmt.Errorf("no sessions found")
	}
	if len(sessions) == 1 {
		return &sessions[0], nil
	}

	// Try fzf if available
	if _, err := exec.LookPath("fzf"); err == nil {
		selected, err := pickWithFzf(sessions)
		if err == nil && selected != nil {
			return selected, nil
		}
	}

	// Fallback to simple terminal prompt
	return pickFallback(sessions)
}

func formatLine(s agent.Session, idx int) string {
	timeStr := FormatRelativeTime(s.LastUpdated)
	return fmt.Sprintf("[%d] [%-6s] %-10s | %s (%s)", idx+1, s.Agent, timeStr, s.Title, s.ID)
}

func pickWithFzf(sessions []agent.Session) (*agent.Session, error) {
	cmd := exec.Command("fzf",
		"--height=40%",
		"--reverse",
		"--header=Select AI agent session to resume (ESC to cancel):",
		"--prompt=> ",
	)
	cmd.Stderr = os.Stderr

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}

	if err := cmd.Start(); err != nil {
		return nil, err
	}

	go func() {
		defer stdin.Close()
		for i, s := range sessions {
			io.WriteString(stdin, formatLine(s, i)+"\n")
		}
	}()

	outBytes, err := io.ReadAll(stdout)
	if err != nil {
		return nil, err
	}

	if err := cmd.Wait(); err != nil {
		return nil, fmt.Errorf("selection cancelled")
	}

	selectedStr := strings.TrimSpace(string(outBytes))
	if selectedStr == "" {
		return nil, fmt.Errorf("no session selected")
	}

	for i, s := range sessions {
		if strings.HasPrefix(selectedStr, fmt.Sprintf("[%d]", i+1)) {
			return &s, nil
		}
	}

	return nil, fmt.Errorf("unrecognized selection")
}

func pickFallback(sessions []agent.Session) (*agent.Session, error) {
	fmt.Println("Available sessions:")
	for i, s := range sessions {
		fmt.Println(formatLine(s, i))
	}
	fmt.Print("\nEnter number to resume [1]: ")

	var choice int
	n, err := fmt.Scanf("%d\n", &choice)
	if err != nil || n == 0 {
		return &sessions[0], nil
	}
	if choice < 1 || choice > len(sessions) {
		return nil, fmt.Errorf("invalid choice: %d", choice)
	}
	return &sessions[choice-1], nil
}
