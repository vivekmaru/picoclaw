import { launcherFetch } from "@/api/http"

export interface AgentRuntimeTask {
  owner_agent_id: string
  id: string
  kind?: string
  label?: string
  agent_id?: string
  teammate_id?: string
  requester_agent_id?: string
  requester_teammate_id?: string
  origin_channel?: string
  origin_chat_id?: string
  status: string
  result?: string
  memory_scope?: string
  workspace_scope?: string[]
  created: number
  started?: number
  completed?: number
  cancelable?: boolean
}

export interface AgentRuntimeSnapshot {
  generated_at: number
  summary: {
    agent_count: number
    teammate_count: number
    task_count: number
    task_statuses?: Record<string, number>
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
