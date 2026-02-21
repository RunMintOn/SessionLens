package session

import (
	"database/sql"
	"os"
	"path/filepath"

	_ "github.com/glebarez/go-sqlite"
)

// OpenCodeScanner scans OpenCode sessions from SQLite database.
type OpenCodeScanner struct {
	DBPath string
}

// NewOpenCodeScanner creates a new OpenCodeScanner with default DB path.
func NewOpenCodeScanner() *OpenCodeScanner {
	return &OpenCodeScanner{
		DBPath: filepath.Join(os.Getenv("HOME"), ".local", "share", "opencode", "opencode.db"),
	}
}

// Scan retrieves OpenCode sessions, optionally filtered by project path.
func (s *OpenCodeScanner) Scan(projectPath string) ([]Session, error) {
	var sessions []Session

	// Check if database file exists
	if _, err := os.Stat(s.DBPath); os.IsNotExist(err) {
		return sessions, nil
	}

	// Open database using standard database/sql interface
	db, err := sql.Open("sqlite", s.DBPath)
	if err != nil {
		return sessions, nil // Gracefully return empty on error
	}
	defer db.Close()

	// Query sessions table - align with Python scanner SQL
	// SELECT id, title, directory, time_updated FROM session WHERE parent_id IS NULL ORDER BY time_updated DESC
	rows, err := db.Query(`
		SELECT id, title, directory, time_updated
		FROM session
		WHERE parent_id IS NULL
		ORDER BY time_updated DESC
	`)
	if err != nil {
		return sessions, nil // Gracefully return empty on error
	}
	defer rows.Close()

	for rows.Next() {
		var id interface{}
		var title, directory interface{}
		var timeUpdated interface{}

		err := rows.Scan(&id, &title, &directory, &timeUpdated)
		if err != nil {
			continue
		}

		// Skip if directory is NULL
		if directory == nil {
			continue
		}

		// Filter by project path if specified
		dirStr := stringFromInterface(directory)
		if projectPath != "" && dirStr != projectPath {
			continue
		}

		// title defaults to "Untitled Session" if NULL
		titleStr := stringFromInterface(title)
		if titleStr == "" {
			titleStr = "Untitled Session"
		}

		sessions = append(sessions, NormalizeSession(Session{
			ID:          stringFromInterface(id),
			Title:       titleStr,
			SourceTool:  SourceOpenCode,
			ProjectPath: dirStr,
			LastUpdated: int64FromInterface(timeUpdated),
		}))
	}

	return sessions, nil
}

// stringFromInterface converts interface{} to string safely.
func stringFromInterface(v interface{}) string {
	if v == nil {
		return ""
	}
	switch val := v.(type) {
	case string:
		return val
	case []byte:
		return string(val)
	default:
		return ""
	}
}

// int64FromInterface converts interface{} to int64 safely.
func int64FromInterface(v interface{}) int64 {
	if v == nil {
		return 0
	}
	var ts int64
	switch val := v.(type) {
	case int64:
		ts = val
	case int:
		ts = int64(val)
	case float64:
		ts = int64(val)
	default:
		return 0
	}
	if ts > 1_000_000_000_000 {
		return ts / 1000
	}
	return ts
}
