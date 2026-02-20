## Task 5.1: Search Functionality

### Modified Files
- `cmd/session-manager/main.go` - Added search mode with `/` key

### Implementation Details
- **Search State**: Added `searching bool` and `query string` fields to model
- **Key Handling**:
  - `/` - Enter search mode
  - `Esc` - Exit search mode and clear query
  - `Enter` - Exit search mode (keep query for reference)
  - `Backspace` - Delete character from query
  - Other keys - Add to query (character by character)
- **Filtering**: Case-insensitive filtering using `strings.Contains(strings.ToLower(...), strings.ToLower(...))`
- **Search Targets**: Filters by both session ID and source (OpenCode/Claude/Qwen)

### Styles Added
- `searchBoxStyle`: Rounded border with blue (#569CD6) border
- `searchPromptStyle`: Blue prompt "/" 
- `searchInputStyle`: White text for query input

### UI Updates
- Search box appears below header when in search mode
- Session list shows filtered count: "Sessions (X/Y):" when query is active
- Footer changes to "type to filter  Esc exit" when in search mode

### Key Decisions
- Cursor resets to 0 when query changes (allows fresh navigation in filtered results)
- Cursor resets when exiting search mode
- Empty query shows all sessions (no filtering)

### Verification
- `go build ./cmd/session-manager` - PASSED
- No lsp_diagnostics errors
