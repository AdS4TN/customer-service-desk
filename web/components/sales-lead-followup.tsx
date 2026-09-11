"use client";

import { useEffect, useState } from "react";
import { Controller, useForm, useWatch } from "react-hook-form";
import { MessageSquareIcon, RefreshCwIcon } from "lucide-react";
import { useRouter } from "next/navigation";
import { toast } from "sonner";
import { useAuth } from "@/components/auth-provider";
import { useConfirm } from "@/components/confirm-provider";
import { OptionCombobox, type ComboboxOption } from "@/components/option-combobox";
import { Alert, AlertDescription } from "@/components/ui/alert";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Field, FieldGroup, FieldLabel, FieldError } from "@/components/ui/field";
import { Input } from "@/components/ui/input";
import { Textarea } from "@/components/ui/textarea";
import { useI18n } from "@/i18n/provider";
import { fetchAgentProfilesAll } from "@/lib/api/admin";
import { recordLeadFollowUp, type SalesLead } from "@/lib/api/sales-lead";
import { LeadStatus } from "@/lib/generated/enums";
import { useAgentConversationsStore } from "@/lib/stores/agent-conversations";
import { formatDateTime } from "@/lib/utils";

export function leadIsClosed(status: LeadStatus) {
  return status === LeadStatus.Won || status === LeadStatus.Lost || status === LeadStatus.Archived;
}

export function LeadHistory({ item }: { item: SalesLead }) {
  const t = useI18n();
  return <details className="text-sm">
    <summary className="cursor-pointer font-medium">{t("lead.history")}</summary>
    <ol className="flex flex-col gap-4 py-3">
      {item.events.map((event) => <li key={event.id} className="flex min-w-0 flex-col gap-1">
        <p className="text-muted-foreground">{event.followUp ? t("leadWork.recorded") : t(`lead.events.${event.kind}`)} · {formatDateTime(event.createdAt)}{event.actorId ? ` · #${event.actorId}` : ""}</p>
        {event.followUp && <>
          <p>{t(`lead.statuses.${event.followUp.status}`)} · {t("lead.owner")}: #{event.followUp.ownerId}</p>
          <p className="whitespace-pre-wrap break-words">{event.followUp.result}</p>
          {event.followUp.nextAction && <p className="whitespace-pre-wrap break-words">{t("leadWork.nextAction")}: {event.followUp.nextAction}</p>}
          {event.followUp.followUpAt && <p>{t("lead.followUp")}: {formatDateTime(event.followUp.followUpAt)}</p>}
        </>}
      </li>)}
    </ol>
  </details>;
}

type Values = { status: LeadStatus; ownerId: string; result: string; nextAction: string; followUpAt: string };

export function LeadFollowUpForm({ item, onDirty, onSaving, onClose, onSaved }: {
  item: SalesLead; onDirty: (dirty: boolean) => void; onSaving: (saving: boolean) => void; onClose: () => void; onSaved: () => void;
}) {
  const t = useI18n();
  const router = useRouter();
  const confirm = useConfirm();
  const { session } = useAuth();
  const canEdit = !!session?.permissions.some((p) => p === "*" || p === "conversation.send");
  const [owners, setOwners] = useState<ComboboxOption[]>([]);
  const [ownerError, setOwnerError] = useState(false);
  const [attempt, setAttempt] = useState(0);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState("");
  const form = useForm<Values>({ defaultValues: {
    status: item.status === LeadStatus.New ? LeadStatus.Following : item.status,
    ownerId: item.ownerId ? String(item.ownerId) : "",
    result: "", nextAction: item.nextAction || "", followUpAt: "",
  }});
  const { register, control, handleSubmit, formState: { isDirty, errors } } = form;
  const status = useWatch({ control, name: "status" });
  const closed = leadIsClosed(status);
  useEffect(() => { onDirty(isDirty); }, [isDirty, onDirty]);
  useEffect(() => {
    let active = true;
    if (canEdit) fetchAgentProfilesAll().then((items) => {
      if (!active) return;
      setOwners(items.map((u) => ({ value: String(u.userId), label: u.displayName || u.nickname || u.username || String(u.userId) })));
      setOwnerError(false);
    }).catch(() => { if (active) setOwnerError(true); });
    return () => { active = false; };
  }, [canEdit, attempt]);
  const options = [...owners];
  if (session?.user.id && !options.some((o) => o.value === String(session.user.id))) options.push({ value:String(session.user.id), label:session.user.nickname || session.user.username });
  if (item.ownerId && !options.some((o) => o.value === String(item.ownerId))) options.push({ value: String(item.ownerId), label: item.ownerName || String(item.ownerId) });
  async function save(values: Values) {
    if (saving || !canEdit) return;
    if (closed && !(await confirm({ title: t("leadWork.closeTitle", {status:t(`lead.statuses.${values.status}`)}), description:t("leadWork.closeBody"), confirmText:t("leadWork.save"), cancelText:t("lead.keepEditing") }))) return;
    setSaving(true); onSaving(true); setError("");
    try {
      await recordLeadFollowUp(item.id, { revision: item.revision, status: values.status, ownerId:Number(values.ownerId), result:values.result.trim(), nextAction:closed ? "" : values.nextAction.trim(), followUpAt:closed ? null : new Date(values.followUpAt).toISOString() });
      toast.success(t("leadWork.saved")); onSaved();
    } catch (e) { setError(e instanceof Error ? e.message : t("lead.failed")); }
    finally { setSaving(false); onSaving(false); }
  }
  async function openConversation() {
    try {
      await useAgentConversationsStore.getState().selectConversation(item.conversationId);
      onClose(); router.push("/dashboard/conversations");
    } catch (e) { setError(e instanceof Error ? e.message : t("lead.failed")); }
  }
  return <form id="lead-followup-form" className="flex min-w-0 flex-col gap-5 pt-3" onSubmit={handleSubmit(save)}>
    <div className="flex flex-wrap items-start justify-between gap-3">
      <div className="flex min-w-0 flex-1 flex-col gap-1">
        <p className="break-words font-medium">{item.sourceValid ? item.data.title : t("lead.invalidSourceShort")}</p>
        <p className="text-sm text-muted-foreground">{item.customerName} · #{item.id}</p>
        <div className="flex flex-wrap gap-2"><Badge variant="outline">{t(`lead.statuses.${item.status}`)}</Badge><span className="text-sm">{item.ownerName || t("lead.unassigned")}</span></div>
      </div>
      <Button type="button" variant="outline" size="sm" disabled={isDirty || saving} onClick={() => void openConversation()}><MessageSquareIcon data-icon="inline-start" />{t("lead.conversation")}</Button>
    </div>
    {item.followUpAt && !leadIsClosed(item.status) && <p className="text-sm">{t("lead.followUp")}: {formatDateTime(item.followUpAt)}{new Date(item.followUpAt).getTime() <= Date.now() ? ` · ${t("leadWork.overdue")}` : ""}<span className="block whitespace-pre-wrap break-words">{item.nextAction}</span></p>}
    {(!item.sourceValid || item.withdrawn) && <Alert><AlertDescription>{t(!item.sourceValid ? "lead.invalidSource" : "lead.withdrawn")}</AlertDescription></Alert>}
    {error && <Alert variant="destructive"><AlertDescription>{error}</AlertDescription></Alert>}
    {ownerError && <Alert variant="destructive"><AlertDescription>{t("lead.ownersFailed")}<Button type="button" variant="outline" size="sm" onClick={() => setAttempt((v) => v+1)}><RefreshCwIcon data-icon="inline-start" />{t("lead.retry")}</Button></AlertDescription></Alert>}
    {canEdit && <>
      <FieldGroup className="grid grid-cols-1 gap-4 sm:grid-cols-2">
        <Field data-invalid={!!errors.ownerId}>
          <FieldLabel id="follow-owner-label" htmlFor="follow-owner">{t("lead.owner")}</FieldLabel>
          <Controller control={control} name="ownerId" rules={{required:t("leadWork.ownerRequired")}} render={({field}) => <OptionCombobox id="follow-owner" aria-labelledby="follow-owner-label" aria-invalid={!!errors.ownerId} aria-describedby={errors.ownerId ? "follow-owner-error" : undefined} value={field.value} onChange={field.onChange} disabled={saving} placeholder={t("leadWork.ownerRequired")} options={options} />} />
          {errors.ownerId && <FieldError id="follow-owner-error">{errors.ownerId.message}</FieldError>}
        </Field>
        <Field><FieldLabel id="follow-status-label" htmlFor="follow-status">{t("lead.status")}</FieldLabel><Controller control={control} name="status" render={({field}) => <OptionCombobox id="follow-status" aria-labelledby="follow-status-label" value={field.value} onChange={field.onChange} disabled={saving} placeholder={t("lead.status")} options={Object.values(LeadStatus).map((value) => ({value, label:t(`lead.statuses.${value}`)}))} />} /></Field>
        <Field className="sm:col-span-2" data-invalid={!!errors.result}>
          <FieldLabel htmlFor="follow-result">{t(closed ? "leadWork.outcome" : "leadWork.result")}</FieldLabel>
          <Textarea id="follow-result" rows={3} maxLength={2000} disabled={saving} aria-invalid={!!errors.result} aria-describedby={errors.result ? "follow-result-error" : undefined} {...register("result", {validate:(v) => !!v.trim() || t("leadWork.resultRequired")})} />
          {errors.result && <FieldError id="follow-result-error">{errors.result.message}</FieldError>}
        </Field>
        {!closed && <>
          <Field data-invalid={!!errors.nextAction}>
            <FieldLabel htmlFor="follow-next">{t("leadWork.nextAction")}</FieldLabel>
            <Input id="follow-next" maxLength={500} disabled={saving} aria-invalid={!!errors.nextAction} aria-describedby={errors.nextAction ? "follow-next-error" : undefined} {...register("nextAction", {validate:(v) => leadIsClosed(form.getValues("status")) || !!v.trim() || t("leadWork.nextRequired")})} />
            {errors.nextAction && <FieldError id="follow-next-error">{errors.nextAction.message}</FieldError>}
          </Field>
          <Field data-invalid={!!errors.followUpAt}>
            <FieldLabel htmlFor="follow-date">{t("lead.followUp")}</FieldLabel>
            <Input id="follow-date" type="datetime-local" disabled={saving} aria-invalid={!!errors.followUpAt} aria-describedby={errors.followUpAt ? "follow-date-error" : undefined} {...register("followUpAt", {validate:(v) => leadIsClosed(form.getValues("status")) || new Date(v).getTime() > Date.now() || t("leadWork.dateRequired")})} />
            {errors.followUpAt && <FieldError id="follow-date-error">{errors.followUpAt.message}</FieldError>}
          </Field>
        </>}
      </FieldGroup>
    </>}
    <LeadHistory item={item} />
  </form>;
}
