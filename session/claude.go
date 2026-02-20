package session

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
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
	IsMeta    bool          `json:"isMeta"`
	Cwd       string        `json:"cwd"`
}

// ClaudeRawMsg represents the raw message content.
type ClaudeRawMsg struct {
	Content interface{} `json:"content"`
}

// DefaultClaudeBasePath is the default Claude projects path.
const DefaultClaudeBasePath = ".claude/projects"

var commandNamePattern = regexp.MustCompile(`<command-name>\s*([^<\s]+)\s*</command-name>`)

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

	var firstUserIntent string
	var commandName string // 暂存的命令名（如/clear）
	var lastUpdated int64
	var projectPath string

	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		var msg ClaudeMessage
		if err := json.Unmarshal([]byte(line), &msg); err != nil {
			continue
		}

		if ts := extractTimestampRFC3339Aware(msg.Timestamp); ts > lastUpdated {
			lastUpdated = ts
		}
		if projectPath == "" && strings.TrimSpace(msg.Cwd) != "" {
			projectPath = msg.Cwd
		}

		// 只在还没有找到用户意图时继续查找
		if firstUserIntent == "" {
			title, found := extractClaudeUserTitle(msg)
			if found {
				// 如果是命令名，先存起来，但继续找真正的用户输入
				if strings.HasPrefix(title, "/") && commandName == "" {
					commandName = title
				} else {
					// 真正的用户输入
					firstUserIntent = title
				}
			}
		}
	}

	// 优先级：1. 真正的用户输入 > 2. 命令名 > 3. 返回 nil（没有有效内容）
	if firstUserIntent == "" {
		if commandName != "" {
			firstUserIntent = commandName
		} else {
			// 没有找到任何有效的用户消息或命令，跳过这个会话
			return nil
		}
	}

	sessionID := strings.TrimSuffix(filepath.Base(jsonlPath), filepath.Ext(jsonlPath))
	if projectPath == "" {
		projectPath = projectDir
	}
	normalized := NormalizeSession(Session{
		ID:          sessionID,
		Title:       firstUserIntent,
		SourceTool:  SourceClaude,
		ProjectPath: projectPath,
		LastUpdated: lastUpdated,
	})
	return &normalized
}

func extractClaudeUserTitle(msg ClaudeMessage) (string, bool) {
	if msg.Type != "user" || msg.IsMeta || msg.Message == nil {
		return "", false
	}

	text := extractClaudeUserContent(msg.Message.Content)
	if text == "" {
		return "", false
	}

	// 跳过系统生成的消息
	if strings.Contains(text, "<local-command-caveat>") {
		return "", false
	}
	if strings.Contains(text, "<local-command-stdout>") {
		return "", false
	}

	// 提取命令名（如/clear, /plugin 等）
	if command, ok := extractCommandNameFromXMLLike(text); ok {
		// 命令名可能已经带/了，不要重复添加
		if !strings.HasPrefix(command, "/") {
			return "/" + command, true
		}
		return command, true
	}

	// 返回真正的用户输入
	return text, true
}

func extractClaudeUserContent(content interface{}) string {
	switch val := content.(type) {
	case string:
		return val
	case []interface{}:
		for _, item := range val {
			part, ok := item.(map[string]interface{})
			if !ok {
				continue
			}
			partType, _ := part["type"].(string)
			if partType != "text" {
				continue
			}
			text, _ := part["text"].(string)
			if strings.TrimSpace(text) != "" {
				return text
			}
		}
	}
	return ""
}

func extractCommandNameFromXMLLike(content string) (string, bool) {
	match := commandNamePattern.FindStringSubmatch(content)
	if len(match) != 2 {
		return "", false
	}
	command := strings.TrimSpace(match[1])
	if command == "" {
		return "", false
	}
	return command, true
}

// extractTimestampRFC3339Aware extracts timestamp as unix seconds from various types.
func extractTimestampRFC3339Aware(ts interface{}) int64 {
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
		if parsed, err := time.Parse(time.RFC3339Nano, v); err == nil {
			return parsed.Unix()
		}
	}

	return 0
}
