# Usage Guide

This guide covers the main ways to run PicoClaw and the safest order to
configure it.

## Choose A Runtime Mode

### Launcher Mode

Use `picoclaw-launcher` when you want:

- a browser UI
- model and credential management
- chat without manual channel setup

### CLI Agent Mode

Use `picoclaw agent` when you want:

- terminal-first interaction
- quick local testing
- scripted or SSH-driven workflows

### Gateway Mode

Use `picoclaw gateway` when you want:

- long-running channel integrations
- webhooks, sockets, and scheduled jobs
- deployment on a server, VM, or edge device

## Recommended First-Time Setup

1. Copy [config.example.json](../config/config.example.json) or let
   `picoclaw onboard` create the baseline files.
2. Keep secrets out of `config.json` where possible.
   Use [Security Configuration](security_configuration.md) for API keys,
   channel tokens, and similar config-managed secrets.
3. Set up one working model in `model_list`.
4. Set `agents.defaults.model_name` to that model alias.
5. Start the launcher or gateway.
6. Confirm a basic round-trip chat before enabling more tools or channels.

## Recommended Secure Baseline

- keep `restrict_to_workspace` enabled unless you have a clear reason not to
- prefer `.security.yml` or `enc://` over plaintext secrets in `config.json`
- treat `~/.picoclaw/auth.json` and `~/.picoclaw/auth.key` as sensitive local state
- avoid `-public` launcher mode unless you also set a strong
  `PICOCLAW_LAUNCHER_TOKEN`
- prefer `execution_mode: safe` for CLI-backed coding models
- review the trust policy and exec allowlists before enabling automation that can run commands

## Common Workflows

### Local Personal Assistant

- run `picoclaw-launcher`
- add one model
- use the built-in Pico chat page
- define a small teammate set with clear roles before enabling more tools
- use the `Teammates` page to review delegated tasks, approve risky work, and
  hand off completed tasks to another teammate for follow-up or review
- use the same launcher page to browse approved memory across shared and
  teammate scopes, then pin important entries, archive stale ones, or export
  the catalog when you want a backup or an audit snapshot
- use the runtime memory backup endpoints when you want a restorable snapshot
  of workspace memory state, not just a human-readable catalog export
- add skills or MCP servers after the base flow works

See [Operator And Teammate Workflows](operator-teammate-workflows.md) for the
recommended day-to-day review and handoff pattern.

### Remote Or Headless Gateway

- run `picoclaw gateway` under a service manager
- configure one or more chat channels
- keep the launcher private or use it only as an admin surface
- use health endpoints and logs for operations

### Multi-Channel Bot

- configure channel credentials in `.security.yml`
- enable each channel in `config.json`
- use agent bindings when different traffic should route to different agents

### Scheduled Automation

- enable cron
- prefer agent-turn jobs when possible
- use command jobs only where local execution is required and trusted

## Operations Checklist

Before calling a deployment ready, verify:

- one working default model
- one tested ingress path
- workspace permissions and storage location
- backup strategy for config, `.security.yml`, and workspace state
- validate memory backups before restore, then use `replace` restores only when
  you intend to overwrite the included workspace memory state
- explicit review of exec, MCP, and CLI-provider trust boundaries
- launcher token strategy if the web UI is enabled

## Where To Go Next

- [Architecture](architecture.md)
- [Operator And Teammate Workflows](operator-teammate-workflows.md)
- [Advanced Use Cases](advanced-use-cases.md)
- [Providers & Models](providers.md)
- [Tools Configuration](tools_configuration.md)
- [Security Review](security-review.md)
