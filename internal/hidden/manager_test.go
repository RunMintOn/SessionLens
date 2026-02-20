package hidden

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestNewManager(t *testing.T) {
	t.Run("creates manager with valid app name", func(t *testing.T) {
		mgr, err := NewManager("test-app")
		if err != nil {
			t.Fatalf("NewManager failed: %v", err)
		}
		if mgr == nil {
			t.Fatal("manager is nil")
		}
		if mgr.appName != "test-app" {
			t.Errorf("expected appName 'test-app', got '%s'", mgr.appName)
		}
	})

	t.Run("fails with empty app name", func(t *testing.T) {
		_, err := NewManager("")
		if err == nil {
			t.Error("expected error for empty appName, got nil")
		}
	})

	t.Run("filePath includes appName", func(t *testing.T) {
		mgr, err := NewManager("myapp")
		if err != nil {
			t.Fatalf("NewManager failed: %v", err)
		}
		if !strings.Contains(mgr.filePath, "myapp") {
			t.Errorf("filePath %q should contain appName", mgr.filePath)
		}
		if !strings.HasSuffix(mgr.filePath, "hidden-sessions.json") {
			t.Errorf("filePath %q should end with hidden-sessions.json", mgr.filePath)
		}
	})
}

func TestConcurrentAccess(t *testing.T) {
	t.Run("concurrent operations on non-loaded manager", func(t *testing.T) {
		tmpDir := t.TempDir()

		dataDir := filepath.Join(tmpDir, ".local", "share", "testapp")
		filePath := filepath.Join(dataDir, "hidden-sessions.json")

		mgr := &Manager{
			appName:   "testapp",
			filePath:  filePath,
			hiddenSet: make(map[string]bool),
			loaded:    false,
		}

		// Spawn multiple goroutines doing different operations
		done := make(chan bool, 5)

		// Goroutine 1: Add operations
		go func() {
			for i := 0; i < 10; i++ {
				mgr.Add(fmt.Sprintf("add-%d", i))
			}
			done <- true
		}()

		// Goroutine 2: Remove operations
		go func() {
			for i := 0; i < 5; i++ {
				mgr.Remove(fmt.Sprintf("remove-%d", i))
			}
			done <- true
		}()

		// Goroutine 3: IsHidden checks
		go func() {
			for i := 0; i < 20; i++ {
				mgr.IsHidden(fmt.Sprintf("check-%d", i))
			}
			done <- true
		}()

		// Goroutine 4: List operations
		go func() {
			for i := 0; i < 10; i++ {
				mgr.List()
			}
			done <- true
		}()

		// Goroutine 5: Mixed operations
		go func() {
			for i := 0; i < 10; i++ {
				mgr.Add(fmt.Sprintf("mixed-%d", i))
				mgr.IsHidden(fmt.Sprintf("mixed-%d", i))
				mgr.List()
			}
			done <- true
		}()

		// Wait for all goroutines with timeout
		timeout := time.After(5 * time.Second)
		completed := 0
		for completed < 5 {
			select {
			case <-done:
				completed++
			case <-timeout:
				t.Fatal("timeout: probable deadlock detected")
			}
		}

		// Verify final state is consistent
		list := mgr.List()
		if len(list) == 0 {
			t.Error("expected some sessions to be added")
		}
	})

	t.Run("ensureLoaded double-check prevents duplicate load", func(t *testing.T) {
		tmpDir := t.TempDir()
		mgr := createTestManager(t, tmpDir)

		if err := mgr.Add("test-1"); err != nil {
			t.Fatalf("Add failed: %v", err)
		}

		dataDir := filepath.Join(tmpDir, ".local", "share", "testapp")
		filePath := filepath.Join(dataDir, "hidden-sessions.json")

		mgr2 := &Manager{
			appName:   "testapp",
			filePath:  filePath,
			hiddenSet: make(map[string]bool),
			loaded:    false,
		}

		// Multiple concurrent calls to ensureLoaded should not cause issues
		done := make(chan bool, 3)
		for i := 0; i < 3; i++ {
			go func() {
				mgr2.ensureLoaded()
				done <- true
			}()
		}

		for i := 0; i < 3; i++ {
			select {
			case <-done:
			case <-time.After(1 * time.Second):
				t.Fatal("timeout in ensureLoaded concurrent calls")
			}
		}

		if !mgr2.IsHidden("test-1") {
			t.Error("session should be loaded after ensureLoaded")
		}
	})
}

func TestAdd(t *testing.T) {
	t.Run("adds session ID successfully", func(t *testing.T) {
		tmpDir := t.TempDir()
		mgr := createTestManager(t, tmpDir)

		err := mgr.Add("session-1")
		if err != nil {
			t.Fatalf("Add failed: %v", err)
		}

		if !mgr.IsHidden("session-1") {
			t.Error("session-1 should be hidden")
		}

		list := mgr.List()
		if len(list) != 1 {
			t.Errorf("expected 1 hidden session, got %d", len(list))
		}
		if list[0] != "session-1" {
			t.Errorf("expected 'session-1', got '%s'", list[0])
		}
	})

	t.Run("add multiple sessions", func(t *testing.T) {
		tmpDir := t.TempDir()
		mgr := createTestManager(t, tmpDir)

		sessions := []string{"s1", "s2", "s3"}
		for _, s := range sessions {
			if err := mgr.Add(s); err != nil {
				t.Fatalf("Add(%s) failed: %v", s, err)
			}
		}

		list := mgr.List()
		if len(list) != 3 {
			t.Errorf("expected 3 hidden sessions, got %d", len(list))
		}

		for _, s := range sessions {
			if !mgr.IsHidden(s) {
				t.Errorf("%s should be hidden", s)
			}
		}
	})

	t.Run("adding duplicate is no-op", func(t *testing.T) {
		tmpDir := t.TempDir()
		mgr := createTestManager(t, tmpDir)

		if err := mgr.Add("s1"); err != nil {
			t.Fatalf("first Add failed: %v", err)
		}
		if err := mgr.Add("s1"); err != nil {
			t.Fatalf("second Add failed: %v", err)
		}

		list := mgr.List()
		if len(list) != 1 {
			t.Errorf("expected 1 hidden session after duplicate add, got %d", len(list))
		}
	})

	t.Run("fails with empty session ID", func(t *testing.T) {
		tmpDir := t.TempDir()
		mgr := createTestManager(t, tmpDir)

		err := mgr.Add("")
		if err == nil {
			t.Error("expected error for empty sessionID, got nil")
		}
	})
}

func TestRemove(t *testing.T) {
	t.Run("removes existing session", func(t *testing.T) {
		tmpDir := t.TempDir()
		mgr := createTestManager(t, tmpDir)

		if err := mgr.Add("s1"); err != nil {
			t.Fatalf("Add failed: %v", err)
		}
		if err := mgr.Remove("s1"); err != nil {
			t.Fatalf("Remove failed: %v", err)
		}

		if mgr.IsHidden("s1") {
			t.Error("s1 should not be hidden after removal")
		}

		list := mgr.List()
		if len(list) != 0 {
			t.Errorf("expected 0 hidden sessions, got %d", len(list))
		}
	})

	t.Run("removing non-existent is no-op", func(t *testing.T) {
		tmpDir := t.TempDir()
		mgr := createTestManager(t, tmpDir)

		if err := mgr.Remove("nonexistent"); err != nil {
			t.Fatalf("Remove failed: %v", err)
		}

		list := mgr.List()
		if len(list) != 0 {
			t.Errorf("expected 0 hidden sessions, got %d", len(list))
		}
	})

	t.Run("fails with empty session ID", func(t *testing.T) {
		tmpDir := t.TempDir()
		mgr := createTestManager(t, tmpDir)

		err := mgr.Remove("")
		if err == nil {
			t.Error("expected error for empty sessionID, got nil")
		}
	})
}

func TestIsHidden(t *testing.T) {
	t.Run("returns false for non-hidden session", func(t *testing.T) {
		tmpDir := t.TempDir()
		mgr := createTestManager(t, tmpDir)

		if mgr.IsHidden("s1") {
			t.Error("non-hidden session should return false")
		}
	})

	t.Run("returns true for hidden session", func(t *testing.T) {
		tmpDir := t.TempDir()
		mgr := createTestManager(t, tmpDir)

		if err := mgr.Add("s1"); err != nil {
			t.Fatalf("Add failed: %v", err)
		}

		if !mgr.IsHidden("s1") {
			t.Error("hidden session should return true")
		}
	})
}

func TestList(t *testing.T) {
	t.Run("returns empty list when no sessions", func(t *testing.T) {
		tmpDir := t.TempDir()
		mgr := createTestManager(t, tmpDir)

		list := mgr.List()
		if list == nil {
			t.Error("List should return empty slice, not nil")
		}
		if len(list) != 0 {
			t.Errorf("expected empty list, got %d items", len(list))
		}
	})

	t.Run("returns all hidden sessions", func(t *testing.T) {
		tmpDir := t.TempDir()
		mgr := createTestManager(t, tmpDir)

		sessions := []string{"a", "b", "c"}
		for _, s := range sessions {
			if err := mgr.Add(s); err != nil {
				t.Fatalf("Add(%s) failed: %v", s, err)
			}
		}

		list := mgr.List()
		if len(list) != 3 {
			t.Errorf("expected 3 sessions, got %d", len(list))
		}

		sessionSet := make(map[string]bool)
		for _, s := range list {
			sessionSet[s] = true
		}
		for _, s := range sessions {
			if !sessionSet[s] {
				t.Errorf("expected session %s in list", s)
			}
		}
	})
}

func TestPersistence(t *testing.T) {
	t.Run("persists data to disk", func(t *testing.T) {
		tmpDir := t.TempDir()
		mgr1 := createTestManager(t, tmpDir)

		sessions := []string{"s1", "s2", "s3"}
		for _, s := range sessions {
			if err := mgr1.Add(s); err != nil {
				t.Fatalf("Add failed: %v", err)
			}
		}

		list1 := mgr1.List()
		if len(list1) != 3 {
			t.Fatalf("expected 3 sessions before reload, got %d", len(list1))
		}

		mgr2 := createTestManager(t, tmpDir)
		list2 := mgr2.List()
		if len(list2) != 3 {
			t.Errorf("expected 3 sessions after reload, got %d", len(list2))
		}

		for _, s := range sessions {
			if !mgr2.IsHidden(s) {
				t.Errorf("session %s should be hidden after reload", s)
			}
		}
	})

	t.Run("starts with missing file", func(t *testing.T) {
		tmpDir := t.TempDir()
		mgr := createTestManager(t, tmpDir)

		if mgr.IsHidden("anything") {
			t.Error("should not have any hidden sessions with missing file")
		}

		list := mgr.List()
		if len(list) != 0 {
			t.Errorf("expected empty list with missing file, got %d items", len(list))
		}
	})

	t.Run("handles corrupt JSON file", func(t *testing.T) {
		tmpDir := t.TempDir()
		mgr := createTestManager(t, tmpDir)

		dataPath := mgr.filePath
		dataDir := filepath.Dir(dataPath)
		if err := os.MkdirAll(dataDir, 0755); err != nil {
			t.Fatalf("failed to create data dir: %v", err)
		}

		if err := os.WriteFile(dataPath, []byte("corrupt json{invalid"), 0644); err != nil {
			t.Fatalf("failed to write corrupt file: %v", err)
		}

		mgr2 := createTestManager(t, tmpDir)

		list := mgr2.List()
		if len(list) != 0 {
			t.Errorf("expected empty list after corrupt file, got %d items", len(list))
		}

		entries, err := os.ReadDir(dataDir)
		if err != nil {
			t.Fatalf("failed to read data dir: %v", err)
		}

		var hasCorrupt bool
		var hasDataFile bool
		for _, e := range entries {
			name := e.Name()
			if strings.Contains(name, ".corrupt-") {
				hasCorrupt = true
			}
			if name == "hidden-sessions.json" {
				hasDataFile = true
			}
		}

		if !hasCorrupt {
			t.Error("corrupt file should be renamed to .corrupt-<timestamp>")
		}
		if !hasDataFile {
			t.Error("new data file should be created after corrupt recovery")
		}
	})

	t.Run("atomic write prevents data loss", func(t *testing.T) {
		tmpDir := t.TempDir()
		mgr1 := createTestManager(t, tmpDir)

		if err := mgr1.Add("s1"); err != nil {
			t.Fatalf("Add failed: %v", err)
		}

		mgr2 := createTestManager(t, tmpDir)
		if !mgr2.IsHidden("s1") {
			t.Error("s1 should persist across manager instances")
		}

		if err := mgr2.Add("s2"); err != nil {
			t.Fatalf("Add failed: %v", err)
		}

		mgr3 := createTestManager(t, tmpDir)
		if !mgr3.IsHidden("s1") {
			t.Error("s1 should still exist after s2 added")
		}
		if !mgr3.IsHidden("s2") {
			t.Error("s2 should persist")
		}

		list := mgr3.List()
		if len(list) != 2 {
			t.Errorf("expected 2 sessions, got %d", len(list))
		}
	})
}

func TestJSONFormat(t *testing.T) {
	t.Run("writes valid JSON format", func(t *testing.T) {
		tmpDir := t.TempDir()
		mgr := createTestManager(t, tmpDir)

		if err := mgr.Add("test-session"); err != nil {
			t.Fatalf("Add failed: %v", err)
		}

		data, err := os.ReadFile(mgr.filePath)
		if err != nil {
			t.Fatalf("failed to read file: %v", err)
		}

		var result hiddenData
		if err := json.Unmarshal(data, &result); err != nil {
			t.Fatalf("failed to unmarshal JSON: %v", err)
		}

		if len(result.Hidden) != 1 {
			t.Errorf("expected 1 hidden item in JSON, got %d", len(result.Hidden))
		}
		if result.Hidden[0] != "test-session" {
			t.Errorf("expected 'test-session', got '%s'", result.Hidden[0])
		}
	})
}

func createTestManager(t *testing.T, tmpDir string) *Manager {
	t.Helper()

	dataDir := filepath.Join(tmpDir, ".local", "share", "testapp")
	filePath := filepath.Join(dataDir, "hidden-sessions.json")

	mgr := &Manager{
		appName:   "testapp",
		filePath:  filePath,
		hiddenSet: make(map[string]bool),
		loaded:    false,
	}

	if err := mgr.ensureLoaded(); err != nil {
		t.Fatalf("failed to load manager: %v", err)
	}

	return mgr
}
