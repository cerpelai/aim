package mover

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/cerpelai/aim/internal/agent"
)

// Report tracks moved/updated items per agent.
type Report struct {
	Agent   string
	Details []string
}

// MoveSessionReferences updates all agent session files referencing oldDir to newDir.
// If dryRun is true, changes are only simulated and reported.
func MoveSessionReferences(oldDir, newDir string, moveTrust bool, dryRun bool) ([]Report, error) {
	normOld := agent.NormalizePath(oldDir)
	normNew := agent.NormalizePath(newDir)

	if normOld == "" || normNew == "" {
		return nil, fmt.Errorf("both old and new directories must be specified")
	}
	if normOld == normNew {
		return nil, fmt.Errorf("old and new directories are the same")
	}

	oldVariants := agent.PathVariants(oldDir)

	var reports []Report

	// 1. Claude
	if r, err := migrateClaude(normOld, normNew, oldVariants, dryRun); err == nil && len(r.Details) > 0 {
		reports = append(reports, r)
	}

	// 2. Agy (Google Antigravity)
	if r, err := migrateAgy(normOld, normNew, oldVariants, dryRun); err == nil && len(r.Details) > 0 {
		reports = append(reports, r)
	}

	// 3. Gemini CLI
	if r, err := migrateGemini(normOld, normNew, oldVariants, dryRun); err == nil && len(r.Details) > 0 {
		reports = append(reports, r)
	}

	// 4. Omp (Oh-My-Pi)
	if r, err := migratePiFamily("omp", filepath.Join(".omp", "agent", "sessions"), normOld, normNew, oldVariants, dryRun); err == nil && len(r.Details) > 0 {
		reports = append(reports, r)
	}

	// 5. Pi (Pi Coding Agent)
	if r, err := migratePiFamily("pi", filepath.Join(".pi", "agent", "sessions"), normOld, normNew, oldVariants, dryRun); err == nil && len(r.Details) > 0 {
		reports = append(reports, r)
	}

	// 6. Agent trust / permissions (optional)
	if moveTrust {
		if r, err := MoveAgentTrust(normOld, normNew, oldVariants, dryRun); err == nil && len(r.Details) > 0 {
			reports = append(reports, r)
		}
	}

	return reports, nil
}

func claudeSlug(dir string) string {
	cleaned := strings.Trim(dir, "/")
	re := regexp.MustCompile(`[/.]`)
	slug := re.ReplaceAllString(cleaned, "-")
	return "-" + slug
}

func migrateClaude(oldDir, newDir string, oldVariants []string, dryRun bool) (Report, error) {
	rep := Report{Agent: "claude"}
	home, err := os.UserHomeDir()
	if err != nil {
		return rep, err
	}

	projectsDir := filepath.Join(home, ".claude", "projects")
	entries, err := os.ReadDir(projectsDir)
	if err != nil {
		return rep, nil
	}

	newSlug := claudeSlug(newDir)
	newProjDir := filepath.Join(projectsDir, newSlug)

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		projDirPath := filepath.Join(projectsDir, entry.Name())
		// Check both slug match and actual jsonl contents
		matchedBySlug := false
		for _, variant := range oldVariants {
			if entry.Name() == claudeSlug(variant) {
				matchedBySlug = true
				break
			}
		}

		matchedByContent := hasMatchingPathReference(projDirPath, oldVariants)

		if !matchedBySlug && !matchedByContent {
			continue
		}

		rep.Details = append(rep.Details, fmt.Sprintf("Project directory: %s -> %s", entry.Name(), newSlug))

		jsonlFiles, _ := os.ReadDir(projDirPath)
		for _, jf := range jsonlFiles {
			if !strings.HasSuffix(jf.Name(), ".jsonl") {
				continue
			}
			jsonlPath := filepath.Join(projDirPath, jf.Name())
			if fileContainsAny(jsonlPath, oldVariants) {
				rep.Details = append(rep.Details, fmt.Sprintf("Update session reference in %s", jf.Name()))
				if !dryRun {
					for _, variant := range oldVariants {
						replaceInFile(jsonlPath, variant, newDir)
					}
				}
			}
		}

		if !dryRun {
			if projDirPath != newProjDir {
				_ = os.Rename(projDirPath, newProjDir)
			}
		}
	}

	return rep, nil
}

func migrateAgy(oldDir, newDir string, oldVariants []string, dryRun bool) (Report, error) {
	rep := Report{Agent: "agy"}
	home, err := os.UserHomeDir()
	if err != nil {
		return rep, err
	}

	dbPath := filepath.Join(home, ".gemini", "antigravity-cli", "conversation_summaries.db")
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		return rep, nil
	}

	for _, variant := range oldVariants {
		oldURI := "file://" + variant
		newURI := "file://" + newDir

		checkSQL := fmt.Sprintf("SELECT conversation_id, title FROM conversation_summaries WHERE workspace_uris LIKE '%%%s%%';", oldURI)
		out, err := exec.Command("sqlite3", dbPath, checkSQL).Output()
		if err == nil && len(strings.TrimSpace(string(out))) > 0 {
			lines := strings.Split(strings.TrimSpace(string(out)), "\n")
			for _, line := range lines {
				rep.Details = append(rep.Details, fmt.Sprintf("Update conversation %s", line))
			}

			if !dryRun {
				updateSQL := fmt.Sprintf(
					"UPDATE conversation_summaries SET workspace_uris = replace(workspace_uris, '%s', '%s') WHERE workspace_uris LIKE '%%%s%%';",
					oldURI, newURI, oldURI,
				)
				_ = exec.Command("sqlite3", dbPath, updateSQL).Run()
			}
		}
	}

	// Update last_conversations cache if present
	cachePath := filepath.Join(home, ".gemini", "antigravity-cli", "cache", "last_conversations.json")
	if _, err := os.Stat(cachePath); err == nil {
		if fileContainsAny(cachePath, oldVariants) {
			rep.Details = append(rep.Details, "Update cache/last_conversations.json")
			if !dryRun {
				for _, variant := range oldVariants {
					replaceInFile(cachePath, variant, newDir)
				}
			}
		}
	}

	return rep, nil
}

func migrateGemini(oldDir, newDir string, oldVariants []string, dryRun bool) (Report, error) {
	rep := Report{Agent: "gemini"}
	home, err := os.UserHomeDir()
	if err != nil {
		return rep, err
	}

	projectsFile := filepath.Join(home, ".gemini", "projects.json")
	data, err := os.ReadFile(projectsFile)
	if err != nil {
		return rep, nil
	}

	var m map[string]map[string]string
	if err := json.Unmarshal(data, &m); err != nil {
		return rep, nil
	}

	projects, ok := m["projects"]
	if !ok {
		return rep, nil
	}

	var matchedKeys []string
	var projName string
	for k, v := range projects {
		for _, variant := range oldVariants {
			if agent.PathsMatch(k, variant) {
				matchedKeys = append(matchedKeys, k)
				projName = v
				break
			}
		}
	}

	if len(matchedKeys) == 0 {
		return rep, nil
	}

	for _, k := range matchedKeys {
		rep.Details = append(rep.Details, fmt.Sprintf("Update projects.json mapping: %s -> %s (%s)", k, newDir, projName))
	}

	// Check chats in ~/.gemini/tmp/<projName>/chats
	if projName != "" {
		chatsDir := filepath.Join(home, ".gemini", "tmp", projName, "chats")
		if entries, err := os.ReadDir(chatsDir); err == nil {
			for _, e := range entries {
				if strings.HasSuffix(e.Name(), ".json") {
					chatFilePath := filepath.Join(chatsDir, e.Name())
					if fileContainsAny(chatFilePath, oldVariants) {
						rep.Details = append(rep.Details, fmt.Sprintf("Update path in chat session %s", e.Name()))
						if !dryRun {
							for _, variant := range oldVariants {
								replaceInFile(chatFilePath, variant, newDir)
							}
						}
					}
				}
			}
		}
	}

	if !dryRun {
		for _, k := range matchedKeys {
			delete(projects, k)
		}
		projects[newDir] = projName
		newData, _ := json.MarshalIndent(m, "", "  ")
		_ = os.WriteFile(projectsFile, newData, 0644)
	}

	return rep, nil
}

func piFamilySlug(agentName, dir string) string {
	home, _ := os.UserHomeDir()
	trimmed := strings.Trim(dir, "/")
	slug := strings.ReplaceAll(trimmed, "/", "-")
	if agentName == "pi" {
		return "--" + slug + "--"
	}

	if home != "" && strings.HasPrefix(dir, home) {
		rel := strings.TrimPrefix(dir, home)
		rel = strings.Trim(rel, "/")
		return "-" + strings.ReplaceAll(rel, "/", "-")
	}

	return "--" + slug + "--"
}

func migratePiFamily(agentName, relSessionsDir, oldDir, newDir string, oldVariants []string, dryRun bool) (Report, error) {
	rep := Report{Agent: agentName}
	home, err := os.UserHomeDir()
	if err != nil {
		return rep, err
	}

	sessionsDir := filepath.Join(home, relSessionsDir)
	if _, err := os.Stat(sessionsDir); os.IsNotExist(err) {
		return rep, nil
	}

	entries, err := os.ReadDir(sessionsDir)
	if err != nil {
		return rep, nil
	}

	var matchedDirs []string
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		dirPath := filepath.Join(sessionsDir, entry.Name())
		if hasMatchingPathReference(dirPath, oldVariants) {
			matchedDirs = append(matchedDirs, entry.Name())
		}
	}

	if len(matchedDirs) == 0 {
		return rep, nil
	}

	targetSlug := piFamilySlug(agentName, newDir)

	for _, d := range matchedDirs {
		oldDirPath := filepath.Join(sessionsDir, d)
		newDirPath := filepath.Join(sessionsDir, targetSlug)
		rep.Details = append(rep.Details, fmt.Sprintf("Rename session dir: %s -> %s", d, targetSlug))

		subEntries, _ := os.ReadDir(oldDirPath)
		for _, se := range subEntries {
			if strings.HasSuffix(se.Name(), ".jsonl") {
				sessPath := filepath.Join(oldDirPath, se.Name())
				if fileContainsAny(sessPath, oldVariants) {
					rep.Details = append(rep.Details, fmt.Sprintf("Update session reference in %s", se.Name()))
					if !dryRun {
						for _, variant := range oldVariants {
							replaceInFile(sessPath, variant, newDir)
						}
					}
				}
			}
		}

		if !dryRun {
			if oldDirPath != newDirPath {
				_ = os.Rename(oldDirPath, newDirPath)
			}
		}
	}

	return rep, nil
}

func hasMatchingPathReference(dirPath string, variants []string) bool {
	files, err := os.ReadDir(dirPath)
	if err != nil {
		return false
	}
	for _, f := range files {
		if !strings.HasSuffix(f.Name(), ".jsonl") && !strings.HasSuffix(f.Name(), ".json") {
			continue
		}
		fp := filepath.Join(dirPath, f.Name())
		if fileContainsAny(fp, variants) {
			return true
		}
	}
	return false
}

func fileContainsAny(filePath string, variants []string) bool {
	file, err := os.Open(filePath)
	if err != nil {
		return false
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 256*1024), 1024*1024)
	for scanner.Scan() {
		line := scanner.Bytes()
		for _, v := range variants {
			if bytes.Contains(line, []byte(v)) {
				return true
			}
		}
	}
	return false
}

func replaceInFile(filePath, oldStr, newStr string) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return
	}
	if !bytes.Contains(data, []byte(oldStr)) {
		return
	}
	replaced := bytes.ReplaceAll(data, []byte(oldStr), []byte(newStr))
	_ = os.WriteFile(filePath, replaced, 0644)
}
