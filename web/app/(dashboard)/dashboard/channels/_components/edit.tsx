"use client"

import { useEffect, useMemo, useState, useSyncExternalStore } from "react"
import { zodResolver } from "@hookform/resolvers/zod"
import { Controller, type Resolver, useForm, useWatch } from "react-hook-form"
import { z } from "zod/v4"
import { CopyIcon, ExternalLinkIcon } from "lucide-react"
import { toast } from "sonner"

import { getWidgetDemoPath } from "@/components/support-chat/demo-navigation"
import { OptionCombobox } from "@/components/option-combobox"
import { ProjectDialog } from "@/components/project-dialog"
import { Button } from "@/components/ui/button"
import { Field, FieldContent, FieldError, FieldLabel, FieldGroup } from "@/components/ui/field"
import { Input } from "@/components/ui/input"
import { Textarea } from "@/components/ui/textarea"
import {
  type AIAgent,
  type AdminChannel,
  type CreateAdminChannelPayload,
  fetchAIAgentsAll,
  fetchChannel,
  resetChannelUserTokenSecret,
} from "@/lib/api/admin"
import { useI18n } from "@/i18n/provider"

type ChannelFormDialogProps = {
  fixedChannelType: "web" | "whatsapp" | "messenger"
  open: boolean
  saving: boolean
  itemId: number | null
  onOpenChange: (open: boolean) => void
  onSubmit: (payload: CreateAdminChannelPayload) => Promise<void>
}

type Translate = (key: string, values?: Record<string, string | number>) => string

type WebChannelConfig = {
  title: string
  subtitle: string
  themeColor: string
  position: "left" | "right"
  width: string
  userTokenSecret: string
}

type EditForm = {
  channelType: "web" | "whatsapp" | "messenger"
  aiAgentId: string
  name: string
  userTokenSecret: string
  remark: string
}

function defaultWebConfig(t: Translate): WebChannelConfig {
  return {
    title: t("channel.defaultTitleWeb"),
    subtitle: t("channel.defaultSubtitle"),
    themeColor: "#2563eb",
    position: "right",
    width: "380px",
    userTokenSecret: "",
  }
}

function parseWebConfig(configJson: string, t: Translate): WebChannelConfig {
  const defaults = defaultWebConfig(t)
  if (!configJson.trim()) return defaults
  try {
    const parsed = JSON.parse(configJson) as Partial<WebChannelConfig>
    return {
      title: parsed.title?.trim() || defaults.title,
      subtitle: parsed.subtitle?.trim() ?? defaults.subtitle,
      themeColor: parsed.themeColor?.trim() || defaults.themeColor,
      position: parsed.position === "left" ? "left" : "right",
      width: parsed.width?.trim() || defaults.width,
      userTokenSecret: parsed.userTokenSecret?.trim() || "",
    }
  } catch {
    return defaults
  }
}

function createSchema(t: Translate) {
  return z.object({
    channelType: z.enum(["web", "whatsapp", "messenger"]),
    aiAgentId: z.string().trim().regex(/^\d+$/, t("channel.agentRequired")),
    name: z.string().trim().min(1, t("channel.nameRequired")),
    userTokenSecret: z.string().trim(),
    remark: z.string().trim(),
  })
}

function buildForm(item: AdminChannel | null, t: Translate, defaultType: EditForm["channelType"]): EditForm {
  if (!item) {
    return { channelType: defaultType, aiAgentId: "", name: "", userTokenSecret: "", remark: "" }
  }
  return {
    channelType: item.channelType === "messenger" ? "messenger" : item.channelType === "whatsapp" ? "whatsapp" : "web",
    aiAgentId: item.aiAgentId > 0 ? String(item.aiAgentId) : "",
    name: item.name,
    userTokenSecret: parseWebConfig(item.configJson, t).userTokenSecret,
    remark: item.remark || "",
  }
}

function buildPayload(
  form: EditForm,
  item: AdminChannel | null,
  status: number,
  t: Translate,
): CreateAdminChannelPayload {
  const config = parseWebConfig(item?.configJson || "", t)
  return {
    channelType: form.channelType,
    aiAgentId: Number(form.aiAgentId),
    aiAgentRolloutPercent: item?.aiAgentRolloutPercent || 100,
    name: form.name.trim(),
    configJson: form.channelType !== "web" ? "{}" : JSON.stringify({
      ...config,
      userTokenSecret: form.userTokenSecret.trim(),
    }),
    status,
    remark: form.remark.trim(),
  }
}

function isAgentChannelBindable(agent: AIAgent | undefined) {
  return Boolean(agent && agent.publishedRevisionId > 0)
}

function subscribeToOrigin() {
  return () => undefined
}

function useBrowserOrigin() {
  return useSyncExternalStore(
    subscribeToOrigin,
    () => window.location.origin,
    () => "",
  )
}

export function EditDialog(props: ChannelFormDialogProps) {
  if (!props.open) return null
  return <ChannelFormBody key={props.itemId ? `edit-${props.itemId}` : "create"} {...props} />
}

function ChannelFormBody({
  fixedChannelType,
  saving,
  itemId,
  onOpenChange,
  onSubmit,
}: ChannelFormDialogProps) {
  const t = useI18n()
  const formId = "channel-edit-form"
  const schema = useMemo(() => createSchema(t), [t])
  const emptyForm = useMemo(() => buildForm(null, t, fixedChannelType), [t, fixedChannelType])
  const resolver = useMemo(
    () => zodResolver(schema as never) as Resolver<EditForm>,
    [schema],
  )
  const [loading, setLoading] = useState(false)
  const [aiAgents, setAIAgents] = useState<AIAgent[]>([])
  const [channelDetail, setChannelDetail] = useState<AdminChannel | null>(null)
  const [currentStatus, setCurrentStatus] = useState(0)
  const form = useForm<EditForm>({ resolver, defaultValues: emptyForm })
  const {
    control,
    handleSubmit,
    register,
    reset,
    setValue,
    formState: { errors },
  } = form
  const aiAgentId = useWatch({ control, name: "aiAgentId" })
  const channelType = useWatch({ control, name: "channelType" })
  const userTokenSecret = useWatch({ control, name: "userTokenSecret" })

  useEffect(() => {
    fetchAIAgentsAll({ status: 1 })
      .then(setAIAgents)
      .catch((error) => console.error("Failed to load AI agents:", error))
  }, [])

  useEffect(() => {
    async function loadDetail() {
      if (!itemId) {
        setCurrentStatus(0)
        setChannelDetail(null)
        reset(emptyForm)
        return
      }
      setLoading(true)
      try {
        const data = await fetchChannel(itemId)
        setChannelDetail(data)
        setCurrentStatus(data.status)
        reset(buildForm(data, t, fixedChannelType))
      } catch (error) {
        console.error("Failed to load web embed:", error)
      } finally {
        setLoading(false)
      }
    }
    void loadDetail()
  }, [emptyForm, itemId, reset, t, fixedChannelType])

  const selectedAIAgent = aiAgents.find((item) => String(item.id) === aiAgentId)
  const aiAgentOptions = aiAgents.map((item) => ({
    value: String(item.id),
    label: isAgentChannelBindable(item)
      ? item.name
      : `${item.name} · ${t("aiAgent.agentNotPublishedShort")}`,
    disabled: !isAgentChannelBindable(item),
  }))

  async function submit(values: EditForm) {
    const selected = aiAgents.find((item) => String(item.id) === values.aiAgentId)
    if (!isAgentChannelBindable(selected)) {
      toast.error(t("aiAgent.agentNotPublishedWarning"))
      return
    }
    await onSubmit(buildPayload(values, channelDetail, currentStatus, t))
  }

  async function resetSecret() {
    if (!itemId || !window.confirm(t("channel.resetSecretConfirm"))) return
    try {
      const result = await resetChannelUserTokenSecret(itemId)
      setValue("userTokenSecret", result.userTokenSecret, { shouldDirty: true })
      if (channelDetail) {
        const config = parseWebConfig(channelDetail.configJson, t)
        setChannelDetail({
          ...channelDetail,
          configJson: JSON.stringify({
            ...config,
            userTokenSecret: result.userTokenSecret,
          }),
        })
      }
      toast.success(t("channel.resetSecretSuccess"))
    } catch (error) {
      toast.error(error instanceof Error ? error.message : t("channel.resetSecretFailed"))
    }
  }

  async function copySecret() {
    if (!userTokenSecret) return
    try {
      await navigator.clipboard.writeText(userTokenSecret)
      toast.success(t("channel.copySecretSuccess"))
    } catch {
      toast.error(t("channel.copyFailed"))
    }
  }

  return (
    <ProjectDialog
      open
      onOpenChange={onOpenChange}
      title={itemId ? t("channel.editTitle") : t(fixedChannelType === "web" ? "channel.newWeb" : fixedChannelType === "messenger" ? "channel.newMessenger" : "channel.newWhatsApp")}
      size="lg"
      footer={
        <>
          <Button type="button" variant="outline" onClick={() => onOpenChange(false)}>
            {t("channel.cancel")}
          </Button>
          <Button type="submit" form={formId} disabled={saving || loading}>
            {saving ? t("channel.saving") : t("channel.save")}
          </Button>
        </>
      }
    >
      {loading ? (
        <div className="py-12 text-center text-sm text-muted-foreground">
          {t("channel.loadingDetail")}
        </div>
      ) : (
        <form id={formId} onSubmit={handleSubmit(submit)} className="flex flex-col gap-6">
          <FieldGroup>
            <Field data-invalid={!!errors.name}>
              <FieldLabel htmlFor="channel-name">{t("channel.name")}</FieldLabel>
              <FieldContent>
                <Input id="channel-name" {...register("name")} />
                <FieldError errors={[errors.name]} />
              </FieldContent>
            </Field>

            <Field data-invalid={!!errors.aiAgentId}>
              <FieldLabel>{t("channel.columnAgent")}</FieldLabel>
              <FieldContent>
                <Controller
                  control={control}
                  name="aiAgentId"
                  render={({ field }) => (
                    <OptionCombobox
                      value={field.value}
                      options={aiAgentOptions}
                      placeholder={t("channel.agentRequired")}
                      searchPlaceholder={t("channel.searchAiAgent")}
                      emptyText={t("channel.emptyAiAgent")}
                      onChange={field.onChange}
                    />
                  )}
                />
                {selectedAIAgent && !isAgentChannelBindable(selectedAIAgent) ? (
                  <div className="rounded-md border border-amber-200 bg-amber-50 px-3 py-2 text-xs text-amber-900">
                    {t("aiAgent.agentNotPublishedWarning")}
                  </div>
                ) : null}
                <FieldError errors={[errors.aiAgentId]} />
              </FieldContent>
            </Field>
          </FieldGroup>

          {channelType === "web" && <WebAccessGuide channelId={channelDetail?.channelId || ""} />}

          {channelType === "web" && <div className="space-y-3 border-t pt-5">
            <div>
              <div className="text-sm font-medium">{t("channel.userJwtSecret")}</div>
              <div className="mt-1 text-xs text-muted-foreground">
                {t("channel.userJwtSecretDescription")}
              </div>
            </div>
            {!itemId ? (
              <div className="rounded-md bg-muted px-3 py-2 text-sm text-muted-foreground">
                {t("channel.secretAfterSave")}
              </div>
            ) : (
              <Field data-invalid={!!errors.userTokenSecret}>
                <FieldLabel htmlFor="channel-user-token-secret">Secret</FieldLabel>
                <FieldContent>
                  <div className="flex gap-2">
                    <Input
                      id="channel-user-token-secret"
                      readOnly
                      className="font-mono text-xs"
                      {...register("userTokenSecret")}
                    />
                    <Button
                      type="button"
                      variant="outline"
                      size="icon"
                      title={t("channel.copy")}
                      onClick={copySecret}
                      disabled={!userTokenSecret}
                    >
                      <CopyIcon />
                    </Button>
                    <Button type="button" variant="outline" onClick={() => void resetSecret()}>
                      {t("channel.reset")}
                    </Button>
                  </div>
                  <FieldError errors={[errors.userTokenSecret]} />
                </FieldContent>
              </Field>
            )}
          </div>}

          <Field data-invalid={!!errors.remark}>
            <FieldLabel htmlFor="channel-remark">{t("channel.remark")}</FieldLabel>
            <FieldContent>
              <Textarea id="channel-remark" rows={3} {...register("remark")} />
              <FieldError errors={[errors.remark]} />
            </FieldContent>
          </Field>
        </form>
      )}
    </ProjectDialog>
  )
}

function WebAccessGuide({ channelId }: { channelId: string }) {
  const t = useI18n()
  const origin = useBrowserOrigin()

  const accessUrl = useMemo(() => {
    if (!origin || !channelId) return ""
    const url = new URL("/support/chat/", origin)
    url.searchParams.set("channelId", channelId)
    return url.toString()
  }, [channelId, origin])

  const testUrl = useMemo(() => {
    if (!origin || !channelId) return ""
    const url = new URL(getWidgetDemoPath(), origin)
    url.searchParams.set("channelId", channelId)
    return url.toString()
  }, [channelId, origin])

  const snippet = useMemo(() => {
    if (!origin || !channelId) return ""
    return `<script>\n  window.AgentDeskConfig = {\n    channelId: "${channelId}"\n  };\n</script>\n<script async src="${origin}/sdk/agent-desk-sdk.min.js"></script>`
  }, [channelId, origin])

  async function copyText(text: string, message: string) {
    if (!text) return
    try {
      await navigator.clipboard.writeText(text)
      toast.success(message)
    } catch {
      toast.error(t("channel.copyFailed"))
    }
  }

  return (
    <div className="space-y-4 border-t pt-5">
      <div>
        <div className="text-sm font-medium">{t("channel.webAccessInfo")}</div>
        {channelId ? (
          <div className="mt-1 text-xs text-muted-foreground">
            {t("channel.webAccessReady")}
          </div>
        ) : null}
      </div>

      {!channelId ? (
        <div className="rounded-md bg-muted px-3 py-2 text-sm text-muted-foreground">
          {t("channel.newChannelPending")}
        </div>
      ) : (
        <div className="space-y-4">
          <div className="space-y-2">
            <FieldLabel>{t("channel.directAccessUrl")}</FieldLabel>
            <div className="flex gap-2">
              <Input readOnly value={accessUrl} className="font-mono text-xs" />
              <Button
                type="button"
                variant="outline"
                size="icon"
                title={t("channel.copyLink")}
                onClick={() => void copyText(accessUrl, t("channel.copiedAccessLink"))}
              >
                <CopyIcon />
              </Button>
              <Button
                type="button"
                variant="outline"
                size="icon"
                title={t("channel.openLink")}
                onClick={() => window.open(accessUrl, "_blank", "noopener,noreferrer")}
              >
                <ExternalLinkIcon />
              </Button>
            </div>
          </div>

          <div className="space-y-2">
            <div className="flex items-center justify-between gap-3">
              <FieldLabel>{t("channel.embeddedSnippet")}</FieldLabel>
              <Button
                type="button"
                variant="outline"
                size="sm"
                onClick={() => void copyText(snippet, t("channel.copiedSnippet"))}
              >
                <CopyIcon />
                {t("channel.copyCode")}
              </Button>
            </div>
            <pre className="max-h-48 overflow-auto rounded-md bg-muted p-3 text-xs leading-5">
              <code>{snippet}</code>
            </pre>
          </div>

          <Button
            type="button"
            variant="outline"
            onClick={() => window.open(testUrl, "_blank", "noopener,noreferrer")}
          >
            <ExternalLinkIcon />
            {t("channel.openTestPage")}
          </Button>
        </div>
      )}
    </div>
  )
}
