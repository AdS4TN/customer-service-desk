"use client"

import { useCallback, useEffect, useRef, useState } from "react"
import { BotIcon, HandIcon, HistoryIcon, LoaderCircleIcon, PlayIcon, RefreshCwIcon } from "lucide-react"
import { toast } from "sonner"
import { Button } from "@/components/ui/button"
import { Badge } from "@/components/ui/badge"
import { Alert, AlertDescription } from "@/components/ui/alert"
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogFooter } from "@/components/ui/dialog"
import { Field, FieldGroup, FieldLabel } from "@/components/ui/field"
import { Input } from "@/components/ui/input"
import { Textarea } from "@/components/ui/textarea"
import { Switch } from "@/components/ui/switch"
import { Skeleton } from "@/components/ui/skeleton"
import { OptionCombobox } from "@/components/option-combobox"
import { useI18n } from "@/i18n/provider"
import { readSession } from "@/lib/auth"
import type { AgentConversation } from "@/lib/api/agent"
import { fetchDelegation, startDelegation, stopDelegation, previewDelegation, type DelegationView, type DelegationInput } from "@/lib/api/conversation-delegation"
import { formatDateTime } from "@/lib/utils"
import { useAgentConversationsStore } from "@/lib/stores/agent-conversations"

export function ConversationDelegation({ conversation }: { conversation: AgentConversation }) {
  const t = useI18n()
  const [data, setData] = useState<DelegationView | null>(null)
  const [error, setError] = useState("")
  const [loadError, setLoadError] = useState("")
  const [open, setOpen] = useState(false)
  const [saving, setSaving] = useState(false)
  const [previewing, setPreviewing] = useState(false)
  const [draft, setDraft] = useState<DelegationInput | null>(null)
  const seq = useRef(0)
  const refresh = useAgentConversationsStore((s) => s.resyncRealtimeData)
  const canSend = readSession()?.permissions?.some((p) => p === "conversation.send" || p === "*")
  const canManage = !!data?.canManage && !!canSend
  const liveAvailable = !!data?.agents.find((a) => a.id === draft?.aiAgentId)?.liveAvailable
  const load = useCallback(async () => {
    const current = ++seq.current
    try {
      const next = await fetchDelegation(conversation.id)
      if (current === seq.current) {
        setData(next); setLoadError("")
        if (!next.active) setDraft((old) => old || { revision: next.revision, aiAgentId: next.aiAgentId || next.agents[0]?.id || 0, previewOnly: true, durationMinutes: 60, instructions: next.instructions })
      }
    } catch (e) { if (current === seq.current) setLoadError(e instanceof Error ? e.message : t("delegation.failed")) }
  }, [conversation.id, t])
  useEffect(() => {
    void load()
    const timer = setInterval(() => { if (document.visibilityState === "visible") void load() }, 5000)
    // Invalidate pending requests, not a DOM ref snapshot.
    const invalidate = () => { seq.current++ }
    return () => { clearInterval(timer); invalidate() }
  }, [load])

  function show() {
    if (!draft && data) setDraft({ revision: data.revision, aiAgentId: data.aiAgentId || data.agents[0]?.id || 0, previewOnly: true, durationMinutes: 60, instructions: data.instructions })
    setOpen(true)
  }
  async function start() {
    if (!data || !draft || saving) return
    setSaving(true); setError("")
    try {
      await startDelegation(conversation.id, { ...draft, revision: data.revision, previewOnly: !liveAvailable || draft.previewOnly })
      setDraft(null)
      toast.success(t(draft.previewOnly || !liveAvailable ? "delegation.trialStarted" : "delegation.started"))
      await load(); await refresh(conversation.id)
    } catch (e) { setError(e instanceof Error ? e.message : t("delegation.failed")) }
    finally { setSaving(false) }
  }
  async function stop() {
    if (!data || saving) return
    setSaving(true); setError("")
    try {
      await stopDelegation(conversation.id, data.revision)
      setDraft(null)
      toast.success(t("delegation.stopped"))
      await load(); await refresh(conversation.id)
    } catch (e) { setError(e instanceof Error ? e.message : t("delegation.failed")) }
    finally { setSaving(false) }
  }
  async function preview() {
    if (!data || previewing) return
    setPreviewing(true); setError("")
    try { await previewDelegation(conversation.id, data.revision); await load() }
    catch (e) { setError(e instanceof Error ? e.message : t("delegation.failed")); await load() }
    finally { setPreviewing(false) }
  }
  const status = data?.active ? (data.previewOnly ? "delegation.trial" : "delegation.active") : "delegation.human"
  const canStop = data?.active && !!canSend && (data.ownerId === readSession()?.user?.id || conversation.currentAssigneeId === readSession()?.user?.id || canManage)

  return <>
    <div className="flex shrink-0 flex-wrap items-center gap-2 border-b px-3 py-2" aria-label={t("delegation.title")}>
      {!data ? <>{!loadError && <Skeleton className="h-5 w-28" />}{loadError && <Button size="sm" variant="ghost" onClick={() => void load()}><RefreshCwIcon data-icon="inline-start" />{t("delegation.retry")}</Button>}</> : <>
        <span className="min-w-0 break-words text-xs text-muted-foreground">{t("delegation.owner", { name: data.ownerName || t("delegation.unassigned") })}</span>
        <Badge variant={data.active ? "secondary" : "outline"}>{t(status)}</Badge>
        <div className="ml-auto flex items-center gap-1">
          {canStop && <Button size="sm" variant="outline" disabled={saving} onClick={() => void stop()}><HandIcon data-icon="inline-start" />{t(saving ? "delegation.stopping" : "delegation.reclaim")}</Button>}
          <Button size="sm" variant="ghost" onClick={show}><BotIcon data-icon="inline-start" />{t("delegation.title")}</Button>
        </div>
      </>}
      {(error || loadError) && !open && <span role="alert" className="w-full break-words text-xs text-destructive">{error || loadError}</span>}
    </div>
    <Dialog open={open} onOpenChange={(next) => { if (!saving) setOpen(next) }}>
      <DialogContent showCloseButton={false} className="flex max-h-[90dvh] flex-col overflow-hidden rounded-md sm:max-w-lg">
        <DialogHeader><DialogTitle>{t("delegation.title")}</DialogTitle></DialogHeader>
        <div className="flex min-h-0 flex-1 flex-col gap-4 overflow-y-auto">
          {(error || loadError) && <Alert variant="destructive"><AlertDescription>{error || loadError}</AlertDescription></Alert>}
          <div className="flex flex-wrap items-center gap-2 text-sm">
            <span className="min-w-0 break-words">{t("delegation.owner", { name: data?.ownerName || t("delegation.unassigned") })}</span>
            <Badge variant="secondary">{t(status)}</Badge>
            <Button className="ml-auto" size="icon-sm" variant="ghost" title={t("delegation.refresh")} aria-label={t("delegation.refresh")} onClick={() => void load()}><RefreshCwIcon /></Button>
          </div>
          {data?.active ? <section className="flex flex-col gap-2 text-sm">
            <p className="break-words">{t("delegation.employee")}: {data.agents.find((a) => a.id === data.aiAgentId)?.name || `#${data.aiAgentId}`}</p>
            <p className="text-xs text-muted-foreground">{t("delegation.startedBy", { name: data.startedByName, time: formatDateTime(data.startedAt) })}</p>
            <p className="text-xs text-muted-foreground">{data.expiresAt ? t("delegation.expires", { time: formatDateTime(data.expiresAt) }) : t("delegation.untilReclaim")}</p>
            {data.instructions && <p className="whitespace-pre-wrap break-words">{data.instructions}</p>}
          </section> : draft && canManage ? <form id="delegation-form" className="flex flex-col gap-4" onSubmit={(e) => { e.preventDefault(); void start() }}>
            <FieldGroup>
              <Field><FieldLabel htmlFor="delegation-agent">{t("delegation.employee")}</FieldLabel><OptionCombobox id="delegation-agent" disabled={saving} value={String(draft.aiAgentId)} onChange={(v) => setDraft({ ...draft, aiAgentId: Number(v), previewOnly: true })} options={data?.agents.map((a) => ({ value: String(a.id), label: a.name })) || []} placeholder={t("delegation.selectEmployee")} /></Field>
              <Field><FieldLabel htmlFor="delegation-instructions">{t("delegation.instructions")}</FieldLabel><Textarea id="delegation-instructions" rows={3} disabled={saving} value={draft.instructions} onChange={(e) => setDraft({ ...draft, instructions: e.target.value })} placeholder={t("delegation.instructionsPlaceholder")} /></Field>
              <Field><FieldLabel htmlFor="delegation-duration">{t("delegation.duration")}</FieldLabel><Input id="delegation-duration" type="number" min={0} max={43200} step={1} required disabled={saving} value={draft.durationMinutes} onChange={(e) => setDraft({ ...draft, durationMinutes: Number(e.target.value) })} /></Field>
              <Field orientation="horizontal" data-disabled={!liveAvailable || saving}><Switch id="delegation-live" disabled={!liveAvailable || saving} checked={liveAvailable && !draft.previewOnly} onCheckedChange={(checked) => setDraft({ ...draft, previewOnly: !checked })} /><FieldLabel htmlFor="delegation-live">{t("delegation.liveSending")}</FieldLabel></Field>
            </FieldGroup>
            {!liveAvailable && <Alert><AlertDescription>{t("delegation.sendingDisabled")}</AlertDescription></Alert>}
          </form> : !data?.active && <p className="text-sm text-muted-foreground">{t("delegation.claimFirst")}</p>}
          {!!data?.events.length && <section className="flex flex-col gap-3" aria-label={t("delegation.history")}>
            <h3 className="flex items-center gap-2 text-sm font-medium"><HistoryIcon className="size-4" />{t("delegation.history")}</h3>
            {data.events.map((event) => <article key={event.id} className="flex flex-col gap-1 border-b pb-3">
              <div className="flex flex-wrap justify-between gap-1 text-xs"><span>{t(`delegation.events.${event.kind}`)}</span><time className="text-muted-foreground">{formatDateTime(event.createdAt)}</time></div>
              {event.actorName && <span className="text-xs text-muted-foreground">{event.actorName}</span>}
              {event.previewOnly && <span className="text-xs text-muted-foreground">{t("delegation.notSent")}</span>}
              {event.reason && <p className="whitespace-pre-wrap break-words text-sm">{event.reason}</p>}
              {event.content && <p className="whitespace-pre-wrap break-words text-sm">{event.content}</p>}
            </article>)}
          </section>}
        </div>
        <DialogFooter className="shrink-0">
          <Button variant="outline" disabled={saving} onClick={() => setOpen(false)}>{t("delegation.close")}</Button>
          {data?.active ? <>
            {data.previewOnly && canManage && <Button variant="outline" disabled={previewing || saving} onClick={() => void preview()}>{previewing ? <LoaderCircleIcon className="animate-spin" data-icon="inline-start" /> : <PlayIcon data-icon="inline-start" />}{t(previewing ? "delegation.generating" : "delegation.preview")}</Button>}
            {canStop && <Button disabled={saving} onClick={() => void stop()}><HandIcon data-icon="inline-start" />{t(saving ? "delegation.stopping" : "delegation.reclaim")}</Button>}
          </> : draft && canManage && <Button type="submit" form="delegation-form" disabled={saving || !draft.aiAgentId}>{saving ? <LoaderCircleIcon className="animate-spin" data-icon="inline-start" /> : <BotIcon data-icon="inline-start" />}{t(saving ? "delegation.starting" : (draft.previewOnly || !liveAvailable ? "delegation.startTrial" : "delegation.startLive"))}</Button>}
        </DialogFooter>
      </DialogContent>
    </Dialog>
  </>
}
