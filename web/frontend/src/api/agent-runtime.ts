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
  target: string
  kind: string
  status: string
  title?: string
  content: string
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
  payload: { actor: string; scope: string; title: string; content: string },
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
