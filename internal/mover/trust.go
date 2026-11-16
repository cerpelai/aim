package mover

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/cerpelai/aim/internal/agent"
)

// MoveAgentTrust checks and migrates directory trust / allowed paths settings for agents.
func MoveAgentTrust(oldDir, newDir string, oldVariants []string, dryRun bool) (Report, error) {
	rep := Report{Agent: "trust"}
	home, err := os.UserHomeDir()
	if err != nil {
		return rep, err
	}

	// 1. Claude settings (~/.claude/settings.json)
	claudeSettings := filepath.Join(home, ".claude", "settings.json")
	if details, err := updateJSONPathArrayOrKeys(claudeSettings, []string{"trustedDirectories", "approvedProjects", "trustedPaths"}, oldVariants, newDir, dryRun); err == nil {
		rep.Details = append(rep.Details, details...)
	}

	// 2. Agy settings (~/.gemini/antigravity-cli/settings.json)
	agySettings := filepath.Join(home, ".gemini", "antigravity-cli", "settings.json")
	if details, err := updateJSONPathArrayOrKeys(agySettings, []string{"trustedFolders", "trustedPaths", "trustedDirectories", "allowedWorkspaces"}, oldVariants, newDir, dryRun); err == nil {
		rep.Details = append(rep.Details, details...)
	}

	// 3. Gemini settings (~/.gemini/settings.json)
	geminiSettings := filepath.Join(home, ".gemini", "settings.json")
	if details, err := updateJSONPathArrayOrKeys(geminiSettings, []string{"trustedDirectories", "trustedFolders", "allowedDirectories"}, oldVariants, newDir, dryRun); err == nil {
		rep.Details = append(rep.Details, details...)
	}

	// 4. Omp settings (~/.omp/agent/config.yml or settings)
	ompSettings := filepath.Join(home, ".omp", "agent", "settings.json")
	if details, err := updateJSONPathArrayOrKeys(ompSettings, []string{"trustedDirectories", "trustedPaths"}, oldVariants, newDir, dryRun); err == nil {
		rep.Details = append(rep.Details, details...)
	}

	// 5. Pi settings (~/.pi/agent/settings.json)
	piSettings := filepath.Join(home, ".pi", "agent", "settings.json")
	if details, err := updateJSONPathArrayOrKeys(piSettings, []string{"trustedDirectories", "trustedPaths"}, oldVariants, newDir, dryRun); err == nil {
		rep.Details = append(rep.Details, details...)
	}

	return rep, nil
}

func updateJSONPathArrayOrKeys(filePath string, targetKeys []string, oldVariants []string, newDir string, dryRun bool) ([]string, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}

	var root map[string]any
	if err := json.Unmarshal(data, &root); err != nil {
		return nil, err
	}

	var details []string
	modified := false

	for _, key := range targetKeys {
		val, exists := root[key]
		if !exists {
			continue
		}

		// Case 1: val is array of strings (e.g. ["/path1", "/path2"])
		if arr, ok := val.([]any); ok {
			var newArr []any
			arrModified := false
			for _, item := range arr {
				if s, ok := item.(string); ok {
					matched := false
					for _, v := range oldVariants {
						if agent.PathsMatch(s, v) {
							matched = true
							break
						}
					}
					if matched {
						newArr = append(newArr, newDir)
						arrModified = true
						details = append(details, fmt.Sprintf("%s: %s -> %s in %s", filepath.Base(filePath), s, newDir, key))
					} else {
						newArr = append(newArr, item)
					}
				} else {
					newArr = append(newArr, item)
				}
			}
			if arrModified {
				root[key] = newArr
				modified = true
			}
		}

		// Case 2: val is map with path keys (e.g. {"/path1": true})
		if m, ok := val.(map[string]any); ok {
			for k, v := range m {
				matched := false
				for _, variant := range oldVariants {
					if agent.PathsMatch(k, variant) {
						matched = true
						break
					}
				}
				if matched {
					delete(m, k)
					m[newDir] = v
					modified = true
					details = append(details, fmt.Sprintf("%s: key %s -> %s in %s", filepath.Base(filePath), k, newDir, key))
				}
			}
		}
	}

	if modified && !dryRun {
		newData, err := json.MarshalIndent(root, "", "  ")
		if err == nil {
			_ = os.WriteFile(filePath, newData, 0644)
		}
	}

	return details, nil
}
