## Task 2.1 Decisions

### SQLite Dependency Choice
- **Selected**: `github.com/glebarez/go-sqlite`
- **Rationale**: Pure Go implementation, CGO-free, compatible with `database/sql`
- **Alternative rejected**: `mattn/go-sqlite3` (requires CGO)

### Interface Design
- Scanner interface mirrors Python's `scanner.scan(project_path)` pattern
- Returns `[]Session` and `error` - simple and compatible with existing types
- Scanner implementations (OpenCode, Claude, Qwen) will be added in future tasks

### Package Structure
- `session/` package contains both types (`types.go`) and interfaces (`scanner.go`)
- This mirrors the Python `scanner/` module structure
- Types are already defined in `types.go` (from Task 2.2), scanner.go adds the interface
