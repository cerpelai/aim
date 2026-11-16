package agent

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

func init() {
	Register(NewAgyProvider())
}

type AgyProvider struct{}

func NewAgyProvider() *AgyProvider {
	return &AgyProvider{}
}

func (p *AgyProvider) Name() string {
	return "agy"
}

func (p *AgyProvider) Available() bool {
	_, err := exec.LookPath("agy")
	return err == nil
}

func (p *AgyProvider) FindSessions(targetDir string) ([]Session, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}

	normTarget := NormalizePath(targetDir)
	dbPath := filepath.Join(home, ".gemini", "antigravity-cli", "conversation_summaries.db")
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		return nil, nil
	}

	// Query sqlite3 CLI for tab-separated rows
	// conversation_id, title, workspace_uris, last_modified_time
	cmd := exec.Command("sqlite3", "-separator", "\t", dbPath,
		"SELECT conversation_id, title, workspace_uris, last_modified_time FROM conversation_summaries ORDER BY last_modified_time DESC;")
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	lines := strings.Split(string(out), "\n")
	var sessions []Session

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.Split(line, "\t")
		if len(parts) < 4 {
			continue
		}

		convID := parts[0]
		title := parts[1]
		urisJSON := parts[2]
		lastModStr := parts[3]

		var uris []string
		if err := json.Unmarshal([]byte(urisJSON), &uris); err != nil {
			// Fallback: simple string contains
			uris = []string{urisJSON}
		}

		matched := false
		var matchedDir string
		for _, u := range uris {
			p := strings.TrimPrefix(u, "file://")
			if PathsMatch(p, normTarget) {
				matched = true
				matchedDir = p
				break
			}
		}

		if !matched {
			continue
		}

		// parse last modified time
		// format example: 2026-11-17 01:59:14.495822+00:00 or ISO8601
		var lastTime time.Time
		timeFormats := []string{
			"2006-01-02 15:04:05.999999-07:00",
			"2006-01-02 15:04:05.999999+00:00",
			"2006-01-02 15:04:05.999999",
			"2006-01-02 15:04:05",
			time.RFC3339Nano,
			time.RFC3339,
		}
		for _, tf := range timeFormats {
			if t, err := time.Parse(tf, lastModStr); err == nil {
				lastTime = t
				break
			}
		}

		if title == "" {
			title = "Agy session (" + convID[:min(8, len(convID))] + ")"
		}

		sessions = append(sessions, Session{
			Agent:       "agy",
			ID:          convID,
			Title:       title,
			Directory:   matchedDir,
			LastUpdated: lastTime,
		})
	}

	return sessions, nil
}

func (p *AgyProvider) ResumeCommand(s Session) (string, []string, string) {
	return "agy", []string{"--conversation", s.ID}, s.Directory
}
