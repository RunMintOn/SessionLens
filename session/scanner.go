package session

import (
	"fmt"
	"log"
	"sort"
)

// Scanner interface defines the contract for session scanners
type Scanner interface {
	Scan(projectPath string) ([]Session, error)
}

// scannersForTest allows overriding scanners in tests
var scannersForTest = []struct {
	name    string
	scanner Scanner
}{
	{"OpenCode", NewOpenCodeScanner()},
	{"Claude", NewClaudeScanner()},
	{"Qwen", NewQwenScanner()},
	{"Codex", NewCodexScanner()},
}

// scan_all scans all sessions from all scanners and groups them by project path.
func scan_all() ([]Project, error) {
	var allSessions []Session

	for _, s := range scannersForTest {
		sessions, err := s.scanner.Scan("")
		if err != nil {
			log.Printf("Error scanning %s: %v", s.name, err)
			continue
		}
		allSessions = append(allSessions, sessions...)
	}

	seen := make(map[string]bool)
	var uniqueSessions []Session
	for _, sess := range allSessions {
		key := fmt.Sprintf("%s|%s", sess.ID, sess.SourceTool)
		if !seen[key] {
			seen[key] = true
			uniqueSessions = append(uniqueSessions, sess)
		}
	}

	sort.Slice(uniqueSessions, func(i, j int) bool {
		return uniqueSessions[i].LastUpdated > uniqueSessions[j].LastUpdated
	})

	projectMap := make(map[string]*Project)
	for _, sess := range uniqueSessions {
		if proj, exists := projectMap[sess.ProjectPath]; exists {
			proj.Sessions = append(proj.Sessions, sess)
		} else {
			projectMap[sess.ProjectPath] = &Project{
				Path:     sess.ProjectPath,
				Sessions: []Session{sess},
			}
		}
	}

	var projects []Project
	for _, proj := range projectMap {
		projects = append(projects, *proj)
	}

	sort.Slice(projects, func(i, j int) bool {
		var iTime, jTime int64
		if len(projects[i].Sessions) > 0 {
			iTime = projects[i].Sessions[0].LastUpdated
		}
		if len(projects[j].Sessions) > 0 {
			jTime = projects[j].Sessions[0].LastUpdated
		}
		return iTime > jTime
	})

	return projects, nil
}
