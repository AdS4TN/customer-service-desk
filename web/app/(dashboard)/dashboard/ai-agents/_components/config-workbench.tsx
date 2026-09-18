"use client"

import { useCallback, useEffect, useMemo, useState, type ReactNode } from "react"
import {
  HistoryIcon,
  MessageSquareTextIcon,
  PlugIcon,
  SaveIcon,
  BookOpenIcon,
  Trash2Icon,
  UserRoundCheckIcon,
  ArrowLeftIcon,
  ExternalLinkIcon,
} from "lucide-react"
import { toast } from "sonner"

import { useConfirm } from "@/components/confirm-provider"
import { EmployeePreview } from "./employee-preview"
import { EmployeeResources } from "./employee-resources"
import { Tabs, TabsList, TabsTrigger } from "@/components/ui/tabs"
import { ReceptionState } from "./reception-state"
import { ReceptionPolicyEditor } from "./reception-policy"
import { emptyReceptionPolicy, receptionPolicyError, type ReceptionPolicy } from "@/lib/reception"
import { ImageInput } from "@/components/image-input"
import { OptionCombobox } from "@/components/option-combobox"
import { ProjectDialog } from "@/components/project-dialog"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table"
import { Textarea } from "@/components/ui/textarea"
import { useI18n } from "@/i18n/provider"
import {
  createAIAgent,
  fetchAIAgent,
  fetchAIAgentRevisions,
  fetchAIConfigsAll,
  fetchAIWorkflows,
  fetchAgentTeamsAll,
  fetchKnowledgeBasesAll,
  fetchMCPCatalog,
  fetchSkillDefinitionsAll,
  publishAIAgent,
  rollbackAIAgent,
  updateAIAgent,
  type AIAgent,
  type AIAgentWorkflowBindingInput,
  type AgentRevision,
  type AIConfig,
  type AIWorkflow,
  type AdminAgentTeam,
  type CreateAIAgentPayload,
  type KnowledgeBase,
  type MCPToolCatalogItem,
  type SkillDefinition,
} from "@/lib/api/admin"
import {
  AIAgentFallbackMode,
  AIAgentHandoffMode,
  AIModelType,
  IMConversationServiceMode,
  Status,
} from "@/lib/generated/enums"
import { cn } from "@/lib/utils"

type SectionKey = "persona" | "knowledge" | "capability" | "service"
type MCPToolItem = CreateAIAgentPayload["mcpTools"][number]

type MCPToolOption = {
  value: string
  label: string
  group: string
  subtitle: string
  description?: string
  meta: MCPToolItem
}

function normalizeMCPToolsWithCatalog(
  tools: MCPToolItem[],
  catalog: MCPToolCatalogItem[],
) {
  return tools.map((tool) => {
    const catalogTool = catalog.find((item) => item.toolCode === tool.toolCode)
    if (!catalogTool || catalogTool.riskEditable) return tool
    return {
      ...tool,
      title: catalogTool.title || tool.title,
      description: catalogTool.description || tool.description,
      riskLevel: catalogTool.riskLevel,
      requireConfirmation: catalogTool.requireConfirmation,
    }
  })
}

function toText(value: string | number | undefined | null) {
  if (value === undefined || value === null || value === 0) return ""
  return String(value)
}

function uniqueNumbers(input: number[]) {
  return Array.from(new Set(input.filter((id) => Number.isFinite(id) && id > 0)))
}

export function AIAgentConfigWorkbench({
  agentId,
  onAgentSaved,
  onAgentCreated,
  onCancel,
  onPolicyStateChange,
}: {
  agentId?: number | null
  onAgentSaved?: () => void
  onAgentCreated?: (agent: AIAgent) => void
  onCancel?: () => void
  onPolicyStateChange?: (state: { dirty: boolean; saving: boolean }) => void
}) {
  const t = useI18n()
  const confirm = useConfirm()
  const [receptionPolicy, setReceptionPolicy] = useState<ReceptionPolicy>(emptyReceptionPolicy)
  const [formBaseline, setFormBaseline] = useState<string | null>(null)
  const [refreshKey, setRefreshKey] = useState(0)
  const [loadError, setLoadError] = useState(false)
  const [publicationRefreshFailed, setPublicationRefreshFailed] = useState(false)
  const [refreshingPublication, setRefreshingPublication] = useState(false)
  const [policyError, setPolicyError] = useState("")
  const [currentAgentId, setCurrentAgentId] = useState(agentId ?? null)
  const [activeSection, setActiveSection] = useState<SectionKey>("persona")
  const [mobileView, setMobileView] = useState("configure")
  const [agent, setAgent] = useState<AIAgent | null>(null)
  const [agentRevisions, setAgentRevisions] = useState<AgentRevision[]>([])
  const [loading, setLoading] = useState(true)
  const [saving, setSaving] = useState(false)
  const [versionDialogOpen, setVersionDialogOpen] = useState(false)

  const [name, setName] = useState("")
  const [displayName, setDisplayName] = useState("")
  const [avatar, setAvatar] = useState("")
  const [statusText, setStatusText] = useState("")
  const [description, setDescription] = useState("")
  const [aiConfigId, setAIConfigId] = useState("")
  const [translationAIConfigId, setTranslationAIConfigId] = useState("0")
  const [serviceMode, setServiceMode] = useState(String(IMConversationServiceMode.HumanOnly))
  const [systemPrompt, setSystemPrompt] = useState("")
  const [welcomeMessage, setWelcomeMessage] = useState("")
  const [replyTimeoutSeconds, setReplyTimeoutSeconds] = useState("180")
  const [handoffMode, setHandoffMode] = useState(String(AIAgentHandoffMode.WaitPool))
  const [fallbackMode, setFallbackMode] = useState(String(AIAgentFallbackMode.NoAnswer))
  const [fallbackMessage, setFallbackMessage] = useState("")
  const [selectedTeamIds, setSelectedTeamIds] = useState<number[]>([])
  const [selectedSkillIds, setSelectedSkillIds] = useState<number[]>([])
  const [selectedKnowledgeBaseIds, setSelectedKnowledgeBaseIds] = useState<number[]>([])
  const [mcpTools, setMCPTools] = useState<MCPToolItem[]>([])
  const [workflowBindings, setWorkflowBindings] = useState<AIAgentWorkflowBindingInput[]>([])

  const [aiConfigs, setAIConfigs] = useState<AIConfig[]>([])
  const [translationAIConfigs, setTranslationAIConfigs] = useState<AIConfig[]>([])
  const [agentTeams, setAgentTeams] = useState<AdminAgentTeam[]>([])
  const [skills, setSkills] = useState<SkillDefinition[]>([])
  const [knowledgeBases, setKnowledgeBases] = useState<KnowledgeBase[]>([])
  const [toolCatalog, setToolCatalog] = useState<MCPToolCatalogItem[]>([])
  const [publishedWorkflows, setPublishedWorkflows] = useState<AIWorkflow[]>([])

  const [teamToAdd, setTeamToAdd] = useState("")

  useEffect(() => {
    setCurrentAgentId(agentId ?? null)
  }, [agentId])

  const loadData = useCallback(async () => {
    setLoading(true)
    setLoadError(false)
    setFormBaseline(null)
    try {
      const [configsRes, translationConfigsRes, teamsRes, skillListRes, knowledgeBaseListRes, catalogRes, workflowPageRes] =
        await Promise.allSettled([
          fetchAIConfigsAll({ modelType: AIModelType.LLM }),
          fetchAIConfigsAll({ modelType: AIModelType.Translation }),
          fetchAgentTeamsAll(),
          fetchSkillDefinitionsAll({ status: Status.Ok }),
          fetchKnowledgeBasesAll({ status: Status.Ok }),
          fetchMCPCatalog(),
          fetchAIWorkflows({ limit: 100 }),
        ])

      if ([configsRes, translationConfigsRes, teamsRes, skillListRes, knowledgeBaseListRes, catalogRes, workflowPageRes].some((result) => result.status === "rejected")) {
        throw new Error(t("aiReception.dependenciesFailed"))
      }

      const configs = configsRes.status === "fulfilled" ? configsRes.value : []
      const translationConfigs = translationConfigsRes.status === "fulfilled" ? translationConfigsRes.value : []
      const teams = teamsRes.status === "fulfilled" ? teamsRes.value : []
      const skillList = skillListRes.status === "fulfilled" ? skillListRes.value : []
      const knowledgeBaseList = knowledgeBaseListRes.status === "fulfilled" ? knowledgeBaseListRes.value : []
      const catalog = catalogRes.status === "fulfilled" ? catalogRes.value : []
      const workflowPage = workflowPageRes.status === "fulfilled" ? workflowPageRes.value : { results: [] }

      setAIConfigs(configs ?? [])
      setTranslationAIConfigs(translationConfigs ?? [])
      setAgentTeams(teams ?? [])
      setSkills(skillList ?? [])
      setKnowledgeBases(knowledgeBaseList ?? [])
      setToolCatalog(catalog ?? [])
      setPublishedWorkflows(
        (workflowPage.results ?? []).filter((item) => item.publishedVersionId > 0),
      )

      if (!currentAgentId || currentAgentId <= 0) {
        setAgent(null)
        setAgentRevisions([])
        setName("")
        setDisplayName("")
        setAvatar("")
        setStatusText("")
        setDescription("")
        setAIConfigId(configs && configs.length > 0 ? String(configs[0].id) : "")
        setTranslationAIConfigId("0")
        setServiceMode(String(IMConversationServiceMode.HumanOnly))
        setSystemPrompt("")
        setReceptionPolicy(emptyReceptionPolicy())
        setPolicyError("")
        setWelcomeMessage("")
        setReplyTimeoutSeconds("180")
        setHandoffMode(String(AIAgentHandoffMode.WaitPool))
        setFallbackMode(String(AIAgentFallbackMode.NoAnswer))
        setFallbackMessage("")
        setSelectedTeamIds([])
        setSelectedSkillIds([])
        setSelectedKnowledgeBaseIds([])
        setMCPTools([])
        setWorkflowBindings([])
        return
      }

      const [detail, revisions] = await Promise.all([
        fetchAIAgent(currentAgentId),
        fetchAIAgentRevisions(currentAgentId),
      ])
      setAgent(detail)
      setAgentRevisions(revisions ?? [])
      setName(detail.name)
      setDisplayName(detail.displayName || "")
      setAvatar(detail.avatar || "")
      setStatusText(detail.statusText || "")
      setDescription(detail.description || "")
      setAIConfigId(toText(detail.aiConfigId))
      setTranslationAIConfigId(detail.translationAiConfigId ? String(detail.translationAiConfigId) : "0")
      setServiceMode(String(detail.serviceMode || IMConversationServiceMode.AIFirst))
      setSystemPrompt(detail.systemPrompt || "")
      setReceptionPolicy(detail.receptionPolicy ?? emptyReceptionPolicy())
      setPolicyError("")
      setWelcomeMessage(detail.welcomeMessage || "")
      setReplyTimeoutSeconds(String(detail.replyTimeoutSeconds ?? 180))
      setHandoffMode(String(detail.handoffMode || AIAgentHandoffMode.WaitPool))
      setFallbackMode(String(detail.fallbackMode || AIAgentFallbackMode.NoAnswer))
      setFallbackMessage(detail.fallbackMessage || "")
      setSelectedTeamIds((detail.teams ?? []).map((team) => team.id))
      setSelectedSkillIds(detail.skillIds ?? [])
      setSelectedKnowledgeBaseIds(detail.knowledgeBaseIds ?? [])
      setMCPTools(normalizeMCPToolsWithCatalog(detail.mcpTools ?? [], catalog ?? []))
      setWorkflowBindings(
        (detail.workflowBindings ?? []).map(
          ({ workflowVersionId, toolName, triggerInstruction, priority, enabled }) => ({
            workflowVersionId,
            toolName,
            triggerInstruction,
            priority,
            enabled,
          }),
        ),
      )
    } catch (error) {
      setLoadError(true)
      toast.error(error instanceof Error ? error.message : t("aiAgent.loadDetailFailed"))
    } finally {
      setLoading(false)
    }
  }, [currentAgentId, t])

  useEffect(() => {
    void loadData()
  }, [loadData])

  async function refreshAgentPublicationState(id: number) {
    setRefreshingPublication(true)
    try {
      const [detail, revisions] = await Promise.all([
        fetchAIAgent(id),
        fetchAIAgentRevisions(id),
      ])
      setAgent(detail)
      setAgentRevisions(revisions ?? [])
      setPublicationRefreshFailed(false)
    } catch {
      setPublicationRefreshFailed(true)
    } finally {
      setRefreshingPublication(false)
    }
  }

  const handoffModeOptions = useMemo(
    () => [
      { value: String(AIAgentHandoffMode.WaitPool), label: t("aiAgent.handoffWaitPool") },
      {
        value: String(AIAgentHandoffMode.DefaultTeamPool),
        label: t("aiAgent.handoffDefaultTeamPool"),
      },
      {
        value: String(AIAgentHandoffMode.AIHoldAndNotify),
        label: t("aiAgent.handoffAiHoldAndNotify"),
      },
    ],
    [t],
  )
  const fallbackModeOptions = useMemo(
    () => [
      { value: String(AIAgentFallbackMode.NoAnswer), label: t("aiAgent.fallbackNoAnswer") },
      { value: String(AIAgentFallbackMode.SuggestRetry), label: t("aiAgent.fallbackSuggestRetry") },
      { value: String(AIAgentFallbackMode.Handoff), label: t("aiAgent.sectionHandoff") },
    ],
    [t],
  )
  const aiConfigOptions = useMemo(
    () =>
      aiConfigs.map((item) => ({
        value: String(item.id),
        label: `${item.name} · ${item.modelName}`,
      })),
    [aiConfigs],
  )
  const translationAIConfigOptions = useMemo(
    () => [
      { value: "0", label: t("aiAgent.translationModelSameAsReply") },
      ...translationAIConfigs.map((item) => ({
        value: String(item.id),
        label: `${item.name} · ${item.modelName}`,
      })),
    ],
    [translationAIConfigs, t],
  )
  const teamOptions = useMemo(
    () => agentTeams.map((item) => ({ value: String(item.id), label: item.name })),
    [agentTeams],
  )
  const skillOptions = useMemo(
    () => skills.map((item) => ({ value: String(item.id), label: item.name })),
    [skills],
  )
  const knowledgeBaseOptions = useMemo(
    () => knowledgeBases.map((item) => ({ value: String(item.id), label: item.name })),
    [knowledgeBases],
  )
  const mcpToolOptions = useMemo<MCPToolOption[]>(
    () =>
      toolCatalog
        .filter((tool) => !tool.autoInjected && tool.sourceType === "mcp")
        .map((tool) => ({
          value: tool.toolCode,
          label: tool.title || tool.toolName,
          group: tool.serverCode,
          subtitle: tool.toolCode,
          description: tool.description || undefined,
          meta: {
            toolCode: tool.toolCode,
            serverCode: tool.serverCode,
            toolName: tool.toolName,
            title: tool.title || tool.toolName,
            description: tool.description || "",
            riskLevel: tool.riskLevel,
            requireConfirmation: tool.requireConfirmation,
            arguments: undefined,
          },
        })),
    [toolCatalog],
  )
  const workflowOptions = useMemo(
    () =>
      publishedWorkflows.map((workflow) => ({
        value: String(workflow.publishedVersionId),
        label: workflow.name,
        subtitle: t("aiAgent.fixedVersion", { version: String(workflow.publishedVersionId) }),
      })),
    [publishedWorkflows, t],
  )

  function selectedOptions(ids: number[], options: { value: string; label: string }[]) {
    return ids
      .map((id) => options.find((option) => Number(option.value) === id))
      .filter((option): option is { value: string; label: string } => Boolean(option))
  }

  function addSelected(value: string, current: number[], setNext: (ids: number[]) => void) {
    const id = Number(value)
    if (!Number.isFinite(id) || id <= 0 || current.includes(id)) return
    setNext([...current, id])
  }

  function setMCPToolSelection(values: string[]) {
    setMCPTools(
      values.flatMap((value) => {
        const current = mcpTools.find((item) => item.toolCode === value)
        if (current) return [current]
        const option = mcpToolOptions.find((item) => item.value === value)
        return option ? [option.meta] : []
      }),
    )
  }

  function setWorkflowSelection(values: string[]) {
    const selectedVersionIds = values.map(Number).filter((value) => value > 0)
    setWorkflowBindings(
      selectedVersionIds.flatMap((workflowVersionId, index) => {
        const current = workflowBindings.find(
          (binding) => binding.workflowVersionId === workflowVersionId,
        )
        if (current) {
          return [{ ...current, priority: index + 1, enabled: true }]
        }
        const workflow = publishedWorkflows.find(
          (item) => item.publishedVersionId === workflowVersionId,
        )
        if (!workflow) return []
        return [
          {
            workflowVersionId,
            toolName: workflow.name,
            triggerInstruction: "",
            priority: index + 1,
            enabled: true,
          },
        ]
      }),
    )
  }

  function validateForm() {
    const error = receptionPolicyError(receptionPolicy)
    setPolicyError(error)
    if (error) {
      setActiveSection("persona")
      toast.error(t(`reception.${error}`))
      return false
    }
    if (!name.trim()) {
      setActiveSection("persona")
      toast.error(t("aiAgent.nameRequired"))
      return false
    }
    if (!Number(aiConfigId)) {
      setActiveSection("persona")
      toast.error(t("aiAgent.aiConfigRequired"))
      return false
    }
    return true
  }

  function buildPayload(): CreateAIAgentPayload {
    return {
      name: name.trim(),
      displayName: displayName.trim(),
      avatar: avatar.trim(),
      statusText: statusText.trim(),
      description: description.trim(),
      aiConfigId: Number(aiConfigId),
      translationAiConfigId: Number(translationAIConfigId),
      serviceMode: Number(serviceMode),
      maxSteps: agent?.maxSteps,
      contextWindow: agent?.contextWindow,
      toolPolicy: agent?.toolPolicy,
      knowledgePolicy: agent?.knowledgePolicy,
      systemPrompt: systemPrompt.trim(),
      receptionPolicy,
      welcomeMessage: welcomeMessage.trim(),
      replyTimeoutSeconds: Number(replyTimeoutSeconds),
      rolloutPercent: agent?.rolloutPercent || 100,
      teamIds: uniqueNumbers(selectedTeamIds),
      handoffMode: Number(handoffMode),
      fallbackMode: Number(fallbackMode),
      fallbackMessage: fallbackMessage.trim(),
      knowledgeBaseIds: uniqueNumbers(selectedKnowledgeBaseIds),
      skillIds: uniqueNumbers(selectedSkillIds),
      mcpTools,
      workflowBindings,
    }
  }

  const formValue = JSON.stringify(buildPayload())
  const formDirty = formBaseline !== null && formValue !== formBaseline
  useEffect(() => {
    if (!loading && !loadError && formBaseline === null) setFormBaseline(formValue)
  }, [loading, loadError, formBaseline, formValue])
  useEffect(() => { onPolicyStateChange?.({ dirty: formDirty, saving }) }, [onPolicyStateChange, formDirty, saving])
  useEffect(() => {
    if (!formDirty) return
    const warn = (event: BeforeUnloadEvent) => { event.preventDefault(); event.returnValue = "" }
    window.addEventListener("beforeunload", warn)
    return () => window.removeEventListener("beforeunload", warn)
  }, [formDirty])

  async function persistAgentDraft(payload: CreateAIAgentPayload) {
    if (!agent) return
    await updateAIAgent({ id: agent.id, ...payload, capabilitiesOnly: true })
    // A metadata read can fail after a successful write. Commit local saved
    // state and invalidate runtime independently of revision-history refresh.
    setAgent((current) => current ? { ...current, name: payload.name, serviceMode: payload.serviceMode } : current)
    setFormBaseline(formValue)
    setRefreshKey((value) => value + 1)
    onAgentSaved?.()
  }

  async function saveAgentSettings() {
    if (!validateForm()) return
    setSaving(true)
    try {
      const payload = buildPayload()
      if (agent) {
        await persistAgentDraft(payload)
        toast.success(
          agent.publishedRevisionId > 0
            ? t("aiAgent.savedActiveNote")
            : t("aiAgent.savedNote"),
        )
        await refreshAgentPublicationState(agent.id)
      } else {
        const created = await createAIAgent(payload)
        setCurrentAgentId(created.id)
        setAgent(created)
        toast.success(t("aiAgent.createdNote"))
        onAgentCreated?.(created)
        onAgentSaved?.()
      }
      setFormBaseline(formValue)
    } catch (error) {
      toast.error(error instanceof Error ? error.message : t("aiAgent.saveFailed"))
    } finally {
      setSaving(false)
    }
  }

  async function publishAgent() {
    if (!agent) return
    if (!validateForm()) return
    setSaving(true)
    try {
      const payload = buildPayload()
      await persistAgentDraft(payload)
      await publishAIAgent(agent.id)
      setRefreshKey((value) => value + 1)
      await refreshAgentPublicationState(agent.id)
      setFormBaseline(formValue)
      toast.success(t("aiAgent.publishedSuccess"))
      onAgentSaved?.()
    } catch (error) {
      toast.error(error instanceof Error ? error.message : t("aiAgent.publishFailed"))
    } finally {
      setSaving(false)
    }
  }

  async function rollbackAgentRevision(revisionId: number) {
    if (!agent || revisionId <= 0 || revisionId === agent.publishedRevisionId) return
    if (!await confirm({ title: t("aiReception.rollbackTitle"), description: t("aiReception.rollbackBody"), confirmText: t("aiAgent.rollback") })) return
    setSaving(true)
    try {
      await rollbackAIAgent(agent.id, revisionId)
      setRefreshKey((value) => value + 1)
      toast.success(t("aiAgent.rollbackSuccess"))
      await refreshAgentPublicationState(agent.id)
      onAgentSaved?.()
    } catch (error) {
      toast.error(error instanceof Error ? error.message : t("aiAgent.rollbackFailed"))
    } finally {
      setSaving(false)
    }
  }

  const agentPublished = (agent?.publishedRevisionId ?? 0) > 0

  const sections: {
    key: SectionKey
    title: string
    icon: ReactNode
  }[] = [
    { key: "persona", title: t("employee.persona"), icon: <MessageSquareTextIcon /> },
    { key: "knowledge", title: t("employee.knowledge"), icon: <BookOpenIcon /> },
    { key: "capability", title: t("employee.actions"), icon: <PlugIcon /> },
    { key: "service", title: t("employee.assignment"), icon: <UserRoundCheckIcon /> },
  ]

  if (loading) {
    return (
      <div className="flex h-full items-center justify-center text-sm text-muted-foreground">
        {t("common.loading")}
      </div>
    )
  }

  if (loadError) return <div className="flex h-full flex-col items-center justify-center gap-4 p-6" role="alert">
    <p className="text-sm">{t("aiAgent.loadDetailFailed")}</p>
    <Button type="button" variant="outline" onClick={() => void loadData()}>{t("aiAgent.refresh")}</Button>
  </div>

  return (
    <div className="flex h-full min-h-0 flex-col overflow-hidden bg-background">
      <header className="flex min-h-16 shrink-0 items-center justify-between gap-4 border-b px-4 py-3">
        <Button type="button" variant="ghost" size="icon" title={t("employee.back")} aria-label={t("employee.back")} onClick={onCancel} disabled={saving}><ArrowLeftIcon /></Button>
        <div className="flex min-w-0 flex-1 flex-wrap items-center justify-between gap-3">
          <div className="flex min-w-0 items-center gap-2">
            <h1 className="truncate text-base font-semibold">{name || t("employee.new")}</h1>
            <Badge variant={agentPublished ? "default" : "outline"}>
              {agentPublished ? t("aiAgent.published") : agent ? t("aiAgent.unpublished") : t("aiAgent.notCreated")}
            </Badge>
          </div>
          {agent ? (
            <Button type="button" variant="link" onClick={() => setVersionDialogOpen(true)}>
              <HistoryIcon />
              {t("employee.history")}
            </Button>
          ) : null}
        </div>
      </header>
      {publicationRefreshFailed && agent ? <div role="alert" className="flex shrink-0 flex-wrap items-center justify-between gap-2 border-b px-5 py-2">
        <p className="min-w-0 flex-1 text-sm">{t("aiReception.metadataRefreshFailed")}</p>
        <Button type="button" variant="outline" size="sm" disabled={refreshingPublication || saving} onClick={() => void refreshAgentPublicationState(agent.id)}>{t("aiReception.retryMetadata")}</Button>
      </div> : null}
      <div className="flex shrink-0 flex-wrap items-center justify-between gap-3 border-b px-4 py-2">
        <div className="hidden min-w-0 overflow-x-auto md:block">
          <Tabs value={activeSection} onValueChange={(value) => setActiveSection(value as SectionKey)}>
            <TabsList variant="line">{sections.map((section) => <TabsTrigger key={section.key} value={section.key}>{section.icon}{section.title}</TabsTrigger>)}</TabsList>
          </Tabs>
        </div>
        <div className="min-w-0 flex-1 md:hidden"><OptionCombobox value={activeSection} placeholder={t("employee.configure")} options={sections.map((section) => ({ value: section.key, label: section.title }))} onChange={(value) => { setActiveSection(value as SectionKey); setMobileView("configure") }} /></div>
        <div className="xl:hidden"><Tabs value={mobileView} onValueChange={(value) => setMobileView(String(value))}><TabsList><TabsTrigger value="configure">{t("employee.configure")}</TabsTrigger><TabsTrigger value="trial">{t("employee.trial")}</TabsTrigger></TabsList></Tabs></div>
      </div>
      <div className="flex min-h-0 flex-1">
        <div className={cn("min-w-0 flex-1 overflow-y-auto bg-background xl:block", mobileView === "trial" && "hidden")}>
          <fieldset disabled={saving} className="mx-auto flex min-w-0 w-full max-w-4xl flex-col gap-8 p-4 lg:p-6">
            {activeSection === "persona" ? <div className="flex flex-col gap-6">
              <div className="grid grid-cols-[5rem_minmax(0,1fr)] items-start gap-4">
                <ImageInput value={avatar} onChange={setAvatar} prefix="agent-avatars" accept="image/png,image/jpeg,image/webp" className="size-20 rounded-md" placeholder={t("aiAgent.uploadAvatar")} />
                <FieldBlock label={t("aiAgent.name")} required><Input aria-label={t("aiAgent.name")} value={name} onChange={(event) => setName(event.target.value)} /></FieldBlock>
              </div>
              <FieldBlock label={t("employee.persona")}>
                <Textarea className="min-h-64" aria-label={t("employee.persona")} rows={12} value={systemPrompt} onChange={(event) => setSystemPrompt(event.target.value)} />
              </FieldBlock>
              <details><summary className="cursor-pointer text-sm font-medium">{t("reception.title")}</summary><div className="pt-4"><ReceptionPolicyEditor value={receptionPolicy} disabled={saving} error={policyError} onChange={(value) => { setReceptionPolicy(value); setPolicyError("") }} /></div></details>
              <details><summary className="cursor-pointer text-sm font-medium">{t("aiAgent.publicIdentity")}</summary>
                <div className="flex flex-col gap-4 pt-4">
                  <FieldBlock label={t("aiAgent.displayName")}><Input aria-label={t("aiAgent.displayName")} value={displayName} placeholder={name} onChange={(event) => setDisplayName(event.target.value)} /></FieldBlock>
                  <FieldBlock label={t("aiAgent.statusText")}><Input aria-label={t("aiAgent.statusText")} value={statusText} onChange={(event) => setStatusText(event.target.value)} /></FieldBlock>
                  <FieldBlock label={t("aiAgent.description")}><Textarea aria-label={t("aiAgent.description")} rows={2} value={description} onChange={(event) => setDescription(event.target.value)} /></FieldBlock>
                  <FieldBlock label={t("aiAgent.sectionGreeting")}><Textarea aria-label={t("aiAgent.sectionGreeting")} rows={3} value={welcomeMessage} onChange={(event) => setWelcomeMessage(event.target.value)} /></FieldBlock>
                </div>
              </details>
              <details open={!aiConfigId}><summary className="cursor-pointer text-sm font-medium">{t("employee.advanced")}</summary>
                <div className="flex flex-col gap-4 pt-4">
                  <FieldBlock label={t("aiAgent.replyModel")} required><OptionCombobox value={aiConfigId} options={aiConfigOptions} placeholder={t("aiAgent.selectAiConfig")} searchPlaceholder={t("aiAgent.searchAiConfig")} emptyText={t("aiAgent.emptyAiConfig")} onChange={setAIConfigId} /></FieldBlock>
                  <FieldBlock label={t("aiAgent.translationModel")} description={t("aiAgent.translationModelDescription")}><OptionCombobox value={translationAIConfigId} options={translationAIConfigOptions} placeholder={t("aiAgent.translationModelSameAsReply")} searchPlaceholder={t("aiAgent.searchAiConfig")} emptyText={t("aiAgent.emptyAiConfig")} onChange={setTranslationAIConfigId} /></FieldBlock>
                  <FieldBlock label={t("aiAgent.replyTimeout")}><Input aria-label={t("aiAgent.replyTimeout")} type="number" min={0} step={1} value={replyTimeoutSeconds} onChange={(event) => setReplyTimeoutSeconds(event.target.value)} /></FieldBlock>
                </div>
              </details>
            </div> : null}

            {activeSection === "knowledge" ? (
              <div className="flex flex-col gap-8">
                <FormSection
                  title={t("aiAgent.sectionKnowledge")}
                  description={t("aiAgent.knowledgeDescription")}
                >
                  <OptionCombobox
                    multiple
                    values={selectedKnowledgeBaseIds.map(String)}
                    options={knowledgeBaseOptions}
                    placeholder={t("aiAgent.selectKnowledge")}
                    emptyText={t("aiAgent.emptyKnowledge")}
                    onValuesChange={(values) =>
                      setSelectedKnowledgeBaseIds(values.map(Number))
                    }
                  />
                </FormSection>

                <FormSection
                  title={t("aiAgent.sectionSkill")}
                  description={t("aiAgent.skillDescription")}
                >
                  <OptionCombobox
                    multiple
                    values={selectedSkillIds.map(String)}
                    options={skillOptions}
                    placeholder={t("aiAgent.selectSkill")}
                    emptyText={t("aiAgent.emptySkill")}
                    onValuesChange={(values) => setSelectedSkillIds(values.map(Number))}
                  />
                </FormSection>

                <EmployeeResources knowledgeIds={selectedKnowledgeBaseIds} skillIds={selectedSkillIds} knowledgeBases={knowledgeBases} skills={skills}
                  onResourcesChange={(bases, definitions) => { setKnowledgeBases(bases); setSkills(definitions) }}
                  onSkillAdded={(skill) => { setSkills((items) => [...items.filter((item) => item.id !== skill.id), skill]); setSelectedSkillIds((ids) => uniqueNumbers([...ids, skill.id])) }} />
              </div>
            ) : null}

            {activeSection === "capability" ? (
              <div className="flex flex-col gap-10">
                <FormSection
                  title={t("aiAgent.sectionWorkflow")}
                  description={t("aiAgent.workflowDescription")}
                >
                  <OptionCombobox
                    multiple
                    values={workflowBindings.map((binding) =>
                      String(binding.workflowVersionId),
                    )}
                    options={workflowOptions}
                    placeholder={t("aiAgent.selectWorkflow")}
                    emptyText={t("aiAgent.emptyWorkflow")}
                    onValuesChange={setWorkflowSelection}
                  />
                </FormSection>

                <FormSection
                  title={t("aiAgent.sectionMcp")}
                  description={t("aiAgent.mcpDescription")}
                >
                  <OptionCombobox
                    multiple
                    values={mcpTools.map((tool) => tool.toolCode)}
                    options={mcpToolOptions}
                    placeholder={t("aiAgent.selectMCPTool")}
                    emptyText={t("aiAgent.emptyMCPTool")}
                    onValuesChange={setMCPToolSelection}
                  />
                  {mcpTools.length > 0 ? (
                    <div className="space-y-2">
                      {mcpTools.map((tool) => {
                        const catalogTool = toolCatalog.find(
                          (item) => item.toolCode === tool.toolCode,
                        )
                        return (
                          <div
                            key={tool.toolCode}
                            className="flex flex-wrap items-center justify-between gap-3 rounded-lg border px-3 py-2"
                          >
                            <div className="min-w-0">
                              <div className="truncate text-sm font-medium">
                                {tool.title || tool.toolCode}
                              </div>
                              <div className="truncate font-mono text-xs text-muted-foreground">
                                {tool.toolCode}
                              </div>
                            </div>
                            <div className="flex items-center gap-2">
                              {catalogTool && !catalogTool.riskEditable ? (
                                <Badge variant="secondary">
                                  {tool.riskLevel === "read"
                                    ? `${t("aiAgent.readOnly")} (${t("aiAgent.systemDefined")})`
                                    : `${t("aiAgent.writeAction")} (${t("aiAgent.systemDefined")})`}
                                </Badge>
                              ) : (
                                <>
                                  <Button
                                    type="button"
                                    size="sm"
                                    variant={
                                      tool.riskLevel === "read" ? "default" : "outline"
                                    }
                                    onClick={() =>
                                      setMCPTools((items) =>
                                        items.map((item) =>
                                          item.toolCode === tool.toolCode
                                            ? {
                                                ...item,
                                                riskLevel: "read",
                                                requireConfirmation: false,
                                              }
                                            : item,
                                        ),
                                      )
                                    }
                                  >
                                    {t("aiAgent.readOnly")}
                                  </Button>
                                  <Button
                                    type="button"
                                    size="sm"
                                    variant={
                                      tool.riskLevel === "write"
                                        ? "destructive"
                                        : "outline"
                                    }
                                    onClick={() =>
                                      setMCPTools((items) =>
                                        items.map((item) =>
                                          item.toolCode === tool.toolCode
                                            ? {
                                                ...item,
                                                riskLevel: "write",
                                                requireConfirmation: true,
                                              }
                                            : item,
                                        ),
                                      )
                                    }
                                  >
                                    {t("aiAgent.writeAction")}
                                  </Button>
                                </>
                              )}
                            </div>
                          </div>
                        )
                      })}
                    </div>
                  ) : null}
                </FormSection>
              </div>
            ) : null}

            {activeSection === "service" ? (
              <div className="flex flex-col gap-8">
                {agent ? <ReceptionState agentId={agent.id} refreshKey={refreshKey} /> : <p className="text-sm text-muted-foreground">{t("aiReception.createFirst")}</p>}
                <p className="text-sm text-muted-foreground">{t("employee.rulesHint")}</p>
                <Button variant="outline" render={<a href="/dashboard/automation" target="_blank" rel="noreferrer" />}><ExternalLinkIcon data-icon="inline-start" />{t("employee.openRules")}</Button>
                <details><summary className="cursor-pointer text-sm font-medium">{t("employee.fallback")}</summary><div className="flex flex-col gap-6 pt-4">
                <FormSection
                  title={t("aiAgent.sectionHandoff")}
                  description={t("aiAgent.handoffDescription")}
                >
                  <div className="grid gap-5 md:grid-cols-2">
                    <FieldBlock label={t("aiAgent.handoffMode")}>
                      <OptionCombobox
                        value={handoffMode}
                        options={handoffModeOptions}
                        placeholder={t("aiAgent.selectHandoffMode")}
                        onChange={setHandoffMode}
                      />
                    </FieldBlock>
                    <FieldBlock label={t("aiAgent.teams")}>
                      <div className="flex gap-2">
                        <div className="min-w-0 flex-1">
                          <OptionCombobox
                            value={teamToAdd}
                            options={teamOptions.filter(
                              (option) =>
                                !selectedTeamIds.includes(Number(option.value)),
                            )}
                            placeholder={t("aiAgent.selectTeam")}
                            onChange={setTeamToAdd}
                          />
                        </div>
                        <Button
                          type="button"
                          variant="outline"
                          disabled={!teamToAdd}
                          onClick={() => {
                            addSelected(teamToAdd, selectedTeamIds, setSelectedTeamIds)
                            setTeamToAdd("")
                          }}
                        >
                          {t("aiAgent.add")}
                        </Button>
                      </div>
                    </FieldBlock>
                  </div>
                  <BadgeList
                    empty={t("aiAgent.noTeams")}
                    items={selectedOptions(selectedTeamIds, teamOptions)}
                    onRemove={(id) =>
                      setSelectedTeamIds((current) =>
                        current.filter((item) => item !== id),
                      )
                    }
                  />
                </FormSection>

                <FormSection
                  title={t("aiAgent.sectionFallback")}
                  description={t("aiAgent.fallbackDescription")}
                >
                  <FieldBlock label={t("aiAgent.fallbackMode")}>
                    <OptionCombobox
                      value={fallbackMode}
                      options={fallbackModeOptions}
                      placeholder={t("aiAgent.selectFallbackMode")}
                      onChange={setFallbackMode}
                    />
                  </FieldBlock>
                  <FieldBlock label={t("aiAgent.fallbackMessage")}>
                    <Textarea
                      rows={5}
                      value={fallbackMessage}
                      onChange={(event) => setFallbackMessage(event.target.value)}
                    />
                  </FieldBlock>
                </FormSection>
                </div></details>
              </div>
            ) : null}
          </fieldset>
        </div>
        <aside className={cn("min-h-0 min-w-0 flex-1 border-l xl:block xl:w-[380px] xl:max-w-[40%] xl:flex-none 2xl:w-[440px]", mobileView !== "trial" && "hidden")}>
          <EmployeePreview draft={buildPayload()} resourceKey={JSON.stringify(skills.filter((skill) => selectedSkillIds.includes(skill.id)))} />
        </aside>
      </div>

      <footer className="flex min-h-16 shrink-0 flex-wrap items-center justify-between gap-3 border-t bg-background px-5 py-3">
        <div className="min-w-0 flex-1 basis-full text-xs text-muted-foreground sm:basis-0">
          <span>
            {formDirty ? t("aiReception.unsaved") : agentPublished ? t("employee.published") : t("aiAgent.unpublishedNote")}
          </span>
        </div>
        <div className="grid w-full grid-cols-2 gap-2 sm:flex sm:w-auto sm:items-center">
          <Button type="button" variant="outline" disabled={saving} onClick={onCancel}>
            {t("employee.back")}
          </Button>
          <Button
            type="button"
            variant="outline"
            disabled={saving}
            onClick={saveAgentSettings}
          >
            <SaveIcon />
            {saving ? t("aiReception.saving") : t("aiAgent.saveConfig")}
          </Button>
          {agent ? (
            <Button type="button" className="col-span-2" disabled={saving} onClick={publishAgent}>
              {t("aiAgent.publishAgent")}
            </Button>
          ) : null}
        </div>
      </footer>

      <ProjectDialog
        open={versionDialogOpen}
        onOpenChange={setVersionDialogOpen}
        title={t("aiAgent.versionHistory")}
        size="xl"
      >
        <VersionRecordsTable
          agent={agent}
          agentRevisions={agentRevisions}
          onRollback={rollbackAgentRevision}
          rollbackDisabled={saving}
        />
      </ProjectDialog>
    </div>
  )
}

function FormSection({
  title,
  description,
  action,
  children,
}: {
  title: string
  description: string
  action?: ReactNode
  children: ReactNode
}) {
  return (
    <section className="flex flex-col gap-4">
      <div className="flex items-start justify-between gap-4">
        <div>
          <h2 className="text-sm font-semibold text-foreground">
            {title}
          </h2>
          <p className="mt-1 text-xs leading-5 text-muted-foreground">{description}</p>
        </div>
        {action}
      </div>
      <div className="space-y-4">{children}</div>
    </section>
  )
}

function FieldBlock({
  label,
  required,
  description,
  className,
  children,
}: {
  label: string
  required?: boolean
  description?: string
  className?: string
  children: ReactNode
}) {
  return (
    <div className={cn("space-y-2", className)}>
      <Label>
        {label}
        {required ? <span className="ml-1 text-destructive">*</span> : null}
      </Label>
      {children}
      {description ? <p className="text-xs leading-5 text-muted-foreground">{description}</p> : null}
    </div>
  )
}

function BadgeList({
  empty,
  items,
  onRemove,
}: {
  empty: string
  items: { value: string; label: string }[]
  onRemove: (id: number) => void
}) {
  if (items.length === 0) {
    return <div className="text-sm text-muted-foreground">{empty}</div>
  }
  return (
    <div className="flex flex-wrap gap-2">
      {items.map((item) => (
        <Badge key={item.value} variant="secondary" className="gap-1 pr-1">
          {item.label}
          <Button
            type="button"
            variant="ghost"
            size="icon"
            className="size-5"
            onClick={() => onRemove(Number(item.value))}
          >
            <Trash2Icon className="size-3" />
          </Button>
        </Badge>
      ))}
    </div>
  )
}

function VersionRecordsTable({
  agent,
  agentRevisions,
  onRollback,
  rollbackDisabled,
}: {
  agent: AIAgent | null
  agentRevisions: AgentRevision[]
  onRollback: (revisionId: number) => void
  rollbackDisabled: boolean
}) {
  const t = useI18n()

  return (
    <div className="max-h-[60vh] overflow-auto rounded-md border">
      {agentRevisions.length > 0 ? (
        <Table>
          <TableHeader className="bg-muted/40">
            <TableRow>
              <TableHead className="w-28">{t("aiAgent.versionHistory")}</TableHead>
              <TableHead>{t("common.name")}</TableHead>
              <TableHead>{t("common.status")}</TableHead>
              <TableHead>{t("aiAgent.description")}</TableHead>
              <TableHead className="text-right">{t("aiAgent.columnActions")}</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {agentRevisions.map((revision) => {
              const active = agent?.publishedRevisionId === revision.id
              return (
                <TableRow key={revision.id}>
                  <TableCell className="font-medium">
                    r{revision.revision}
                    {active ? (
                      <Badge variant="secondary" className="ml-2">
                        {t("aiAgent.activeRevision")}
                      </Badge>
                    ) : null}
                  </TableCell>
                  <TableCell className="text-muted-foreground">
                    {revision.publishedAt || "-"}
                  </TableCell>
                  <TableCell>{revision.publishedByName || "-"}</TableCell>
                  <TableCell className="font-mono text-xs text-muted-foreground">
                    {revision.definitionHash?.slice(0, 12) || "-"}
                  </TableCell>
                  <TableCell className="text-right">
                    <Button
                      type="button"
                      variant="outline"
                      size="sm"
                      disabled={active || rollbackDisabled}
                      onClick={() => onRollback(revision.id)}
                    >
                      {t("aiAgent.rollback")}
                    </Button>
                  </TableCell>
                </TableRow>
              )
            })}
          </TableBody>
        </Table>
      ) : (
        <div className="p-5 text-sm text-muted-foreground">{t("aiAgent.noRevisions")}</div>
      )}
    </div>
  )
}
