package session

import (
	"database/sql"
	"os"
	"path/filepath"
	"testing"

	_ "github.com/glebarez/go-sqlite"
)

func TestOpenCodeScanner_Scan_NormalReading(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("failed to open db: %v", err)
	}
	defer db.Close()

	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS session (
			id INTEGER PRIMARY KEY,
			title TEXT,
			directory TEXT,
			time_updated INTEGER,
			parent_id INTEGER
		);
		INSERT INTO session (id, title, directory, time_updated, parent_id) VALUES 
		(1, 'Test Session', '/path/to/project', 1700000000, NULL),
		(2, 'Another Session', '/path/to/other', 1700000001, NULL);
	`)
	if err != nil {
		t.Fatalf("failed to setup db: %v", err)
	}

	scanner := &OpenCodeScanner{DBPath: dbPath}
	sessions, err := scanner.Scan("")

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(sessions) != 2 {
		t.Errorf("expected 2 sessions, got %d", len(sessions))
	}

	if sessions[0].Title != "Another Session" {
		t.Errorf("expected first session to be 'Another Session', got %s", sessions[0].Title)
	}

	if sessions[0].SourceTool != SourceOpenCode {
		t.Errorf("expected SourceTool to be opencode, got %s", sessions[0].SourceTool)
	}
}

func TestOpenCodeScanner_Scan_DirectoryNULL(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("failed to open db: %v", err)
	}
	defer db.Close()

	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS session (
			id INTEGER PRIMARY KEY,
			title TEXT,
			directory TEXT,
			time_updated INTEGER,
			parent_id INTEGER
		);
		INSERT INTO session (id, title, directory, time_updated, parent_id) VALUES 
		(1, 'Valid Session', '/path/to/project', 1700000000, NULL),
		(2, 'NULL Directory Session', NULL, 1700000001, NULL);
	`)
	if err != nil {
		t.Fatalf("failed to setup db: %v", err)
	}

	scanner := &OpenCodeScanner{DBPath: dbPath}
	sessions, err := scanner.Scan("")

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(sessions) != 1 {
		t.Errorf("expected 1 session (NULL directory filtered), got %d", len(sessions))
	}

	if sessions[0].Title != "Valid Session" {
		t.Errorf("expected session title 'Valid Session', got %s", sessions[0].Title)
	}
}

func TestOpenCodeScanner_Scan_TitleNULL(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("failed to open db: %v", err)
	}
	defer db.Close()

	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS session (
			id INTEGER PRIMARY KEY,
			title TEXT,
			directory TEXT,
			time_updated INTEGER,
			parent_id INTEGER
		);
		INSERT INTO session (id, title, directory, time_updated, parent_id) VALUES 
		(1, NULL, '/path/to/project', 1700000000, NULL);
	`)
	if err != nil {
		t.Fatalf("failed to setup db: %v", err)
	}

	scanner := &OpenCodeScanner{DBPath: dbPath}
	sessions, err := scanner.Scan("")

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(sessions) != 1 {
		t.Errorf("expected 1 session, got %d", len(sessions))
	}

	if sessions[0].Title != "Untitled Session" {
		t.Errorf("expected title 'Untitled Session' for NULL title, got %s", sessions[0].Title)
	}
}

func TestOpenCodeScanner_Scan_MissingDBFile(t *testing.T) {
	scanner := &OpenCodeScanner{DBPath: "/nonexistent/path/test.db"}
	sessions, err := scanner.Scan("")

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(sessions) != 0 {
		t.Errorf("expected 0 sessions for missing db, got %d", len(sessions))
	}
}

func TestOpenCodeScanner_Scan_MissingTable(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("failed to open db: %v", err)
	}
	defer db.Close()

	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS other_table (
			id INTEGER PRIMARY KEY,
			name TEXT
		);
	`)
	if err != nil {
		t.Fatalf("failed to setup db: %v", err)
	}

	scanner := &OpenCodeScanner{DBPath: dbPath}
	sessions, err := scanner.Scan("")

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(sessions) != 0 {
		t.Errorf("expected 0 sessions for missing table, got %d", len(sessions))
	}
}

func TestOpenCodeScanner_Scan_FilterByProjectPath(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("failed to open db: %v", err)
	}
	defer db.Close()

	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS session (
			id INTEGER PRIMARY KEY,
			title TEXT,
			directory TEXT,
			time_updated INTEGER,
			parent_id INTEGER
		);
		INSERT INTO session (id, title, directory, time_updated, parent_id) VALUES 
		(1, 'Project A Session', '/path/to/projectA', 1700000000, NULL),
		(2, 'Project B Session', '/path/to/projectB', 1700000001, NULL);
	`)
	if err != nil {
		t.Fatalf("failed to setup db: %v", err)
	}

	scanner := &OpenCodeScanner{DBPath: dbPath}
	sessions, err := scanner.Scan("/path/to/projectA")

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(sessions) != 1 {
		t.Errorf("expected 1 session, got %d", len(sessions))
	}

	if sessions[0].ProjectPath != "/path/to/projectA" {
		t.Errorf("expected project path '/path/to/projectA', got %s", sessions[0].ProjectPath)
	}
}

func TestOpenCodeScanner_NewOpenCodeScanner(t *testing.T) {
	scanner := NewOpenCodeScanner()

	expectedPath := filepath.Join(os.Getenv("HOME"), ".local", "share", "opencode", "opencode.db")
	if scanner.DBPath != expectedPath {
		t.Errorf("expected DBPath %s, got %s", expectedPath, scanner.DBPath)
	}
}
