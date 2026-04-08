# Architecture

This document describes the current PicoClaw runtime at a system level.

## High-Level Components

### Core CLI And Gateway

The main binary is `picoclaw`.

Key entrypoints:

- `picoclaw agent`
- `picoclaw gateway`
- `picoclaw auth`
- `picoclaw skills`

Primary code paths:

- `cmd/picoclaw/main.go`
- `pkg/gateway/gateway.go`
- `pkg/agent/*`

### Web Launcher

The launcher under `web/` provides:

- the dashboard frontend
- launcher authentication
- gateway lifecycle management
- Pico WebSocket proxying
- config, model, channel, tool, log, and startup APIs

Primary code paths:

- `web/backend/main.go`
- `web/backend/api/*`
- `web/frontend/src/*`

### Workspace And Persistent State

Most mutable state lives under the configured workspace:

- `sessions/`
- `memory/`
- `state/`
- `cron/`
- `skills/`
- prompt files such as `AGENT.md`, `SOUL.md`, `USER.md`, `HEARTBEAT.md`

Secrets and credentials are split across:

- `config.json`
- `.security.yml`
- `auth.json` + `auth.key`

Agent coordination is split across two layers:

- `agents`
  runtime execution units with workspaces, tools, models, and routing
- `teammates`
  human-facing profiles that map onto agents and carry role, memory-scope, and
  approval metadata for delegation

Memory is now layered the same way:

- shared memory stays in `workspace/memory/MEMORY.md`
- teammate-local memory lives under `workspace/memory/teammates/<id>/`
- custom scopes live under `workspace/memory/scopes/...`

At prompt-build time, PicoClaw merges shared memory with the active teammate's
local memory namespace so delegated work can keep separate working context
without losing the shared project memory.

Pending and reviewed memory writes also persist under `workspace/state/memory/`.
Memory proposals can now carry richer review metadata such as:

- domain (`project`, `server`, `shared_team`, `teammate_local`, ...)
- entry type (`fact`, `decision`, `runbook`, `incident`, ...)
- optional confidence level

That metadata is visible in the launcher review queue and is written into the
approved memory entry so long-term memory remains inspectable outside the UI.

Approved long-term memory is now also readable back through a runtime memory
catalog. That catalog:

- enumerates shared and scoped memory namespaces across active workspaces
- parses reviewed memory entries back into structured records
- preserves legacy free-form `MEMORY.md` content as legacy catalog entries
- can be exported from the launcher as Markdown or JSON for backup and audit
- tracks reversible lifecycle state such as pinning and archiving under
  `workspace/state/memory/catalog_state.json`

PicoClaw now also has a structured memory backup artifact for restore-oriented
operations. Unlike the catalog export, the backup captures:

- long-term memory content per scope
- daily notes per scope
- pending memory proposals
- lifecycle state for pinned and archived catalog entries

The restore flow is explicit:

- `validate` checks schema, scope paths, and workspace membership without
  writing
- `replace` rewrites the included workspace memory state from the backup

Delegation state now also persists under `workspace/state/subagents/<agent-id>/`.
Tracked tasks can form explicit parent/child handoff chains, so the launcher
can show:

- the original task
- follow-up or review tasks spawned from it
- who initiated the handoff
- the handoff note and kind (`follow_up` or `review`)

### Channels

Channels are ingress and egress adapters for chat systems, sockets, and device
surfaces.

Examples:

- Telegram, Discord, Slack, WeCom, IRC, Matrix
- Pico native WebSocket channel
- device-oriented channels such as MaixCAM

Primary code paths:

- `pkg/channels/*`
- `pkg/channels/manager.go`

### Providers

Providers abstract model execution.

Supported families include:

- hosted HTTP APIs such as OpenAI, Anthropic, Gemini-compatible, Groq,
  Bedrock, Azure, and others
- local OpenAI-compatible endpoints such as LM Studio, Ollama, and vLLM
- CLI-backed providers such as `codex-cli/*` and `claude-cli/*`

Primary code paths:

- `pkg/providers/*`
- `pkg/providers/factory_provider.go`

### Tools, Skills, Hooks, And MCP

The runtime can load capabilities from:

- built-in tools in `pkg/tools/*`
- Markdown skills in workspace/global/builtin skill roots
- process hooks in `pkg/agent/hook_process.go`
- MCP servers configured under `tools.mcp`

## Request Flow

### CLI Or Channel Request

1. A message enters through `agent`, a channel adapter, or the launcher's Pico
   WebSocket flow.
2. The gateway normalizes the request.
3. Bindings and routing choose the logical agent.
4. The selected provider receives the prompt and tool definitions.
5. Tool calls execute locally when requested.
6. The response returns through the originating channel.
7. Session and memory state persist to the workspace.

### Web Launcher Request

1. The browser authenticates to the launcher backend.
2. The frontend calls backend APIs for config, models, credentials, tools,
   logs, and gateway lifecycle.
3. Chat traffic uses `/pico/ws`, which the launcher proxies to the gateway's
   Pico channel.
4. The teammate runtime page polls live task, approval, memory-review, and
   handoff-chain state from the gateway runtime APIs.
5. The same launcher surface now also reads the approved memory catalog and can
   export it for offline inspection or backup.
6. Runtime memory can also be exported as a structured JSON backup and later
   restored through validate-or-replace flows.

## Trust Boundaries

The main boundaries in the current architecture are:

- launcher auth boundary
- gateway and channel boundary
- tool execution boundary
- provider boundary
- workspace filesystem boundary

PicoClaw includes policy and guardrails, but it is not a hardened multi-tenant
sandbox. Treat it as trusted-user software, especially when enabling exec
tools, MCP servers, or CLI-backed providers.

## Security-Relevant Notes

- Config-managed secrets can move out of `config.json` into `.security.yml`.
- `enc://` currently covers config-managed secrets, not every credential store.
- Launcher dashboard auth is token-based and best suited to local/private
  administration.
- CLI-backed providers inherit the trust model of the local tools they invoke.

For the issue list and remediation plan, see [Security Review](security-review.md).
