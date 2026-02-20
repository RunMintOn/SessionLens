package hidden

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestNewManager_Validation(t *testing.T) {
	if _, err := NewManager(""); err == nil {
		t.Fatal("expected error for empty app name")
	}
}

func TestManager_AddListRemove(t *testing.T) {
	mgr := createTestManager(t)

	if err := mgr.Add("s1", "Session One"); err != nil {
		t.Fatalf("Add failed: %v", err)
	}
	if err := mgr.Add("s2", "Session Two"); err != nil {
		t.Fatalf("Add failed: %v", err)
	}

	if !mgr.IsHidden("s1") || !mgr.IsHidden("s2") {
		t.Fatal("expected both sessions to be hidden")
	}

	list := mgr.List()
	if len(list) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(list))
	}
	if list[0].ID != "s1" || list[1].ID != "s2" {
		t.Fatalf("expected stable sorted order by ID, got %+v", list)
	}

	if err := mgr.Remove("s1"); err != nil {
		t.Fatalf("Remove failed: %v", err)
	}
	if mgr.IsHidden("s1") {
		t.Fatal("s1 should be removed")
	}
	if !mgr.IsHidden("s2") {
		t.Fatal("s2 should still be hidden")
	}
}

func TestManager_DuplicateAdd_NoOverwrite(t *testing.T) {
	mgr := createTestManager(t)

	if err := mgr.Add("same-id", "first title"); err != nil {
		t.Fatalf("Add failed: %v", err)
	}
	if err := mgr.Add("same-id", "second title"); err != nil {
		t.Fatalf("Add failed: %v", err)
	}

	list := mgr.List()
	if len(list) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(list))
	}
	if list[0].Title != "first title" {
		t.Fatalf("duplicate add should keep first title, got %q", list[0].Title)
	}
}

func TestManager_Persistence(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := hiddenFilePath(tmpDir)

	mgr1 := newManagerAtPath(filePath)
	if err := mgr1.Add("p1", "Persisted Session"); err != nil {
		t.Fatalf("Add failed: %v", err)
	}

	mgr2 := newManagerAtPath(filePath)
	if !mgr2.IsHidden("p1") {
		t.Fatal("expected hidden entry after reload")
	}
	list := mgr2.List()
	if len(list) != 1 || list[0].Title != "Persisted Session" {
		t.Fatalf("unexpected persisted entries: %+v", list)
	}
}

func TestManager_LoadLegacyStringFormat(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := hiddenFilePath(tmpDir)
	if err := os.MkdirAll(filepath.Dir(filePath), 0755); err != nil {
		t.Fatalf("failed to create data dir: %v", err)
	}

	legacy := struct {
		Hidden []string `json:"hidden"`
	}{
		Hidden: []string{"legacy-1", "legacy-2"},
	}
	raw, err := json.Marshal(legacy)
	if err != nil {
		t.Fatalf("failed to marshal legacy json: %v", err)
	}
	if err := os.WriteFile(filePath, raw, 0644); err != nil {
		t.Fatalf("failed to write legacy json: %v", err)
	}

	mgr := newManagerAtPath(filePath)
	list := mgr.List()
	if len(list) != 2 {
		t.Fatalf("expected 2 legacy entries, got %d", len(list))
	}
	if list[0].ID != "legacy-1" || list[0].Title != "legacy-1" {
		t.Fatalf("expected legacy title fallback to ID, got %+v", list[0])
	}
	if list[1].ID != "legacy-2" || list[1].Title != "legacy-2" {
		t.Fatalf("expected legacy title fallback to ID, got %+v", list[1])
	}
}

func TestManager_CorruptJSONRecovery(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := hiddenFilePath(tmpDir)
	if err := os.MkdirAll(filepath.Dir(filePath), 0755); err != nil {
		t.Fatalf("failed to create data dir: %v", err)
	}

	if err := os.WriteFile(filePath, []byte("{invalid-json"), 0644); err != nil {
		t.Fatalf("failed to write corrupt json: %v", err)
	}

	mgr := newManagerAtPath(filePath)
	list := mgr.List()
	if len(list) != 0 {
		t.Fatalf("expected empty list after recovery, got %d", len(list))
	}

	entries, err := os.ReadDir(filepath.Dir(filePath))
	if err != nil {
		t.Fatalf("failed to read data dir: %v", err)
	}

	hasCorruptBackup := false
	for _, entry := range entries {
		if filepath.Ext(entry.Name()) == ".json" {
			continue
		}
		if len(entry.Name()) > len("hidden-sessions.json.corrupt-") &&
			entry.Name()[:len("hidden-sessions.json.corrupt-")] == "hidden-sessions.json.corrupt-" {
			hasCorruptBackup = true
			break
		}
	}

	if !hasCorruptBackup {
		t.Fatal("expected corrupt backup file after recovery")
	}
}

func createTestManager(t *testing.T) *Manager {
	t.Helper()
	return newManagerAtPath(hiddenFilePath(t.TempDir()))
}

func newManagerAtPath(filePath string) *Manager {
	mgr := &Manager{
		appName:   "testapp",
		filePath:  filePath,
		hiddenMap: make(map[string]HiddenEntry),
		loaded:    false,
	}
	_ = mgr.ensureLoaded()
	return mgr
}

func hiddenFilePath(tmpDir string) string {
	return filepath.Join(tmpDir, ".local", "share", "testapp", "hidden-sessions.json")
}
