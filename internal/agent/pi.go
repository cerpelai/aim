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
	Register(NewOmpProvider())
	Register(NewPiProvider())
}

type PiFamilyProvider struct {
	name       string
	binaryName string
	sessionsDir string
}

func NewOmpProvider() *PiFamilyProvider {
	home, _ := os.UserHomeDir()
	return &PiFamilyProvider{
		name:       "omp",
		binaryName: "omp",
		sessionsDir: filepath.Join(home, ".omp", "agent", "sessions"),
	}
}

func NewPiProvider() *PiFamilyProvider {
	home, _ := os.UserHomeDir()
	return &PiFamilyProvider{
		name:       "pi",
		binaryName: "pi",
		sessionsDir: filepath.Join(home, ".pi", "agent", "sessions"),
	}
}

func (p *PiFamilyProvider) Name() string {
	return p.name
}

func (p *PiFamilyProvider) Available() bool {
	_, err := exec.LookPath(p.binaryName)
	return err == nil
}

type piSessionLine struct {
	Type      string `json:"type"`
	ID        string `json:"id"`
	Cwd       string `json:"cwd"`
	Timestamp string `json:"timestamp"`
	Title     string `json:"title"`
	Content   any    `json:"content"`
	Message   struct {
		Role    string `json:"role"`
		Content any    `json:"content"`
	} `json:"message"`
}

func (p *PiFamilyProvider) FindSessions(targetDir string) ([]Session, error) {
	normTarget := NormalizePath(targetDir)
	entries, err := os.ReadDir(p.sessionsDir)
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
		subDir := filepath.Join(p.sessionsDir, entry.Name())
		files, err := os.ReadDir(subDir)
		if err != nil {
			continue
		}

		for _, file := range files {
			if file.IsDir() || !strings.HasSuffix(file.Name(), ".jsonl") {
				continue
			}
			filePath := filepath.Join(subDir, file.Name())
			sess, ok := p.parseSessionFile(filePath, normTarget)
			if ok {
				sessions = append(sessions, sess)
			}
		}
	}

	return sessions, nil
}

func (p *PiFamilyProvider) parseSessionFile(filePath, normTarget string) (Session, bool) {
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
	buf := make([]byte, 1024*1024)
	scanner.Buffer(buf, 10*1024*1024)

	var sessionID string
	var cwd string
	var explicitTitle string
	var firstMsg string
	var lastTime time.Time
	lineCount := 0

	for scanner.Scan() {
		lineCount++
		text := scanner.Text()
		var l piSessionLine
		if err := json.Unmarshal([]byte(text), &l); err != nil {
			continue
		}

		if l.Type == "session" {
			if l.ID != "" {
				sessionID = l.ID
			}
			if l.Cwd != "" {
				cwd = l.Cwd
			}
			if l.Timestamp != "" {
				if t, err := time.Parse(time.RFC3339Nano, l.Timestamp); err == nil {
					lastTime = t
				}
			}
		}

		if l.Type == "title" && l.Title != "" {
			explicitTitle = l.Title
		}

		if l.Timestamp != "" {
			if t, err := time.Parse(time.RFC3339Nano, l.Timestamp); err == nil {
				if t.After(lastTime) {
					lastTime = t
				}
			}
		}

		// check user messages for preview
		if firstMsg == "" {
			if l.Type == "message" && l.Message.Role == "user" {
				firstMsg = extractPiContentText(l.Message.Content)
			} else if l.Type == "user" || l.Type == "user_message" {
				if str, ok := l.Content.(string); ok {
					firstMsg = str
				} else {
					firstMsg = extractPiContentText(l.Message.Content)
				}
			}
		}

		if lineCount > 20 && cwd != "" && !PathsMatch(cwd, normTarget) {
			return Session{}, false
		}
	}

	if sessionID == "" {
		base := filepath.Base(filePath)
		base = strings.TrimSuffix(base, ".jsonl")
		parts := strings.Split(base, "_")
		sessionID = parts[len(parts)-1]
	}

	if !PathsMatch(cwd, normTarget) {
		return Session{}, false
	}

	if lastTime.IsZero() {
		lastTime = info.ModTime()
	}

	title := explicitTitle
	if title == "" {
		if firstMsg != "" {
			clean := strings.TrimSpace(strings.ReplaceAll(firstMsg, "\n", " "))
			if len(clean) > 80 {
				clean = clean[:77] + "..."
			}
			title = clean
		} else {
			title = p.name + " session (" + sessionID[:min(8, len(sessionID))] + ")"
		}
	}

	return Session{
		Agent:       p.name,
		ID:          sessionID,
		Title:       title,
		Directory:   cwd,
		LastUpdated: lastTime,
		FilePath:    filePath,
	}, true
}

func (p *PiFamilyProvider) ResumeCommand(s Session) (string, []string, string) {
	// omp and pi support --resume <id>
	return p.binaryName, []string{"--resume", s.ID}, s.Directory
}

func extractPiContentText(content any) string {
	if content == nil {
		return ""
	}
	if str, ok := content.(string); ok {
		return strings.TrimSpace(str)
	}
	if arr, ok := content.([]any); ok {
		for _, item := range arr {
			if m, ok := item.(map[string]any); ok {
				if t, ok := m["text"].(string); ok && t != "" {
					return strings.TrimSpace(t)
				}
			}
		}
	}
	return ""
}
