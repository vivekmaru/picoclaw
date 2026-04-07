# Operator And Teammate Workflows

This guide is the practical "how to use PicoClaw well right now" companion to
the broader [Usage Guide](usage-guide.md).

The current product shape works best when you treat PicoClaw as a private
operator cockpit for AI teammates, not as an unconstrained autonomous agent.

## What This Workflow Is For

Use this workflow when you want PicoClaw to help with:

- coding and repo work across teammates such as `coder` and `reviewer`
- home-lab or server assistance through an `operator` teammate
- memory capture and curation through shared and teammate-local scopes
- human review before risky actions or before messy task output becomes memory

## Recommended Team Shape

For a strong personal or home-lab setup, start with a small teammate set:

- `coder`: writes or changes code in trusted workspaces
- `reviewer`: checks diffs, design, and regressions before you act on them
- `operator`: inspects servers, configs, dashboards, and proposes next steps
- `memory_keeper`: turns completed work into cleaner durable memory

You do not need all of these on day one. Two or three is enough if the scopes
are clear.

## Recommended Trust Model

Use these defaults unless you have a specific reason not to:

- keep the launcher private to localhost or your private network
- run PicoClaw on a VM, mini PC, or capable SBC
- keep `restrict_to_workspace` enabled
- prefer `execution_mode: safe` for CLI-backed coding tools
- use approval-gated teammates for write, exec, or operator-style work
- review memory proposals before approving them into long-term memory

## Day-To-Day Workflow

### 1. Start With A Clear Teammate

Send work to the teammate that matches the job:

- ask `coder` to implement or modify
- ask `reviewer` to inspect or challenge
- ask `operator` to inspect infrastructure or propose action
- ask `memory_keeper` to clean up what should be remembered

This keeps memory, tools, and approvals aligned with the work instead of
mixing everything into one generic assistant.

### 2. Delegate With `spawn` When Work Should Outlive One Turn

Use `spawn` when the work is:

- long-running
- worth tracking in the launcher
- likely to need approval
- likely to be handed to another teammate afterward

Synchronous delegation still belongs with `subagent`. Use `spawn` when you
want a tracked task with visible state and history.

### 3. Review The Runtime Surface

The launcher `Teammates` page is the current review surface for:

- queued and running tasks
- approval-gated tasks
- follow-up and review handoffs
- pending memory proposals
- approved-memory catalog entries

This is where you keep the system supervised. Let teammates propose; use the
launcher to decide what becomes action or memory.

### 4. Use Handoffs Instead Of Re-Explaining Context

When one teammate finishes, hand the result to the next role instead of
starting a fresh request from scratch.

Useful patterns:

- `coder -> reviewer` for code review
- `researcher -> coder` for implementation based on gathered context
- `operator -> memory_keeper` for runbook or incident notes
- `coder -> memory_keeper` for durable repo conventions or decisions

This preserves lineage and makes the task chain inspectable later.

### 5. Curate Memory Instead Of Letting It Accumulate Blindly

Task output is often too raw to store directly. Use memory review to:

- trim noisy output
- rewrite content into a cleaner note
- change the target scope
- set a better title
- approve only the useful part

The current memory model works best when shared memory stays concise and
teammate-local memory stays role-specific.

## Practical Coding Workflow

For Codex, Claude Code, or similar coding tools:

1. let the `coder` teammate propose or implement work
2. hand completed work to `reviewer`
3. approve only the changes you actually want
4. promote the durable lessons to memory through `memory_keeper`

This gives you a buddy-system pattern instead of one coding model silently
doing everything.

## Practical Operator Workflow

For home-lab or server work:

1. let `operator` inspect logs, configs, status pages, or dashboards
2. require approval before write or exec-style actions
3. hand completed work to `memory_keeper` for runbooks, facts, or decisions
4. pin critical approved memory and archive stale entries in the catalog

This is the safest current way to use PicoClaw for infrastructure help without
turning it into an over-trusted automation layer.

## What To Avoid Right Now

- giving every teammate the same broad tool access
- treating approval-gated tasks as a nuisance and bypassing them
- letting raw task output become long-term memory without review
- exposing the launcher publicly
- running permissive CLI-backed coding tools against sensitive repos without
  isolation

## Related Guides

- [Usage Guide](usage-guide.md)
- [Spawn Tasks](spawn-tasks.md)
- [Advanced Use Cases](advanced-use-cases.md)
- [Configuration](configuration.md)
- [Codex And Claude Code](integrations/codex-claude-code.md)
