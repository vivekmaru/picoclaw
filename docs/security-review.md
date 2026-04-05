# Security Review

This is a maintainer-focused review of the current repository state as of
2026-04-03. It is not a formal third-party audit.

## Summary

PicoClaw already contains useful controls:

- workspace restriction for agent file access
- secret redaction for tool output
- `.security.yml` support for config-managed secrets
- `enc://` encryption for config-managed API keys
- launcher dashboard auth with rate-limited login

The main remaining risks are trust-boundary issues:

- local auth-store encryption now depends on a local machine key file rather
  than the stronger `enc://` passphrase+SSH-key path
- CLI-backed coding integrations are safer by default, but permissive mode is
  still available and should be treated as high trust
- the launcher still relies on a bearer token for manual login, even though the
  auto-open bootstrap path is now one-time and loopback-only

## Findings

### 1. `picoclaw auth` credentials now migrate to encrypted local storage

Severity: Medium

Code:

- `pkg/auth/store.go`

Status:

- fixed in this change set

Changes applied:

1. `auth.json` now stores an encrypted envelope instead of plaintext
   credentials
2. legacy plaintext stores are migrated automatically on load
3. the encryption key is stored separately in `~/.picoclaw/auth.key` with
   `0600` permissions and atomic writes

Residual risk:

- this is materially better than plaintext at-rest storage, but it is still a
  local machine secret rather than the stronger `enc://` passphrase-backed flow

### 2. CLI-backed providers run external coding tools in permissive modes

Severity: Medium

Code:

- `pkg/providers/codex_cli_provider.go`
- `pkg/providers/claude_cli_provider.go`

Status:

- partially fixed in this change set

Changes applied:

1. CLI-backed providers now support explicit `execution_mode`
2. the default is `safe`
3. dangerous CLI flags are only added when `execution_mode` is set to
   `permissive`

Residual risk:

- permissive mode still exists and should only be used for trusted local repos
- the launcher does not yet surface an explicit warning banner for permissive
  model entries

### 3. Launcher bootstrap is now one-time and loopback-only

Severity: Medium

Code:

- `web/backend/main.go`
- `web/backend/middleware/launcher_dashboard_auth.go`

Status:

- fixed in this change set

Changes applied:

1. local browser auto-open now uses a one-time bootstrap code in the URL
   fragment, not `?token=...`
2. bootstrap redemption happens through `POST /api/auth/bootstrap`
3. bootstrap redemption is limited to loopback clients and consumed once
4. public launcher mode disables the convenience bootstrap flow
5. generated dashboard tokens are no longer written to general launcher logs

### 4. Several HTTP servers were missing header and idle timeout hardening

Severity: Medium

Code:

- `web/backend/main.go`
- `pkg/channels/manager.go`
- `pkg/health/server.go`
- `pkg/auth/oauth.go`

Status:

- fixed in this change set

Changes applied:

- added `ReadHeaderTimeout` and `IdleTimeout` to the launcher backend server
- added `ReadHeaderTimeout` and `IdleTimeout` to the shared channel HTTP server
- added `ReadHeaderTimeout` and `IdleTimeout` to the health server
- added timeout hardening to the loopback OAuth callback server

## Practical Guidance

Today, the safest posture is:

- keep the launcher private unless there is a strong reason not to
- use `.security.yml` and `enc://` for config-managed secrets
- treat `auth.json` and `auth.key` as sensitive local state
- use CLI-backed providers only for trusted repos and workspaces, and prefer
  `execution_mode: safe`
- prefer containers or VMs when untrusted code or tools are in scope
