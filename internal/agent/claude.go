package agent

import (
	"bufio"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

func init() {
	Register(NewClaudeProvider())
}

type ClaudeProvider struct{}

func NewClaudeProvider() *ClaudeProvider {
	return &ClaudeProvider{}
}

func (p *ClaudeProvider) Name() string {
	return "claude"
}

func (p *ClaudeProvider) Available() bool {
	_, err := exec.LookPath("claude")
	return err == nil
}

type claudeLine struct {
	Type      string          `json:"type"`
	SessionID string          `json:"sessionId"`
	Cwd       string          `json:"cwd"`
	Timestamp string          `json:"timestamp"`
	Message   claudeMessage   `json:"message"`
}

type claudeMessage struct {
	Content any `json:"content"`
}

func (p *ClaudeProvider) FindSessions(targetDir string) ([]Session, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}

	normTarget := NormalizePath(targetDir)
	projectsDir := filepath.Join(home, ".claude", "projects")
	entries, err := os.ReadDir(projectsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var sessions []Session
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		projPath := filepath.Join(projectsDir, entry.Name())
		files, err := os.ReadDir(projPath)
		if err != nil {
			continue
		}

		for _, file := range files {
			if file.IsDir() || !strings.HasSuffix(file.Name(), ".jsonl") {
				continue
			}
			filePath := filepath.Join(projPath, file.Name())
			sess, ok := p.parseSessionFile(filePath, normTarget)
			if ok {
				sessions = append(sessions, sess)
			}
		}
	}

	return sessions, nil
}

func (p *ClaudeProvider) parseSessionFile(filePath, normTarget string) (Session, bool) {
	f, err := os.Open(filePath)
	if err != nil {
		return Session{}, false
	}
	defer f.Close()

	info, err := f.Stat()
	if err != nil {
		return Session{}, false
	}

	scanner := bufio.NewScanner(f)
	// Some json lines can be large
	buf := make([]byte, 1024*1024)
	scanner.Buffer(buf, 10*1024*1024)

	var sessionID string
	var cwd string
	var title string
	var lastTime time.Time
	lineCount := 0

	for scanner.Scan() {
		lineCount++
		text := scanner.Text()
		var l claudeLine
		if err := json.Unmarshal([]byte(text), &l); err != nil {
			continue
		}

		if l.SessionID != "" && sessionID == "" {
			sessionID = l.SessionID
		}
		if l.Cwd != "" && cwd == "" {
			cwd = l.Cwd
		}
		if l.Timestamp != "" {
			if t, err := time.Parse(time.RFC3339Nano, l.Timestamp); err == nil {
				if t.After(lastTime) {
					lastTime = t
				}
			}
		}
		if title == "" && l.Type == "user" {
			extracted := extractClaudeUserText(l.Message.Content)
			if extracted != "" {
				clean := strings.TrimSpace(strings.ReplaceAll(extracted, "\n", " "))
				if len(clean) > 80 {
					clean = clean[:77] + "..."
				}
				title = clean
			}
		}

		// Don't read whole massive file if cwd is determined and doesn't match
		if lineCount > 50 && cwd != "" && !PathsMatch(cwd, normTarget) {
			return Session{}, false
		}
	}

	if sessionID == "" {
		base := filepath.Base(filePath)
		sessionID = strings.TrimSuffix(base, ".jsonl")
	}

	if cwd == "" {
		// Fallback check from folder name: -Users-akhyar-amarullah-...
		dirName := filepath.Base(filepath.Dir(filePath))
		trimmed := strings.Trim(dirName, "-")
		parts := strings.Split(trimmed, "-")
		reconstructed := "/" + strings.Join(parts, "/")
		if PathsMatch(reconstructed, normTarget) {
			cwd = reconstructed
		}
	}

	if !PathsMatch(cwd, normTarget) {
		return Session{}, false
	}

	if lastTime.IsZero() {
		lastTime = info.ModTime()
	}

	if title == "" {
		title = "Claude session (" + sessionID[:min(8, len(sessionID))] + ")"
	}

	return Session{
		Agent:       "claude",
		ID:          sessionID,
		Title:       title,
		Directory:   cwd,
		LastUpdated: lastTime,
		FilePath:    filePath,
	}, true
}

func (p *ClaudeProvider) ResumeCommand(s Session) (string, []string, string) {
	return "claude", []string{"--resume", s.ID}, s.Directory
}

func extractClaudeUserText(content any) string {
	if content == nil {
		return ""
	}
	if str, ok := content.(string); ok {
		str = strings.TrimSpace(str)
		if !strings.HasPrefix(str, "<system-reminder>") && !strings.HasPrefix(str, "UserPromptSubmit") {
			return str
		}
		return ""
	}
	if arr, ok := content.([]any); ok {
		for _, item := range arr {
			if m, ok := item.(map[string]any); ok {
				if t, ok := m["text"].(string); ok {
					t = strings.TrimSpace(t)
					if !strings.HasPrefix(t, "<system-reminder>") && !strings.HasPrefix(t, "UserPromptSubmit") {
						return t
					}
				}
			}
		}
	}
	return ""
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
