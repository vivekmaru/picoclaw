import {
  useMutation,
  useQuery,
  useQueryClient,
} from "@tanstack/react-query"
import { useEffect, useMemo, useState, type ReactNode } from "react"
import { useTranslation } from "react-i18next"
import { toast } from "sonner"

import {
  cancelAgentRuntimeTask,
  getAgentRuntime,
  getAgentRuntimeTask,
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
import { cn } from "@/lib/utils"

const RUNTIME_POLL_MS = 3000

export function TeammatesPage() {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const [taskFilter, setTaskFilter] = useState("")
  const [statusFilter, setStatusFilter] = useState("all")
  const [selectedTaskKey, setSelectedTaskKey] = useState("")
  const { data, isLoading, error } = useQuery({
    queryKey: ["agent-runtime"],
    queryFn: getAgentRuntime,
    refetchInterval: RUNTIME_POLL_MS,
  })

  const taskStatusEntries = useMemo(() => {
    return Object.entries(data?.summary.task_statuses ?? {}).sort((a, b) =>
      a[0].localeCompare(b[0]),
    )
  }, [data?.summary.task_statuses])

  const filteredTasks = useMemo(() => {
    const normalizedFilter = taskFilter.trim().toLowerCase()
    return (data?.tasks ?? []).filter((task) => {
      if (statusFilter !== "all" && task.status !== statusFilter) {
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
  }, [data?.tasks, statusFilter, taskFilter])

  useEffect(() => {
    if (filteredTasks.length === 0) {
      if (selectedTaskKey !== "") {
        setSelectedTaskKey("")
      }
      return
    }
    const exists = filteredTasks.some((task) => taskKey(task) === selectedTaskKey)
    if (!exists) {
      setSelectedTaskKey(taskKey(filteredTasks[0]))
    }
  }, [filteredTasks, selectedTaskKey])

  const selectedTask = useMemo(() => {
    return filteredTasks.find((task) => taskKey(task) === selectedTaskKey) ?? null
  }, [filteredTasks, selectedTaskKey])

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

  const cancelMutation = useMutation({
    mutationFn: ({
      ownerAgentID,
      taskID,
    }: {
      ownerAgentID: string
      taskID: string
    }) => cancelAgentRuntimeTask(ownerAgentID, taskID),
    onSuccess: (task) => {
      toast.success(t("pages.agent.teammates.task_cancel_success", { id: task.id }))
      void queryClient.invalidateQueries({ queryKey: ["agent-runtime"] })
      void queryClient.invalidateQueries({
        queryKey: ["agent-runtime", "task", task.owner_agent_id, task.id],
      })
    },
    onError: (mutationError: Error) => {
      toast.error(
        mutationError?.message ||
          t("pages.agent.teammates.task_cancel_error"),
      )
    },
  })

  const taskDetail = taskDetailQuery.data ?? selectedTask

  return (
    <div className="bg-background flex h-full flex-col">
      <PageHeader title={t("navigation.teammates")} />

      <div className="flex-1 overflow-auto px-6 py-6">
        <div className="mx-auto w-full max-w-6xl space-y-8">
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
              <section className="grid gap-4 md:grid-cols-3">
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
              </section>

              <section className="grid gap-6 xl:grid-cols-[0.9fr_1.1fr]">
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

                <Card>
                  <CardHeader>
                    <CardTitle>{t("pages.agent.teammates.task_title")}</CardTitle>
                    <CardDescription>
                      {t("pages.agent.teammates.task_description")}
                    </CardDescription>
                  </CardHeader>
                  <CardContent className="space-y-4">
                    {taskStatusEntries.length > 0 ? (
                      <div className="flex flex-wrap gap-2">
                        <StatusFilterButton
                          active={statusFilter === "all"}
                          onClick={() => setStatusFilter("all")}
                        >
                          {t("pages.agent.teammates.filters.status_all")}
                        </StatusFilterButton>
                        {taskStatusEntries.map(([status, count]) => (
                          <StatusFilterButton
                            key={status}
                            active={statusFilter === status}
                            onClick={() => setStatusFilter(status)}
                          >
                            {status}: {count}
                          </StatusFilterButton>
                        ))}
                      </div>
                    ) : null}

                    <Input
                      value={taskFilter}
                      onChange={(event) => setTaskFilter(event.target.value)}
                      placeholder={t(
                        "pages.agent.teammates.filters.search_placeholder",
                      )}
                    />

                    {data.tasks.length === 0 ? (
                      <p className="text-muted-foreground text-sm">
                        {t("pages.agent.teammates.tasks_empty")}
                      </p>
                    ) : (
                      <div className="grid gap-4 xl:grid-cols-[0.9fr_1.1fr]">
                        <div className="space-y-3">
                          {filteredTasks.length === 0 ? (
                            <p className="text-muted-foreground rounded-xl border p-4 text-sm">
                              {t("pages.agent.teammates.filters.no_results")}
                            </p>
                          ) : (
                            filteredTasks.map((task) => {
                              const isSelected = taskKey(task) === selectedTaskKey
                              return (
                                <button
                                  key={taskKey(task)}
                                  type="button"
                                  onClick={() => setSelectedTaskKey(taskKey(task))}
                                  className={cn(
                                    "border-border/60 hover:border-primary/40 hover:bg-muted/40 w-full rounded-xl border p-4 text-left transition-colors",
                                    isSelected && "border-primary/50 bg-muted/60",
                                  )}
                                >
                                  <div className="flex flex-wrap items-center gap-2">
                                    <div className="font-mono text-sm">{task.id}</div>
                                    <TaskStatusBadge status={task.status} />
                                    <Badge variant="secondary">
                                      {task.owner_agent_id}
                                    </Badge>
                                    {task.teammate_id ? (
                                      <Badge variant="outline">
                                        {task.teammate_id}
                                      </Badge>
                                    ) : null}
                                  </div>
                                  {task.label ? (
                                    <p className="mt-2 text-sm font-medium">
                                      {task.label}
                                    </p>
                                  ) : null}
                                  <p className="text-muted-foreground mt-2 line-clamp-3 text-sm whitespace-pre-wrap">
                                    {task.task}
                                  </p>
                                  <div className="text-muted-foreground mt-3 flex flex-wrap gap-3 text-xs">
                                    <span>
                                      {t(
                                        "pages.agent.teammates.task_fields.created",
                                      )}
                                      : {formatTimestamp(task.created)}
                                    </span>
                                    {task.cancelable ? (
                                      <span>
                                        {t(
                                          "pages.agent.teammates.task_actions.cancel",
                                        )}
                                      </span>
                                    ) : null}
                                  </div>
                                </button>
                              )
                            })
                          )}
                        </div>

                        <Card className="bg-muted/20 border-dashed">
                          <CardHeader>
                            <CardTitle>
                              {t("pages.agent.teammates.task_detail_title")}
                            </CardTitle>
                            <CardDescription>
                              {t("pages.agent.teammates.task_detail_description")}
                            </CardDescription>
                          </CardHeader>
                          <CardContent className="space-y-4">
                            {!selectedTask ? (
                              <p className="text-muted-foreground text-sm">
                                {t("pages.agent.teammates.task_detail_empty")}
                              </p>
                            ) : taskDetailQuery.isLoading && !taskDetail ? (
                              <p className="text-muted-foreground text-sm">
                                {t("pages.agent.teammates.task_detail_loading")}
                              </p>
                            ) : taskDetail ? (
                              <>
                                <div className="flex flex-wrap items-center gap-2">
                                  <div className="font-mono text-sm">
                                    {taskDetail.id}
                                  </div>
                                  <TaskStatusBadge status={taskDetail.status} />
                                  <Badge variant="secondary">
                                    {taskDetail.owner_agent_id}
                                  </Badge>
                                  {taskDetail.teammate_id ? (
                                    <Badge variant="outline">
                                      {taskDetail.teammate_id}
                                    </Badge>
                                  ) : null}
                                </div>

                                {taskDetail.label ? (
                                  <p className="text-base font-medium">
                                    {taskDetail.label}
                                  </p>
                                ) : null}

                                <p className="text-sm whitespace-pre-wrap">
                                  {taskDetail.task}
                                </p>

                                <div className="flex flex-wrap gap-2">
                                  <Button
                                    variant="destructive"
                                    size="sm"
                                    disabled={
                                      !taskDetail.cancelable ||
                                      cancelMutation.isPending
                                    }
                                    onClick={() =>
                                      cancelMutation.mutate({
                                        ownerAgentID: taskDetail.owner_agent_id,
                                        taskID: taskDetail.id,
                                      })
                                    }
                                  >
                                    {cancelMutation.isPending
                                      ? t(
                                          "pages.agent.teammates.task_actions.canceling",
                                        )
                                      : t(
                                          "pages.agent.teammates.task_actions.cancel",
                                        )}
                                  </Button>
                                </div>

                                <dl className="text-muted-foreground grid gap-3 text-sm sm:grid-cols-2">
                                  <RuntimeField
                                    label={t(
                                      "pages.agent.teammates.task_fields.requester",
                                    )}
                                    value={
                                      taskDetail.requester_teammate_id ||
                                      taskDetail.requester_agent_id
                                    }
                                  />
                                  <RuntimeField
                                    label={t(
                                      "pages.agent.teammates.task_fields.agent",
                                    )}
                                    value={taskDetail.agent_id}
                                  />
                                  <RuntimeField
                                    label={t(
                                      "pages.agent.teammates.task_fields.memory",
                                    )}
                                    value={taskDetail.memory_scope}
                                  />
                                  <RuntimeField
                                    label={t(
                                      "pages.agent.teammates.task_fields.status",
                                    )}
                                    value={taskDetail.status}
                                  />
                                  <RuntimeField
                                    label={t(
                                      "pages.agent.teammates.task_fields.channel",
                                    )}
                                    value={taskDetail.origin_channel}
                                  />
                                  <RuntimeField
                                    label={t(
                                      "pages.agent.teammates.task_fields.chat",
                                    )}
                                    value={taskDetail.origin_chat_id}
                                  />
                                  <RuntimeField
                                    label={t(
                                      "pages.agent.teammates.task_fields.kind",
                                    )}
                                    value={taskDetail.kind}
                                  />
                                  <RuntimeField
                                    label={t(
                                      "pages.agent.teammates.task_fields.created",
                                    )}
                                    value={formatTimestamp(taskDetail.created)}
                                  />
                                  <RuntimeField
                                    label={t(
                                      "pages.agent.teammates.task_fields.started",
                                    )}
                                    value={formatTimestamp(taskDetail.started)}
                                  />
                                  <RuntimeField
                                    label={t(
                                      "pages.agent.teammates.task_fields.completed",
                                    )}
                                    value={formatTimestamp(taskDetail.completed)}
                                  />
                                </dl>

                                {taskDetail.workspace_scope?.length ? (
                                  <RuntimeList
                                    label={t(
                                      "pages.agent.teammates.task_fields.workspaces",
                                    )}
                                    items={taskDetail.workspace_scope}
                                  />
                                ) : null}

                                <div className="space-y-2">
                                  <div className="text-xs uppercase opacity-70">
                                    {t("pages.agent.teammates.task_fields.result")}
                                  </div>
                                  <pre className="bg-background overflow-x-auto rounded-xl border p-3 text-xs whitespace-pre-wrap">
                                    {taskDetail.result?.trim() || "—"}
                                  </pre>
                                </div>
                              </>
                            ) : null}
                          </CardContent>
                        </Card>
                      </div>
                    )}
                  </CardContent>
                </Card>
              </section>
            </>
          )}
        </div>
      </div>
    </div>
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
      : normalized === "failed" || normalized === "canceled"
        ? "destructive"
        : normalized === "canceling"
          ? "default"
          : "outline"
  return <Badge variant={variant}>{status}</Badge>
}

function RuntimeLoadingState() {
  return (
    <div className="space-y-6">
      <div className="grid gap-4 md:grid-cols-3">
        {[1, 2, 3].map((item) => (
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
        {[1, 2].map((item) => (
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

function formatTimestamp(value?: number) {
  if (!value) {
    return ""
  }
  return new Date(value).toLocaleString()
}
