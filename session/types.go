package session

// SourceType represents the source tool that created the session.
type SourceType string

const (
	SourceOpenCode SourceType = "opencode"
	SourceClaude   SourceType = "claude"
	SourceQwen     SourceType = "qwen"
	SourceCodex    SourceType = "codex"
)

// Session represents an agent session.
type Session struct {
	ID          string
	Title       string
	SourceTool  SourceType
	ProjectPath string
	LastUpdated int64 // Unix timestamp
}

// Project represents a project with sessions.
type Project struct {
	Path     string
	Sessions []Session
}
