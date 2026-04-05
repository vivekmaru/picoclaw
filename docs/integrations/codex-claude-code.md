# Codex And Claude Code

PicoClaw can use external coding CLIs as model backends through:

- `codex-cli/<model>`
- `claude-cli/<model>`

This lets you keep PicoClaw's channels, launcher, routing, cron, and workspace
behavior while delegating model execution to a local Codex CLI or Claude Code
installation.

## Good Fits

- reuse existing local CLI auth and workflows
- bring coding-capable models behind PicoClaw channels
- mix CLI-backed coding models with hosted API-backed models in one
  `model_list`

## Prerequisites

### Codex CLI

- `codex` must be installed and available in `PATH`
- the CLI must already be authenticated

### Claude Code

- `claude` must be installed and available in `PATH`
- the CLI must already be authenticated

## Example Configuration

```json
{
  "agents": {
    "defaults": {
      "workspace": "~/.picoclaw/workspace",
      "model_name": "codex-main"
    }
  },
  "model_list": [
    {
      "model_name": "codex-main",
      "model": "codex-cli/gpt-5.3-codex",
      "workspace": "/Users/alice/dev/project-a",
      "execution_mode": "safe"
    },
    {
      "model_name": "claude-code",
      "model": "claude-cli/claude-sonnet-4.6",
      "workspace": "/Users/alice/dev/project-a",
      "execution_mode": "safe"
    },
    {
      "model_name": "fallback-http",
      "model": "openai/gpt-5.4",
      "api_key": "enc://..."
    }
  ]
}
```

Notes:

- `workspace` controls the subprocess working directory.
- `execution_mode` controls whether PicoClaw adds permissive CLI flags.
- you can keep a hosted fallback model in the same config.

## Operational Behavior

### Codex CLI Provider

PicoClaw runs `codex exec --json` and passes the prompt on stdin.

### Claude CLI Provider

PicoClaw runs `claude -p --output-format json` and passes the prompt on stdin.

## Security Notes

This integration changes the trust model:

1. PicoClaw is delegating execution to another local tool, not a remote API.
2. The default `execution_mode` is `safe`.
3. `execution_mode: permissive` re-enables the dangerous CLI flags for trusted
   local repos when you explicitly want that behavior.
4. The subprocess inherits the trust level of the local workspace and user
   account.

Current implementation details:

- Codex CLI adds `--dangerously-bypass-approvals-and-sandbox` only in
  `execution_mode: permissive`.
- Claude Code adds `--dangerously-skip-permissions` only in
  `execution_mode: permissive`.

Treat permissive mode as trusted-workspace-only behavior. Do not point these
providers at untrusted repositories or multi-user shared workspaces without an additional
containment layer such as a container or VM.

## Recommended Guardrails

- dedicate a workspace per coding agent or repo
- keep `restrict_to_workspace` enabled for PicoClaw itself
- prefer `execution_mode: safe` unless you have a specific reason not to
- run CLI-backed providers only on machines where you already trust the CLI
  session and local filesystem
- prefer hosted HTTP providers when you need a narrower execution surface

## When To Choose Which

Choose `codex-cli/*` when you already standardize on Codex CLI.

Choose `claude-cli/*` when you already use Claude Code locally.

For the broader repository security picture, see [Security Review](../security-review.md).
