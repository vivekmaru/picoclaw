# Documentation Index

This index collects the current entry points for architecture, setup,
operations, security, and integrations.

## Start Here

- [Architecture](architecture.md): runtime components, request flow, and trust
  boundaries.
- [Usage Guide](usage-guide.md): recommended setup and day-to-day workflows.
- [Advanced Use Cases](advanced-use-cases.md): multi-agent routing, cron,
  hooks, MCP, and hybrid deployments.
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
  Claude Code as PicoClaw model backends.
- [Hooks](hooks/README.md): event-driven extension points.
- [MCP And Tool Settings](tools_configuration.md): external tool servers and
  tool policy.

## Agent Runtime

- [Spawn Tasks](spawn-tasks.md): asynchronous work and sub-agent execution.
- [Cron](cron.md): scheduled tasks and command jobs.
- [Steering](steering.md): inject messages into a running agent loop.
- [SubTurn](subturn.md): concurrency and nested agent execution.
