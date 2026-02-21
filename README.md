# Agent Session Manager

A Bubble Tea TUI for browsing and reopening AI coding sessions (Claude Code, OpenCode, Qwen, Codex).

## Status

This release is optimized for WSL-first workflows. The session restore path is:

1. Open a new Windows Terminal tab/window
2. Enter WSL shell
3. `cd` to the session project path
4. Run the restore command for the selected tool

## Build and Run

```bash
go build -o /tmp/session-manager-mvp ./cmd/session-manager
/tmp/session-manager-mvp
```

## Keybindings

- `/`: enter search input mode
- `Esc` (in search mode): clear and exit search
- `Tab` or `Left/Right`: switch panel focus
- `Up/Down` or `k/j`: move cursor
- `Enter` (left panel): open selected project shell (double press within 2s)
- `Enter` (right panel): resume selected session
- `h`: hide selected session
- `H`: show hidden sessions overlay
- `1..5`: source filter (All, Claude, OpenCode, Qwen, Codex)
- `q`: quit

## Configuration (Environment Variables)

You can customize launcher behavior without changing source code.

- `ASM_TERMINAL_CMD` (default: `wt`)
- `ASM_WSL_ENTRY_CMD` (default: `wsl.exe`)
- `ASM_WSL_SHELL` (default: `zsh -lic`)
- `ASM_RESTORE_CMD_CLAUDE` (default: `claude -r {id}`)
- `ASM_RESTORE_CMD_OPENCODE` (default: `opencode -s {id}`)
- `ASM_RESTORE_CMD_QWEN` (default: `qwen -r {id}`)
- `ASM_RESTORE_CMD_CODEX` (default: `codex resume {id}`)

Template placeholders:

- `{id}`: session ID (shell-quoted)
- `{project}`: project path (shell-quoted)

Example:

```bash
export ASM_WSL_SHELL="bash -lc"
export ASM_RESTORE_CMD_QWEN="qwen --resume {id}"
```

## Doctor Mode

Run a quick environment and launcher configuration check:

```bash
go run ./cmd/session-manager --doctor
```

The report includes:

- active launcher configuration
- WSL detection
- terminal binary discovery
- Windows interop status
- command availability for configured restore tools

## Troubleshooting

- `wt not found`: install Windows Terminal or set `ASM_TERMINAL_CMD`.
- `windows interop unavailable`: WSL cannot spawn Windows processes in current environment.
- `qwen: not found` (or similar): ensure command is available in configured shell, or override restore command via `ASM_RESTORE_CMD_*`.

## Tests

```bash
go test ./...
```
