# Documentation Index

This index collects the current entry points for architecture, setup,
operations, security, and integrations.

PicoClaw is now best understood as a low-footprint, self-hosted AI teammate
runtime with an optional private launcher that acts as a review and operator
surface. The core gateway/agent runtime still fits small SBC and VM
deployments; the full launcher and teammate cockpit experience is better suited
to a modest SBC, mini PC, or VM.

## Start Here

- [Architecture](architecture.md): runtime components, request flow, and trust
  boundaries.
- [Usage Guide](usage-guide.md): recommended setup and day-to-day workflows.
- [Operator And Teammate Workflows](operator-teammate-workflows.md): the
  practical supervised workflow for tasks, approvals, handoffs, and memory.
- [Advanced Use Cases](advanced-use-cases.md): multi-agent routing, cron,
  teammate workflows, MCP, and private operator patterns.
- [Configuration](configuration.md): config layout, workspace structure, and
  launcher behavior.
- [Providers & Models](providers.md): provider families, model syntax, and
  `model_list` configuration.

## Security And Operations

- [Security Review](security-review.md): current findings, fixes already
  applied, and the follow-up plan.
- [Security Configuration](security_configuration.md): keep config-managed
  secrets out of `config.json`.
- [Credential Encryption](credential_encryption.md): `enc://` support for
  config-managed API keys.
- [Sensitive Data Filtering](sensitive_data_filtering.md): redact secrets from
  tool output before it reaches the model.
- [Troubleshooting](troubleshooting.md): common operational issues and fixes.
- [Docker](docker.md): containerized launcher and gateway workflows.

## Integrations

- [Codex And Claude Code](integrations/codex-claude-code.md): use Codex CLI and
  Claude Code as PicoClaw teammate backends.
- [Hooks](hooks/README.md): event-driven extension points.
- [MCP And Tool Settings](tools_configuration.md): external tool servers and
  tool policy.

## Agent Runtime

- [Spawn Tasks](spawn-tasks.md): asynchronous delegation, approvals, and task
  handoff chains.
- [Operator And Teammate Workflows](operator-teammate-workflows.md): how to run
  the launcher as a supervised operator cockpit today.
- [Usage Guide](usage-guide.md): launcher memory review plus approved-memory
  catalog browsing/export workflows.
- [Cron](cron.md): scheduled tasks and command jobs.
- [Steering](steering.md): inject messages into a running agent loop.
- [SubTurn](subturn.md): concurrency and nested agent execution.
