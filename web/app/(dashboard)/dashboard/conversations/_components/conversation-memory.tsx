"use client";

import { CheckIcon, ChevronRightIcon, LoaderCircleIcon, MoreHorizontalIcon, PencilIcon, RefreshCwIcon, Trash2Icon } from "lucide-react";
import { useCallback, useEffect, useRef, useState } from "react";
import { toast } from "sonner";
import { useAuth } from "@/components/auth-provider";
import { useConfirm } from "@/components/confirm-provider";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { Alert, AlertDescription } from "@/components/ui/alert";
import { Collapsible, CollapsibleContent, CollapsibleTrigger } from "@/components/ui/collapsible";
import { DropdownMenu, DropdownMenuContent, DropdownMenuGroup, DropdownMenuItem, DropdownMenuSeparator, DropdownMenuTrigger } from "@/components/ui/dropdown-menu";
import { Field, FieldGroup, FieldLabel } from "@/components/ui/field";
import { Separator } from "@/components/ui/separator";
import { Skeleton } from "@/components/ui/skeleton";
import { Textarea } from "@/components/ui/textarea";
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip";
import { useI18n } from "@/i18n/provider";
import type { AgentConversation } from "@/lib/api/agent";
import { fetchConversationMemory, refreshConversationMemory, updateConversationMemory, type ConversationMemory, type MemoryEntry } from "@/lib/api/conversation-memory";
import { MemoryKind, MemoryStatus } from "@/lib/generated/enums";
import { formatDateTime } from "@/lib/utils";

// Keep unfinished edits when the operator switches conversations, without
// persisting private customer information into browser storage.
const drafts = new Map<number, { value: string; entry: MemoryEntry }>();
const kinds = [MemoryKind.OpenQuestion, MemoryKind.NextStep, MemoryKind.Customer];

export function ConversationMemorySection({ conversation }: { conversation: AgentConversation }) {
  return <MemoryBody key={conversation.id} conversation={conversation} />;
}

export function CustomerAutoTagsSection({ conversation, onProfileChanged }: { conversation: AgentConversation; onProfileChanged?: () => Promise<void> }) {
  return <MemoryBody key={conversation.id} conversation={conversation} tagsOnly onProfileChanged={onProfileChanged} />;
}

function MemoryBody({ conversation, tagsOnly = false, onProfileChanged }: { conversation: AgentConversation; tagsOnly?: boolean; onProfileChanged?: () => Promise<void> }) {
  const t = useI18n();
  const { session } = useAuth();
  const canEdit = !!session?.permissions.some((p) => p === "conversation.send" || p === "*");
  const canEditProfile = canEdit && !!session?.permissions.some((p) => p === "customer.update" || p === "*");
  const [data, setData] = useState<ConversationMemory | null>(null);
  const [error, setError] = useState("");
  const [requesting, setRequesting] = useState(false);
  const alive = useRef(true);
  const fetching = useRef(false);
  const load = useCallback(async () => {
    if (fetching.current) return;
    fetching.current = true;
    try {
      const result = await fetchConversationMemory(conversation.id);
      if (alive.current) { setData(result); setError(""); }
    } catch (e) {
      if (alive.current) setError(e instanceof Error ? e.message : t("conversation.memory.loadFailed"));
    } finally { fetching.current = false; }
  }, [conversation.id, t]);

  useEffect(() => {
    alive.current = true;
    void load();
    const timer = window.setInterval(() => { if (!document.hidden) void load(); }, 6000);
    return () => { alive.current = false; window.clearInterval(timer); };
  }, [load]);
  useEffect(() => { void load(); }, [conversation.lastMessageId, load]);
  const dossierVersion = JSON.stringify(data?.dossier?.filter((entry) => entry.kind === MemoryKind.CustomerProfile));
  useEffect(() => { if (dossierVersion && onProfileChanged) void onProfileChanged(); }, [dossierVersion, onProfileChanged]);

  async function refresh() {
    if (requesting) return;
    setRequesting(true);
    try {
      await refreshConversationMemory(conversation.id);
      if (alive.current) await load();
    } catch (e) { if (alive.current) setError(e instanceof Error ? e.message : t("conversation.memory.loadFailed")); }
    finally { if (alive.current) setRequesting(false); }
  }

  const pending = data?.status === MemoryStatus.Queued || data?.status === MemoryStatus.Processing;
  if (tagsOnly) {
    const dossier = conversation.customerId ? data?.dossier ?? data?.entries : data?.entries;
    const tags = dossier?.filter((entry) => entry.kind === MemoryKind.CustomerTag) ?? [];
    const profile = dossier?.filter((entry) => entry.kind === MemoryKind.CustomerProfile && entry.value) ?? [];
    return <section className="flex min-w-0 flex-col gap-3" aria-label={t("conversation.customerProfile.title")}>
      <Separator />
      <div className="flex flex-wrap items-center justify-between gap-2">
        <h3 className="text-sm font-semibold">{t("conversation.customerProfile.title")}</h3>
        {canEdit && <Button size="sm" variant="ghost" disabled={requesting || pending} onClick={() => void refresh()}>
          {requesting || pending ? <LoaderCircleIcon data-icon="inline-start" className="motion-safe:animate-spin" /> : <RefreshCwIcon data-icon="inline-start" />}
          {t(pending || requesting ? "conversation.customerTags.analyzing" : "conversation.customerTags.refresh")}
        </Button>}
      </div>
      <p className="text-xs text-muted-foreground">{t(conversation.customerId ? "conversation.customerProfile.scope" : "conversation.customerTags.scope")}</p>
      {error && <Alert variant="destructive"><AlertDescription>{error}<Button size="sm" variant="outline" onClick={() => void load()}>{t("conversation.memory.retry")}</Button></AlertDescription></Alert>}
      {!data && !error && <Skeleton className="h-8 w-full" />}
      {data?.status === MemoryStatus.Failed && <Alert variant="destructive"><AlertDescription>{t("conversation.customerTags.failed")}</AlertDescription></Alert>}
      {data && !profile.length && <p className="text-sm text-muted-foreground">{t("conversation.customerProfile.empty")}</p>}
      <div className="flex flex-col gap-4">
        {profile.map((entry) => <MemoryItem key={entry.id} entry={entry} canEdit={canEditProfile} onSaved={load} />)}
      </div>
      <Separator />
      <h4 className="text-sm font-semibold">{t("conversation.customerTags.title")}</h4>
      {data && !tags.length && data.status !== MemoryStatus.Failed && <p className="text-sm text-muted-foreground" role="status">{t(pending ? "conversation.customerTags.pending" : "conversation.customerTags.empty")}</p>}
      <div className="flex flex-col gap-4">
        {tags.map((entry) => <MemoryItem key={entry.id} entry={entry} canEdit={canEdit} onSaved={load} />)}
      </div>
    </section>;
  }
  return <section className="flex min-w-0 flex-col gap-4" aria-label={t("conversation.memory.title")}>
    <Separator />
    <div className="flex flex-wrap items-center justify-between gap-2">
      <div className="flex min-w-0 flex-col gap-1">
        <h3 className="text-base font-semibold">{t("conversation.memory.title")}</h3>
        {data && <div className="flex flex-wrap items-center gap-x-2 gap-y-1 text-xs text-muted-foreground" role="status">
          <span>{t(`conversation.memory.status.${data.status}`)}</span>
          {data.updatedAt && <time dateTime={data.updatedAt}>{formatDateTime(data.updatedAt)}</time>}
        </div>}
      </div>
      {canEdit && <Button size="sm" variant="ghost" disabled={requesting || pending} onClick={() => void refresh()}>
        {pending || requesting ? <LoaderCircleIcon data-icon="inline-start" className="motion-safe:animate-spin" /> : <RefreshCwIcon data-icon="inline-start" />}
        {t(pending ? "conversation.memory.organizing" : "conversation.memory.organize")}
      </Button>}
    </div>
    {error && <div role="alert" className="flex flex-col gap-2 text-sm text-destructive">
      <p>{error}</p><Button size="sm" variant="outline" onClick={() => void load()}>{t("conversation.memory.retry")}</Button>
    </div>}
    {!data && !error && <div className="flex flex-col gap-2" aria-label={t("conversation.memory.loading")}><Skeleton className="h-4 w-full" /><Skeleton className="h-4 w-3/4" /></div>}
    {data && <>
      {data.status === MemoryStatus.Failed && <p role="alert" className="text-sm text-destructive">{t(data.errorCode === "model_unavailable" ? "conversation.memory.modelUnavailable" : "conversation.memory.extractionFailed")}</p>}
      {data.handoffReason && <div className="flex flex-col gap-1"><h4 className="text-xs font-medium text-muted-foreground">{t("conversation.memory.handoffReason")}</h4><p className="whitespace-pre-wrap break-words text-sm">{data.handoffReason}</p></div>}
      {!data.entries.some((entry) => entry.kind !== MemoryKind.CustomerTag && entry.kind !== MemoryKind.CustomerProfile) && <p className="text-sm text-muted-foreground">{t(pending ? "conversation.memory.preparing" : "conversation.memory.empty")}</p>}
      {data.entries.some((entry) => entry.kind === MemoryKind.Summary) && <div className="flex flex-col gap-2 pb-1">
        <h4 className="text-sm font-semibold">{t("conversation.memory.kind.summary")}</h4>
        {data.entries.filter((entry) => entry.kind === MemoryKind.Summary).map((entry) => <MemoryItem key={entry.id} entry={entry} canEdit={canEdit} onSaved={load} summary />)}
      </div>}
      <div className="flex flex-col divide-y divide-border">
      {!!data.entries.filter((entry) => entry.confirmed && entry.kind === MemoryKind.Customer).length && <MemoryGroup title={t("copilot.confirmed")} entries={data.entries.filter((entry) => entry.confirmed && entry.kind === MemoryKind.Customer)} canEdit={canEdit} onSaved={load} defaultOpen />}
      <InquiryGroups data={data} canEdit={canEdit} onSaved={load} />
      {kinds.map((kind) => {
        const entries = data.entries.filter((entry) => entry.kind === kind && (kind !== MemoryKind.Customer || !entry.confirmed));
        if (!entries.length) return null;
        return <MemoryGroup key={kind} title={t(`conversation.memory.kind.${kind}`)} entries={entries} canEdit={canEdit} onSaved={load} defaultOpen={kind === MemoryKind.OpenQuestion || kind === MemoryKind.NextStep} />;
      })}
      {!!data.shared.length && <MemoryGroup title={t("conversation.memory.shared")} entries={data.shared} canEdit={false} onSaved={load} />}
      </div>
      <Separator />
    </>}
  </section>;
}

function InquiryGroups({ data, canEdit, onSaved }: { data: ConversationMemory; canEdit: boolean; onSaved: () => Promise<void> }) {
  const t = useI18n();
  const fields = data.receptionPolicy?.enabled ? data.receptionPolicy.fields : [];
  const inquiries = data.entries.filter((entry) => entry.kind === MemoryKind.Inquiry);
  const topics = [...new Set(inquiries.map((entry) => entry.topic))];
  if (!topics.length) {
    return fields.length ? <div className="flex flex-col gap-2 py-3">
      <h4 className="text-sm font-semibold">{t("reception.inquiry")}</h4>
      <p className="text-sm text-muted-foreground">{t("reception.noInquiry")}</p>
    </div> : null;
  }
  return topics.map((topic) => {
    const entries = inquiries.filter((entry) => entry.topic === topic);
    const missing = fields.filter((field) => !entries.some((entry) => entry.fieldKey === field.key));
    return <div key={topic} className="flex min-w-0 flex-col gap-4 py-4">
      <h4 className="break-words text-sm font-semibold">{topic || t("reception.inquiry")}</h4>
      {entries.map((entry) => <MemoryItem key={entry.id} entry={entry} canEdit={canEdit} onSaved={onSaved} />)}
      {!!missing.length && <div className="flex flex-col gap-2">
        <h5 className="text-xs font-medium text-muted-foreground">{t("reception.missing")}</h5>
        {missing.map((field) => <div key={field.key} className="flex flex-wrap items-center justify-between gap-2 text-sm">
          <span className="min-w-0 break-words">{field.label}</span>
          {field.required && <Badge variant="outline">{t("reception.priority")}</Badge>}
        </div>)}
      </div>}
    </div>;
  });
}

function MemoryGroup({ title, entries, canEdit, onSaved, defaultOpen = false }: { title: string; entries: MemoryEntry[]; canEdit: boolean; onSaved: () => Promise<void>; defaultOpen?: boolean }) {
  return <Collapsible defaultOpen={defaultOpen || entries.some((entry) => drafts.has(entry.id))}>
    <h4>
      <CollapsibleTrigger className="group flex min-h-11 w-full items-center gap-2 py-3 text-left text-sm font-semibold outline-none focus-visible:ring-2 focus-visible:ring-ring">
        <ChevronRightIcon className="size-3.5 shrink-0 text-muted-foreground group-data-panel-open:rotate-90" />
        <span className="min-w-0 flex-1 break-words">{title}</span>
        <span className="text-xs font-normal tabular-nums text-muted-foreground">{entries.length}</span>
      </CollapsibleTrigger>
    </h4>
    <CollapsibleContent keepMounted>
      <div className="flex flex-col gap-5 pb-5 pt-1">
        {entries.map((entry) => <MemoryItem key={entry.id} entry={entry} canEdit={canEdit} onSaved={onSaved} />)}
      </div>
    </CollapsibleContent>
  </Collapsible>;
}

function MemoryItem({ entry, canEdit, onSaved, summary = false }: { entry: MemoryEntry; canEdit: boolean; onSaved: () => Promise<void>; summary?: boolean }) {
  const t = useI18n();
  const tag = entry.kind === MemoryKind.CustomerTag;
  const profile = entry.kind === MemoryKind.CustomerProfile;
  const label = profile ? t(`conversation.customerProfile.field.${entry.topic}`) : entry.label;
  const itemText = (key: string, values?: Record<string, string | number>) => t(`conversation.${tag ? "customerTags" : profile ? "customerProfile" : "memory"}.${key}`, values);
  const maxLength = tag ? 40 : profile ? ({ name: 100, company: 200, email: 100, phone: 32, region: 200 }[entry.topic] ?? 200) : 1500;
  const confirm = useConfirm();
  const [editing, setEditing] = useState(() => drafts.has(entry.id));
  const [value, setValue] = useState(() => drafts.get(entry.id)?.value ?? entry.value);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState("");

  useEffect(() => {
    if (!editing) return;
    const warn = (e: BeforeUnloadEvent) => { if (drafts.has(entry.id)) { e.preventDefault(); e.returnValue = ""; } };
    window.addEventListener("beforeunload", warn);
    return () => window.removeEventListener("beforeunload", warn);
  }, [editing, entry.id]);

  async function save(remove = false, confirmOnly = false) {
    if (saving) return;
    if (remove && !await confirm({ title: itemText("removeTitle"), description: itemText("removeDescription", { label: tag ? entry.value : label }), confirmText: itemText("remove"), variant: "destructive" })) return;
    setSaving(true); setError("");
    try {
      const original = drafts.get(entry.id)?.entry ?? entry;
      await updateConversationMemory(entry.conversationId, original, confirmOnly ? entry.value : value, remove);
      drafts.delete(entry.id); setEditing(false);
      toast.success(itemText(remove ? "removed" : "saved"));
      await onSaved();
    } catch (e) { setError(e instanceof Error ? e.message : itemText("saveFailed")); }
    finally { setSaving(false); }
  }

  async function cancel() {
    if (value !== entry.value && !await confirm({ title: t("conversation.memory.discardTitle"), description: itemText("discardDescription"), confirmText: t("conversation.memory.discard") })) return;
    drafts.delete(entry.id); setEditing(false); setError("");
  }

  return <article className="flex min-w-0 flex-col gap-2" aria-label={tag ? entry.value : label}>
    {tag ? <span className="text-xs text-muted-foreground">{t(`conversation.customerTags.category.${entry.topic}`)}</span> : !summary && <h5 className="break-words text-sm font-medium">{label}</h5>}
    {editing && canEdit ? <FieldGroup>
      <Field data-invalid={!!error}>
        <FieldLabel htmlFor={`memory-${entry.id}`} className="sr-only">{label}</FieldLabel>
        <Textarea id={`memory-${entry.id}`} value={value} rows={tag || profile ? 2 : 4} maxLength={maxLength} disabled={saving} aria-invalid={!!error} onChange={(e) => {
          setValue(e.target.value); setError("");
          drafts.set(entry.id, { value: e.target.value, entry: drafts.get(entry.id)?.entry ?? entry });
        }} />
      </Field>
      <div className="flex flex-wrap gap-2">
        <Button size="sm" disabled={saving || !value.trim()} onClick={() => void save()}>{t(saving ? "conversation.memory.saving" : "conversation.memory.save")}</Button>
        <Button size="sm" variant="outline" disabled={saving} onClick={() => void cancel()}>{t("conversation.memory.cancel")}</Button>
      </div>
    </FieldGroup> : tag ? <Badge variant="secondary" className="max-w-full self-start whitespace-normal break-words">{entry.value}</Badge> : <p className="whitespace-pre-wrap break-words text-sm leading-relaxed">{entry.value}</p>}
    {error && <p role="alert" className="text-xs text-destructive">{error}</p>}
    <div className="flex min-h-7 flex-wrap items-center gap-2 text-xs text-muted-foreground">
      <span className="inline-flex items-center gap-1">{entry.confirmed && <CheckIcon className="size-3" />}{t(entry.confirmed ? "conversation.memory.confirmed" : "conversation.memory.unconfirmed")}</span>
      {canEdit && !editing && <div className="ml-auto">
        <DropdownMenu>
          <Tooltip><TooltipTrigger render={<DropdownMenuTrigger render={<Button size="icon-sm" variant="ghost" aria-label={itemText("actions", { label: tag ? entry.value : label })} disabled={saving} />} />}>
            {saving ? <LoaderCircleIcon className="motion-safe:animate-spin" /> : <MoreHorizontalIcon />}
          </TooltipTrigger><TooltipContent>{itemText("manage")}</TooltipContent></Tooltip>
          <DropdownMenuContent align="end" className="w-44">
            <DropdownMenuGroup>
              {!entry.confirmed && <DropdownMenuItem onClick={() => void save(false, true)}><CheckIcon />{t("conversation.memory.confirm")}</DropdownMenuItem>}
              <DropdownMenuItem onClick={() => { setValue(entry.value); drafts.set(entry.id, { value: entry.value, entry }); setEditing(true); }}><PencilIcon />{itemText("edit")}</DropdownMenuItem>
            </DropdownMenuGroup>
            <DropdownMenuSeparator />
            <DropdownMenuGroup><DropdownMenuItem variant="destructive" onClick={() => void save(true)}><Trash2Icon />{itemText("remove")}</DropdownMenuItem></DropdownMenuGroup>
          </DropdownMenuContent>
        </DropdownMenu>
      </div>}
    </div>
    <details className="text-xs">
      <summary className="w-fit cursor-pointer text-muted-foreground outline-none focus-visible:ring-2 focus-visible:ring-ring">{t("conversation.memory.sources", { count: entry.sources.length })}</summary>
      <div className="mt-3 flex flex-col gap-3 bg-muted/40 p-3">
        {entry.topic && !tag && !profile && <p className="break-words font-medium">{entry.topic}</p>}
        <p className="text-muted-foreground">{t("conversation.memory.updatedAt", { time: formatDateTime(entry.updatedAt) })}</p>
        {entry.sources.map((source) => <div key={source.id} className="flex min-w-0 flex-col gap-1">
          <span className="text-muted-foreground">{t("conversation.memory.sourceMessage", { id: source.id, conversationId: source.conversationId })} · {t(source.senderType === "customer" ? "conversation.memory.customerSpeaker" : "conversation.memory.assistantSpeaker")}</span>
          <span className="text-muted-foreground">{formatDateTime(source.createdAt)}</span>
          <p className="whitespace-pre-wrap break-words leading-relaxed">{source.content}</p>
        </div>)}
      </div>
    </details>
  </article>;
}
