"use client"

import { useEffect, useRef, useState } from "react"
import { ClockIcon, LockKeyholeIcon, SaveIcon, RefreshCwIcon } from "lucide-react"
import { toast } from "sonner"
import { Button } from "@/components/ui/button"
import { Badge } from "@/components/ui/badge"
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogFooter } from "@/components/ui/dialog"
import { Field, FieldGroup, FieldLabel } from "@/components/ui/field"
import { Input } from "@/components/ui/input"
import { Textarea } from "@/components/ui/textarea"
import { Skeleton } from "@/components/ui/skeleton"
import { OptionCombobox } from "@/components/option-combobox"
import { useI18n } from "@/i18n/provider"
import { readSession } from "@/lib/auth"
import type { AgentConversation } from "@/lib/api/agent"
import { addPrivateNote, fetchCollaboration, fetchColleagues, updateConversationWork, type Collaboration, type Colleague } from "@/lib/api/conversation-work"
import { ConversationWorkStatus, IMConversationStatus } from "@/lib/generated/enums"
import { useAgentConversationsStore } from "@/lib/stores/agent-conversations"
import { formatDateTime, generateUUID } from "@/lib/utils"
import { isReplyOverdue, workStatusKey } from "@/lib/reception-work"

export function ReceptionWorkbar({ conversation }: { conversation: AgentConversation }) {
  const t = useI18n()
  const [now, setNow] = useState(() => Date.now())
  const [settings, setSettings] = useState(false)
  const [notes, setNotes] = useState(false)
  const [busy, setBusy] = useState(false)
  const canEdit = conversation.status === IMConversationStatus.Active && conversation.currentAssigneeId === readSession()?.user?.id && readSession()?.permissions.includes("conversation.send")
  useEffect(() => { const timer = setInterval(() => setNow(Date.now()), 30000); return () => clearInterval(timer) }, [])
  return <>
    <div className="flex shrink-0 flex-wrap items-center gap-2 border-b px-3 py-2">
      {conversation.status !== IMConversationStatus.Closed && <>
        <Badge variant={isReplyOverdue(conversation, now) ? "destructive" : "secondary"}>{t(isReplyOverdue(conversation, now) ? "reception.overdue" : workStatusKey(conversation.workStatus))}</Badge>
        {(conversation.snoozedUntil || conversation.replyDueAt) && <span className="min-w-0 text-xs text-muted-foreground">{t(conversation.snoozedUntil ? "reception.wakeAt" : "reception.dueAt", { time: formatDateTime(conversation.snoozedUntil || conversation.replyDueAt) })}</span>}
        {canEdit && <Button size="icon-sm" variant="ghost" aria-label={t("reception.editState")} title={t("reception.editState")} onClick={() => setSettings(true)}><ClockIcon /></Button>}
      </>}
      <Button size="sm" variant="ghost" className="ml-auto" onClick={() => setNotes(true)}><LockKeyholeIcon data-icon="inline-start" />{t("reception.collaboration")}</Button>
    </div>
    <Dialog open={settings} onOpenChange={(open) => { if (!busy) setSettings(open) }}>
      {settings && <WorkSettings key={conversation.id} conversation={conversation} close={() => setSettings(false)} onBusy={setBusy} />}
    </Dialog>
    <Dialog open={notes} onOpenChange={(open) => { if (!busy) setNotes(open) }}>
      {notes && <PrivateCollaboration key={conversation.id} conversationId={conversation.id} onBusy={setBusy} />}
    </Dialog>
  </>
}

function WorkSettings({ conversation, close, onBusy }: { conversation: AgentConversation; close: () => void; onBusy: (busy: boolean) => void }) {
  const t = useI18n()
  const [status, setStatus] = useState(conversation.workStatus || ConversationWorkStatus.WaitingCustomer)
  const [target, setTarget] = useState(conversation.replyTargetMinutes || 15)
  const [snooze, setSnooze] = useState(60)
  const [busy, setBusy] = useState(false)
  const revision = useRef(conversation.workRevision || 0)
  const refresh = useAgentConversationsStore((s) => s.resyncRealtimeData)
  async function save() {
    if (busy) return
    setBusy(true); onBusy(true)
    try {
      await updateConversationWork(conversation.id, { revision: revision.current, status, snoozeMinutes: snooze, replyTargetMinutes: target })
      close()
      toast.success(t("reception.saved"))
      await refresh(conversation.id)
    } catch (error) { toast.error(error instanceof Error ? error.message : t("reception.failed")); void refresh(conversation.id).catch(() => {}) }
    finally { setBusy(false); onBusy(false) }
  }
  return <DialogContent className="rounded-md sm:max-w-md">
    <DialogHeader><DialogTitle>{t("reception.editState")}</DialogTitle></DialogHeader>
    <form className="flex flex-col gap-5" onSubmit={(e) => { e.preventDefault(); void save() }}>
      <FieldGroup>
        <Field><FieldLabel>{t("reception.state")}</FieldLabel><OptionCombobox disabled={busy} value={status} onChange={(v) => setStatus(v as ConversationWorkStatus)} placeholder={t("reception.state")} options={Object.values(ConversationWorkStatus).map((value) => ({ value, label: t(workStatusKey(value)) }))} /></Field>
        <Field><FieldLabel htmlFor="reply-target">{t("reception.target")}</FieldLabel><Input id="reply-target" type="number" min={1} max={10080} required disabled={busy} value={target} onChange={(e) => setTarget(Number(e.target.value))} /></Field>
        {status === ConversationWorkStatus.Snoozed && <Field><FieldLabel htmlFor="snooze-duration">{t("reception.snoozeDuration")}</FieldLabel><Input id="snooze-duration" type="number" min={1} max={43200} required disabled={busy} value={snooze} onChange={(e) => setSnooze(Number(e.target.value))} /></Field>}
      </FieldGroup>
      <DialogFooter><Button type="button" variant="ghost" disabled={busy} onClick={() => { revision.current = conversation.workRevision || 0; setStatus(conversation.workStatus || ConversationWorkStatus.WaitingCustomer); setTarget(conversation.replyTargetMinutes || 15) }}>{t("reception.reloadState")}</Button><Button type="button" variant="outline" disabled={busy} onClick={close}>{t("reception.cancel")}</Button><Button type="submit" disabled={busy}><SaveIcon data-icon="inline-start" />{t(busy ? "reception.saving" : "reception.save")}</Button></DialogFooter>
    </form>
  </DialogContent>
}

function PrivateCollaboration({ conversationId, onBusy }: { conversationId: number; onBusy: (busy: boolean) => void }) {
  const t = useI18n()
  const [data, setData] = useState<Collaboration | null>(null)
  const [colleagues, setColleagues] = useState<Colleague[]>([])
  const [failed, setFailed] = useState(false)
  const [loading, setLoading] = useState(true)
  const [colleaguesFailed, setColleaguesFailed] = useState(false)
  const [colleaguesLoading, setColleaguesLoading] = useState(true)
  const [saving, setSaving] = useState(false)
  const draft = useAgentConversationsStore((s) => s.privateNoteDrafts[conversationId])
  const setDraft = useAgentConversationsStore((s) => s.setPrivateNoteDraft)
  const canSend = readSession()?.permissions?.includes("conversation.send")
  const seq = useRef(0)
  const colleaguesSeq = useRef(0)
  const colleagueOptions = colleagues.map((user) => ({ value: String(user.id), label: user.name }))
  for (const id of draft?.mentionIds || []) {
    if (!colleagueOptions.some((option) => option.value === id)) {
      colleagueOptions.push({ value: id, label: t("reception.retainedMention", { id }) })
    }
  }
  async function load(before = 0) {
    const request = ++seq.current
    setLoading(true); setFailed(false)
    try {
      const result = await fetchCollaboration(conversationId, before)
      if (request !== seq.current) return
      setData((old) => before && old ? { ...result, notes: [...old.notes, ...result.notes] } : result)
    } catch { if (request === seq.current) setFailed(true) }
    finally { if (request === seq.current) setLoading(false) }
  }
  async function loadColleagues() {
    const request = ++colleaguesSeq.current
    setColleaguesLoading(true)
    try {
      const items = await fetchColleagues()
      if (request !== colleaguesSeq.current) return
      setColleagues(items); setColleaguesFailed(false)
    } catch { if (request === colleaguesSeq.current) setColleaguesFailed(true) }
    finally { if (request === colleaguesSeq.current) setColleaguesLoading(false) }
  }
  useEffect(() => {
    void load()
    void loadColleagues()
    return () => { seq.current++; colleaguesSeq.current++ }
    // The component is keyed by conversation; refresh is explicit to preserve scroll.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [conversationId])
  function change(next: Partial<NonNullable<typeof draft>>) {
    setDraft(conversationId, { content: draft?.content || "", mentionIds: draft?.mentionIds || [], clientId: generateUUID(), ...next })
  }
  async function save() {
    if (saving || !draft?.content.trim()) return
    setSaving(true); onBusy(true)
    try {
      await addPrivateNote(conversationId, { content: draft.content, mentionIds: draft.mentionIds.map(Number), clientId: draft.clientId })
      setDraft(conversationId, { content: "", mentionIds: [], clientId: generateUUID() })
      toast.success(t("reception.noteSaved")); await load()
    } catch (error) { toast.error(error instanceof Error ? error.message : t("reception.failed")) }
    finally { setSaving(false); onBusy(false) }
  }
  return <DialogContent className="flex max-h-[90dvh] flex-col overflow-hidden rounded-md sm:max-w-2xl">
    <DialogHeader><DialogTitle>{t("reception.collaboration")}</DialogTitle></DialogHeader>
    <div className="flex min-h-0 flex-1 flex-col gap-4 overflow-y-auto">
      <div className="flex items-center justify-between gap-2"><Badge variant="secondary"><LockKeyholeIcon data-icon="inline-start" />{t("reception.private")}</Badge><Button variant="ghost" size="icon-sm" disabled={loading || colleaguesLoading} aria-label={t("reception.refresh")} title={t("reception.refresh")} onClick={() => { void load(); void loadColleagues() }}><RefreshCwIcon /></Button></div>
      {failed && <p role="alert" className="text-sm text-destructive">{t("reception.failed")}</p>}
      {!data && loading && <Skeleton className="h-24 w-full" />}
      {data && data.notes.length === 0 && <p className="text-sm text-muted-foreground">{t("reception.noNotes")}</p>}
      {data?.notes.map((note) => <article key={note.id} className="flex flex-col gap-1 border-b pb-3">
        <div className="flex flex-wrap items-center justify-between gap-1 text-xs"><span className="font-medium">{note.authorName}</span><time className="text-muted-foreground">{formatDateTime(note.createdAt)}</time></div>
        <p className="whitespace-pre-wrap break-words text-sm">{note.content}</p>
        {note.mentions.length > 0 && <p className="break-words text-xs text-muted-foreground">{note.mentions.map((user) => `@${user.name}`).join(" ")}</p>}
      </article>)}
      {data?.hasMore && <Button variant="outline" size="sm" disabled={loading} onClick={() => void load(data.notes.at(-1)?.id)}>{t("reception.older")}</Button>}
      {!!data?.handoffs.length && <section className="flex flex-col gap-2"><h3 className="text-sm font-medium">{t("reception.handoffs")}</h3>{data.handoffs.map((handoff) => <div key={handoff.id} className="flex flex-col gap-1 text-xs"><span>{t("reception.handoff", { from: handoff.from || t("reception.pool"), to: handoff.to || t("reception.pool") })}</span>{handoff.reason && <p className="break-words text-muted-foreground">{handoff.reason}</p>}<time className="text-muted-foreground">{formatDateTime(handoff.createdAt)}</time></div>)}</section>}
    </div>
    {canSend && <form className="flex shrink-0 flex-col gap-3 border-t pt-3" onSubmit={(e) => { e.preventDefault(); void save() }}><FieldGroup>
      <Field><FieldLabel htmlFor="private-note">{t("reception.note")}</FieldLabel><Textarea id="private-note" rows={3} maxLength={5000} disabled={saving} value={draft?.content || ""} onChange={(e) => change({ content: e.target.value })} /></Field>
      <Field><FieldLabel>{t("reception.mentions")}</FieldLabel><OptionCombobox multiple disabled={saving || colleaguesLoading} options={colleagueOptions} values={draft?.mentionIds || []} onValuesChange={(ids) => change({ mentionIds: ids })} placeholder={t("reception.chooseColleagues")} />{colleaguesFailed && <p role="alert" className="text-sm text-destructive">{t("reception.colleaguesFailed")}</p>}</Field>
    </FieldGroup><DialogFooter><Button type="submit" disabled={saving || !draft?.content.trim()}><LockKeyholeIcon data-icon="inline-start" />{t(saving ? "reception.saving" : "reception.addNote")}</Button></DialogFooter></form>}
  </DialogContent>
}
