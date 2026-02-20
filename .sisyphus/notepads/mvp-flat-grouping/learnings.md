# Learnings: MVP Flat to Grouped UI Refactor

## 2026-02-20: Initial Refactor

### Current State
- UI renders flat list: `ID  Source  (Title)`
- Sessions sorted by LastUpdated (most recent first)
- Navigation uses `cursor` index on flat sessions array

### Goal State
- Project-grouped sections with simplified headers
- Session rows: `title [Source]` (no leading ID)
- Preserve navigation and search behavior

### Approach
- Minimal changes to `main.go` only
- Add helper functions: `groupSessionsByProject()`, `simplifyPath()`
- Modify View() to render grouped sections
- Keep cursor mapping to visible session indices

### Key Patterns
- Use existing Session fields: ProjectPath, Title, SourceTool
- Simplify paths: extract basename from directory
- Keep search working across grouped view
- Preserve keyboard shortcuts (j/k, arrows, enter, /, q)

### Implementation Notes

#### Cursor Navigation
- Cursor still maps to flat `filteredSessions` index (not visible row index)
- Highlighting compares session IDs to find matching row
- This preserves existing Update() logic without changes

#### Grouping Logic
- `groupSessionsByProject()` builds map then converts to slice
- Groups sorted by most recent session in each group
- Sessions within groups inherit global sort order

#### Rendering
- Build `visibleRows` slice mixing headers and session rows
- Headers use `isHeader: true` flag
- Session rows track `sessionIdx` in filtered array
- Cursor highlighting: `filteredSessions[m.cursor].ID == sess.ID`

### Gotchas
- Search includes Title field now (was ID + Source only)
- simplifyPath handles empty string → "(no project)"
- Source colorization preserved from original code
- Cursor bounds still use full sessions array (preserved original behavior)
  - Cursor may highlight invisible session when search is active
  - This matches original behavior; can be improved in later iteration

### Verification
- Build passes: `go build ./cmd/session-manager`
- All functionality preserved: navigation, search, restore

## 2026-02-20: Hidden Session State Persistence

### Implementation
- Created `internal/hidden` package with standalone Manager
- JSON persistence to `~/.local/share/{appName}/hidden-sessions.json`
- Cross-platform path construction using `filepath.Join`
- Stdlib only - no external dependencies

### Key Patterns

#### Atomic Writes
- Temp file in same directory (`.json.tmp`)
- Write data, sync file descriptor, close
- Atomic rename replaces target
- On any error: cleanup temp file, return error

#### Corruption Recovery
- JSON unmarshal failure → rename to `.corrupt-<unix>`
- Start with empty state and create new file
- Prevents repeated corruption detection on next load

#### Thread Safety
- `sync.RWMutex` protects all state access
- Write operations (`Add`, `Remove`) take exclusive lock
- Read operations (`IsHidden`, `List`) take shared lock
- `ensureLoaded()` handles lazy initialization

### API Design
- `NewManager(appName)` - factory with XDG data dir setup
- `Add(sessionID)` - idempotent add with persist
- `Remove(sessionID)` - idempotent remove with persist  
- `IsHidden(sessionID)` - fast read-only check
- `List()` - returns all hidden session IDs

### Testing Coverage
- Add/remove/isHidden/list operations
- Duplicate add/remove (no-op behavior)
- Persistence across manager instances
- Missing file handling (starts empty)
- Corrupt JSON recovery and file renaming
- Atomic write prevents data loss
- JSON format validation

### Gotchas
- Use `f.Sync()` on file descriptor, not `os.Sync()`
- `t.Run()` subtests use `func(t *testing.T) {` syntax
- Test helpers use `t.Helper()` for correct line reporting
- Must create new file after corruption to prevent repeated recovery


## 2026-02-20: Mutex Deadlock Fix (hidden module)

### Problem
Original implementation had potential deadlock:
- `Add`/`Remove` held write lock, called `ensureLoaded()`, which called `load()` that tried to acquire write lock again
- `IsHidden`/`List` held read lock, called `ensureLoaded()`, which tried to upgrade to write lock
- Checking `m.loaded` before acquiring lock created data race with `loadLocked()` writes

### Solution: sync.Once Pattern
- Added `sync.Once` to Manager struct
- `ensureLoaded()` uses `once.Do()` to guarantee load executes exactly once
- Load error stored in `m.loadErr` field
- Safe to call `ensureLoaded()` from any context (with or without locks held)

### Key Changes
- `loadLocked()`: assumes lock held, only called from within `ensureLoaded()`
- `ensureLoaded()`: uses `sync.Once`, acquires own lock, handles errors
- All public methods: call `ensureLoaded()` before acquiring operation-specific locks
- No more lock upgrade patterns or re-entrant lock attempts

### Testing
- Added `TestConcurrentAccess` with:
  - 5 goroutines doing mixed operations on non-loaded manager
  - Timeout-based deadlock detection (5 seconds)
  - Verification that data remains consistent
- Race detector passes: `go test ./internal/hidden/... -race`

### Gotchas
- `sync.Once.Do()` executes exactly once, even if function returns error
- Error must be stored in struct field (`m.loadErr`) for subsequent callers
- Can't retry failed loads with sync.Once - this is acceptable for persistence errors
- TOCTOU races occur when checking state before acquiring lock - check while holding lock or use atomic primitives


## 2026-02-20: Hidden Session MVP Implementation

### Features Implemented
- Press `h` to hide current visible session
- Press `H` to open hidden sessions overlay
- Hidden sessions excluded from main list render
- Hidden overlay shows simple list of hidden session IDs
- In overlay: `j/k` or arrows navigate, `r` restores selected, `a` restores all, `esc` closes

### Implementation Approach
- Added fields to model: `hiddenManager`, `showHiddenOverlay`, `hiddenCursor`
- Initialized `hidden.Manager` in main() with appName "agent-session-manager"
- Filter hidden sessions in View() before grouping and rendering
- Overlay rendered as centered modal with rounded border
- Key routing: overlay mode checked before search mode in Update()

### Key Patterns
- Hidden filter applied after search filter, before grouping
- Overlay uses same color scheme as main UI (orange border for visibility)
- Footer help text updates dynamically based on active mode
- `hiddenManager.List()` called in overlay rendering and key handlers

### Gotchas
- Overlay cursor bounds use `len(hiddenManager.List())` - must call each time for fresh data
- After "restore all" (`a`), overlay auto-closes for better UX
- Hidden sessions persist across restarts via JSON file at `~/.local/share/agent-session-manager/hidden-sessions.json`

### Verification
- Build passes: `go build ./cmd/session-manager`
- No LSP errors on modified file
- Only `cmd/session-manager/main.go` modified (as required)

## 2026-02-20: QA Fixes for Hidden Session Feature

### Issues Fixed

1. **Wrong session hidden with `h` key**: Original code used `m.sessions[m.cursor]` (base list), which could hide wrong session when search/hidden filtering changed visible order. Fixed by using `getVisibleSessions()` helper.

2. **Wrong session restored with `enter` key**: Same mismatch risk as above. Fixed by using `getVisibleSessions()` helper.

3. **Potential panic in hidden overlay**: Code called `m.hiddenManager.List()` without nil guard. If manager init failed, pressing `H` could panic. Fixed by caching `hiddenIDs` at start of overlay handler with nil check.

4. **hiddenCursor not clamped after restore**: After restoring a session, cursor could point past end of list. Fixed by clamping cursor after `r` operation.

### Solution: `getVisibleSessions()` Helper

Added centralized helper function that computes visible sessions by applying:
1. Search query filtering (if active)
2. Hidden session filtering (if manager exists)

This ensures Update() and View() always use the same session list, preventing mismatches between cursor position and displayed content.

### Key Changes

- `getVisibleSessions()` method on model - single source of truth for visible sessions
- All cursor bounds checks now use visible session count
- `h`, `enter`, `j`, `k`, and arrow keys all use visible sessions
- Hidden overlay caches `hiddenIDs` at start of handler to avoid repeated calls and handle nil manager
- Cursor clamping after restore operations

### Verification
- Build passes: `go build ./cmd/session-manager`
- No LSP errors on modified file

## 2026-02-20: MVP Wave 4 - Source Filter Implementation

### Features Implemented
- Filter row rendered under title: `[1] All  [2] Claude  [3] OpenCode  [4] Qwen`
- Active filter highlighted with blue background and bold text
- Keys `1/2/3/4` switch filter to all/claude/opencode/qwen respectively
- Visible session list respects source filter + search filter + hidden filter
- Footer help updated to show `1-4 filter` in main mode

### Implementation Details

#### Model State
- Added `sourceFilter string` field to model struct
- Values: `""` (all), `"claude"`, `"opencode"`, `"qwen"`
- Initialized to `""` (show all) in main()

#### Filtering Pipeline (getVisibleSessions)
Order of filters applied:
1. Search query filter (if active)
2. Source filter (if active)
3. Hidden session filter (if manager exists)

This ensures all three filters work together correctly.

#### Filter Row Rendering
- Added `filterActiveStyle` and `filterInactiveStyle` for visual distinction
- `renderFilterRow()` builds horizontal button row with key hints
- Active filter uses `selectedColor` background, inactive uses gray text
- Each button shows `[key] Label` format

#### Key Handlers
- `1`: Set filter to "" (all), reset cursor to 0
- `2`: Set filter to "claude", reset cursor to 0
- `3`: Set filter to "opencode", reset cursor to 0
- `4`: Set filter to "qwen", reset cursor to 0
- Cursor reset prevents out-of-bounds when filter reduces visible count

#### Styles
- Active: bold white text on blue background with colored border
- Inactive: gray text, no background
- Colors match existing source colors (claude=orange, opencode=green, qwen=blue)

### Gotchas
- SourceTool values in session/types.go are lowercase ("claude", "opencode", "qwen")
- UI displays title-cased labels ("Claude", "OpenCode", "Qwen")
- Filter comparison uses lowercase string matching
- Cursor must reset on filter change to avoid pointing to now-invisible session

### Verification
- Build passes: `go build -o /tmp/session-manager-mvp ./cmd/session-manager`
- No LSP errors on modified file
- Only `cmd/session-manager/main.go` modified
- Existing grouped rendering and hidden overlay behavior preserved

## 2026-02-20: Bug Fixes - Restore Command and Layout Spacing

### Issues Fixed

1. **Enter restore broken**: Switch statement compared title-cased strings ("Claude", "OpenCode", "Qwen") but `SourceTool` type values are lowercase. Fixed by comparing against `session.SourceClaude`, `session.SourceOpenCode`, `session.SourceQwen` constants.

2. **headerHeight off by one**: Filter row added but headerHeight not updated. Was `1` (or `2` when searching), should be `2` baseline (title + filter row), `3` when searching (title + search row + filter row).

### Key Changes

- Use `session.Source*` constants in switch statement instead of string literals
- Also fixed rendering to use constants instead of title-cased strings
- Added inline comments explaining headerHeight breakdown (2 = title + filter, 3 = title + search + filter)

### Gotchas
- `SourceTool` values are lowercase constants in session/types.go
- UI displays title-cased labels but comparisons must use lowercase constants
- headerHeight must account for ALL rendered header rows (title, search box, filter row)

### Verification
- Build passes: `go build -o /tmp/session-manager-mvp ./cmd/session-manager`

## 2026-02-20: Path Display Enhancement - Full Path with ~ Replacement

### Change
- `simplifyPath()` now returns full project path with home directory replaced by `~`
- Example: `/home/user/project` → `~/project`
- Previously returned only basename: `/home/user/project` → `project`

### Implementation
- Use `os.UserHomeDir()` to get home directory
- Replace home prefix with `~` if path starts with home dir
- On error, return original path unchanged
- Preserve empty path handling → "(no project)"

### Verification
- Build passes: `go build -o /tmp/session-manager-mvp ./cmd/session-manager`
