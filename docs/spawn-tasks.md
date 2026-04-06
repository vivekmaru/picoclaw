# Spawn Tasks And Handoffs

PicoClaw supports tracked asynchronous delegation through the `spawn` tool.

This is no longer just a fire-and-forget background task mechanism. In the
current runtime, spawned work can carry:

- requester identity
- teammate identity
- approval policy
- memory scope
- persisted task state
- parent/child handoff lineage

## What `spawn` Does

`spawn` creates a tracked subagent task that runs independently of the current
turn. The task is persisted under the workspace so it can survive launcher
refreshes and runtime restarts.

Current task states include:

- `queued`
- `awaiting_approval`
- `running`
- `canceling`
- `completed`
- `failed`
- `canceled`
- `denied`

## Teammate-Aware Delegation

You can target either:

- `agent_id`
- `teammate_id`

Using `teammate_id` is usually preferable because it lets PicoClaw attach:

- role metadata
- memory scope
- approval policy
- workspace scope

That makes the task part of the teammate system rather than just a raw
subprocess-like background job.

## Runtime Visibility

Tracked spawn tasks are visible from the launcher runtime page.

The launcher can now:

- inspect task detail
- cancel queued or running work
- approve or reject approval-gated work
- promote task output into memory review
- create follow-up or review handoff tasks from completed work

## Approval Flow

If the teammate profile requires approval, a spawned task enters
`awaiting_approval` instead of running immediately.

That creates a cleaner trust model:

1. teammate proposes work
2. launcher reviews it
3. human approves or rejects it
4. task proceeds or stops

## Memory Review

Completed task output can be promoted into a memory proposal instead of being
written silently into long-term memory.

That lets you:

- inspect proposed content
- edit scope, title, and content
- approve or reject the memory write
- keep shared and teammate-local memory cleaner

## Task Handoff Chains

Completed tasks can now hand work off to another teammate.

Examples:

- `coder -> reviewer`
- `researcher -> coder`
- `operator -> memory_keeper`

Each handoff creates a new tracked task with explicit lineage:

- parent task
- root task
- handoff kind
- handoff actor
- handoff note

This is the start of a real teammate workflow model rather than a single flat
task queue.

## Operational Notes

- task state lives under `workspace/state/subagents/<agent-id>/tasks.json`
- interrupted in-flight tasks are recovered as failed on restart rather than
  silently resumed
- the launcher is currently the best surface for reviewing and controlling
  tracked tasks

## When To Use `spawn`

Use `spawn` when work is:

- long-running
- independent enough to continue outside the current turn
- a good candidate for teammate review or handoff
- worth tracking in the runtime UI

For synchronous delegation inside one turn, `subagent` is still the simpler
tool.
