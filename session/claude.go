package session

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

// ClaudeScanner scans Claude Code sessions from JSONL files.
type ClaudeScanner struct {
	BasePath string
}

// ClaudeMessage represents a message in Claude JSONL format.
type ClaudeMessage struct {
	Type      string        `json:"type"`
	Timestamp interface{}   `json:"timestamp"`
	Message   *ClaudeRawMsg `json:"message,omitempty"`
}

// ClaudeRawMsg represents the raw message content.
type ClaudeRawMsg struct {
	Content interface{} `json:"content"`
}

// ClaudeTextPart represents a text part in message content.
type ClaudeTextPart struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

// DefaultClaudeBasePath is the default Claude projects path.
const DefaultClaudeBasePath = ".claude/projects"

// NewClaudeScanner creates a new ClaudeScanner with default base path.
func NewClaudeScanner() *ClaudeScanner {
	home, err := os.UserHomeDir()
	if err != nil {
		return &ClaudeScanner{BasePath: ""}
	}
	return &ClaudeScanner{BasePath: filepath.Join(home, DefaultClaudeBasePath)}
}

// Scan scans for Claude Code sessions in the base path.
// If projectPath is provided, only scans that specific project directory.
func (s *ClaudeScanner) Scan(projectPath string) ([]Session, error) {
	var sessions []Session

	if s.BasePath == "" {
		return sessions, nil
	}

	if projectPath != "" {
		projDir := filepath.Join(s.BasePath, projectPath)
		files, err := os.ReadDir(projDir)
		if err != nil {
			return sessions, nil
		}

		for _, file := range files {
			if file.IsDir() || !strings.HasSuffix(file.Name(), ".jsonl") {
				continue
			}

			session := s.parseSession(filepath.Join(projDir, file.Name()), projDir)
			if session != nil {
				sessions = append(sessions, *session)
			}
		}

		sort.Slice(sessions, func(i, j int) bool {
			return sessions[i].LastUpdated > sessions[j].LastUpdated
		})
		return sessions, nil
	}

	entries, err := os.ReadDir(s.BasePath)
	if err != nil {
		return sessions, nil
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		projDir := filepath.Join(s.BasePath, entry.Name())
		files, err := os.ReadDir(projDir)
		if err != nil {
			continue
		}

		for _, file := range files {
			if file.IsDir() || !strings.HasSuffix(file.Name(), ".jsonl") {
				continue
			}

			session := s.parseSession(filepath.Join(projDir, file.Name()), projDir)
			if session != nil {
				sessions = append(sessions, *session)
			}
		}
	}

	sort.Slice(sessions, func(i, j int) bool {
		return sessions[i].LastUpdated > sessions[j].LastUpdated
	})

	return sessions, nil
}

// parseSession parses a single Claude JSONL file.
func (s *ClaudeScanner) parseSession(jsonlPath, projectDir string) *Session {
	file, err := os.Open(jsonlPath)
	if err != nil {
		return nil
	}
	defer file.Close()

	var firstUserMessage string
	var lastUpdated int64

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.TrimSpace(line) == "" {
			continue
		}

		var msg ClaudeMessage
		if err := json.Unmarshal([]byte(line), &msg); err != nil {
			continue
		}

		if msg.Type == "user" && firstUserMessage == "" {
			if msg.Message != nil {
				firstUserMessage = extractUserContent(msg.Message.Content)
			}
		}

		if msg.Timestamp != nil {
			ts := extractTimestamp(msg.Timestamp)
			if ts > lastUpdated {
				lastUpdated = ts
			}
		}
	}

	// Fallback to file stem if no user message found
	if firstUserMessage == "" {
		firstUserMessage = strings.TrimSuffix(filepath.Base(jsonlPath), filepath.Ext(jsonlPath))
	}

	sessionID := strings.TrimSuffix(filepath.Base(jsonlPath), filepath.Ext(jsonlPath))

	return &Session{
		ID:          sessionID,
		Title:       firstUserMessage,
		SourceTool:  SourceClaude,
		ProjectPath: projectDir,
		LastUpdated: lastUpdated,
	}
}

// extractUserContent extracts user message content from various formats.
func extractUserContent(content interface{}) string {
	if content == nil {
		return ""
	}

	// Handle string content
	if str, ok := content.(string); ok {
		return truncate(str, 100)
	}

	// Handle array content
	if arr, ok := content.([]interface{}); ok {
		for _, item := range arr {
			if part, ok := item.(map[string]interface{}); ok {
				if partType, ok := part["type"].(string); ok && partType == "text" {
					if text, ok := part["text"].(string); ok {
						return truncate(text, 100)
					}
				}
			}
		}
	}

	return ""
}

// extractTimestamp extracts timestamp as int64 from various types.
func extractTimestamp(ts interface{}) int64 {
	if ts == nil {
		return 0
	}

	switch v := ts.(type) {
	case float64:
		return int64(v)
	case int64:
		return v
	case int:
		return int64(v)
	case string:
		if i, err := strconv.ParseInt(v, 10, 64); err == nil {
			return i
		}
	}

	return 0
}

// truncate truncates a string to maxLen, appending "..." if truncated.
func truncate(s string, maxLen int) string {
	runes := []rune(s)
	if len(runes) <= maxLen {
		return s
	}
	return string(runes[:maxLen]) + "..."
}
