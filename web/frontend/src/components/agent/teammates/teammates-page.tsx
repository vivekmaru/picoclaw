import { useQuery } from "@tanstack/react-query"
import { useMemo } from "react"
import { useTranslation } from "react-i18next"

import { getAgentRuntime } from "@/api/agent-runtime"
import { PageHeader } from "@/components/page-header"
import { Badge } from "@/components/ui/badge"
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card"
import { Skeleton } from "@/components/ui/skeleton"

const RUNTIME_POLL_MS = 3000

export function TeammatesPage() {
  const { t } = useTranslation()
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

              <section className="grid gap-6 xl:grid-cols-[1.1fr_0.9fr]">
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
                    <CardTitle>
                      {t("pages.agent.teammates.task_title")}
                    </CardTitle>
                    <CardDescription>
                      {t("pages.agent.teammates.task_description")}
                    </CardDescription>
                  </CardHeader>
                  <CardContent className="space-y-4">
                    {taskStatusEntries.length > 0 ? (
                      <div className="flex flex-wrap gap-2">
                        {taskStatusEntries.map(([status, count]) => (
                          <Badge key={status} variant="outline">
                            {status}: {count}
                          </Badge>
                        ))}
                      </div>
                    ) : null}

                    {data.tasks.length === 0 ? (
                      <p className="text-muted-foreground text-sm">
                        {t("pages.agent.teammates.tasks_empty")}
                      </p>
                    ) : (
                      data.tasks.map((task) => (
                        <div
                          key={`${task.owner_agent_id}:${task.id}`}
                          className="border-border/60 space-y-2 rounded-xl border p-4"
                        >
                          <div className="flex flex-wrap items-center gap-2">
                            <div className="font-mono text-sm">{task.id}</div>
                            <TaskStatusBadge status={task.status} />
                            <Badge variant="secondary">
                              {task.owner_agent_id}
                            </Badge>
                            {task.teammate_id ? (
                              <Badge variant="outline">{task.teammate_id}</Badge>
                            ) : null}
                          </div>
                          {task.label ? (
                            <p className="text-sm font-medium">{task.label}</p>
                          ) : null}
                          <p className="text-muted-foreground text-sm whitespace-pre-wrap">
                            {task.task}
                          </p>
                          <dl className="text-muted-foreground grid gap-2 text-sm sm:grid-cols-2">
                            <RuntimeField
                              label={t("pages.agent.teammates.task_fields.requester")}
                              value={task.requester_teammate_id || task.requester_agent_id}
                            />
                            <RuntimeField
                              label={t("pages.agent.teammates.task_fields.memory")}
                              value={task.memory_scope}
                            />
                            <RuntimeField
                              label={t("pages.agent.teammates.task_fields.created")}
                              value={formatTimestamp(task.created)}
                            />
                            <RuntimeField
                              label={t("pages.agent.teammates.task_fields.completed")}
                              value={formatTimestamp(task.completed)}
                            />
                          </dl>
                        </div>
                      ))
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

function TaskStatusBadge({ status }: { status: string }) {
  const normalized = status.toLowerCase()
  const variant =
    normalized === "completed"
      ? "secondary"
      : normalized === "failed" || normalized === "canceled"
        ? "destructive"
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

function formatTimestamp(value?: number) {
  if (!value) {
    return ""
  }
  return new Date(value).toLocaleString()
}
