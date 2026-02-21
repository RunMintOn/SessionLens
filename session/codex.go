package session

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

// CodexScanner scans Codex sessions from JSONL files.
type CodexScanner struct {
	BasePath string
}

// NewCodexScanner creates a new CodexScanner with default base path.
func NewCodexScanner() *CodexScanner {
	home, err := os.UserHomeDir()
	if err != nil {
		return &CodexScanner{BasePath: ""}
	}

	return &CodexScanner{BasePath: filepath.Join(home, ".codex", "sessions")}
}

// Scan implements the Scanner interface.
func (s *CodexScanner) Scan(projectPath string) ([]Session, error) {
	var sessions []Session

	if s.BasePath == "" {
		return sessions, nil
	}

	info, err := os.Stat(s.BasePath)
	if err != nil || !info.IsDir() {
		return sessions, nil
	}

	normalizedFilter := ""
	if projectPath != "" {
		normalizedFilter = NormalizeProjectPath(projectPath)
	}

	err = filepath.WalkDir(s.BasePath, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return nil
		}

		if d.IsDir() {
			if strings.EqualFold(d.Name(), "subagents") {
				return filepath.SkipDir
			}
			return nil
		}

		if !strings.HasSuffix(strings.ToLower(d.Name()), ".jsonl") {
			return nil
		}

		if strings.Contains(path, string(filepath.Separator)+"subagents"+string(filepath.Separator)) {
			return nil
		}

		session := s.parseSession(path)
		if session == nil {
			return nil
		}

		if normalizedFilter != "" && session.ProjectPath != normalizedFilter {
			return nil
		}

		sessions = append(sessions, *session)
		return nil
	})
	if err != nil {
		return sessions, nil
	}

	sort.Slice(sessions, func(i, j int) bool {
		return sessions[i].LastUpdated > sessions[j].LastUpdated
	})

	return sessions, nil
}

func (s *CodexScanner) parseSession(jsonlPath string) *Session {
	file, err := os.Open(jsonlPath)
	if err != nil {
		return nil
	}
	defer file.Close()

	sessionID := strings.TrimSuffix(filepath.Base(jsonlPath), filepath.Ext(jsonlPath))
	projectPath := ""
	title := ""
	var lastUpdated int64

	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		var evt codexEvent
		if err := json.Unmarshal([]byte(line), &evt); err != nil {
			continue
		}

		if ts := extractTimestampRFC3339OrUnix(evt.Timestamp); ts > lastUpdated {
			lastUpdated = ts
		}

		metaID, metaCwd, metaTS := extractCodexSessionMeta(evt)
		if strings.TrimSpace(metaID) != "" {
			sessionID = strings.TrimSpace(metaID)
		}
		if projectPath == "" && strings.TrimSpace(metaCwd) != "" {
			projectPath = strings.TrimSpace(metaCwd)
		}
		if metaTS > lastUpdated {
			lastUpdated = metaTS
		}

		if projectPath == "" {
			if cwd := extractCwdFromPayload(evt.Payload); cwd != "" {
				projectPath = cwd
			}
		}

		if title == "" {
			if t, ok := extractCodexTitle(evt); ok {
				title = t
			}
		}
	}

	if strings.TrimSpace(title) == "" {
		return nil
	}

	if projectPath == "" {
		projectPath = filepath.Dir(jsonlPath)
	}

	if lastUpdated == 0 {
		if info, err := os.Stat(jsonlPath); err == nil {
			lastUpdated = info.ModTime().Unix()
		}
	}

	normalized := NormalizeSession(Session{
		ID:          sessionID,
		Title:       title,
		SourceTool:  SourceCodex,
		ProjectPath: projectPath,
		LastUpdated: lastUpdated,
	})
	return &normalized
}

type codexEvent struct {
	Timestamp any `json:"timestamp"`
	Type      string
	Payload   any
}

func extractCodexSessionMeta(evt codexEvent) (id string, cwd string, ts int64) {
	if evt.Type != "session_meta" {
		return "", "", 0
	}

	payload, ok := evt.Payload.(map[string]any)
	if !ok {
		return "", "", 0
	}

	id = stringFromAny(payload["id"])
	cwd = stringFromAny(payload["cwd"])
	ts = extractTimestampRFC3339OrUnix(payload["timestamp"])
	return id, cwd, ts
}

func extractCwdFromPayload(payload any) string {
	payloadMap, ok := payload.(map[string]any)
	if !ok {
		return ""
	}

	cwd := strings.TrimSpace(stringFromAny(payloadMap["cwd"]))
	if cwd != "" {
		return cwd
	}

	return ""
}

func extractCodexTitle(evt codexEvent) (string, bool) {
	payload, ok := evt.Payload.(map[string]any)
	if !ok {
		return "", false
	}

	switch evt.Type {
	case "event_msg":
		msgType := strings.TrimSpace(stringFromAny(payload["type"]))
		if msgType != "user_message" {
			return "", false
		}
		text := strings.TrimSpace(stringFromAny(payload["message"]))
		if !isValidCodexUserTitle(text) {
			return "", false
		}
		return text, true
	case "response_item":
		if strings.TrimSpace(stringFromAny(payload["role"])) != "user" {
			return "", false
		}
		content, ok := payload["content"].([]any)
		if !ok {
			return "", false
		}
		for _, item := range content {
			itemMap, ok := item.(map[string]any)
			if !ok {
				continue
			}
			if strings.TrimSpace(stringFromAny(itemMap["type"])) != "input_text" {
				continue
			}
			text := strings.TrimSpace(stringFromAny(itemMap["text"]))
			if !isValidCodexUserTitle(text) {
				continue
			}
			return text, true
		}
	}

	return "", false
}

func isValidCodexUserTitle(text string) bool {
	text = strings.TrimSpace(text)
	if text == "" {
		return false
	}
	if strings.HasPrefix(text, "<") {
		return false
	}
	if strings.HasPrefix(text, "# AGENTS.md") {
		return false
	}
	if strings.HasPrefix(text, "<environment_context>") {
		return false
	}
	return true
}

func stringFromAny(v any) string {
	switch val := v.(type) {
	case string:
		return val
	case []byte:
		return string(val)
	default:
		return ""
	}
}

func extractTimestampRFC3339OrUnix(ts any) int64 {
	if ts == nil {
		return 0
	}

	switch v := ts.(type) {
	case float64:
		return normalizeUnixMaybeMillis(int64(v))
	case int:
		return normalizeUnixMaybeMillis(int64(v))
	case int64:
		return normalizeUnixMaybeMillis(v)
	case json.Number:
		if i, err := v.Int64(); err == nil {
			return normalizeUnixMaybeMillis(i)
		}
		if f, err := v.Float64(); err == nil {
			return normalizeUnixMaybeMillis(int64(f))
		}
		return 0
	case string:
		s := strings.TrimSpace(v)
		if s == "" {
			return 0
		}
		if t, err := time.Parse(time.RFC3339Nano, s); err == nil {
			return t.Unix()
		}
		if t, err := time.Parse(time.RFC3339, s); err == nil {
			return t.Unix()
		}
		if i, err := strconv.ParseInt(s, 10, 64); err == nil {
			return normalizeUnixMaybeMillis(i)
		}
	}

	return 0
}

func normalizeUnixMaybeMillis(ts int64) int64 {
	if ts <= 0 {
		return 0
	}
	if ts > 1_000_000_000_000 {
		return ts / 1000
	}
	return ts
}
