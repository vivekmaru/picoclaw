# Advanced Use Cases

This guide focuses on higher-leverage deployment and orchestration patterns.

## 1. Multi-Agent Routing

Use `agents.list` plus `bindings` when different traffic should land on
different specialists.

Good fits:

- one support agent per channel
- one coding agent for developer traffic
- one private agent for direct messages

## 2. Scheduled Work With Cron And Heartbeat

PicoClaw can run:

- scheduled agent turns
- scheduled local command jobs

Useful patterns:

- daily summaries
- periodic repo checks
- device health probes
- content pipelines on low-power hardware

Use command jobs only when you trust the workspace and the invoked tools.

## 3. MCP As An Integration Plane

Model Context Protocol support is a clean way to connect:

- internal documentation
- databases
- ticketing systems
- custom business services

MCP is usually preferable to ad-hoc shell wrappers when a structured server
already exists.

## 4. Hybrid Model Topologies

PicoClaw works well as a router between:

- fast local models for low-cost tasks
- hosted models for high-value reasoning
- CLI-backed coding models for trusted development workflows

## 5. Edge Device Gateway

A typical constrained-device pattern is:

- PicoClaw gateway on ARM or RISC-V hardware
- remote hosted provider for reasoning
- local channels or sensors at the edge
- minimal persistent workspace on-device

## 6. Hooks For Approval Or Auditing

Hooks are useful to:

- log high-risk tool calls
- require approval before certain commands
- enrich prompts with organization-specific policy

## 7. Web Launcher As Control Plane

The launcher is useful even when it is not your primary chat surface.

Examples:

- use Telegram or Discord for daily interaction
- keep the launcher private for model, channel, and admin changes
- use the launcher logs and health views for lightweight operations

## 8. Codex And Claude Code Integration

PicoClaw can delegate model execution to external coding CLIs through
`codex-cli/*` and `claude-cli/*` model entries.

See [Codex And Claude Code](integrations/codex-claude-code.md) for setup and
security notes.
