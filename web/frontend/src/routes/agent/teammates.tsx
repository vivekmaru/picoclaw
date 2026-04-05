import { createFileRoute } from "@tanstack/react-router"

import { TeammatesPage } from "@/components/agent/teammates/teammates-page"

export const Route = createFileRoute("/agent/teammates")({
  component: AgentTeammatesRoute,
})

function AgentTeammatesRoute() {
  return <TeammatesPage />
}
