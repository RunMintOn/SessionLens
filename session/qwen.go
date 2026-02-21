package session

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// QwenScanner scans for Qwen Code sessions from JSONL files.
type QwenScanner struct {
	BasePath string
}

// NewQwenScanner creates a new QwenScanner with default base path.
func NewQwenScanner() *QwenScanner {
	return &QwenScanner{
		BasePath: filepath.Join(os.Getenv("HOME"), ".qwen", "projects"),
	}
}

// Scan implements the Scanner interface.
func (s *QwenScanner) Scan(projectPath string) ([]Session, error) {
	var sessions []Session

	basePath := s.BasePath
	if basePath == "" {
		basePath = filepath.Join(os.Getenv("HOME"), ".qwen", "projects")
	}

	info, err := os.Stat(basePath)
	if err != nil || !info.IsDir() {
		return sessions, nil
	}

	entries, err := os.ReadDir(basePath)
	if err != nil {
		return nil, err
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		projDir := entry.Name()
		if projectPath != "" && !strings.HasPrefix(projDir, "-"+projectPath) {
			continue
		}

		chatsDir := filepath.Join(basePath, projDir, "chats")
		sessions = append(sessions, s.scanChatsDir(chatsDir, filepath.Join(basePath, projDir))...)
	}

	sort.Slice(sessions, func(i, j int) bool {
		return sessions[i].LastUpdated > sessions[j].LastUpdated
	})

	return sessions, nil
}

// scanChatsDir scans a chats directory for Qwen session files.
func (s *QwenScanner) scanChatsDir(chatsDir, projectDir string) []Session {
	var sessions []Session

	info, err := os.Stat(chatsDir)
	if err != nil || !info.IsDir() {
		return sessions
	}

	entries, err := os.ReadDir(chatsDir)
	if err != nil {
		return sessions
	}

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".jsonl") {
			continue
		}

		jsonlPath := filepath.Join(chatsDir, entry.Name())
		session := s.parseSession(jsonlPath, projectDir)
		if session != nil {
			sessions = append(sessions, *session)
		}
	}

	return sessions
}

// qwenMessage represents a Qwen JSONL message structure.
type qwenMessage struct {
	Type      string             `json:"type"`
	Message   qwenMessageContent `json:"message"`
	Timestamp any                `json:"timestamp"`
	CreatedAt any                `json:"createdAt"`
	Cwd       string             `json:"cwd"`
}

// qwenMessageContent represents the message content.
type qwenMessageContent struct {
	Parts []any `json:"parts"`
}

// parseSession parses a single Qwen JSONL file.
func (s *QwenScanner) parseSession(jsonlPath, projectDir string) *Session {
	file, err := os.Open(jsonlPath)
	if err != nil {
		return nil
	}
	defer file.Close()

	var firstUserMessage string
	var lastUpdated int64
	var projectPath string

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		var msg qwenMessage
		if err := json.Unmarshal([]byte(line), &msg); err != nil {
			continue // skip malformed JSON
		}
		if projectPath == "" && strings.TrimSpace(msg.Cwd) != "" {
			projectPath = msg.Cwd
		}

		if msg.Type == "user" && firstUserMessage == "" {
			// Extract text from message.parts
			for _, part := range msg.Message.Parts {
				if partMap, ok := part.(map[string]any); ok {
					if text, ok := partMap["text"].(string); ok {
						firstUserMessage = text
						break
					}
				}
			}
		}

		// Get timestamp - try timestamp first, then createdAt
		ts := s.extractTimestamp(msg.Timestamp)
		if ts == 0 {
			ts = s.extractTimestamp(msg.CreatedAt)
		}
		if ts > lastUpdated {
			lastUpdated = ts
		}
	}

	if firstUserMessage == "" {
		// Fallback to file stem
		firstUserMessage = strings.TrimSuffix(filepath.Base(jsonlPath), filepath.Ext(jsonlPath))
	}

	sessionID := strings.TrimSuffix(filepath.Base(jsonlPath), filepath.Ext(jsonlPath))
	if projectPath == "" {
		projectPath = projectDir
	}

	normalized := NormalizeSession(Session{
		ID:          sessionID,
		Title:       firstUserMessage,
		SourceTool:  SourceQwen,
		ProjectPath: projectPath,
		LastUpdated: lastUpdated,
	})
	return &normalized
}

// extractTimestamp extracts an int64 timestamp from various types.
func (s *QwenScanner) extractTimestamp(ts any) int64 {
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
		// Try to parse as integer
		var result int64
		for _, c := range v {
			if c >= '0' && c <= '9' {
				result = result*10 + int64(c-'0')
			}
		}
		return result
	}
	return 0
}
