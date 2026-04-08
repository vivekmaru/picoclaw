import {
  useMutation,
  useQuery,
  useQueryClient,
} from "@tanstack/react-query"
import { useDeferredValue, useMemo, useState, type ReactNode } from "react"
import { useTranslation } from "react-i18next"
import { toast } from "sonner"

import {
  archiveAgentRuntimeMemoryCatalogEntry,
  approveAgentRuntimeMemoryProposal,
  approveAgentRuntimeTask,
  cancelAgentRuntimeTask,
  createAgentRuntimeMemoryProposal,
  downloadAgentRuntimeMemoryCatalog,
  getAgentRuntimeMemoryHistory,
  pinAgentRuntimeMemoryCatalogEntry,
  restoreAgentRuntimeMemoryCatalogEntry,
  unpinAgentRuntimeMemoryCatalogEntry,
  getAgentRuntime,
  getAgentRuntimeMemoryCatalog,
  getAgentRuntimeTask,
  handoffAgentRuntimeTask,
  rejectAgentRuntimeMemoryProposal,
  rejectAgentRuntimeTask,
  updateAgentRuntimeMemoryProposal,
  type AgentRuntimeMemoryCatalog,
  type AgentRuntimeMemoryCatalogEntry,
  type AgentRuntimeMemoryHistory,
  type AgentRuntimeMemoryHistoryEvent,
  type AgentRuntimeMemoryProposal,
  type AgentRuntimeSnapshot,
  type AgentRuntimeTask,
} from "@/api/agent-runtime"
import { PageHeader } from "@/components/page-header"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card"
import { Input } from "@/components/ui/input"
import { Skeleton } from "@/components/ui/skeleton"
import { Textarea } from "@/components/ui/textarea"
import { cn } from "@/lib/utils"

const RUNTIME_POLL_MS = 3000
const MEMORY_DOMAINS = [
  "personal",
  "project",
  "server",
  "shared_team",
  "teammate_local",
] as const
const MEMORY_ENTRY_TYPES = [
  "fact",
  "preference",
  "runbook",
  "decision",
  "incident",
  "todo",
] as const
const MEMORY_CONFIDENCE_LEVELS = ["low", "medium", "high"] as const
type RuntimeTeammate = AgentRuntimeSnapshot["teammates"][number]
type TaskHandoffForm = {
  actor: string
  note: string
  teammateID: string
  label: string
  task: string
  kind: string
}
type MemoryProposalEditor = {
  actor: string
  note: string
  scope: string
  domain: string
  entryType: string
  title: string
  content: string
  confidence: string
}

export function TeammatesPage() {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const [taskFilter, setTaskFilter] = useState("")
  const [taskStatusFilter, setTaskStatusFilter] = useState("all")
  const [memoryStatusFilter, setMemoryStatusFilter] = useState("all")
  const [memoryFilter, setMemoryFilter] = useState("")
  const [memoryDomainFilter, setMemoryDomainFilter] = useState("all")
  const [memoryTypeFilter, setMemoryTypeFilter] = useState("all")
  const [catalogFilter, setCatalogFilter] = useState("")
  const [catalogScopeFilter, setCatalogScopeFilter] = useState("all")
  const [catalogDomainFilter, setCatalogDomainFilter] = useState("all")
  const [catalogTypeFilter, setCatalogTypeFilter] = useState("all")
  const [catalogArchiveFilter, setCatalogArchiveFilter] = useState("active")
  const [historyFilter, setHistoryFilter] = useState("")
  const [historyKindFilter, setHistoryKindFilter] = useState("all")
  const [selectedTaskKey, setSelectedTaskKey] = useState("")
  const [selectedProposalKey, setSelectedProposalKey] = useState("")
  const [selectedCatalogEntryKey, setSelectedCatalogEntryKey] = useState("")
  const [taskReviewForms, setTaskReviewForms] = useState<
    Record<string, { actor: string; note: string }>
  >({})
  const [taskHandoffForms, setTaskHandoffForms] = useState<Record<string, TaskHandoffForm>>(
    {},
  )
  const [proposalEditors, setProposalEditors] = useState<Record<string, MemoryProposalEditor>>(
    {},
  )
  const deferredCatalogFilter = useDeferredValue(catalogFilter)
  const deferredHistoryFilter = useDeferredValue(historyFilter)

  const { data, isLoading, error } = useQuery({
    queryKey: ["agent-runtime"],
    queryFn: getAgentRuntime,
    refetchInterval: RUNTIME_POLL_MS,
  })

  const memoryCatalogBaseQuery = useQuery({
    queryKey: ["agent-runtime", "memory-catalog"],
    queryFn: () => getAgentRuntimeMemoryCatalog(),
  })

  const memoryCatalogQuery = useQuery({
    queryKey: [
      "agent-runtime",
      "memory-catalog",
      "filtered",
      deferredCatalogFilter,
      catalogScopeFilter,
      catalogDomainFilter,
      catalogTypeFilter,
      catalogArchiveFilter,
    ],
    queryFn: () =>
      getAgentRuntimeMemoryCatalog({
        search: deferredCatalogFilter,
        scope: catalogScopeFilter === "all" ? undefined : catalogScopeFilter,
        domain: catalogDomainFilter === "all" ? undefined : catalogDomainFilter,
        entryType: catalogTypeFilter === "all" ? undefined : catalogTypeFilter,
        archive: catalogArchiveFilter,
      }),
    refetchInterval: RUNTIME_POLL_MS,
  })

  const memoryHistoryQuery = useQuery({
    queryKey: ["agent-runtime", "memory-history", deferredHistoryFilter, historyKindFilter],
    queryFn: () =>
      getAgentRuntimeMemoryHistory({
        search: deferredHistoryFilter,
        kind: historyKindFilter === "all" ? undefined : historyKindFilter,
        limit: 20,
      }),
    refetchInterval: RUNTIME_POLL_MS,
  })

  const taskStatusEntries = useMemo(
    () =>
      Object.entries(data?.summary.task_statuses ?? {}).sort((a, b) =>
        a[0].localeCompare(b[0]),
      ),
    [data?.summary.task_statuses],
  )

  const memoryStatusEntries = useMemo(
    () =>
      Object.entries(data?.summary.memory_proposal_statuses ?? {}).sort((a, b) =>
        a[0].localeCompare(b[0]),
      ),
    [data?.summary.memory_proposal_statuses],
  )

  const memoryDomainEntries = useMemo(
    () =>
      Array.from(
        new Set(
          (data?.memory_proposals ?? [])
            .map((proposal) => proposal.domain)
            .filter((value): value is string => Boolean(value)),
        ),
      ).sort((a, b) => a.localeCompare(b)),
    [data?.memory_proposals],
  )

  const memoryTypeEntries = useMemo(
    () =>
      Array.from(
        new Set(
          (data?.memory_proposals ?? [])
            .map((proposal) => proposal.entry_type)
            .filter((value): value is string => Boolean(value)),
        ),
      ).sort((a, b) => a.localeCompare(b)),
    [data?.memory_proposals],
  )

  const catalogScopeEntries = useMemo(
    () =>
      (memoryCatalogBaseQuery.data?.scopes ?? []).slice().sort((a, b) =>
        [a.display_name, a.scope, a.owner_agent_id].join("\n").localeCompare(
          [b.display_name, b.scope, b.owner_agent_id].join("\n"),
        ),
      ),
    [memoryCatalogBaseQuery.data?.scopes],
  )

  const catalogDomainEntries = useMemo(
    () =>
      Array.from(
        new Set(
          (memoryCatalogBaseQuery.data?.entries ?? [])
            .map((entry) => entry.domain)
            .filter((value): value is string => Boolean(value)),
        ),
      ).sort((a, b) => a.localeCompare(b)),
    [memoryCatalogBaseQuery.data?.entries],
  )

  const catalogTypeEntries = useMemo(
    () =>
      Array.from(
        new Set(
          (memoryCatalogBaseQuery.data?.entries ?? [])
            .map((entry) => entry.entry_type)
            .filter((value): value is string => Boolean(value)),
        ),
      ).sort((a, b) => a.localeCompare(b)),
    [memoryCatalogBaseQuery.data?.entries],
  )

  const filteredTasks = useMemo(() => {
    const normalizedFilter = taskFilter.trim().toLowerCase()
    return (data?.tasks ?? []).filter((task) => {
      if (taskStatusFilter !== "all" && task.status !== taskStatusFilter) {
        return false
      }
      if (!normalizedFilter) {
        return true
      }
      return [
        task.id,
        task.label,
        task.task,
        task.teammate_id,
        task.requester_teammate_id,
        task.owner_agent_id,
      ]
        .join("\n")
        .toLowerCase()
        .includes(normalizedFilter)
    })
  }, [data?.tasks, taskFilter, taskStatusFilter])

  const filteredProposals = useMemo(() => {
    const normalizedFilter = memoryFilter.trim().toLowerCase()
    return (data?.memory_proposals ?? []).filter((proposal) => {
      if (memoryStatusFilter !== "all" && proposal.status !== memoryStatusFilter) {
        return false
      }
      if (memoryDomainFilter !== "all" && proposal.domain !== memoryDomainFilter) {
        return false
      }
      if (memoryTypeFilter !== "all" && proposal.entry_type !== memoryTypeFilter) {
        return false
      }
      if (!normalizedFilter) {
        return true
      }
      return [
        proposal.id,
        proposal.scope,
        proposal.domain,
        proposal.entry_type,
        proposal.title,
        proposal.content,
        proposal.source_teammate_id,
        proposal.owner_agent_id,
      ]
        .join("\n")
        .toLowerCase()
        .includes(normalizedFilter)
    })
  }, [data?.memory_proposals, memoryDomainFilter, memoryFilter, memoryStatusFilter, memoryTypeFilter])

  const filteredCatalogEntries = useMemo(
    () => memoryCatalogQuery.data?.entries ?? [],
    [memoryCatalogQuery.data?.entries],
  )

  const memoryHistoryKindEntries = useMemo(
    () =>
      Object.entries(memoryHistoryQuery.data?.summary.kind_counts ?? {}).sort((a, b) =>
        a[0].localeCompare(b[0]),
      ),
    [memoryHistoryQuery.data?.summary.kind_counts],
  )

  const effectiveSelectedTaskKey = useMemo(() => {
    if ((data?.tasks ?? []).some((task) => taskKey(task) === selectedTaskKey)) {
      return selectedTaskKey
    }
    if (filteredTasks.some((task) => taskKey(task) === selectedTaskKey)) {
      return selectedTaskKey
    }
    return filteredTasks.length > 0 ? taskKey(filteredTasks[0]) : ""
  }, [data?.tasks, filteredTasks, selectedTaskKey])

  const effectiveSelectedProposalKey = useMemo(() => {
    if (
      filteredProposals.some((proposal) => proposalKey(proposal) === selectedProposalKey)
    ) {
      return selectedProposalKey
    }
    return filteredProposals.length > 0 ? proposalKey(filteredProposals[0]) : ""
  }, [filteredProposals, selectedProposalKey])

  const selectedTask = useMemo(
    () => (data?.tasks ?? []).find((task) => taskKey(task) === effectiveSelectedTaskKey) ?? null,
    [data?.tasks, effectiveSelectedTaskKey],
  )
  const selectedProposal = useMemo(
    () =>
      filteredProposals.find((proposal) => proposalKey(proposal) === effectiveSelectedProposalKey) ??
      null,
    [effectiveSelectedProposalKey, filteredProposals],
  )

  const effectiveSelectedCatalogEntryKey = useMemo(() => {
    if (
      filteredCatalogEntries.some((entry) => catalogEntryKey(entry) === selectedCatalogEntryKey)
    ) {
      return selectedCatalogEntryKey
    }
    return filteredCatalogEntries.length > 0
      ? catalogEntryKey(filteredCatalogEntries[0])
      : ""
  }, [filteredCatalogEntries, selectedCatalogEntryKey])

  const selectedCatalogEntry = useMemo(
    () =>
      filteredCatalogEntries.find(
        (entry) => catalogEntryKey(entry) === effectiveSelectedCatalogEntryKey,
      ) ?? null,
    [effectiveSelectedCatalogEntryKey, filteredCatalogEntries],
  )

  const taskDetailQuery = useQuery({
    queryKey: [
      "agent-runtime",
      "task",
      selectedTask?.owner_agent_id,
      selectedTask?.id,
    ],
    queryFn: () =>
      getAgentRuntimeTask(selectedTask!.owner_agent_id, selectedTask!.id),
    enabled: selectedTask !== null,
    refetchInterval: selectedTask ? RUNTIME_POLL_MS : false,
    initialData: selectedTask ?? undefined,
  })

  const taskDetail = taskDetailQuery.data ?? selectedTask

  const selectedTaskReview = taskDetail
    ? taskReviewForms[taskKey(taskDetail)] ?? { actor: "launcher", note: "" }
    : { actor: "launcher", note: "" }

  const selectedTaskHandoff = taskDetail
    ? taskHandoffForms[taskKey(taskDetail)] ??
      createDefaultTaskHandoffForm(taskDetail, data?.teammates ?? [])
    : createEmptyTaskHandoffForm()

  const selectedProposalEditor = selectedProposal
    ? proposalEditors[proposalKey(selectedProposal)] ?? {
        actor: "launcher",
        note: "",
        scope: selectedProposal.scope,
        domain: selectedProposal.domain ?? defaultMemoryProposalDomain(selectedProposal.scope),
        entryType: selectedProposal.entry_type ?? "fact",
        title: selectedProposal.title ?? "",
        content: selectedProposal.content,
        confidence: selectedProposal.confidence ?? "",
      }
    : {
        actor: "launcher",
        note: "",
        scope: "shared",
        domain: "shared_team",
        entryType: "fact",
        title: "",
        content: "",
        confidence: "",
      }

  const invalidateRuntime = () => {
    void queryClient.invalidateQueries({ queryKey: ["agent-runtime"] })
    void queryClient.invalidateQueries({ queryKey: ["agent-runtime", "memory-catalog"] })
    void queryClient.invalidateQueries({ queryKey: ["agent-runtime", "memory-history"] })
    if (taskDetail) {
      void queryClient.invalidateQueries({
        queryKey: ["agent-runtime", "task", taskDetail.owner_agent_id, taskDetail.id],
      })
    }
  }

  const cancelTaskMutation = useMutation({
    mutationFn: ({ ownerAgentID, taskID }: { ownerAgentID: string; taskID: string }) =>
      cancelAgentRuntimeTask(ownerAgentID, taskID),
    onSuccess: (task) => {
      toast.success(t("pages.agent.teammates.task_cancel_success", { id: task.id }))
      invalidateRuntime()
    },
    onError: (mutationError: Error) => {
      toast.error(mutationError?.message || t("pages.agent.teammates.task_cancel_error"))
    },
  })

  const approveTaskMutation = useMutation({
    mutationFn: ({
      ownerAgentID,
      taskID,
      actor,
      note,
    }: {
      ownerAgentID: string
      taskID: string
      actor: string
      note: string
    }) => approveAgentRuntimeTask(ownerAgentID, taskID, actor, note),
    onSuccess: (task) => {
      toast.success(t("pages.agent.teammates.task_approve_success", { id: task.id }))
      setTaskReviewForms((current) => {
        const next = { ...current }
        delete next[taskKey(task)]
        return next
      })
      invalidateRuntime()
    },
    onError: (mutationError: Error) => {
      toast.error(mutationError?.message || t("pages.agent.teammates.task_approve_error"))
    },
  })

  const rejectTaskMutation = useMutation({
    mutationFn: ({
      ownerAgentID,
      taskID,
      actor,
      note,
    }: {
      ownerAgentID: string
      taskID: string
      actor: string
      note: string
    }) => rejectAgentRuntimeTask(ownerAgentID, taskID, actor, note),
    onSuccess: (task) => {
      toast.success(t("pages.agent.teammates.task_reject_success", { id: task.id }))
      setTaskReviewForms((current) => {
        const next = { ...current }
        delete next[taskKey(task)]
        return next
      })
      invalidateRuntime()
    },
    onError: (mutationError: Error) => {
      toast.error(mutationError?.message || t("pages.agent.teammates.task_reject_error"))
    },
  })

  const handoffTaskMutation = useMutation({
    mutationFn: ({
      ownerAgentID,
      taskID,
      actor,
      note,
      teammateID,
      label,
      task,
      kind,
    }: {
      ownerAgentID: string
      taskID: string
      actor: string
      note: string
      teammateID: string
      label: string
      task: string
      kind: string
    }) =>
      handoffAgentRuntimeTask(ownerAgentID, taskID, {
        actor,
        note,
        teammate_id: teammateID || undefined,
        label,
        task,
        kind,
      }),
    onSuccess: (task) => {
      toast.success(t("pages.agent.teammates.task_handoff_success", { id: task.id }))
      setTaskHandoffForms((current) => {
        const next = { ...current }
        if (task.parent_owner_agent_id && task.parent_task_id) {
          delete next[`${task.parent_owner_agent_id}:${task.parent_task_id}`]
        }
        return next
      })
      revealTask(taskKey(task))
      invalidateRuntime()
    },
    onError: (mutationError: Error) => {
      toast.error(mutationError?.message || t("pages.agent.teammates.task_handoff_error"))
    },
  })

  const createMemoryProposalMutation = useMutation({
    mutationFn: ({
      ownerAgentID,
      taskID,
      scope,
    }: {
      ownerAgentID: string
      taskID: string
      scope: string
    }) => createAgentRuntimeMemoryProposal(ownerAgentID, taskID, scope),
    onSuccess: (proposal) => {
      toast.success(
        t("pages.agent.teammates.memory_proposal_create_success", {
          id: proposal.id,
        }),
      )
      invalidateRuntime()
    },
    onError: (mutationError: Error) => {
      toast.error(
        mutationError?.message ||
          t("pages.agent.teammates.memory_proposal_create_error"),
      )
    },
  })

  const approveMemoryProposalMutation = useMutation({
    mutationFn: ({
      ownerAgentID,
      proposalID,
      actor,
      note,
    }: {
      ownerAgentID: string
      proposalID: string
      actor: string
      note: string
    }) => approveAgentRuntimeMemoryProposal(ownerAgentID, proposalID, actor, note),
    onSuccess: (proposal) => {
      toast.success(
        t("pages.agent.teammates.memory_proposal_approve_success", {
          id: proposal.id,
        }),
      )
      setProposalEditors((current) => {
        const next = { ...current }
        delete next[proposalKey(proposal)]
        return next
      })
      invalidateRuntime()
    },
    onError: (mutationError: Error) => {
      toast.error(
        mutationError?.message ||
          t("pages.agent.teammates.memory_proposal_approve_error"),
      )
    },
  })

  const rejectMemoryProposalMutation = useMutation({
    mutationFn: ({
      ownerAgentID,
      proposalID,
      actor,
      note,
    }: {
      ownerAgentID: string
      proposalID: string
      actor: string
      note: string
    }) => rejectAgentRuntimeMemoryProposal(ownerAgentID, proposalID, actor, note),
    onSuccess: (proposal) => {
      toast.success(
        t("pages.agent.teammates.memory_proposal_reject_success", {
          id: proposal.id,
        }),
      )
      setProposalEditors((current) => {
        const next = { ...current }
        delete next[proposalKey(proposal)]
        return next
      })
      invalidateRuntime()
    },
    onError: (mutationError: Error) => {
      toast.error(
        mutationError?.message ||
          t("pages.agent.teammates.memory_proposal_reject_error"),
      )
    },
  })

  const updateMemoryProposalMutation = useMutation({
    mutationFn: ({
      ownerAgentID,
      proposalID,
      actor,
      scope,
      domain,
      entryType,
      title,
      content,
      confidence,
    }: {
      ownerAgentID: string
      proposalID: string
      actor: string
      scope: string
      domain: string
      entryType: string
      title: string
      content: string
      confidence: string
    }) =>
      updateAgentRuntimeMemoryProposal(ownerAgentID, proposalID, {
        actor,
        scope,
        domain,
        entry_type: entryType,
        title,
        content,
        confidence,
      }),
    onSuccess: (proposal) => {
      toast.success(
        t("pages.agent.teammates.memory_proposal_update_success", {
          id: proposal.id,
        }),
      )
      setProposalEditors((current) => {
        const existing = current[proposalKey(proposal)]
        if (!existing) {
          return current
        }
        return {
          ...current,
          [proposalKey(proposal)]: {
            actor: existing.actor,
            note: existing.note,
            scope: proposal.scope,
            domain: proposal.domain ?? defaultMemoryProposalDomain(proposal.scope),
            entryType: proposal.entry_type ?? "fact",
            title: proposal.title ?? "",
            content: proposal.content,
            confidence: proposal.confidence ?? "",
          },
        }
      })
      invalidateRuntime()
    },
    onError: (mutationError: Error) => {
      toast.error(
        mutationError?.message ||
          t("pages.agent.teammates.memory_proposal_update_error"),
      )
    },
  })

  const exportMemoryCatalogMutation = useMutation({
    mutationFn: (format: "markdown" | "json") => downloadAgentRuntimeMemoryCatalog(format),
    onSuccess: ({ blob, filename }) => {
      const url = window.URL.createObjectURL(blob)
      const link = document.createElement("a")
      link.href = url
      link.download = filename
      document.body.appendChild(link)
      link.click()
      link.remove()
      window.URL.revokeObjectURL(url)
      toast.success(t("pages.agent.teammates.memory_catalog_export_success", { filename }))
    },
    onError: (mutationError: Error) => {
      toast.error(
        mutationError?.message ||
          t("pages.agent.teammates.memory_catalog_export_error"),
      )
    },
  })

  const pinMemoryCatalogEntryMutation = useMutation({
    mutationFn: ({ entryID }: { entryID: string }) =>
      pinAgentRuntimeMemoryCatalogEntry(entryID, "launcher"),
    onSuccess: (entry) => {
      toast.success(
        t("pages.agent.teammates.memory_catalog_pin_success", { title: entry.title }),
      )
      invalidateRuntime()
    },
    onError: (mutationError: Error) => {
      toast.error(
        mutationError?.message || t("pages.agent.teammates.memory_catalog_pin_error"),
      )
    },
  })

  const unpinMemoryCatalogEntryMutation = useMutation({
    mutationFn: ({ entryID }: { entryID: string }) =>
      unpinAgentRuntimeMemoryCatalogEntry(entryID, "launcher"),
    onSuccess: (entry) => {
      toast.success(
        t("pages.agent.teammates.memory_catalog_unpin_success", { title: entry.title }),
      )
      invalidateRuntime()
    },
    onError: (mutationError: Error) => {
      toast.error(
        mutationError?.message || t("pages.agent.teammates.memory_catalog_unpin_error"),
      )
    },
  })

  const archiveMemoryCatalogEntryMutation = useMutation({
    mutationFn: ({ entryID }: { entryID: string }) =>
      archiveAgentRuntimeMemoryCatalogEntry(entryID, "launcher"),
    onSuccess: (entry) => {
      toast.success(
        t("pages.agent.teammates.memory_catalog_archive_success", { title: entry.title }),
      )
      invalidateRuntime()
    },
    onError: (mutationError: Error) => {
      toast.error(
        mutationError?.message || t("pages.agent.teammates.memory_catalog_archive_error"),
      )
    },
  })

  const restoreMemoryCatalogEntryMutation = useMutation({
    mutationFn: ({ entryID }: { entryID: string }) =>
      restoreAgentRuntimeMemoryCatalogEntry(entryID, "launcher"),
    onSuccess: (entry) => {
      toast.success(
        t("pages.agent.teammates.memory_catalog_restore_success", { title: entry.title }),
      )
      invalidateRuntime()
    },
    onError: (mutationError: Error) => {
      toast.error(
        mutationError?.message || t("pages.agent.teammates.memory_catalog_restore_error"),
      )
    },
  })

  const updateSelectedTaskReview = (patch: Partial<{ actor: string; note: string }>) => {
    if (!taskDetail) {
      return
    }
    const key = taskKey(taskDetail)
    setTaskReviewForms((current) => ({
      ...current,
      [key]: {
        actor: selectedTaskReview.actor,
        note: selectedTaskReview.note,
        ...patch,
      },
    }))
  }

  const updateSelectedTaskHandoff = (patch: Partial<TaskHandoffForm>) => {
    if (!taskDetail) {
      return
    }
    const key = taskKey(taskDetail)
    setTaskHandoffForms((current) => ({
      ...current,
      [key]: {
        ...selectedTaskHandoff,
        ...patch,
      },
    }))
  }

  const revealTask = (key: string) => {
    setTaskStatusFilter("all")
    setTaskFilter("")
    setSelectedTaskKey(key)
  }

  const updateSelectedProposalEditor = (
    patch: Partial<{
      actor: string
      note: string
      scope: string
      domain: string
      entryType: string
      title: string
      content: string
      confidence: string
    }>,
  ) => {
    if (!selectedProposal) {
      return
    }
    const key = proposalKey(selectedProposal)
    setProposalEditors((current) => ({
      ...current,
      [key]: {
        actor: selectedProposalEditor.actor,
        note: selectedProposalEditor.note,
        scope: selectedProposalEditor.scope,
        domain: selectedProposalEditor.domain,
        entryType: selectedProposalEditor.entryType,
        title: selectedProposalEditor.title,
        content: selectedProposalEditor.content,
        confidence: selectedProposalEditor.confidence,
        ...patch,
      },
    }))
  }

  return (
    <div className="bg-background flex h-full flex-col">
      <PageHeader title={t("navigation.teammates")} />

      <div className="flex-1 overflow-auto px-6 py-6">
        <div className="mx-auto w-full max-w-7xl space-y-8">
          {error ? (
            <Card className="border-destructive/50 bg-destructive/10">
              <CardContent className="py-10">
                <p className="text-destructive font-medium">
                  {t("pages.agent.teammates.load_error")}
                </p>
                <p className="text-muted-foreground mt-2 text-sm">
                  {error instanceof Error ? error.message : ""}
                </p>
              </CardContent>
            </Card>
          ) : isLoading ? (
            <RuntimeLoadingState />
          ) : !data ? null : (
            <>
              <section className="grid gap-4 md:grid-cols-4">
                <MetricCard
                  title={t("pages.agent.teammates.summary.agents")}
                  value={data.summary.agent_count}
                  description={t("pages.agent.teammates.summary.agents_desc")}
                />
                <MetricCard
                  title={t("pages.agent.teammates.summary.teammates")}
                  value={data.summary.teammate_count}
                  description={t("pages.agent.teammates.summary.teammates_desc")}
                />
                <MetricCard
                  title={t("pages.agent.teammates.summary.tasks")}
                  value={data.summary.task_count}
                  description={t("pages.agent.teammates.summary.tasks_desc")}
                />
                <MetricCard
                  title={t("pages.agent.teammates.summary.memory_proposals")}
                  value={data.summary.memory_proposal_count}
                  description={t("pages.agent.teammates.summary.memory_proposals_desc")}
                />
              </section>

              <section className="grid gap-6 xl:grid-cols-[0.8fr_1.2fr]">
                <Card>
                  <CardHeader>
                    <CardTitle>{t("pages.agent.teammates.title")}</CardTitle>
                    <CardDescription>
                      {t("pages.agent.teammates.description")}
                    </CardDescription>
                  </CardHeader>
                  <CardContent className="space-y-4">
                    {data.teammates.length === 0 ? (
                      <p className="text-muted-foreground text-sm">
                        {t("pages.agent.teammates.empty")}
                      </p>
                    ) : (
                      data.teammates.map((teammate) => (
                        <div
                          key={teammate.id}
                          className="border-border/60 space-y-2 rounded-xl border p-4"
                        >
                          <div className="flex flex-wrap items-center gap-2">
                            <div className="font-medium">
                              {teammate.name || teammate.id}
                            </div>
                            <Badge variant="secondary">{teammate.id}</Badge>
                            {teammate.role ? (
                              <Badge variant="outline">{teammate.role}</Badge>
                            ) : null}
                          </div>
                          <dl className="text-muted-foreground grid gap-2 text-sm sm:grid-cols-2">
                            <RuntimeField
                              label={t("pages.agent.teammates.fields.agent")}
                              value={teammate.agent_id}
                            />
                            <RuntimeField
                              label={t("pages.agent.teammates.fields.model")}
                              value={teammate.model}
                            />
                            <RuntimeField
                              label={t("pages.agent.teammates.fields.memory")}
                              value={teammate.memory_scope}
                            />
                            <RuntimeField
                              label={t("pages.agent.teammates.fields.approval")}
                              value={teammate.approval_policy}
                            />
                          </dl>
                          {teammate.workspace_scope?.length ? (
                            <RuntimeList
                              label={t("pages.agent.teammates.fields.workspaces")}
                              items={teammate.workspace_scope}
                            />
                          ) : null}
                          {teammate.toolset?.length ? (
                            <RuntimeList
                              label={t("pages.agent.teammates.fields.tools")}
                              items={teammate.toolset}
                            />
                          ) : null}
                        </div>
                      ))
                    )}
                  </CardContent>
                </Card>

                <TaskWorkbench
                  t={t}
                  teammates={data.teammates}
                  allTasks={data.tasks}
                  tasks={filteredTasks}
                  taskStatusEntries={taskStatusEntries}
                  taskStatusFilter={taskStatusFilter}
                  setTaskStatusFilter={setTaskStatusFilter}
                  taskFilter={taskFilter}
                  setTaskFilter={setTaskFilter}
                  selectedTaskKey={selectedTaskKey}
                  setSelectedTaskKey={setSelectedTaskKey}
                  taskDetail={taskDetail}
                  taskDetailLoading={taskDetailQuery.isLoading && !taskDetail}
                  onCancel={(task) =>
                    cancelTaskMutation.mutate({
                      ownerAgentID: task.owner_agent_id,
                      taskID: task.id,
                    })
                  }
                  onApprove={(task) =>
                    approveTaskMutation.mutate({
                      ownerAgentID: task.owner_agent_id,
                      taskID: task.id,
                      actor: selectedTaskReview.actor,
                      note: selectedTaskReview.note,
                    })
                  }
                  onReject={(task) =>
                    rejectTaskMutation.mutate({
                      ownerAgentID: task.owner_agent_id,
                      taskID: task.id,
                      actor: selectedTaskReview.actor,
                      note: selectedTaskReview.note,
                    })
                  }
                  onHandoff={(task) =>
                    handoffTaskMutation.mutate({
                      ownerAgentID: task.owner_agent_id,
                      taskID: task.id,
                      actor: selectedTaskHandoff.actor,
                      note: selectedTaskHandoff.note,
                      teammateID: selectedTaskHandoff.teammateID,
                      label: selectedTaskHandoff.label,
                      task: selectedTaskHandoff.task,
                      kind: selectedTaskHandoff.kind,
                    })
                  }
                  reviewActor={selectedTaskReview.actor}
                  reviewNote={selectedTaskReview.note}
                  setReviewActor={(value) => updateSelectedTaskReview({ actor: value })}
                  setReviewNote={(value) => updateSelectedTaskReview({ note: value })}
                  handoffActor={selectedTaskHandoff.actor}
                  handoffNote={selectedTaskHandoff.note}
                  handoffTeammateID={selectedTaskHandoff.teammateID}
                  handoffLabel={selectedTaskHandoff.label}
                  handoffTask={selectedTaskHandoff.task}
                  handoffKind={selectedTaskHandoff.kind}
                  setHandoffActor={(value) => updateSelectedTaskHandoff({ actor: value })}
                  setHandoffNote={(value) => updateSelectedTaskHandoff({ note: value })}
                  setHandoffTeammateID={(value) =>
                    updateSelectedTaskHandoff({ teammateID: value })
                  }
                  setHandoffLabel={(value) => updateSelectedTaskHandoff({ label: value })}
                  setHandoffTask={(value) => updateSelectedTaskHandoff({ task: value })}
                  setHandoffKind={(value) => updateSelectedTaskHandoff({ kind: value })}
                  onSelectTask={revealTask}
                  onProposeShared={(task) =>
                    createMemoryProposalMutation.mutate({
                      ownerAgentID: task.owner_agent_id,
                      taskID: task.id,
                      scope: "shared",
                    })
                  }
                  onProposeTeammate={(task) =>
                    createMemoryProposalMutation.mutate({
                      ownerAgentID: task.owner_agent_id,
                      taskID: task.id,
                      scope: task.memory_scope || "shared",
                    })
                  }
                  busy={
                    cancelTaskMutation.isPending ||
                    approveTaskMutation.isPending ||
                    rejectTaskMutation.isPending ||
                    handoffTaskMutation.isPending ||
                    createMemoryProposalMutation.isPending
                  }
                />
              </section>

              <MemoryReviewSection
                t={t}
                proposals={filteredProposals}
                memoryStatusEntries={memoryStatusEntries}
                memoryDomainEntries={memoryDomainEntries}
                memoryTypeEntries={memoryTypeEntries}
                memoryFilter={memoryFilter}
                setMemoryFilter={setMemoryFilter}
                memoryStatusFilter={memoryStatusFilter}
                setMemoryStatusFilter={setMemoryStatusFilter}
                memoryDomainFilter={memoryDomainFilter}
                setMemoryDomainFilter={setMemoryDomainFilter}
                memoryTypeFilter={memoryTypeFilter}
                setMemoryTypeFilter={setMemoryTypeFilter}
                selectedProposalKey={selectedProposalKey}
                setSelectedProposalKey={setSelectedProposalKey}
                selectedProposal={selectedProposal}
                onApprove={(proposal) =>
                  approveMemoryProposalMutation.mutate({
                    ownerAgentID: proposal.owner_agent_id,
                    proposalID: proposal.id,
                    actor: selectedProposalEditor.actor,
                    note: selectedProposalEditor.note,
                  })
                }
                onReject={(proposal) =>
                  rejectMemoryProposalMutation.mutate({
                    ownerAgentID: proposal.owner_agent_id,
                    proposalID: proposal.id,
                    actor: selectedProposalEditor.actor,
                    note: selectedProposalEditor.note,
                  })
                }
                onUpdate={(proposal) =>
                  updateMemoryProposalMutation.mutate({
                    ownerAgentID: proposal.owner_agent_id,
                    proposalID: proposal.id,
                    actor: selectedProposalEditor.actor,
                    scope: selectedProposalEditor.scope,
                    domain: selectedProposalEditor.domain,
                    entryType: selectedProposalEditor.entryType,
                    title: selectedProposalEditor.title,
                    content: selectedProposalEditor.content,
                    confidence: selectedProposalEditor.confidence,
                  })
                }
                editor={selectedProposalEditor}
                setEditorActor={(value) => updateSelectedProposalEditor({ actor: value })}
                setEditorNote={(value) => updateSelectedProposalEditor({ note: value })}
                setEditorScope={(value) => updateSelectedProposalEditor({ scope: value })}
                setEditorDomain={(value) => updateSelectedProposalEditor({ domain: value })}
                setEditorEntryType={(value) =>
                  updateSelectedProposalEditor({ entryType: value })
                }
                setEditorTitle={(value) => updateSelectedProposalEditor({ title: value })}
                setEditorContent={(value) => updateSelectedProposalEditor({ content: value })}
                setEditorConfidence={(value) =>
                  updateSelectedProposalEditor({ confidence: value })
                }
                busy={
                  updateMemoryProposalMutation.isPending ||
                  approveMemoryProposalMutation.isPending ||
                  rejectMemoryProposalMutation.isPending
                }
              />

              <MemoryCatalogSection
                t={t}
                catalog={memoryCatalogQuery.data ?? memoryCatalogBaseQuery.data ?? null}
                loading={memoryCatalogQuery.isLoading || memoryCatalogBaseQuery.isLoading}
                error={memoryCatalogQuery.error instanceof Error ? memoryCatalogQuery.error : null}
                entries={filteredCatalogEntries}
                scopeEntries={catalogScopeEntries}
                domainEntries={catalogDomainEntries}
                typeEntries={catalogTypeEntries}
                filter={catalogFilter}
                setFilter={setCatalogFilter}
                scopeFilter={catalogScopeFilter}
                setScopeFilter={setCatalogScopeFilter}
                domainFilter={catalogDomainFilter}
                setDomainFilter={setCatalogDomainFilter}
                typeFilter={catalogTypeFilter}
                setTypeFilter={setCatalogTypeFilter}
                archiveFilter={catalogArchiveFilter}
                setArchiveFilter={setCatalogArchiveFilter}
                selectedEntryKey={selectedCatalogEntryKey}
                setSelectedEntryKey={setSelectedCatalogEntryKey}
                selectedEntry={selectedCatalogEntry}
                onPin={(entry) => pinMemoryCatalogEntryMutation.mutate({ entryID: entry.id })}
                onUnpin={(entry) => unpinMemoryCatalogEntryMutation.mutate({ entryID: entry.id })}
                onArchive={(entry) => archiveMemoryCatalogEntryMutation.mutate({ entryID: entry.id })}
                onRestore={(entry) => restoreMemoryCatalogEntryMutation.mutate({ entryID: entry.id })}
                onExport={(format) => exportMemoryCatalogMutation.mutate(format)}
                busy={
                  exportMemoryCatalogMutation.isPending ||
                  pinMemoryCatalogEntryMutation.isPending ||
                  unpinMemoryCatalogEntryMutation.isPending ||
                  archiveMemoryCatalogEntryMutation.isPending ||
                  restoreMemoryCatalogEntryMutation.isPending
                }
              />

              <MemoryHistorySection
                t={t}
                history={memoryHistoryQuery.data ?? null}
                loading={memoryHistoryQuery.isLoading}
                error={memoryHistoryQuery.error instanceof Error ? memoryHistoryQuery.error : null}
                filter={historyFilter}
                setFilter={setHistoryFilter}
                kindFilter={historyKindFilter}
                setKindFilter={setHistoryKindFilter}
                kindEntries={memoryHistoryKindEntries}
                events={memoryHistoryQuery.data?.events ?? []}
              />
            </>
          )}
        </div>
      </div>
    </div>
  )
}

function TaskWorkbench(props: {
  t: (key: string, options?: Record<string, unknown>) => string
  teammates: RuntimeTeammate[]
  allTasks: AgentRuntimeTask[]
  tasks: AgentRuntimeTask[]
  taskStatusEntries: Array<[string, number]>
  taskStatusFilter: string
  setTaskStatusFilter: (value: string) => void
  taskFilter: string
  setTaskFilter: (value: string) => void
  selectedTaskKey: string
  setSelectedTaskKey: (value: string) => void
  taskDetail: AgentRuntimeTask | null
  taskDetailLoading: boolean
  onCancel: (task: AgentRuntimeTask) => void
  onApprove: (task: AgentRuntimeTask) => void
  onReject: (task: AgentRuntimeTask) => void
  onHandoff: (task: AgentRuntimeTask) => void
  reviewActor: string
  setReviewActor: (value: string) => void
  reviewNote: string
  setReviewNote: (value: string) => void
  handoffActor: string
  setHandoffActor: (value: string) => void
  handoffNote: string
  setHandoffNote: (value: string) => void
  handoffTeammateID: string
  setHandoffTeammateID: (value: string) => void
  handoffLabel: string
  setHandoffLabel: (value: string) => void
  handoffTask: string
  setHandoffTask: (value: string) => void
  handoffKind: string
  setHandoffKind: (value: string) => void
  onSelectTask: (value: string) => void
  onProposeShared: (task: AgentRuntimeTask) => void
  onProposeTeammate: (task: AgentRuntimeTask) => void
  busy: boolean
}) {
  const selectedTask = props.taskDetail
  const parentTask = selectedTask
    ? props.allTasks.find(
        (task) =>
          task.owner_agent_id === selectedTask.parent_owner_agent_id &&
          task.id === selectedTask.parent_task_id,
      ) ?? null
    : null
  const childTasks = selectedTask
    ? props.allTasks.filter(
        (task) =>
          task.parent_owner_agent_id === selectedTask.owner_agent_id &&
          task.parent_task_id === selectedTask.id,
      )
    : []

  return (
    <Card>
      <CardHeader>
        <CardTitle>{props.t("pages.agent.teammates.task_title")}</CardTitle>
        <CardDescription>
          {props.t("pages.agent.teammates.task_description")}
        </CardDescription>
      </CardHeader>
      <CardContent className="space-y-4">
        {props.taskStatusEntries.length > 0 ? (
          <div className="flex flex-wrap gap-2">
            <StatusFilterButton
              active={props.taskStatusFilter === "all"}
              onClick={() => props.setTaskStatusFilter("all")}
            >
              {props.t("pages.agent.teammates.filters.status_all")}
            </StatusFilterButton>
            {props.taskStatusEntries.map(([status, count]) => (
              <StatusFilterButton
                key={status}
                active={props.taskStatusFilter === status}
                onClick={() => props.setTaskStatusFilter(status)}
              >
                {status}: {count}
              </StatusFilterButton>
            ))}
          </div>
        ) : null}

        <Input
          value={props.taskFilter}
          onChange={(event) => props.setTaskFilter(event.target.value)}
          placeholder={props.t("pages.agent.teammates.filters.search_placeholder")}
        />

        {props.tasks.length === 0 ? (
          <p className="text-muted-foreground text-sm">
            {props.t("pages.agent.teammates.tasks_empty")}
          </p>
        ) : (
          <div className="grid gap-4 xl:grid-cols-[0.9fr_1.1fr]">
            <div className="space-y-3">
              {props.tasks.map((task) => {
                const isSelected = taskKey(task) === props.selectedTaskKey
                return (
                  <button
                    key={taskKey(task)}
                    type="button"
                    onClick={() => props.setSelectedTaskKey(taskKey(task))}
                    className={cn(
                      "border-border/60 hover:border-primary/40 hover:bg-muted/40 w-full rounded-xl border p-4 text-left transition-colors",
                      isSelected && "border-primary/50 bg-muted/60",
                    )}
                  >
                    <div className="flex flex-wrap items-center gap-2">
                      <div className="font-mono text-sm">{task.id}</div>
                      <TaskStatusBadge status={task.status} />
                      <Badge variant="secondary">{task.owner_agent_id}</Badge>
                      {task.teammate_id ? (
                        <Badge variant="outline">{task.teammate_id}</Badge>
                      ) : null}
                    </div>
                    {task.label ? (
                      <p className="mt-2 text-sm font-medium">{task.label}</p>
                    ) : null}
                    <p className="text-muted-foreground mt-2 line-clamp-3 text-sm whitespace-pre-wrap">
                      {task.task}
                    </p>
                  </button>
                )
              })}
              {props.tasks.length === 0 ? (
                <p className="text-muted-foreground rounded-xl border p-4 text-sm">
                  {props.t("pages.agent.teammates.filters.no_results")}
                </p>
              ) : null}
            </div>

            <Card className="bg-muted/20 border-dashed">
              <CardHeader>
                <CardTitle>{props.t("pages.agent.teammates.task_detail_title")}</CardTitle>
                <CardDescription>
                  {props.t("pages.agent.teammates.task_detail_description")}
                </CardDescription>
              </CardHeader>
              <CardContent className="space-y-4">
                {!selectedTask ? (
                  <p className="text-muted-foreground text-sm">
                    {props.t("pages.agent.teammates.task_detail_empty")}
                  </p>
                ) : props.taskDetailLoading ? (
                  <p className="text-muted-foreground text-sm">
                    {props.t("pages.agent.teammates.task_detail_loading")}
                  </p>
                ) : (
                  <>
                    <div className="flex flex-wrap items-center gap-2">
                      <div className="font-mono text-sm">{selectedTask.id}</div>
                      <TaskStatusBadge status={selectedTask.status} />
                      <Badge variant="secondary">{selectedTask.owner_agent_id}</Badge>
                      {selectedTask.teammate_id ? (
                        <Badge variant="outline">{selectedTask.teammate_id}</Badge>
                      ) : null}
                    </div>

                    {selectedTask.label ? (
                      <p className="text-base font-medium">{selectedTask.label}</p>
                    ) : null}

                    <p className="text-sm whitespace-pre-wrap">{selectedTask.task}</p>

                    <div className="flex flex-wrap gap-2">
                      {selectedTask.approvable || selectedTask.rejectable ? (
                        <div className="w-full space-y-3 rounded-xl border p-3">
                          <div className="grid gap-3 md:grid-cols-2">
                            <div className="space-y-2">
                              <div className="text-xs uppercase opacity-70">
                                {props.t("pages.agent.teammates.review_fields.actor")}
                              </div>
                              <Input
                                value={props.reviewActor}
                                onChange={(event) =>
                                  props.setReviewActor(event.target.value)
                                }
                                placeholder={props.t(
                                  "pages.agent.teammates.review_fields.actor_placeholder",
                                )}
                              />
                            </div>
                            <div className="space-y-2 md:col-span-2">
                              <div className="text-xs uppercase opacity-70">
                                {props.t("pages.agent.teammates.review_fields.note")}
                              </div>
                              <Textarea
                                value={props.reviewNote}
                                onChange={(event) =>
                                  props.setReviewNote(event.target.value)
                                }
                                placeholder={props.t(
                                  "pages.agent.teammates.review_fields.note_placeholder",
                                )}
                                className="min-h-24"
                              />
                            </div>
                          </div>
                        </div>
                      ) : null}
                      {selectedTask.approvable ? (
                        <Button
                          size="sm"
                          disabled={props.busy}
                          onClick={() => props.onApprove(selectedTask)}
                        >
                          {props.t("pages.agent.teammates.task_actions.approve")}
                        </Button>
                      ) : null}
                      {selectedTask.rejectable ? (
                        <Button
                          variant="destructive"
                          size="sm"
                          disabled={props.busy}
                          onClick={() => props.onReject(selectedTask)}
                        >
                          {props.t("pages.agent.teammates.task_actions.reject")}
                        </Button>
                      ) : null}
                      {selectedTask.cancelable ? (
                        <Button
                          variant="outline"
                          size="sm"
                          disabled={props.busy}
                          onClick={() => props.onCancel(selectedTask)}
                        >
                          {props.t("pages.agent.teammates.task_actions.cancel")}
                        </Button>
                      ) : null}
                    </div>

                    {selectedTask.handoffable ? (
                      <div className="space-y-3 rounded-xl border p-3">
                        <div className="text-xs uppercase opacity-70">
                          {props.t("pages.agent.teammates.handoff.title")}
                        </div>
                        <div className="grid gap-3 md:grid-cols-2">
                          <div className="space-y-2">
                            <div className="text-xs uppercase opacity-70">
                              {props.t("pages.agent.teammates.handoff.target")}
                            </div>
                            <select
                              value={props.handoffTeammateID}
                              onChange={(event) =>
                                props.setHandoffTeammateID(event.target.value)
                              }
                              className="border-input bg-background ring-offset-background flex h-10 w-full rounded-md border px-3 py-2 text-sm"
                            >
                              <option value="">
                                {props.t("pages.agent.teammates.handoff.target_placeholder")}
                              </option>
                              {props.teammates.map((teammate) => (
                                <option key={teammate.id} value={teammate.id}>
                                  {teammate.name || teammate.id} ({teammate.id})
                                </option>
                              ))}
                            </select>
                          </div>
                          <div className="space-y-2">
                            <div className="text-xs uppercase opacity-70">
                              {props.t("pages.agent.teammates.handoff.kind")}
                            </div>
                            <select
                              value={props.handoffKind}
                              onChange={(event) => props.setHandoffKind(event.target.value)}
                              className="border-input bg-background ring-offset-background flex h-10 w-full rounded-md border px-3 py-2 text-sm"
                            >
                              <option value="follow_up">
                                {props.t("pages.agent.teammates.handoff.kinds.follow_up")}
                              </option>
                              <option value="review">
                                {props.t("pages.agent.teammates.handoff.kinds.review")}
                              </option>
                            </select>
                          </div>
                          <div className="space-y-2">
                            <div className="text-xs uppercase opacity-70">
                              {props.t("pages.agent.teammates.handoff.label")}
                            </div>
                            <Input
                              value={props.handoffLabel}
                              onChange={(event) =>
                                props.setHandoffLabel(event.target.value)
                              }
                              placeholder={props.t(
                                "pages.agent.teammates.handoff.label_placeholder",
                              )}
                            />
                          </div>
                          <div className="space-y-2">
                            <div className="text-xs uppercase opacity-70">
                              {props.t("pages.agent.teammates.handoff.actor")}
                            </div>
                            <Input
                              value={props.handoffActor}
                              onChange={(event) =>
                                props.setHandoffActor(event.target.value)
                              }
                              placeholder={props.t(
                                "pages.agent.teammates.handoff.actor_placeholder",
                              )}
                            />
                          </div>
                          <div className="space-y-2 md:col-span-2">
                            <div className="text-xs uppercase opacity-70">
                              {props.t("pages.agent.teammates.handoff.task")}
                            </div>
                            <Textarea
                              value={props.handoffTask}
                              onChange={(event) =>
                                props.setHandoffTask(event.target.value)
                              }
                              placeholder={props.t(
                                "pages.agent.teammates.handoff.task_placeholder",
                              )}
                              className="min-h-32"
                            />
                          </div>
                          <div className="space-y-2 md:col-span-2">
                            <div className="text-xs uppercase opacity-70">
                              {props.t("pages.agent.teammates.handoff.note")}
                            </div>
                            <Textarea
                              value={props.handoffNote}
                              onChange={(event) =>
                                props.setHandoffNote(event.target.value)
                              }
                              placeholder={props.t(
                                "pages.agent.teammates.handoff.note_placeholder",
                              )}
                              className="min-h-24"
                            />
                          </div>
                        </div>
                        <div className="flex flex-wrap gap-2">
                          <Button
                            size="sm"
                            disabled={
                              props.busy ||
                              !props.handoffTeammateID.trim() ||
                              !props.handoffTask.trim()
                            }
                            onClick={() => props.onHandoff(selectedTask)}
                          >
                            {props.t("pages.agent.teammates.task_actions.handoff")}
                          </Button>
                        </div>
                      </div>
                    ) : null}

                    {canPromoteTaskToMemory(selectedTask) ? (
                      <div className="space-y-2">
                        <div className="text-xs uppercase opacity-70">
                          {props.t("pages.agent.teammates.memory_actions.title")}
                        </div>
                        <div className="flex flex-wrap gap-2">
                          <Button
                            variant="secondary"
                            size="sm"
                            disabled={props.busy}
                            onClick={() => props.onProposeShared(selectedTask)}
                          >
                            {props.t("pages.agent.teammates.memory_actions.shared")}
                          </Button>
                          {selectedTask.memory_scope &&
                          selectedTask.memory_scope !== "shared" ? (
                            <Button
                              variant="outline"
                              size="sm"
                              disabled={props.busy}
                              onClick={() => props.onProposeTeammate(selectedTask)}
                            >
                              {props.t("pages.agent.teammates.memory_actions.teammate")}
                            </Button>
                          ) : null}
                        </div>
                      </div>
                    ) : null}

                    <dl className="text-muted-foreground grid gap-3 text-sm sm:grid-cols-2">
                      <RuntimeField
                        label={props.t("pages.agent.teammates.task_fields.requester")}
                        value={
                          selectedTask.requester_teammate_id ||
                          selectedTask.requester_agent_id
                        }
                      />
                      <RuntimeField
                        label={props.t("pages.agent.teammates.task_fields.agent")}
                        value={selectedTask.agent_id}
                      />
                      <RuntimeField
                        label={props.t("pages.agent.teammates.task_fields.approval_policy")}
                        value={selectedTask.approval_policy}
                      />
                      <RuntimeField
                        label={props.t("pages.agent.teammates.task_fields.review_note")}
                        value={selectedTask.review_note}
                      />
                      <RuntimeField
                        label={props.t("pages.agent.teammates.task_fields.memory")}
                        value={selectedTask.memory_scope}
                      />
                      <RuntimeField
                        label={props.t("pages.agent.teammates.task_fields.status")}
                        value={selectedTask.status}
                      />
                      <RuntimeField
                        label={props.t("pages.agent.teammates.task_fields.channel")}
                        value={selectedTask.origin_channel}
                      />
                      <RuntimeField
                        label={props.t("pages.agent.teammates.task_fields.chat")}
                        value={selectedTask.origin_chat_id}
                      />
                      <RuntimeField
                        label={props.t("pages.agent.teammates.task_fields.kind")}
                        value={selectedTask.kind}
                      />
                      <RuntimeField
                        label={props.t("pages.agent.teammates.task_fields.parent")}
                        value={formatTaskRef(
                          selectedTask.parent_owner_agent_id,
                          selectedTask.parent_task_id,
                        )}
                      />
                      <RuntimeField
                        label={props.t("pages.agent.teammates.task_fields.root")}
                        value={formatTaskRef(
                          selectedTask.root_owner_agent_id,
                          selectedTask.root_task_id,
                        )}
                      />
                      <RuntimeField
                        label={props.t("pages.agent.teammates.task_fields.handoff_kind")}
                        value={selectedTask.handoff_kind}
                      />
                      <RuntimeField
                        label={props.t("pages.agent.teammates.task_fields.handoff_by")}
                        value={selectedTask.handoff_actor}
                      />
                      <RuntimeField
                        label={props.t("pages.agent.teammates.task_fields.created")}
                        value={formatTimestamp(selectedTask.created)}
                      />
                      <RuntimeField
                        label={props.t("pages.agent.teammates.task_fields.started")}
                        value={formatTimestamp(selectedTask.started)}
                      />
                      <RuntimeField
                        label={props.t("pages.agent.teammates.task_fields.completed")}
                        value={formatTimestamp(selectedTask.completed)}
                      />
                      <RuntimeField
                        label={props.t("pages.agent.teammates.task_fields.approved")}
                        value={formatReviewed(selectedTask.approved_by, selectedTask.approved_at)}
                      />
                      <RuntimeField
                        label={props.t("pages.agent.teammates.task_fields.rejected")}
                        value={formatReviewed(selectedTask.rejected_by, selectedTask.rejected_at)}
                      />
                    </dl>

                    {selectedTask.workspace_scope?.length ? (
                      <RuntimeList
                        label={props.t("pages.agent.teammates.task_fields.workspaces")}
                        items={selectedTask.workspace_scope}
                      />
                    ) : null}

                    {parentTask ? (
                      <div className="space-y-2">
                        <div className="text-xs uppercase opacity-70">
                          {props.t("pages.agent.teammates.handoff.parent_task")}
                        </div>
                        <Button
                          type="button"
                          size="sm"
                          variant="outline"
                          onClick={() => props.onSelectTask(taskKey(parentTask))}
                        >
                          {renderTaskSummary(parentTask)}
                        </Button>
                      </div>
                    ) : null}

                    {childTasks.length > 0 ? (
                      <div className="space-y-2">
                        <div className="text-xs uppercase opacity-70">
                          {props.t("pages.agent.teammates.handoff.child_tasks")}
                        </div>
                        <div className="flex flex-wrap gap-2">
                          {childTasks.map((task) => (
                            <Button
                              key={taskKey(task)}
                              type="button"
                              size="sm"
                              variant="outline"
                              onClick={() => props.onSelectTask(taskKey(task))}
                            >
                              {renderTaskSummary(task)}
                            </Button>
                          ))}
                        </div>
                      </div>
                    ) : null}

                    <div className="space-y-2">
                      <div className="text-xs uppercase opacity-70">
                        {props.t("pages.agent.teammates.task_fields.result")}
                      </div>
                      <pre className="bg-background overflow-x-auto rounded-xl border p-3 text-xs whitespace-pre-wrap">
                        {selectedTask.result?.trim() || "—"}
                      </pre>
                    </div>
                  </>
                )}
              </CardContent>
            </Card>
          </div>
        )}
      </CardContent>
    </Card>
  )
}

function MemoryReviewSection(props: {
  t: (key: string, options?: Record<string, unknown>) => string
  proposals: AgentRuntimeMemoryProposal[]
  memoryStatusEntries: Array<[string, number]>
  memoryDomainEntries: string[]
  memoryTypeEntries: string[]
  memoryFilter: string
  setMemoryFilter: (value: string) => void
  memoryStatusFilter: string
  setMemoryStatusFilter: (value: string) => void
  memoryDomainFilter: string
  setMemoryDomainFilter: (value: string) => void
  memoryTypeFilter: string
  setMemoryTypeFilter: (value: string) => void
  selectedProposalKey: string
  setSelectedProposalKey: (value: string) => void
  selectedProposal: AgentRuntimeMemoryProposal | null
  editor: MemoryProposalEditor
  setEditorActor: (value: string) => void
  setEditorNote: (value: string) => void
  setEditorScope: (value: string) => void
  setEditorDomain: (value: string) => void
  setEditorEntryType: (value: string) => void
  setEditorTitle: (value: string) => void
  setEditorContent: (value: string) => void
  setEditorConfidence: (value: string) => void
  onUpdate: (proposal: AgentRuntimeMemoryProposal) => void
  onApprove: (proposal: AgentRuntimeMemoryProposal) => void
  onReject: (proposal: AgentRuntimeMemoryProposal) => void
  busy: boolean
}) {
  return (
    <Card>
      <CardHeader>
        <CardTitle>{props.t("pages.agent.teammates.memory_review_title")}</CardTitle>
        <CardDescription>
          {props.t("pages.agent.teammates.memory_review_description")}
        </CardDescription>
      </CardHeader>
      <CardContent className="space-y-4">
        {props.memoryStatusEntries.length > 0 ? (
          <div className="flex flex-wrap gap-2">
            <StatusFilterButton
              active={props.memoryStatusFilter === "all"}
              onClick={() => props.setMemoryStatusFilter("all")}
            >
              {props.t("pages.agent.teammates.filters.status_all")}
            </StatusFilterButton>
            {props.memoryStatusEntries.map(([status, count]) => (
              <StatusFilterButton
                key={status}
                active={props.memoryStatusFilter === status}
                onClick={() => props.setMemoryStatusFilter(status)}
              >
                {status}: {count}
              </StatusFilterButton>
            ))}
          </div>
        ) : null}

        <div className="grid gap-3 md:grid-cols-3">
          <Input
            value={props.memoryFilter}
            onChange={(event) => props.setMemoryFilter(event.target.value)}
            placeholder={props.t("pages.agent.teammates.memory_filters.search_placeholder")}
          />
          <select
            value={props.memoryDomainFilter}
            onChange={(event) => props.setMemoryDomainFilter(event.target.value)}
            className="border-input bg-background ring-offset-background flex h-10 w-full rounded-md border px-3 py-2 text-sm"
          >
            <option value="all">
              {props.t("pages.agent.teammates.memory_filters.domain_all")}
            </option>
            {props.memoryDomainEntries.map((domain) => (
              <option key={domain} value={domain}>
                {domain}
              </option>
            ))}
          </select>
          <select
            value={props.memoryTypeFilter}
            onChange={(event) => props.setMemoryTypeFilter(event.target.value)}
            className="border-input bg-background ring-offset-background flex h-10 w-full rounded-md border px-3 py-2 text-sm"
          >
            <option value="all">
              {props.t("pages.agent.teammates.memory_filters.type_all")}
            </option>
            {props.memoryTypeEntries.map((entryType) => (
              <option key={entryType} value={entryType}>
                {entryType}
              </option>
            ))}
          </select>
        </div>

        {props.proposals.length === 0 ? (
          <p className="text-muted-foreground text-sm">
            {props.t("pages.agent.teammates.memory_review_empty")}
          </p>
        ) : (
          <div className="grid gap-4 xl:grid-cols-[0.9fr_1.1fr]">
            <div className="space-y-3">
              {props.proposals.map((proposal) => {
                const isSelected = proposalKey(proposal) === props.selectedProposalKey
                return (
                  <button
                    key={proposalKey(proposal)}
                    type="button"
                    onClick={() => props.setSelectedProposalKey(proposalKey(proposal))}
                    className={cn(
                      "border-border/60 hover:border-primary/40 hover:bg-muted/40 w-full rounded-xl border p-4 text-left transition-colors",
                      isSelected && "border-primary/50 bg-muted/60",
                    )}
                  >
                    <div className="flex flex-wrap items-center gap-2">
                      <div className="font-mono text-sm">{proposal.id}</div>
                      <MemoryProposalBadge status={proposal.status} />
                      <Badge variant="secondary">{proposal.owner_agent_id}</Badge>
                      {proposal.entry_type ? (
                        <Badge variant="outline">{proposal.entry_type}</Badge>
                      ) : null}
                      {proposal.domain ? (
                        <Badge variant="outline">{proposal.domain}</Badge>
                      ) : null}
                    </div>
                    {proposal.title ? (
                      <p className="mt-2 text-sm font-medium">{proposal.title}</p>
                    ) : null}
                    <p className="text-muted-foreground mt-2 line-clamp-3 text-sm whitespace-pre-wrap">
                      {proposal.content}
                    </p>
                  </button>
                )
              })}
            </div>

            <Card className="bg-muted/20 border-dashed">
              <CardHeader>
                <CardTitle>{props.t("pages.agent.teammates.memory_detail_title")}</CardTitle>
                <CardDescription>
                  {props.t("pages.agent.teammates.memory_detail_description")}
                </CardDescription>
              </CardHeader>
              <CardContent className="space-y-4">
                {!props.selectedProposal ? (
                  <p className="text-muted-foreground text-sm">
                    {props.t("pages.agent.teammates.memory_detail_empty")}
                  </p>
                ) : (
                  <>
                    <div className="flex flex-wrap items-center gap-2">
                      <div className="font-mono text-sm">{props.selectedProposal.id}</div>
                      <MemoryProposalBadge status={props.selectedProposal.status} />
                      <Badge variant="secondary">{props.selectedProposal.owner_agent_id}</Badge>
                    </div>

                    {(props.selectedProposal.status === "pending"
                      ? props.editor.title.trim()
                      : props.selectedProposal.title) ? (
                      <p className="text-base font-medium">
                        {props.selectedProposal.status === "pending"
                          ? props.editor.title.trim()
                          : props.selectedProposal.title}
                      </p>
                    ) : null}

                    {props.selectedProposal.status === "pending" ? (
                      <div className="space-y-4 rounded-xl border p-3">
                        <div className="grid gap-3 md:grid-cols-2">
                          <div className="space-y-2">
                            <div className="text-xs uppercase opacity-70">
                              {props.t("pages.agent.teammates.memory_fields.scope")}
                            </div>
                            <Input
                              value={props.editor.scope}
                              onChange={(event) => props.setEditorScope(event.target.value)}
                              placeholder="shared"
                            />
                          </div>
                          <div className="space-y-2">
                            <div className="text-xs uppercase opacity-70">
                              {props.t("pages.agent.teammates.memory_fields.domain")}
                            </div>
                            <select
                              value={props.editor.domain}
                              onChange={(event) => props.setEditorDomain(event.target.value)}
                              className="border-input bg-background ring-offset-background flex h-10 w-full rounded-md border px-3 py-2 text-sm"
                            >
                              {MEMORY_DOMAINS.map((domain) => (
                                <option key={domain} value={domain}>
                                  {domain}
                                </option>
                              ))}
                            </select>
                          </div>
                          <div className="space-y-2">
                            <div className="text-xs uppercase opacity-70">
                              {props.t("pages.agent.teammates.memory_fields.entry_type")}
                            </div>
                            <select
                              value={props.editor.entryType}
                              onChange={(event) =>
                                props.setEditorEntryType(event.target.value)
                              }
                              className="border-input bg-background ring-offset-background flex h-10 w-full rounded-md border px-3 py-2 text-sm"
                            >
                              {MEMORY_ENTRY_TYPES.map((entryType) => (
                                <option key={entryType} value={entryType}>
                                  {entryType}
                                </option>
                              ))}
                            </select>
                          </div>
                          <div className="space-y-2">
                            <div className="text-xs uppercase opacity-70">
                              {props.t("pages.agent.teammates.memory_fields.confidence")}
                            </div>
                            <select
                              value={props.editor.confidence}
                              onChange={(event) =>
                                props.setEditorConfidence(event.target.value)
                              }
                              className="border-input bg-background ring-offset-background flex h-10 w-full rounded-md border px-3 py-2 text-sm"
                            >
                              <option value="">—</option>
                              {MEMORY_CONFIDENCE_LEVELS.map((confidence) => (
                                <option key={confidence} value={confidence}>
                                  {confidence}
                                </option>
                              ))}
                            </select>
                          </div>
                          <div className="space-y-2">
                            <div className="text-xs uppercase opacity-70">
                              {props.t("pages.agent.teammates.memory_fields.title")}
                            </div>
                            <Input
                              value={props.editor.title}
                              onChange={(event) => props.setEditorTitle(event.target.value)}
                              placeholder={props.t(
                                "pages.agent.teammates.memory_fields.title_placeholder",
                              )}
                            />
                          </div>
                          <div className="space-y-2 md:col-span-2">
                            <div className="text-xs uppercase opacity-70">
                              {props.t("pages.agent.teammates.memory_fields.content")}
                            </div>
                            <Textarea
                              value={props.editor.content}
                              onChange={(event) =>
                                props.setEditorContent(event.target.value)
                              }
                              className="min-h-32"
                            />
                          </div>
                          <div className="space-y-2">
                            <div className="text-xs uppercase opacity-70">
                              {props.t("pages.agent.teammates.review_fields.actor")}
                            </div>
                            <Input
                              value={props.editor.actor}
                              onChange={(event) => props.setEditorActor(event.target.value)}
                              placeholder={props.t(
                                "pages.agent.teammates.review_fields.actor_placeholder",
                              )}
                            />
                          </div>
                          <div className="space-y-2 md:col-span-2">
                            <div className="text-xs uppercase opacity-70">
                              {props.t("pages.agent.teammates.review_fields.note")}
                            </div>
                            <Textarea
                              value={props.editor.note}
                              onChange={(event) => props.setEditorNote(event.target.value)}
                              placeholder={props.t(
                                "pages.agent.teammates.review_fields.note_placeholder",
                              )}
                              className="min-h-24"
                            />
                          </div>
                        </div>

                        <div className="flex flex-wrap gap-2">
                          <Button
                            variant="outline"
                            size="sm"
                            disabled={props.busy || !isProposalEditorDirty(props.selectedProposal, props.editor)}
                            onClick={() => props.onUpdate(props.selectedProposal!)}
                          >
                            {props.t("pages.agent.teammates.memory_actions.save")}
                          </Button>
                          {props.selectedProposal.approvable ? (
                            <Button
                              size="sm"
                              disabled={
                                props.busy ||
                                isProposalEditorDirty(props.selectedProposal, props.editor)
                              }
                              onClick={() => props.onApprove(props.selectedProposal!)}
                            >
                              {props.t("pages.agent.teammates.memory_actions.approve")}
                            </Button>
                          ) : null}
                          {props.selectedProposal.rejectable ? (
                            <Button
                              variant="destructive"
                              size="sm"
                              disabled={props.busy}
                              onClick={() => props.onReject(props.selectedProposal!)}
                            >
                              {props.t("pages.agent.teammates.memory_actions.reject")}
                            </Button>
                          ) : null}
                        </div>
                      </div>
                    ) : (
                      <div className="flex flex-wrap gap-2">
                        {props.selectedProposal.approvable ? (
                          <Button
                            size="sm"
                            disabled={props.busy}
                            onClick={() => props.onApprove(props.selectedProposal!)}
                          >
                            {props.t("pages.agent.teammates.memory_actions.approve")}
                          </Button>
                        ) : null}
                        {props.selectedProposal.rejectable ? (
                          <Button
                            variant="destructive"
                            size="sm"
                            disabled={props.busy}
                            onClick={() => props.onReject(props.selectedProposal!)}
                          >
                            {props.t("pages.agent.teammates.memory_actions.reject")}
                          </Button>
                        ) : null}
                      </div>
                    )}

                    <dl className="text-muted-foreground grid gap-3 text-sm sm:grid-cols-2">
                      <RuntimeField
                        label={props.t("pages.agent.teammates.memory_fields.scope")}
                        value={props.selectedProposal.scope}
                      />
                      <RuntimeField
                        label={props.t("pages.agent.teammates.memory_fields.domain")}
                        value={props.selectedProposal.domain}
                      />
                      <RuntimeField
                        label={props.t("pages.agent.teammates.memory_fields.entry_type")}
                        value={props.selectedProposal.entry_type}
                      />
                      <RuntimeField
                        label={props.t("pages.agent.teammates.memory_fields.target")}
                        value={props.selectedProposal.target}
                      />
                      <RuntimeField
                        label={props.t("pages.agent.teammates.memory_fields.kind")}
                        value={props.selectedProposal.kind}
                      />
                      <RuntimeField
                        label={props.t("pages.agent.teammates.memory_fields.status")}
                        value={props.selectedProposal.status}
                      />
                      <RuntimeField
                        label={props.t("pages.agent.teammates.memory_fields.source_task")}
                        value={props.selectedProposal.source_task_id}
                      />
                      <RuntimeField
                        label={props.t("pages.agent.teammates.memory_fields.source_teammate")}
                        value={props.selectedProposal.source_teammate_id}
                      />
                      <RuntimeField
                        label={props.t("pages.agent.teammates.memory_fields.confidence")}
                        value={props.selectedProposal.confidence}
                      />
                      <RuntimeField
                        label={props.t("pages.agent.teammates.memory_fields.created")}
                        value={formatTimestamp(props.selectedProposal.created)}
                      />
                      <RuntimeField
                        label={props.t("pages.agent.teammates.memory_fields.updated")}
                        value={formatReviewed(
                          props.selectedProposal.updated_by,
                          props.selectedProposal.updated_at,
                        )}
                      />
                      <RuntimeField
                        label={props.t("pages.agent.teammates.memory_fields.reviewed")}
                        value={formatReviewed(
                          props.selectedProposal.reviewed_by,
                          props.selectedProposal.reviewed_at,
                        )}
                      />
                      <RuntimeField
                        label={props.t("pages.agent.teammates.memory_fields.review_note")}
                        value={props.selectedProposal.review_note}
                      />
                    </dl>

                    {props.selectedProposal.status !== "pending" ? (
                      <div className="space-y-2">
                        <div className="text-xs uppercase opacity-70">
                          {props.t("pages.agent.teammates.memory_fields.content")}
                        </div>
                        <pre className="bg-background overflow-x-auto rounded-xl border p-3 text-xs whitespace-pre-wrap">
                          {props.selectedProposal.content}
                        </pre>
                      </div>
                    ) : null}
                  </>
                )}
              </CardContent>
            </Card>
          </div>
        )}
      </CardContent>
    </Card>
  )
}

function MetricCard(props: { title: string; value: number; description: string }) {
  return (
    <Card>
      <CardHeader className="pb-2">
        <CardDescription>{props.title}</CardDescription>
        <CardTitle className="text-3xl">{props.value}</CardTitle>
      </CardHeader>
      <CardContent className="text-muted-foreground text-sm">
        {props.description}
      </CardContent>
    </Card>
  )
}

function RuntimeField(props: { label: string; value?: string }) {
  return (
    <div>
      <dt className="text-xs uppercase opacity-70">{props.label}</dt>
      <dd className="text-foreground break-all">
        {props.value && props.value.trim() ? props.value : "—"}
      </dd>
    </div>
  )
}

function RuntimeList(props: { label: string; items: string[] }) {
  return (
    <div className="space-y-2">
      <div className="text-xs uppercase opacity-70">{props.label}</div>
      <div className="flex flex-wrap gap-2">
        {props.items.map((item) => (
          <Badge key={item} variant="outline" className="max-w-full break-all">
            {item}
          </Badge>
        ))}
      </div>
    </div>
  )
}

function StatusFilterButton(props: {
  active: boolean
  children: ReactNode
  onClick: () => void
}) {
  return (
    <Button
      type="button"
      size="sm"
      variant={props.active ? "secondary" : "outline"}
      onClick={props.onClick}
    >
      {props.children}
    </Button>
  )
}

function TaskStatusBadge({ status }: { status: string }) {
  const normalized = status.toLowerCase()
  const variant =
    normalized === "completed"
      ? "secondary"
      : normalized === "failed" || normalized === "canceled" || normalized === "denied"
        ? "destructive"
        : normalized === "awaiting_approval"
          ? "default"
          : normalized === "canceling"
            ? "default"
            : "outline"
  return <Badge variant={variant}>{status}</Badge>
}

function MemoryProposalBadge({ status }: { status: string }) {
  const normalized = status.toLowerCase()
  const variant =
    normalized === "approved"
      ? "secondary"
      : normalized === "rejected"
        ? "destructive"
        : "default"
  return <Badge variant={variant}>{status}</Badge>
}

function MemoryCatalogSection(props: {
  t: (key: string, options?: Record<string, unknown>) => string
  catalog: AgentRuntimeMemoryCatalog | null
  loading: boolean
  error: Error | null
  entries: AgentRuntimeMemoryCatalogEntry[]
  scopeEntries: AgentRuntimeMemoryCatalog["scopes"]
  domainEntries: string[]
  typeEntries: string[]
  filter: string
  setFilter: (value: string) => void
  scopeFilter: string
  setScopeFilter: (value: string) => void
  domainFilter: string
  setDomainFilter: (value: string) => void
  typeFilter: string
  setTypeFilter: (value: string) => void
  archiveFilter: string
  setArchiveFilter: (value: string) => void
  selectedEntryKey: string
  setSelectedEntryKey: (value: string) => void
  selectedEntry: AgentRuntimeMemoryCatalogEntry | null
  onPin: (entry: AgentRuntimeMemoryCatalogEntry) => void
  onUnpin: (entry: AgentRuntimeMemoryCatalogEntry) => void
  onArchive: (entry: AgentRuntimeMemoryCatalogEntry) => void
  onRestore: (entry: AgentRuntimeMemoryCatalogEntry) => void
  onExport: (format: "markdown" | "json") => void
  busy: boolean
}) {
  return (
    <section className="grid gap-6 xl:grid-cols-[0.9fr_1.1fr]">
      <Card>
        <CardHeader>
          <div className="flex flex-col gap-4 lg:flex-row lg:items-start lg:justify-between">
            <div className="space-y-1">
              <CardTitle>{props.t("pages.agent.teammates.memory_catalog_title")}</CardTitle>
              <CardDescription>
                {props.t("pages.agent.teammates.memory_catalog_description")}
              </CardDescription>
            </div>
            <div className="flex flex-wrap gap-2">
              <Button
                type="button"
                variant="outline"
                disabled={props.busy}
                onClick={() => props.onExport("markdown")}
              >
                {props.t("pages.agent.teammates.memory_catalog_actions.export_markdown")}
              </Button>
              <Button
                type="button"
                variant="outline"
                disabled={props.busy}
                onClick={() => props.onExport("json")}
              >
                {props.t("pages.agent.teammates.memory_catalog_actions.export_json")}
              </Button>
            </div>
          </div>
        </CardHeader>
        <CardContent className="space-y-4">
          {props.catalog ? (
            <div className="grid gap-3 sm:grid-cols-5">
              <RuntimeField
                label={props.t("pages.agent.teammates.memory_catalog_stats.entries")}
                value={String(props.catalog.summary.entry_count)}
              />
              <RuntimeField
                label={props.t("pages.agent.teammates.memory_catalog_stats.scopes")}
                value={String(props.catalog.summary.scope_count)}
              />
              <RuntimeField
                label={props.t("pages.agent.teammates.memory_catalog_stats.workspaces")}
                value={String(props.catalog.summary.workspace_count)}
              />
              <RuntimeField
                label={props.t("pages.agent.teammates.memory_catalog_stats.pinned")}
                value={String(props.catalog.summary.pinned_count)}
              />
              <RuntimeField
                label={props.t("pages.agent.teammates.memory_catalog_stats.archived")}
                value={String(props.catalog.summary.archived_count)}
              />
            </div>
          ) : null}

          <div className="grid gap-3 md:grid-cols-5">
            <Input
              value={props.filter}
              onChange={(event) => props.setFilter(event.target.value)}
              placeholder={props.t("pages.agent.teammates.memory_catalog_filters.search_placeholder")}
            />
            <select
              value={props.scopeFilter}
              onChange={(event) => props.setScopeFilter(event.target.value)}
              className="border-input bg-background ring-offset-background flex h-10 w-full rounded-md border px-3 py-2 text-sm"
            >
              <option value="all">
                {props.t("pages.agent.teammates.memory_catalog_filters.scope_all")}
              </option>
              {(props.scopeEntries ?? []).map((scope) => (
                <option key={`${scope.owner_agent_id}:${scope.scope}`} value={scope.scope}>
                  {scope.display_name} ({scope.entry_count})
                </option>
              ))}
            </select>
            <select
              value={props.domainFilter}
              onChange={(event) => props.setDomainFilter(event.target.value)}
              className="border-input bg-background ring-offset-background flex h-10 w-full rounded-md border px-3 py-2 text-sm"
            >
              <option value="all">
                {props.t("pages.agent.teammates.memory_catalog_filters.domain_all")}
              </option>
              {props.domainEntries.map((domain) => (
                <option key={domain} value={domain}>
                  {domain}
                </option>
              ))}
            </select>
            <select
              value={props.typeFilter}
              onChange={(event) => props.setTypeFilter(event.target.value)}
              className="border-input bg-background ring-offset-background flex h-10 w-full rounded-md border px-3 py-2 text-sm"
            >
              <option value="all">
                {props.t("pages.agent.teammates.memory_catalog_filters.type_all")}
              </option>
              {props.typeEntries.map((entryType) => (
                <option key={entryType} value={entryType}>
                  {entryType}
                </option>
              ))}
            </select>
            <select
              value={props.archiveFilter}
              onChange={(event) => props.setArchiveFilter(event.target.value)}
              className="border-input bg-background ring-offset-background flex h-10 w-full rounded-md border px-3 py-2 text-sm"
            >
              <option value="active">
                {props.t("pages.agent.teammates.memory_catalog_filters.archived_active")}
              </option>
              <option value="archived">
                {props.t("pages.agent.teammates.memory_catalog_filters.archived_only")}
              </option>
              <option value="all">
                {props.t("pages.agent.teammates.memory_catalog_filters.archived_all")}
              </option>
            </select>
          </div>

          {props.error ? (
            <div className="border-destructive/50 bg-destructive/10 rounded-xl border p-4 text-sm">
              <div className="text-destructive font-medium">
                {props.t("pages.agent.teammates.memory_catalog_load_error")}
              </div>
              <div className="text-muted-foreground mt-1">{props.error.message}</div>
            </div>
          ) : props.loading ? (
            <div className="space-y-3">
              {[1, 2, 3].map((row) => (
                <div key={row} className="rounded-xl border p-4">
                  <Skeleton className="h-4 w-40" />
                  <Skeleton className="mt-2 h-4 w-full" />
                </div>
              ))}
            </div>
          ) : props.entries.length === 0 ? (
            <p className="text-muted-foreground text-sm">
              {props.t("pages.agent.teammates.memory_catalog_empty")}
            </p>
          ) : (
            <div className="space-y-3">
              {props.entries.map((entry) => {
                const isSelected = catalogEntryKey(entry) === props.selectedEntryKey
                return (
                  <button
                    key={catalogEntryKey(entry)}
                    type="button"
                    onClick={() => props.setSelectedEntryKey(catalogEntryKey(entry))}
                    className={cn(
                      "border-border/60 hover:border-primary/40 hover:bg-muted/60 w-full rounded-xl border p-4 text-left transition-colors",
                      isSelected && "border-primary/50 bg-primary/5",
                    )}
                  >
                    <div className="flex flex-wrap items-center gap-2">
                      <div className="font-medium">{entry.title}</div>
                      <Badge variant="secondary">{entry.scope_display_name}</Badge>
                      {entry.pinned ? (
                        <Badge>{props.t("pages.agent.teammates.memory_catalog_fields.pinned")}</Badge>
                      ) : null}
                      {entry.archived ? (
                        <Badge variant="outline">
                          {props.t("pages.agent.teammates.memory_catalog_fields.archived")}
                        </Badge>
                      ) : null}
                      {entry.domain ? <Badge variant="outline">{entry.domain}</Badge> : null}
                      {entry.entry_type ? (
                        <Badge variant="outline">{entry.entry_type}</Badge>
                      ) : null}
                      {entry.legacy ? (
                        <Badge variant="destructive">
                          {props.t("pages.agent.teammates.memory_catalog_fields.legacy")}
                        </Badge>
                      ) : null}
                    </div>
                    <p className="text-muted-foreground mt-2 line-clamp-3 text-sm whitespace-pre-wrap">
                      {entry.content}
                    </p>
                    <div className="text-muted-foreground mt-3 flex flex-wrap gap-3 text-xs">
                      <span>{entry.owner_agent_id}</span>
                      {entry.source_teammate_id ? <span>{entry.source_teammate_id}</span> : null}
                      <span>{entry.added_at ? formatTimestamp(entry.added_at) : entry.added_at_display}</span>
                    </div>
                  </button>
                )
              })}
            </div>
          )}
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle>{props.t("pages.agent.teammates.memory_catalog_detail_title")}</CardTitle>
          <CardDescription>
            {props.t("pages.agent.teammates.memory_catalog_detail_description")}
          </CardDescription>
        </CardHeader>
        <CardContent>
          {!props.selectedEntry ? (
            <p className="text-muted-foreground text-sm">
              {props.t("pages.agent.teammates.memory_catalog_detail_empty")}
            </p>
          ) : (
            <div className="space-y-6">
              <div className="space-y-2">
                <div className="flex flex-wrap items-center gap-2">
                  <div className="font-mono text-sm">{props.selectedEntry.id}</div>
                  <Badge variant="secondary">{props.selectedEntry.owner_agent_id}</Badge>
                  <Badge variant="outline">{props.selectedEntry.scope_display_name}</Badge>
                  {props.selectedEntry.pinned ? (
                    <Badge>{props.t("pages.agent.teammates.memory_catalog_fields.pinned")}</Badge>
                  ) : null}
                  {props.selectedEntry.archived ? (
                    <Badge variant="outline">
                      {props.t("pages.agent.teammates.memory_catalog_fields.archived")}
                    </Badge>
                  ) : null}
                </div>
                <h3 className="text-lg font-semibold">{props.selectedEntry.title}</h3>
              </div>

              <div className="flex flex-wrap gap-2">
                {props.selectedEntry.pinned ? (
                  <Button
                    type="button"
                    variant="outline"
                    disabled={props.busy}
                    onClick={() => props.onUnpin(props.selectedEntry!)}
                  >
                    {props.t("pages.agent.teammates.memory_catalog_actions.unpin")}
                  </Button>
                ) : (
                  <Button
                    type="button"
                    variant="outline"
                    disabled={props.busy}
                    onClick={() => props.onPin(props.selectedEntry!)}
                  >
                    {props.t("pages.agent.teammates.memory_catalog_actions.pin")}
                  </Button>
                )}
                {props.selectedEntry.archived ? (
                  <Button
                    type="button"
                    variant="outline"
                    disabled={props.busy}
                    onClick={() => props.onRestore(props.selectedEntry!)}
                  >
                    {props.t("pages.agent.teammates.memory_catalog_actions.restore")}
                  </Button>
                ) : (
                  <Button
                    type="button"
                    variant="outline"
                    disabled={props.busy}
                    onClick={() => props.onArchive(props.selectedEntry!)}
                  >
                    {props.t("pages.agent.teammates.memory_catalog_actions.archive")}
                  </Button>
                )}
              </div>

              <dl className="grid gap-4 sm:grid-cols-2">
                <RuntimeField
                  label={props.t("pages.agent.teammates.memory_catalog_fields.scope")}
                  value={props.selectedEntry.scope}
                />
                <RuntimeField
                  label={props.t("pages.agent.teammates.memory_catalog_fields.workspace")}
                  value={props.selectedEntry.workspace}
                />
                <RuntimeField
                  label={props.t("pages.agent.teammates.memory_catalog_fields.domain")}
                  value={props.selectedEntry.domain}
                />
                <RuntimeField
                  label={props.t("pages.agent.teammates.memory_catalog_fields.entry_type")}
                  value={props.selectedEntry.entry_type}
                />
                <RuntimeField
                  label={props.t("pages.agent.teammates.memory_catalog_fields.confidence")}
                  value={props.selectedEntry.confidence}
                />
                <RuntimeField
                  label={props.t("pages.agent.teammates.memory_catalog_fields.added")}
                  value={
                    props.selectedEntry.added_at
                      ? formatTimestamp(props.selectedEntry.added_at)
                      : props.selectedEntry.added_at_display
                  }
                />
                <RuntimeField
                  label={props.t("pages.agent.teammates.memory_catalog_fields.source_task")}
                  value={props.selectedEntry.source_task_id}
                />
                <RuntimeField
                  label={props.t("pages.agent.teammates.memory_catalog_fields.source_teammate")}
                  value={props.selectedEntry.source_teammate_id}
                />
                <RuntimeField
                  label={props.t("pages.agent.teammates.memory_catalog_fields.reviewed_by")}
                  value={props.selectedEntry.reviewed_by}
                />
                <RuntimeField
                  label={props.t("pages.agent.teammates.memory_catalog_fields.pinned_by")}
                  value={formatReviewed(props.selectedEntry.pinned_by, props.selectedEntry.pinned_at)}
                />
                <RuntimeField
                  label={props.t("pages.agent.teammates.memory_catalog_fields.archived_by")}
                  value={formatReviewed(props.selectedEntry.archived_by, props.selectedEntry.archived_at)}
                />
                <RuntimeField
                  label={props.t("pages.agent.teammates.memory_catalog_fields.source_path")}
                  value={props.selectedEntry.source_path}
                />
              </dl>

              <div className="space-y-2">
                <div className="text-xs uppercase opacity-70">
                  {props.t("pages.agent.teammates.memory_catalog_fields.content")}
                </div>
                <div className="bg-muted/40 rounded-xl border p-4 text-sm whitespace-pre-wrap">
                  {props.selectedEntry.content}
                </div>
              </div>
            </div>
          )}
        </CardContent>
      </Card>
    </section>
  )
}

function MemoryHistorySection(props: {
  t: (key: string, options?: Record<string, unknown>) => string
  history: AgentRuntimeMemoryHistory | null
  loading: boolean
  error: Error | null
  filter: string
  setFilter: (value: string) => void
  kindFilter: string
  setKindFilter: (value: string) => void
  kindEntries: Array<[string, number]>
  events: AgentRuntimeMemoryHistoryEvent[]
}) {
  return (
    <Card>
      <CardHeader>
        <CardTitle>{props.t("pages.agent.teammates.memory_history_title")}</CardTitle>
        <CardDescription>
          {props.t("pages.agent.teammates.memory_history_description")}
        </CardDescription>
      </CardHeader>
      <CardContent className="space-y-4">
        {props.history ? (
          <div className="grid gap-3 sm:grid-cols-3">
            <RuntimeField
              label={props.t("pages.agent.teammates.memory_history_stats.events")}
              value={String(props.history.summary.event_count)}
            />
            <RuntimeField
              label={props.t("pages.agent.teammates.memory_history_stats.catalog")}
              value={String(props.history.summary.catalog_event_count)}
            />
            <RuntimeField
              label={props.t("pages.agent.teammates.memory_history_stats.proposals")}
              value={String(props.history.summary.proposal_event_count)}
            />
          </div>
        ) : null}

        <div className="grid gap-3 md:grid-cols-[2fr_1fr]">
          <Input
            value={props.filter}
            onChange={(event) => props.setFilter(event.target.value)}
            placeholder={props.t("pages.agent.teammates.memory_history_filters.search_placeholder")}
          />
          <select
            value={props.kindFilter}
            onChange={(event) => props.setKindFilter(event.target.value)}
            className="border-input bg-background ring-offset-background flex h-10 w-full rounded-md border px-3 py-2 text-sm"
          >
            <option value="all">
              {props.t("pages.agent.teammates.memory_history_filters.kind_all")}
            </option>
            {props.kindEntries.map(([kind, count]) => (
              <option key={kind} value={kind}>
                {kind} ({count})
              </option>
            ))}
          </select>
        </div>

        {props.error ? (
          <div className="border-destructive/50 bg-destructive/10 rounded-xl border p-4 text-sm">
            <div className="text-destructive font-medium">
              {props.t("pages.agent.teammates.memory_history_load_error")}
            </div>
            <div className="text-muted-foreground mt-1">{props.error.message}</div>
          </div>
        ) : props.loading ? (
          <div className="space-y-3">
            {[1, 2, 3].map((row) => (
              <div key={row} className="rounded-xl border p-4">
                <Skeleton className="h-4 w-40" />
                <Skeleton className="mt-2 h-4 w-full" />
              </div>
            ))}
          </div>
        ) : props.events.length === 0 ? (
          <p className="text-muted-foreground text-sm">
            {props.t("pages.agent.teammates.memory_history_empty")}
          </p>
        ) : (
          <div className="space-y-3">
            {props.events.map((event) => (
              <div key={event.id} className="rounded-xl border p-4">
                <div className="flex flex-wrap items-center gap-2">
                  <div className="font-medium">{event.title || event.subject_id}</div>
                  <Badge variant="secondary">{event.kind}</Badge>
                  {event.scope_display_name ? (
                    <Badge variant="outline">{event.scope_display_name}</Badge>
                  ) : null}
                  {event.domain ? <Badge variant="outline">{event.domain}</Badge> : null}
                  {event.entry_type ? <Badge variant="outline">{event.entry_type}</Badge> : null}
                </div>
                {event.content ? (
                  <p className="text-muted-foreground mt-2 line-clamp-2 text-sm whitespace-pre-wrap">
                    {event.content}
                  </p>
                ) : null}
                <div className="text-muted-foreground mt-3 flex flex-wrap gap-3 text-xs">
                  <span>{event.subject_type}</span>
                  {event.actor ? <span>{event.actor}</span> : null}
                  <span>{formatTimestamp(event.timestamp)}</span>
                </div>
              </div>
            ))}
          </div>
        )}
      </CardContent>
    </Card>
  )
}

function RuntimeLoadingState() {
  return (
    <div className="space-y-6">
      <div className="grid gap-4 md:grid-cols-4">
        {[1, 2, 3, 4].map((item) => (
          <Card key={item}>
            <CardHeader className="pb-2">
              <Skeleton className="h-4 w-24" />
              <Skeleton className="h-8 w-16" />
            </CardHeader>
            <CardContent>
              <Skeleton className="h-4 w-40" />
            </CardContent>
          </Card>
        ))}
      </div>
      <div className="grid gap-6 xl:grid-cols-2">
        {[1, 2, 3].map((item) => (
          <Card key={item}>
            <CardHeader>
              <Skeleton className="h-5 w-32" />
              <Skeleton className="h-4 w-56" />
            </CardHeader>
            <CardContent className="space-y-4">
              {[1, 2].map((row) => (
                <div key={row} className="space-y-2 rounded-xl border p-4">
                  <Skeleton className="h-4 w-36" />
                  <Skeleton className="h-4 w-full" />
                  <Skeleton className="h-4 w-2/3" />
                </div>
              ))}
            </CardContent>
          </Card>
        ))}
      </div>
    </div>
  )
}

function taskKey(task: Pick<AgentRuntimeTask, "owner_agent_id" | "id">) {
  return `${task.owner_agent_id}:${task.id}`
}

function createEmptyTaskHandoffForm(): TaskHandoffForm {
  return {
    actor: "launcher",
    note: "",
    teammateID: "",
    label: "",
    task: "",
    kind: "follow_up",
  }
}

function createDefaultTaskHandoffForm(
  task: AgentRuntimeTask,
  teammates: RuntimeTeammate[],
): TaskHandoffForm {
  const targetTeammate =
    teammates.find((teammate) => teammate.role === "reviewer" && teammate.id !== task.teammate_id) ??
    teammates.find((teammate) => teammate.id !== task.teammate_id) ??
    teammates[0]
  const kind = targetTeammate?.role === "reviewer" ? "review" : "follow_up"
  const labelBase = task.label?.trim() || task.id
  return {
    actor: "launcher",
    note: "",
    teammateID: targetTeammate?.id ?? "",
    label: kind === "review" ? `Review: ${labelBase}` : `Follow-up: ${labelBase}`,
    task: buildDefaultHandoffTaskText(task, kind),
    kind,
  }
}

function proposalKey(
  proposal: Pick<AgentRuntimeMemoryProposal, "owner_agent_id" | "id">,
) {
  return `${proposal.owner_agent_id}:${proposal.id}`
}

function catalogEntryKey(
  entry: Pick<AgentRuntimeMemoryCatalogEntry, "owner_agent_id" | "id">,
) {
  return `${entry.owner_agent_id}:${entry.id}`
}

function formatTimestamp(value?: number) {
  if (!value) {
    return ""
  }
  return new Date(value).toLocaleString()
}

function formatReviewed(actor?: string, at?: number) {
  if (!actor && !at) {
    return ""
  }
  if (actor && at) {
    return `${actor} · ${formatTimestamp(at)}`
  }
  return actor || formatTimestamp(at)
}

function canPromoteTaskToMemory(task: AgentRuntimeTask) {
  if (!task.result?.trim()) {
    return false
  }
  switch (task.status.toLowerCase()) {
    case "completed":
    case "failed":
    case "canceled":
    case "denied":
      return true
    default:
      return false
  }
}

function isProposalEditorDirty(
  proposal: AgentRuntimeMemoryProposal,
  editor: MemoryProposalEditor,
) {
  return (
    editor.scope.trim() !== proposal.scope ||
    editor.domain.trim() !== (proposal.domain ?? defaultMemoryProposalDomain(proposal.scope)) ||
    editor.entryType.trim() !== (proposal.entry_type ?? "fact") ||
    editor.title.trim() !== (proposal.title ?? "") ||
    editor.content.trim() !== proposal.content.trim() ||
    editor.confidence.trim() !== (proposal.confidence ?? "")
  )
}

function defaultMemoryProposalDomain(scope: string) {
  if (scope.trim() === "shared") {
    return "shared_team"
  }
  if (scope.trim().startsWith("teammate:")) {
    return "teammate_local"
  }
  return "project"
}

function buildDefaultHandoffTaskText(task: AgentRuntimeTask, kind: string) {
  const intro =
    kind === "review"
      ? "Review the completed task below and provide feedback, risks, or next steps."
      : "Continue from the completed task below and handle the next follow-up work."
  const parts = [
    intro,
    "",
    `Source task ID: ${task.id}`,
    task.label?.trim() ? `Source label: ${task.label.trim()}` : "",
    task.teammate_id?.trim() ? `Source teammate: ${task.teammate_id.trim()}` : "",
    `Source status: ${task.status}`,
    "",
    "Original task:",
    task.task.trim(),
  ]
  if (task.result?.trim()) {
    parts.push("", "Result:", task.result.trim())
  }
  return parts.filter(Boolean).join("\n")
}

function formatTaskRef(ownerAgentID?: string, taskID?: string) {
  if (!ownerAgentID?.trim() || !taskID?.trim()) {
    return ""
  }
  return `${ownerAgentID}:${taskID}`
}

function renderTaskSummary(task: AgentRuntimeTask) {
  const base = task.label?.trim() || task.id
  return `${task.owner_agent_id}:${base}`
}
