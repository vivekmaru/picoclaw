# Advanced Use Cases

This guide focuses on higher-leverage deployment and orchestration patterns.

## 1. Private AI Teammate Cockpit

PicoClaw now supports a stronger operator pattern than "one agent with many
tools". A practical deployment shape is:

- one private gateway
- one or more agents
- teammate profiles for `coder`, `reviewer`, `operator`, or `researcher`
- launcher review surfaces for tasks, approvals, memory proposals, approved
  memory catalog browsing, and task handoffs

This is a good fit when you want PicoClaw to coordinate Claude Code, Codex, or
other local coding tools without turning them into an unconstrained autonomous
runtime.

## 2. Multi-Agent Routing

Use `agents.list` plus `bindings` when different traffic should land on
different specialists.

Good fits:

- one support agent per channel
- one coding agent for developer traffic
- one private agent for direct messages

## 3. Scheduled Work With Cron And Heartbeat

PicoClaw can run:

- scheduled agent turns
- scheduled local command jobs

Useful patterns:

- daily summaries
- periodic repo checks
- device health probes
- content pipelines on low-power hardware

Use command jobs only when you trust the workspace and the invoked tools.

## 4. MCP As An Integration Plane

Model Context Protocol support is a clean way to connect:

- internal documentation
- databases
- ticketing systems
- custom business services

MCP is usually preferable to ad-hoc shell wrappers when a structured server
already exists.

## 5. Hybrid Model Topologies

PicoClaw works well as a router between:

- fast local models for low-cost tasks
- hosted models for high-value reasoning
- CLI-backed coding models for trusted development workflows

## 6. Edge Device Gateway

A typical constrained-device pattern is:

- PicoClaw gateway on ARM or RISC-V hardware
- remote hosted provider for reasoning
- local channels or sensors at the edge
- minimal persistent workspace on-device

The current product direction adds one important nuance:

- the core runtime still works well on constrained hardware
- the full launcher and teammate cockpit experience is better on a modest SBC
  or VM

## 7. Hooks For Approval Or Auditing

Hooks are useful to:

- log high-risk tool calls
- require approval before certain commands
- enrich prompts with organization-specific policy

## 8. Web Launcher As Review Surface

The launcher is now useful as a teammate review surface even when it is not
your primary chat surface.

Examples:

- use Telegram or Discord for daily interaction
- keep the launcher private for model, channel, and admin changes
- use the launcher runtime page for task approvals, memory review, approved
  memory browsing/export, and follow-up task handoffs
- use the launcher logs and health views for lightweight operations

## 9. Codex And Claude Code Integration

PicoClaw can delegate model execution to external coding CLIs through
`codex-cli/*` and `claude-cli/*` model entries.

See [Codex And Claude Code](integrations/codex-claude-code.md) for setup and
security notes.

## 10. Home-Lab Operator Assistant

PicoClaw is a good fit for a private home-lab assistant when you want:

- one place to keep operational memory
- help inspecting logs, configs, or dashboards
- approval before risky actions
- a clean way to hand completed work from one teammate to another

Recommended operating shape:

- deploy on a private VM, mini PC, or capable SBC
- keep launcher access private
- treat server-changing actions as approval-gated
- use CLI-backed coding models only in trusted workspaces
