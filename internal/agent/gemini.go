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
	Register(NewGeminiProvider())
}

type GeminiProvider struct{}

func NewGeminiProvider() *GeminiProvider {
	return &GeminiProvider{}
}

func (p *GeminiProvider) Name() string {
	return "gemini"
}

func (p *GeminiProvider) Available() bool {
	_, err := exec.LookPath("gemini")
	return err == nil
}

type geminiSessionFile struct {
	SessionID   string `json:"sessionId"`
	StartTime   string `json:"startTime"`
	LastUpdated string `json:"lastUpdated"`
	Messages    []struct {
		Type    string `json:"type"`
		Content any    `json:"content"`
	} `json:"messages"`
}

type geminiProjectsJSON struct {
	Projects map[string]string `json:"projects"`
}

func (p *GeminiProvider) FindSessions(targetDir string) ([]Session, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}

	normTarget := NormalizePath(targetDir)
	projectsFile := filepath.Join(home, ".gemini", "projects.json")
	data, err := os.ReadFile(projectsFile)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var projMap geminiProjectsJSON
	if err := json.Unmarshal(data, &projMap); err != nil {
		return nil, err
	}

	var matchedProjectName string
	var matchedPath string
	for dirPath, projName := range projMap.Projects {
		if PathsMatch(dirPath, normTarget) {
			matchedProjectName = projName
			matchedPath = dirPath
			break
		}
	}

	if matchedProjectName == "" {
		return nil, nil
	}

	// Check chats in ~/.gemini/tmp/<matchedProjectName>/chats
	chatsDir := filepath.Join(home, ".gemini", "tmp", matchedProjectName, "chats")
	files, err := os.ReadDir(chatsDir)
	if err != nil {
		return nil, nil
	}

	var sessions []Session
	for _, f := range files {
		if f.IsDir() || !strings.HasSuffix(f.Name(), ".json") {
			continue
		}
		filePath := filepath.Join(chatsDir, f.Name())
		sess, ok := p.parseSessionFile(filePath, matchedPath)
		if ok {
			sessions = append(sessions, sess)
		}
	}

	return sessions, nil
}

func (p *GeminiProvider) parseSessionFile(filePath, dirPath string) (Session, bool) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return Session{}, false
	}

	var sf geminiSessionFile
	if err := json.Unmarshal(data, &sf); err != nil {
		return Session{}, false
	}

	if sf.SessionID == "" {
		return Session{}, false
	}

	var lastTime time.Time
	if sf.LastUpdated != "" {
		lastTime, _ = time.Parse(time.RFC3339Nano, sf.LastUpdated)
	}
	if lastTime.IsZero() && sf.StartTime != "" {
		lastTime, _ = time.Parse(time.RFC3339Nano, sf.StartTime)
	}

	var title string
	for _, m := range sf.Messages {
		if m.Type == "user" {
			if str, ok := m.Content.(string); ok && strings.TrimSpace(str) != "" {
				clean := strings.TrimSpace(strings.ReplaceAll(str, "\n", " "))
				if len(clean) > 80 {
					clean = clean[:77] + "..."
				}
				title = clean
				break
			}
		}
	}

	if title == "" {
		title = "Gemini session (" + sf.SessionID[:min(8, len(sf.SessionID))] + ")"
	}

	return Session{
		Agent:       "gemini",
		ID:          sf.SessionID,
		Title:       title,
		Directory:   dirPath,
		LastUpdated: lastTime,
		FilePath:    filePath,
	}, true
}

func (p *GeminiProvider) ResumeCommand(s Session) (string, []string, string) {
	return "gemini", []string{"--resume", s.ID}, s.Directory
}
