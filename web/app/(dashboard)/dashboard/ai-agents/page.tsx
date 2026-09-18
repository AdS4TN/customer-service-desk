"use client"

import { useRouter } from "next/navigation"
import { BotMessageSquareIcon } from "lucide-react"
import { DashboardCrudPage } from "@/components/dashboard/crud"
import { Button } from "@/components/ui/button"
import { Avatar, AvatarFallback, AvatarImage } from "@/components/ui/avatar"
import { Badge } from "@/components/ui/badge"
import { useI18n } from "@/i18n/provider"
import { createAIAgent, updateAIAgent, deleteAIAgent, fetchAIAgents, type AIAgent, type CreateAIAgentPayload } from "@/lib/api/admin"
import { IMConversationServiceMode, Status } from "@/lib/generated/enums"

export default function DashboardAIAgentsPage() {
  const t = useI18n()
  const router = useRouter()
  const open = (id?: number) => router.push("/dashboard/ai-agents/editor" + (id ? `?id=${id}` : ""))
  return <DashboardCrudPage<AIAgent, CreateAIAgentPayload>
    filters={[{ name: "name", label: t("aiAgent.filterName"), placeholder: t("aiAgent.filterName"), defaultValue: "", trim: true }]}
    columns={[
      { key: "employee", label: t("employee.title"), render: (item) => <div className="flex items-center gap-3"><Avatar><AvatarImage src={item.avatar} /><AvatarFallback><BotMessageSquareIcon className="size-4" /></AvatarFallback></Avatar><div className="min-w-0"><Button variant="link" onClick={() => open(item.id)}>{item.name}</Button>{item.description ? <p className="max-w-md line-clamp-2 text-sm text-muted-foreground">{item.description}</p> : null}</div></div> },
      { key: "resources", label: t("employee.knowledge"), render: (item) => <div className="flex flex-wrap gap-1">{item.skills?.map((skill) => <Badge variant="secondary" key={skill.id}>{skill.name}</Badge>)}<span className="text-sm text-muted-foreground">{t("aiReception.resources", { knowledge: item.knowledgeBaseIds?.length || 0, skills: item.skillIds?.length || 0 })}</span></div> },
      { key: "reception", label: t("employee.assignment"), render: (item) => <span className="text-sm">{t(item.status !== Status.Ok || item.serviceMode === IMConversationServiceMode.HumanOnly ? "employee.paused" : "employee.ready")}</span> },
      { key: "publication", label: t("aiAgent.publishStatus"), render: (item) => <Badge variant="outline">{t(item.publishedRevisionId > 0 ? "employee.published" : "employee.draft")}</Badge> },
    ]}
    fetchList={(query) => fetchAIAgents({ name: String(query.name || ""), page: Number(query.page), limit: Number(query.limit) })}
    getItemId={(item) => item.id}
    createItem={createAIAgent} updateItem={(item, payload) => updateAIAgent({ id: item.id, ...payload })}
    onCreateItem={() => open()} onEditItem={(item) => open(item.id)}
    deleteItem={(item) => deleteAIAgent(item.id)}
    labels={{
      refresh: t("aiAgent.refresh"), create: t("employee.new"), query: t("aiAgent.query"), loading: t("aiAgent.loadingRows"), empty: t("aiAgent.emptyRows"),
      actions: t("aiAgent.columnActions"), edit: t("employee.open"), delete: t("employee.delete"), processing: t("aiAgent.processing"),
      moreActions: (item) => t("aiAgent.moreActions", { name: item.name }), loadFailed: t("aiAgent.loadFailed"), saveFailed: t("aiAgent.saveFailed"), deleteFailed: t("aiAgent.deleteFailed"),
      created: (payload) => t("aiAgent.created", { name: payload.name }), updated: (_item, payload) => t("aiAgent.updated", { name: payload.name }), deleted: (item) => t("aiAgent.deleted", { name: item.name }),
    }}
  />
}
