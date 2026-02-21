# agent-session-manager

Install globally:

```bash
npm i -g agent-session-manager
```

Run from any directory:

```bash
agent-manager
```

Notes:

- This package downloads a prebuilt Go binary from GitHub Releases during `postinstall`.
- Supported platforms (first release): `linux-x64`, `win32-x64`.
- For local testing (without published releases), you can override:
  - `ASM_RELEASE_REPO` (e.g. `owner/repo`)
  - `ASM_RELEASE_BASE_URL` (full release download base URL)
  - `ASM_SKIP_POSTINSTALL=1` (skip auto-download)
- For troubleshooting, run:

```bash
agent-manager --doctor
```
