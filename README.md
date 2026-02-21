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
- `ASM_TERMINAL_ARGS` (default platform-dependent, space-separated)
- `ASM_WSL_ENTRY_CMD` (default: `wsl.exe`)
- `ASM_WSL_SHELL` (default: `zsh -lic`)
- `ASM_HOST_SHELL_CMD` (default platform-dependent, usually `sh`)
- `ASM_HOST_SHELL_ARGS` (default platform-dependent, usually `-lc`)
- `ASM_RESTORE_CMD_CLAUDE` (default: `claude -r {id}`)
- `ASM_RESTORE_CMD_OPENCODE` (default: `opencode -s {id}`)
- `ASM_RESTORE_CMD_QWEN` (default: `qwen -r {id}`)
- `ASM_RESTORE_CMD_CODEX` (default: `codex resume {id}`)
- `ASM_CONFIG_PATH` (optional, override config file location)

Template placeholders:

- `{id}`: session ID (shell-quoted)
- `{project}`: project path (shell-quoted)

Example:

```bash
export ASM_WSL_SHELL="bash -lc"
export ASM_RESTORE_CMD_QWEN="qwen --resume {id}"
```

## Config File

Default config file path:

- Linux/WSL/macOS: `~/.config/agent-session-manager/config.json`
- Or override with `ASM_CONFIG_PATH`

Example config:

```json
{
  "terminal_cmd": "wt",
  "terminal_args": [],
  "wsl_entry_cmd": "wsl.exe",
  "wsl_shell": "zsh -lic",
  "host_shell_cmd": "sh",
  "host_shell_args": ["-lc"],
  "restore_cmd_claude": "claude -r {id}",
  "restore_cmd_opencode": "opencode -s {id}",
  "restore_cmd_qwen": "qwen -r {id}",
  "restore_cmd_codex": "codex resume {id}"
}
```

Priority:

1. `ASM_*` environment variables
2. Config file
3. Built-in defaults

## Doctor Mode

Run a quick environment and launcher configuration check:

```bash
go run ./cmd/session-manager --doctor
```

Machine-readable report:

```bash
go run ./cmd/session-manager --doctor --json
```

The report includes:

- active launcher configuration
- config value sources (`env` / `file` / `default`)
- config file path
- WSL detection
- terminal binary discovery
- Windows interop status
- command availability for configured restore tools
- structured fix suggestions

Print current effective config (after default + file + env merge):

```bash
go run ./cmd/session-manager --print-effective-config
```

Generate starter config:

```bash
# dry-run (prints target path + JSON)
go run ./cmd/session-manager --init-config

# write file
go run ./cmd/session-manager --init-config --write

# overwrite existing file
go run ./cmd/session-manager --init-config --write --force
```

## AI Auto-Config Protocol

Use this section if an AI agent should configure the tool for a user.

Scope constraints:

- Allowed: user shell/profile env vars and user config file.
- Not allowed: patching source code for per-user setup.

Steps:

1. Detect platform and shell (`uname -a`, `$SHELL`, `echo $WSL_DISTRO_NAME`).
2. Run doctor:
   ```bash
   go run ./cmd/session-manager --doctor
   go run ./cmd/session-manager --doctor --json
   ```
3. Decide minimal changes:
   - Read JSON `checks` and `suggestions`.
   - If terminal missing, set `ASM_TERMINAL_CMD`.
   - If tool command missing, update `ASM_RESTORE_CMD_*` or user PATH.
   - If shell mismatch, set `ASM_WSL_SHELL` or `ASM_HOST_SHELL_CMD/ARGS`.
4. Initialize config template:
   ```bash
   go run ./cmd/session-manager --init-config
   ```
5. Write config to either:
   - shell profile (`~/.zshrc`, `~/.bashrc`), or
   - `~/.config/agent-session-manager/config.json`
6. Re-run doctor and verify all required commands are `ok`.

Success criteria:

- doctor reports terminal command and restore tool command as available.
- resuming a sample session works from the right panel.

## Troubleshooting

- `wt not found`: install Windows Terminal or set `ASM_TERMINAL_CMD`.
- `windows interop unavailable`: WSL cannot spawn Windows processes in current environment.
- `qwen: not found` (or similar): ensure command is available in configured shell, or override restore command via `ASM_RESTORE_CMD_*`.

## Tests

```bash
go test ./...
```
