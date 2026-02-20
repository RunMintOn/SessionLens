package session

import (
	"errors"
	"testing"
)

// mockScanner implements Scanner interface for testing
type mockScanner struct {
	sessions []Session
	err      error
}

func (m *mockScanner) Scan(projectPath string) ([]Session, error) {
	return m.sessions, m.err
}

// TestScanAll_BasicGroups tests that sessions are grouped by ProjectPath
func TestScanAll_BasicGroups(t *testing.T) {
	originalScanners := scannersForTest
	defer func() { scannersForTest = originalScanners }()

	scannersForTest = []struct {
		name    string
		scanner Scanner
	}{
		{"mock1", &mockScanner{
			sessions: []Session{
				{ID: "1", Title: "Session 1", SourceTool: SourceOpenCode, ProjectPath: "/path/a", LastUpdated: 100},
				{ID: "2", Title: "Session 2", SourceTool: SourceOpenCode, ProjectPath: "/path/b", LastUpdated: 200},
			},
		}},
		{"mock2", &mockScanner{
			sessions: []Session{
				{ID: "3", Title: "Session 3", SourceTool: SourceClaude, ProjectPath: "/path/a", LastUpdated: 150},
			},
		}},
	}

	projects, err := scan_all()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(projects) != 2 {
		t.Fatalf("expected 2 projects, got %d", len(projects))
	}

	projectPaths := make(map[string]bool)
	for _, p := range projects {
		projectPaths[p.Path] = true
		if len(p.Sessions) == 0 {
			t.Errorf("project %s has no sessions", p.Path)
		}
	}

	if !projectPaths["/path/a"] {
		t.Error("missing /path/a project")
	}
	if !projectPaths["/path/b"] {
		t.Error("missing /path/b project")
	}
}

// TestScanAll_Deduplication tests that duplicate sessions (same ID + Source) are removed
func TestScanAll_Deduplication(t *testing.T) {
	originalScanners := scannersForTest
	defer func() { scannersForTest = originalScanners }()

	scannersForTest = []struct {
		name    string
		scanner Scanner
	}{
		{"mock1", &mockScanner{
			sessions: []Session{
				{ID: "1", Title: "Session 1", SourceTool: SourceOpenCode, ProjectPath: "/path/a", LastUpdated: 100},
			},
		}},
		{"mock2", &mockScanner{
			sessions: []Session{
				{ID: "1", Title: "Session 1 Duplicate", SourceTool: SourceOpenCode, ProjectPath: "/path/b", LastUpdated: 200},
			},
		}},
	}

	projects, err := scan_all()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var totalSessions int
	for _, p := range projects {
		totalSessions += len(p.Sessions)
	}

	if totalSessions != 1 {
		t.Errorf("expected 1 session after deduplication, got %d", totalSessions)
	}
}

// TestScanAll_SortByLastUpdated tests that sessions within projects are sorted by LastUpdated descending
func TestScanAll_SortByLastUpdated(t *testing.T) {
	originalScanners := scannersForTest
	defer func() { scannersForTest = originalScanners }()

	scannersForTest = []struct {
		name    string
		scanner Scanner
	}{
		{"mock1", &mockScanner{
			sessions: []Session{
				{ID: "1", Title: "Old", SourceTool: SourceOpenCode, ProjectPath: "/path/a", LastUpdated: 100},
				{ID: "2", Title: "New", SourceTool: SourceOpenCode, ProjectPath: "/path/a", LastUpdated: 300},
				{ID: "3", Title: "Middle", SourceTool: SourceClaude, ProjectPath: "/path/a", LastUpdated: 200},
			},
		}},
	}

	projects, err := scan_all()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(projects) != 1 {
		t.Fatalf("expected 1 project, got %d", len(projects))
	}

	sessions := projects[0].Sessions
	if len(sessions) != 3 {
		t.Fatalf("expected 3 sessions, got %d", len(sessions))
	}

	if sessions[0].LastUpdated != 300 {
		t.Errorf("expected first session to have LastUpdated=300, got %d", sessions[0].LastUpdated)
	}
	if sessions[1].LastUpdated != 200 {
		t.Errorf("expected second session to have LastUpdated=200, got %d", sessions[1].LastUpdated)
	}
	if sessions[2].LastUpdated != 100 {
		t.Errorf("expected third session to have LastUpdated=100, got %d", sessions[2].LastUpdated)
	}
}

// TestScanAll_ScannerErrorContinues tests that one scanner failure doesn't affect others
func TestScanAll_ScannerErrorContinues(t *testing.T) {
	originalScanners := scannersForTest
	defer func() { scannersForTest = originalScanners }()

	scannersForTest = []struct {
		name    string
		scanner Scanner
	}{
		{"mock1", &mockScanner{
			sessions: []Session{
				{ID: "1", Title: "Session 1", SourceTool: SourceOpenCode, ProjectPath: "/path/a", LastUpdated: 100},
			},
		}},
		{"mock2", &mockScanner{
			sessions: nil,
			err:      errors.New("scanner error"),
		}},
		{"mock3", &mockScanner{
			sessions: []Session{
				{ID: "2", Title: "Session 2", SourceTool: SourceClaude, ProjectPath: "/path/b", LastUpdated: 200},
			},
		}},
	}

	projects, err := scan_all()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var totalSessions int
	for _, p := range projects {
		totalSessions += len(p.Sessions)
	}

	if totalSessions != 2 {
		t.Errorf("expected 2 sessions (1 success + 1 success after error), got %d", totalSessions)
	}
}

// TestScanAll_DifferentSourcesNotDuplicates tests that same ID with different sources are not duplicates
func TestScanAll_DifferentSourcesNotDuplicates(t *testing.T) {
	originalScanners := scannersForTest
	defer func() { scannersForTest = originalScanners }()

	scannersForTest = []struct {
		name    string
		scanner Scanner
	}{
		{"mock1", &mockScanner{
			sessions: []Session{
				{ID: "1", Title: "Session 1", SourceTool: SourceOpenCode, ProjectPath: "/path/a", LastUpdated: 100},
			},
		}},
		{"mock2", &mockScanner{
			sessions: []Session{
				{ID: "1", Title: "Session 1", SourceTool: SourceClaude, ProjectPath: "/path/b", LastUpdated: 200},
			},
		}},
	}

	projects, err := scan_all()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var totalSessions int
	for _, p := range projects {
		totalSessions += len(p.Sessions)
	}

	if totalSessions != 2 {
		t.Errorf("expected 2 sessions (different sources), got %d", totalSessions)
	}
}

// TestScanAll_EmptyResults tests that empty scanner results work correctly
func TestScanAll_EmptyResults(t *testing.T) {
	originalScanners := scannersForTest
	defer func() { scannersForTest = originalScanners }()

	scannersForTest = []struct {
		name    string
		scanner Scanner
	}{
		{"mock1", &mockScanner{sessions: []Session{}}},
		{"mock2", &mockScanner{sessions: []Session{}}},
	}

	projects, err := scan_all()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(projects) != 0 {
		t.Errorf("expected 0 projects, got %d", len(projects))
	}
}
