import { launcherFetch } from "@/api/http"

export interface AgentRuntimeTask {
  owner_agent_id: string
  id: string
  kind?: string
  task: string
  label?: string
  agent_id?: string
  teammate_id?: string
  requester_agent_id?: string
  requester_teammate_id?: string
  origin_channel?: string
  origin_chat_id?: string
  parent_task_id?: string
  parent_owner_agent_id?: string
  root_task_id?: string
  root_owner_agent_id?: string
  handoff_kind?: string
  handoff_actor?: string
  handoff_note?: string
  approval_policy?: string
  approved_by?: string
  approved_at?: number
  rejected_by?: string
  rejected_at?: number
  review_note?: string
  status: string
  result?: string
  memory_scope?: string
  workspace_scope?: string[]
  created: number
  started?: number
  completed?: number
  cancelable?: boolean
  approvable?: boolean
  rejectable?: boolean
  handoffable?: boolean
}

export interface AgentRuntimeMemoryProposal {
  owner_agent_id: string
  id: string
  scope: string
  domain?: string
  target: string
  kind: string
  entry_type?: string
  status: string
  title?: string
  content: string
  confidence?: string
  source_task_id?: string
  source_agent_id?: string
  source_teammate_id?: string
  requester_agent_id?: string
  requester_teammate_id?: string
  created: number
  updated_at?: number
  updated_by?: string
  reviewed_at?: number
  reviewed_by?: string
  review_note?: string
  approvable?: boolean
  rejectable?: boolean
}

export interface AgentRuntimeMemoryCatalogScope {
  owner_agent_id: string
  workspace: string
  scope: string
  display_name: string
  long_term_path: string
  entry_count: number
  has_long_term: boolean
}

export interface AgentRuntimeMemoryCatalogEntry {
  id: string
  owner_agent_id: string
  workspace: string
  scope: string
  scope_display_name: string
  source_path: string
  title: string
  content: string
  domain?: string
  entry_type?: string
  confidence?: string
  added_at?: number
  added_at_display?: string
  source_task_id?: string
  source_teammate_id?: string
  reviewed_by?: string
  pinned?: boolean
  pinned_at?: number
  pinned_by?: string
  archived?: boolean
  archived_at?: number
  archived_by?: string
  legacy?: boolean
}

export interface AgentRuntimeMemoryCatalog {
  generated_at: number
  summary: {
    scope_count: number
    entry_count: number
    workspace_count: number
    pinned_count: number
    archived_count: number
    domain_counts?: Record<string, number>
    entry_type_counts?: Record<string, number>
    workspace_entries?: Record<string, number>
  }
  scopes?: AgentRuntimeMemoryCatalogScope[]
  entries?: AgentRuntimeMemoryCatalogEntry[]
}

export interface AgentRuntimeMemoryHistoryEvent {
  id: string
  kind: string
  owner_agent_id: string
  workspace?: string
  scope?: string
  scope_display_name?: string
  subject_id: string
  subject_type: string
  title?: string
  content?: string
  domain?: string
  entry_type?: string
  status?: string
  actor?: string
  timestamp: number
}

export interface AgentRuntimeMemoryHistory {
  generated_at: number
  summary: {
    event_count: number
    catalog_event_count: number
    proposal_event_count: number
    kind_counts?: Record<string, number>
  }
  events?: AgentRuntimeMemoryHistoryEvent[]
}

export interface AgentRuntimeSnapshot {
  generated_at: number
  summary: {
    agent_count: number
    teammate_count: number
    task_count: number
    task_statuses?: Record<string, number>
    memory_proposal_count: number
    memory_proposal_statuses?: Record<string, number>
  }
  agents: Array<{
    id: string
    name?: string
    model?: string
    workspace?: string
  }>
  teammates: Array<{
    id: string
    name?: string
    role?: string
    agent_id?: string
    model?: string
    memory_scope?: string
    approval_policy?: string
    workspace_scope?: string[]
    toolset?: string[]
  }>
  tasks: AgentRuntimeTask[]
  memory_proposals?: AgentRuntimeMemoryProposal[]
}

export async function getAgentRuntime(): Promise<AgentRuntimeSnapshot> {
  const res = await launcherFetch("/api/agent/runtime")
  if (!res.ok) {
    const message = (await res.text()) || `API error: ${res.status}`
    throw new Error(message)
  }
  return res.json() as Promise<AgentRuntimeSnapshot>
}

type RuntimeMemoryCatalogQuery = {
  search?: string
  scope?: string
  domain?: string
  entryType?: string
  archive?: string
  ownerAgentID?: string
  limit?: number
}

type RuntimeMemoryHistoryQuery = {
  search?: string
  kind?: string
  scope?: string
  actor?: string
  ownerAgentID?: string
  limit?: number
}

function toRuntimeQueryString(query?: Record<string, string | number | undefined>): string {
  if (!query) {
    return ""
  }
  const params = new URLSearchParams()
  for (const [key, value] of Object.entries(query)) {
    if (value === undefined || value === null) {
      continue
    }
    const normalized = String(value).trim()
    if (!normalized) {
      continue
    }
    params.set(key, normalized)
  }
  const encoded = params.toString()
  return encoded ? `?${encoded}` : ""
}

export async function getAgentRuntimeMemoryCatalog(
  query?: RuntimeMemoryCatalogQuery,
): Promise<AgentRuntimeMemoryCatalog> {
  const res = await launcherFetch(
    `/api/agent/runtime/memory-catalog${toRuntimeQueryString({
      search: query?.search,
      scope: query?.scope,
      domain: query?.domain,
      entry_type: query?.entryType,
      archive: query?.archive,
      owner_agent_id: query?.ownerAgentID,
      limit: query?.limit,
    })}`,
  )
  if (!res.ok) {
    const message = (await res.text()) || `API error: ${res.status}`
    throw new Error(message)
  }
  return res.json() as Promise<AgentRuntimeMemoryCatalog>
}

export async function getAgentRuntimeMemoryHistory(
  query?: RuntimeMemoryHistoryQuery,
): Promise<AgentRuntimeMemoryHistory> {
  const res = await launcherFetch(
    `/api/agent/runtime/memory-history${toRuntimeQueryString({
      search: query?.search,
      kind: query?.kind,
      scope: query?.scope,
      actor: query?.actor,
      owner_agent_id: query?.ownerAgentID,
      limit: query?.limit,
    })}`,
  )
  if (!res.ok) {
    const message = (await res.text()) || `API error: ${res.status}`
    throw new Error(message)
  }
  return res.json() as Promise<AgentRuntimeMemoryHistory>
}

export async function downloadAgentRuntimeMemoryCatalog(format: "markdown" | "json"): Promise<{
  blob: Blob
  filename: string
}> {
  const res = await launcherFetch(
    `/api/agent/runtime/memory-catalog/export?format=${encodeURIComponent(format)}`,
  )
  if (!res.ok) {
    const message = (await res.text()) || `API error: ${res.status}`
    throw new Error(message)
  }
  const disposition = res.headers.get("Content-Disposition")
  const filename =
    disposition?.match(/filename="?([^"]+)"?/)?.[1] ??
    (format === "json" ? "memory-catalog.json" : "memory-catalog.md")
  return {
    blob: await res.blob(),
    filename,
  }
}

async function postMemoryCatalogEntryAction(
  entryID: string,
  action: "pin" | "unpin" | "archive" | "restore",
  actor: string,
): Promise<AgentRuntimeMemoryCatalogEntry> {
  const res = await launcherFetch(
    `/api/agent/runtime/memory-catalog/entries/${encodeURIComponent(entryID)}/${action}`,
    {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ actor }),
    },
  )
  if (!res.ok) {
    const message = (await res.text()) || `API error: ${res.status}`
    throw new Error(message)
  }
  return res.json() as Promise<AgentRuntimeMemoryCatalogEntry>
}

export function pinAgentRuntimeMemoryCatalogEntry(
  entryID: string,
  actor: string,
): Promise<AgentRuntimeMemoryCatalogEntry> {
  return postMemoryCatalogEntryAction(entryID, "pin", actor)
}

export function unpinAgentRuntimeMemoryCatalogEntry(
  entryID: string,
  actor: string,
): Promise<AgentRuntimeMemoryCatalogEntry> {
  return postMemoryCatalogEntryAction(entryID, "unpin", actor)
}

export function archiveAgentRuntimeMemoryCatalogEntry(
  entryID: string,
  actor: string,
): Promise<AgentRuntimeMemoryCatalogEntry> {
  return postMemoryCatalogEntryAction(entryID, "archive", actor)
}

export function restoreAgentRuntimeMemoryCatalogEntry(
  entryID: string,
  actor: string,
): Promise<AgentRuntimeMemoryCatalogEntry> {
  return postMemoryCatalogEntryAction(entryID, "restore", actor)
}

export async function getAgentRuntimeTask(
  ownerAgentID: string,
  taskID: string,
): Promise<AgentRuntimeTask> {
  const res = await launcherFetch(
    `/api/agent/runtime/tasks/${encodeURIComponent(ownerAgentID)}/${encodeURIComponent(taskID)}`,
  )
  if (!res.ok) {
    const message = (await res.text()) || `API error: ${res.status}`
    throw new Error(message)
  }
  return res.json() as Promise<AgentRuntimeTask>
}

export async function cancelAgentRuntimeTask(
  ownerAgentID: string,
  taskID: string,
): Promise<AgentRuntimeTask> {
  const res = await launcherFetch(
    `/api/agent/runtime/tasks/${encodeURIComponent(ownerAgentID)}/${encodeURIComponent(taskID)}/cancel`,
    {
      method: "POST",
    },
  )
  if (!res.ok) {
    const message = (await res.text()) || `API error: ${res.status}`
    throw new Error(message)
  }
  return res.json() as Promise<AgentRuntimeTask>
}

export async function approveAgentRuntimeTask(
  ownerAgentID: string,
  taskID: string,
  actor: string,
  note: string,
): Promise<AgentRuntimeTask> {
  const res = await launcherFetch(
    `/api/agent/runtime/tasks/${encodeURIComponent(ownerAgentID)}/${encodeURIComponent(taskID)}/approve`,
    {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ actor, note }),
    },
  )
  if (!res.ok) {
    const message = (await res.text()) || `API error: ${res.status}`
    throw new Error(message)
  }
  return res.json() as Promise<AgentRuntimeTask>
}

export async function rejectAgentRuntimeTask(
  ownerAgentID: string,
  taskID: string,
  actor: string,
  note: string,
): Promise<AgentRuntimeTask> {
  const res = await launcherFetch(
    `/api/agent/runtime/tasks/${encodeURIComponent(ownerAgentID)}/${encodeURIComponent(taskID)}/reject`,
    {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ actor, note }),
    },
  )
  if (!res.ok) {
    const message = (await res.text()) || `API error: ${res.status}`
    throw new Error(message)
  }
  return res.json() as Promise<AgentRuntimeTask>
}

export async function handoffAgentRuntimeTask(
  ownerAgentID: string,
  taskID: string,
  payload: {
    actor: string
    note: string
    agent_id?: string
    teammate_id?: string
    label: string
    task: string
    kind?: string
  },
): Promise<AgentRuntimeTask> {
  const res = await launcherFetch(
    `/api/agent/runtime/tasks/${encodeURIComponent(ownerAgentID)}/${encodeURIComponent(taskID)}/handoff`,
    {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(payload),
    },
  )
  if (!res.ok) {
    const message = (await res.text()) || `API error: ${res.status}`
    throw new Error(message)
  }
  return res.json() as Promise<AgentRuntimeTask>
}

export async function createAgentRuntimeMemoryProposal(
  ownerAgentID: string,
  taskID: string,
  scope: string,
): Promise<AgentRuntimeMemoryProposal> {
  const res = await launcherFetch(
    `/api/agent/runtime/tasks/${encodeURIComponent(ownerAgentID)}/${encodeURIComponent(taskID)}/memory-proposals`,
    {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ scope }),
    },
  )
  if (!res.ok) {
    const message = (await res.text()) || `API error: ${res.status}`
    throw new Error(message)
  }
  return res.json() as Promise<AgentRuntimeMemoryProposal>
}

export async function updateAgentRuntimeMemoryProposal(
  ownerAgentID: string,
  proposalID: string,
  payload: {
    actor: string
    scope: string
    domain: string
    entry_type: string
    title: string
    content: string
    confidence: string
  },
): Promise<AgentRuntimeMemoryProposal> {
  const res = await launcherFetch(
    `/api/agent/runtime/memory-proposals/${encodeURIComponent(ownerAgentID)}/${encodeURIComponent(proposalID)}/update`,
    {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(payload),
    },
  )
  if (!res.ok) {
    const message = (await res.text()) || `API error: ${res.status}`
    throw new Error(message)
  }
  return res.json() as Promise<AgentRuntimeMemoryProposal>
}

export async function approveAgentRuntimeMemoryProposal(
  ownerAgentID: string,
  proposalID: string,
  actor: string,
  note: string,
): Promise<AgentRuntimeMemoryProposal> {
  const res = await launcherFetch(
    `/api/agent/runtime/memory-proposals/${encodeURIComponent(ownerAgentID)}/${encodeURIComponent(proposalID)}/approve`,
    {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ actor, note }),
    },
  )
  if (!res.ok) {
    const message = (await res.text()) || `API error: ${res.status}`
    throw new Error(message)
  }
  return res.json() as Promise<AgentRuntimeMemoryProposal>
}

export async function rejectAgentRuntimeMemoryProposal(
  ownerAgentID: string,
  proposalID: string,
  actor: string,
  note: string,
): Promise<AgentRuntimeMemoryProposal> {
  const res = await launcherFetch(
    `/api/agent/runtime/memory-proposals/${encodeURIComponent(ownerAgentID)}/${encodeURIComponent(proposalID)}/reject`,
    {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ actor, note }),
    },
  )
  if (!res.ok) {
    const message = (await res.text()) || `API error: ${res.status}`
    throw new Error(message)
  }
  return res.json() as Promise<AgentRuntimeMemoryProposal>
}
