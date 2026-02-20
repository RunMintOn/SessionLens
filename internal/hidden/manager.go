package hidden

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"
)

// HiddenEntry represents a hidden session with ID and title.
type HiddenEntry struct {
	ID    string `json:"id"`
	Title string `json:"title"`
}

// Manager manages hidden sessions with persistent storage.
type Manager struct {
	mu         sync.RWMutex
	once       sync.Once
	appName    string
	filePath   string
	hiddenMap  map[string]HiddenEntry  // ID -> Entry
	loaded     bool
	loadErr    error
}

// hiddenData represents the JSON structure stored on disk.
type hiddenData struct {
	Hidden []HiddenEntry `json:"hidden"`
}

// NewManager creates a new Manager for the given app name.
// The persistent storage file will be at ~/.local/share/{appName}/hidden-sessions.json
// on Linux, with appropriate paths on other platforms.
func NewManager(appName string) (*Manager, error) {
	if appName == "" {
		return nil, fmt.Errorf("appName cannot be empty")
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("failed to get home directory: %w", err)
	}

	// Use XDG data directory pattern: ~/.local/share/{appName}/
	dataDir := filepath.Join(homeDir, ".local", "share", appName)
	filePath := filepath.Join(dataDir, "hidden-sessions.json")

	m := &Manager{
		appName:   appName,
		filePath:  filePath,
		hiddenMap: make(map[string]HiddenEntry),
		loaded:    false,
	}

	// Initial load (non-fatal if it fails)
	m.mu.Lock()
	m.loadLocked()
	m.mu.Unlock()

	return m, nil
}

// loadLocked loads hidden session entries from disk. Must be called WITH mu lock held.
func (m *Manager) loadLocked() error {

	data, err := os.ReadFile(m.filePath)
	if err != nil {
		if os.IsNotExist(err) {
			// File doesn't exist yet - start with empty state
			m.hiddenMap = make(map[string]HiddenEntry)
			m.loaded = true
			return nil
		}
		return fmt.Errorf("failed to read file: %w", err)
	}

	var hidden hiddenData
	if err := json.Unmarshal(data, &hidden); err != nil {
		// Backward compatibility: old format was {"hidden":["id1","id2"]}.
		var legacy struct {
			Hidden []string `json:"hidden"`
		}
		if legacyErr := json.Unmarshal(data, &legacy); legacyErr == nil {
			m.hiddenMap = make(map[string]HiddenEntry)
			for _, id := range legacy.Hidden {
				if id == "" {
					continue
				}
				m.hiddenMap[id] = HiddenEntry{ID: id, Title: id}
			}
			m.loaded = true
			return nil
		}

		// Corrupt JSON - rename file and start fresh
		corruptPath := fmt.Sprintf("%s.corrupt-%d", m.filePath, time.Now().Unix())
		if err := os.Rename(m.filePath, corruptPath); err != nil {
			return fmt.Errorf("corrupt JSON detected and failed to rename file: %w", err)
		}
		m.hiddenMap = make(map[string]HiddenEntry)
		m.loaded = true
		// Create new empty file to prevent repeated corruption detection
		if err := m.save(); err != nil {
			return fmt.Errorf("failed to create new file after corruption recovery: %w", err)
		}
		return nil
	}

	m.hiddenMap = make(map[string]HiddenEntry)
	for _, entry := range hidden.Hidden {
		m.hiddenMap[entry.ID] = entry
	}
	m.loaded = true
	return nil
}

// save writes hidden session entries to disk atomically. Must be called with mu held.
func (m *Manager) save() error {
	// Ensure data directory exists
	dataDir := filepath.Dir(m.filePath)
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return fmt.Errorf("failed to create data directory: %w", err)
	}

	// Build list of hidden entries
	var hiddenList []HiddenEntry
	for _, entry := range m.hiddenMap {
		hiddenList = append(hiddenList, entry)
	}
	sort.Slice(hiddenList, func(i, j int) bool {
		return hiddenList[i].ID < hiddenList[j].ID
	})

	data := hiddenData{Hidden: hiddenList}
	jsonData, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal JSON: %w", err)
	}

	tmpPath := m.filePath + ".tmp"

	f, err := os.OpenFile(tmpPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}

	if _, err := f.Write(jsonData); err != nil {
		f.Close()
		os.Remove(tmpPath)
		return fmt.Errorf("failed to write temp file: %w", err)
	}

	// Sync to ensure data is written to disk
	if err := f.Sync(); err != nil {
		f.Close()
		os.Remove(tmpPath)
		return fmt.Errorf("failed to sync temp file: %w", err)
	}

	if err := f.Close(); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("failed to close temp file: %w", err)
	}

	// Atomic rename
	if err := os.Rename(tmpPath, m.filePath); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("failed to rename temp file: %w", err)
	}

	return nil
}

// ensureLoaded ensures data is loaded from disk before operations.
// Uses sync.Once to guarantee load is called exactly once.
// Safe to call from any context (with or without locks held).
func (m *Manager) ensureLoaded() error {
	m.once.Do(func() {
		m.mu.Lock()
		m.loadErr = m.loadLocked()
		m.mu.Unlock()
	})
	return m.loadErr
}

// Add adds a session to the hidden list and persists to disk.
func (m *Manager) Add(sessionID, title string) error {
	if sessionID == "" {
		return fmt.Errorf("sessionID cannot be empty")
	}

	if err := m.ensureLoaded(); err != nil {
		return err
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.hiddenMap[sessionID]; exists {
		return nil
	}

	m.hiddenMap[sessionID] = HiddenEntry{ID: sessionID, Title: title}
	return m.save()
}

// Remove removes a session ID from the hidden list and persists to disk.
func (m *Manager) Remove(sessionID string) error {
	if sessionID == "" {
		return fmt.Errorf("sessionID cannot be empty")
	}

	if err := m.ensureLoaded(); err != nil {
		return err
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.hiddenMap[sessionID]; !exists {
		return nil
	}

	delete(m.hiddenMap, sessionID)
	return m.save()
}

// IsHidden returns true if the session ID is in the hidden list.
func (m *Manager) IsHidden(sessionID string) bool {
	m.ensureLoaded()

	m.mu.RLock()
	defer m.mu.RUnlock()
	_, exists := m.hiddenMap[sessionID]
	return exists
}

// List returns all hidden session entries.
func (m *Manager) List() []HiddenEntry {
	m.ensureLoaded()

	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]HiddenEntry, 0, len(m.hiddenMap))
	for _, entry := range m.hiddenMap {
		result = append(result, entry)
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].ID < result[j].ID
	})
	return result
}
