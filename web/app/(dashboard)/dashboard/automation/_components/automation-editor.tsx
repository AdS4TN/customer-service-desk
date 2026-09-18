"use client";

import { useEffect, useState } from "react";
import { useForm, useWatch } from "react-hook-form";
import { PlusIcon, Trash2Icon, PlayIcon, SaveIcon, RefreshCwIcon } from "lucide-react";
import { toast } from "sonner";
import { useI18n } from "@/i18n/provider";
import { useAuth } from "@/components/auth-provider";
import { useConfirm } from "@/components/confirm-provider";
import { ProjectDialog } from "@/components/project-dialog";
import { OptionCombobox, type ComboboxOption } from "@/components/option-combobox";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Textarea } from "@/components/ui/textarea";
import { Switch } from "@/components/ui/switch";
import { Badge } from "@/components/ui/badge";
import { Field, FieldGroup, FieldLabel, FieldSet, FieldLegend, FieldError } from "@/components/ui/field";
import { fetchAgentProfilesAll } from "@/lib/api/admin";
import { saveAutomation, testAutomation, type AutomationDraft, type AutomationDefinition } from "@/lib/api/automation";

export function AutomationEditor({ initial, onClose, onSaved }: { initial: AutomationDraft; onClose: () => void; onSaved: () => void }) {
  const t = useI18n();
  const confirm = useConfirm();
  const { session } = useAuth();
  const form = useForm<AutomationDraft>({ defaultValues: structuredClone(initial) });
  const d = useWatch({ control: form.control, name: "definition" });
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState("");
  const [owners, setOwners] = useState<ComboboxOption[]>([]);
  const [ownerError, setOwnerError] = useState(false);
  const [ownerAttempt, setOwnerAttempt] = useState(0);
  const [sample, setSample] = useState("");
  const [channel, setChannel] = useState("web");
  const [leadTags, setLeadTags] = useState<string[]>([]);
  const [testing, setTesting] = useState(false);
  const [testResult, setTestResult] = useState<{ matched: boolean; conditions: boolean[]; key: string } | null>(null);
  const [testError, setTestError] = useState("");
  const sampleKey = JSON.stringify([d, sample, channel, leadTags]);
  const currentResult = testResult?.key === sampleKey ? testResult : null;
  const options = (codes: string[], prefix: string) => codes.map(value => ({ value, label: t(`automation.${prefix}.${value}`) }));
  const change = (patch: Partial<AutomationDefinition>) => { if (saving) return; form.setValue("definition", { ...d, ...patch }, { shouldDirty: true }); form.clearErrors("definition"); setError(""); };
  useEffect(() => {
    let active = true;
    fetchAgentProfilesAll().then(rows => {
      if (!active) return;
      const choices = rows.map(u => ({ value: String(u.userId), label: u.displayName || u.nickname || u.username || String(u.userId) }));
      if (session?.user.id && !choices.some(c => c.value === String(session.user.id))) choices.push({ value: String(session.user.id), label: session.user.nickname || session.user.username });
      setOwners(choices); setOwnerError(false);
    }).catch(() => { if (active) setOwnerError(true); });
    return () => { active = false; };
  }, [ownerAttempt, session?.user.id, session?.user.nickname, session?.user.username]);
  useEffect(() => {
    if (!form.formState.isDirty) return;
    const warn = (event: BeforeUnloadEvent) => { event.preventDefault(); };
    window.addEventListener("beforeunload", warn);
    return () => window.removeEventListener("beforeunload", warn);
  }, [form.formState.isDirty]);
  async function close() {
    if (saving || testing) return;
    if (form.formState.isDirty && !(await confirm({ title: t("automation.discardTitle"), description: t("automation.discardBody"), confirmText: t("automation.discard"), cancelText: t("automation.keepEditing") }))) return;
    onClose();
  }
  async function save(values: AutomationDraft) {
    if (saving) return;
    if (!validateOwners(values.definition)) return;
    setSaving(true); setError("");
    try { await saveAutomation(values); toast.success(t("automation.saved")); onSaved(); }
    catch (e) { setError(e instanceof Error ? e.message : t("automation.failed")); }
    finally { setSaving(false); }
  }
  async function test() {
    if (!validateOwners(d)) return;
    setTesting(true); setTestError(""); setTestResult(null);
    const key = sampleKey;
    try { setTestResult({ ...await testAutomation(d, channel, sample, leadTags), key }); }
    catch (e) { setTestError(e instanceof Error ? e.message : t("automation.failed")); }
    finally { setTesting(false); }
  }
  function validateOwners(definition: AutomationDefinition) {
    const index = definition.actions.findIndex(a => a.type === "assign_lead" && a.ownerId <= 0);
    if (index < 0) return true;
    form.setError(`definition.actions.${index}.ownerId`, { type: "required", message: t("automation.ownerRequired") });
    return false;
  }
  async function trigger(value: string) {
    if (value === d.trigger) return;
    if (!(await confirm({ title: t("automation.changeTrigger"), description: t("automation.changeTriggerBody"), confirmText: t("common.confirm"), cancelText: t("common.cancel") }))) return;
    change({ trigger: value, conditions: [], actions: [{ type: value === "lead_created" ? "tag_lead" : "extract_lead", value: "", ownerId: 0 }], oncePerConversation: false });
  }
  return <ProjectDialog open title={t(initial.id ? "automation.edit" : "automation.create")} size="xl" onOpenChange={open => { if (!open) void close(); }}
    footer={<><Button variant="outline" disabled={saving || testing} onClick={() => void close()}>{t("common.cancel")}</Button><Button type="submit" form="automation-editor" disabled={saving || testing}><SaveIcon data-icon="inline-start" />{t(saving ? "automation.saving" : "automation.saveDraft")}</Button></>}>
    <form id="automation-editor" onSubmit={form.handleSubmit(save)} className="flex flex-col gap-6">
      <fieldset disabled={saving} className="flex min-w-0 flex-col gap-6">
      <FieldGroup className="sm:grid sm:grid-cols-[1fr_9rem]">
        <Field data-invalid={!!form.formState.errors.name}><FieldLabel htmlFor="automation-name">{t("automation.name")}</FieldLabel><Input id="automation-name" maxLength={120} aria-invalid={!!form.formState.errors.name} {...form.register("name", { required: t("automation.required"), validate: v => !!v.trim() || t("automation.required") })} /><FieldError>{form.formState.errors.name?.message}</FieldError></Field>
        <Field data-invalid={!!form.formState.errors.priority}><FieldLabel htmlFor="automation-priority">{t("automation.priority")}</FieldLabel><Input id="automation-priority" type="number" min={0} max={10000} step={1} aria-invalid={!!form.formState.errors.priority} {...form.register("priority", { valueAsNumber: true, required: true, min: 0, max: 10000 })} /><FieldError>{form.formState.errors.priority && t("automation.priorityInvalid")}</FieldError></Field>
      </FieldGroup>
      <FieldSet><FieldLegend>{t("automation.triggerTitle")}</FieldLegend><FieldGroup>
        <Field><FieldLabel htmlFor="automation-trigger">{t("automation.trigger")}</FieldLabel><OptionCombobox id="automation-trigger" value={d.trigger} options={options(["message_received", "lead_created"], "triggers")} placeholder={t("automation.trigger")} onChange={v => void trigger(v)} /></Field>
        {d.trigger === "message_received" && <Field orientation="horizontal"><FieldLabel htmlFor="automation-once">{t("automation.once")}</FieldLabel><Switch id="automation-once" checked={d.oncePerConversation} disabled={d.actions.some(a => a.type === "create_ticket")} onCheckedChange={v => change({ oncePerConversation: v })} /></Field>}
      </FieldGroup></FieldSet>
      <FieldSet><FieldLegend>{t("automation.conditions")}</FieldLegend><FieldGroup>
        <Field><FieldLabel htmlFor="automation-match">{t("automation.match")}</FieldLabel><OptionCombobox id="automation-match" value={d.match} options={options(["all", "any"], "matches")} placeholder={t("automation.match")} onChange={v => change({ match: v })} /></Field>
        {d.conditions.length === 0 && <p className="text-sm text-muted-foreground">{t("automation.allCustomers")}</p>}
        {d.conditions.map((c, i) => <div key={i} className="flex items-end gap-2">
          <FieldGroup className="flex-1 sm:grid sm:grid-cols-2">
            <Field><FieldLabel htmlFor={`condition-field-${i}`}>{t("automation.conditionField", { number: i + 1 })}</FieldLabel><OptionCombobox id={`condition-field-${i}`} value={c.field} options={options(d.trigger === "lead_created" ? ["channel", "text", "lead_tag"] : ["channel", "text"], "fields")} placeholder={t("automation.conditions")} onChange={v => change({ conditions: d.conditions.map((item, j) => j === i ? { field: v, value: "" } : item) })} /></Field>
            <Field><FieldLabel htmlFor={`condition-value-${i}`}>{t(c.field === "text" ? "automation.contains" : "automation.equals")}</FieldLabel>{c.field === "text" ? <Input id={`condition-value-${i}`} required maxLength={200} value={c.value} onChange={e => change({ conditions: d.conditions.map((item, j) => j === i ? { ...item, value: e.target.value } : item) })} /> : <OptionCombobox id={`condition-value-${i}`} value={c.value} options={options(c.field === "channel" ? ["web", "whatsapp", "messenger"] : ["quote", "sample", "bulk", "urgent", "contact"], c.field === "channel" ? "channels" : "tags")} placeholder={t("automation.selectValue")} onChange={v => change({ conditions: d.conditions.map((item, j) => j === i ? { ...item, value: v } : item) })} />}</Field>
          </FieldGroup>
          <Button type="button" variant="ghost" size="icon" title={t("automation.removeCondition")} aria-label={t("automation.removeCondition")} onClick={() => change({ conditions: d.conditions.filter((_, j) => i !== j) })}><Trash2Icon /></Button>
        </div>)}
        <Button type="button" variant="outline" className="self-start" disabled={d.conditions.length >= 20} onClick={() => change({ conditions: [...d.conditions, { field: "text", value: "" }] })}><PlusIcon data-icon="inline-start" />{t("automation.addCondition")}</Button>
      </FieldGroup></FieldSet>
      <FieldSet><FieldLegend>{t("automation.actionsTitle")}</FieldLegend><FieldGroup>
        {d.actions.map((a, i) => <div key={i} className="flex items-end gap-2"><FieldGroup className="flex-1">
          <Field><FieldLabel htmlFor={`action-type-${i}`}>{t("automation.actionNumber", { number: i + 1 })}</FieldLabel><OptionCombobox id={`action-type-${i}`} value={a.type} options={options(d.trigger === "lead_created" ? ["tag_lead", "assign_lead"] : ["extract_lead", "create_ticket"], "actions").map(o => ({ ...o, disabled: o.value !== a.type && d.actions.some(x => x.type === o.value) }))} placeholder={t("automation.actionsTitle")} onChange={v => change({ actions: d.actions.map((item, j) => j === i ? { type: v, value: "", ownerId: 0 } : item), oncePerConversation: v === "create_ticket" || d.oncePerConversation })} /></Field>
          {(a.type === "tag_lead" || a.type === "create_ticket") && <Field><FieldLabel htmlFor={`action-value-${i}`}>{t(a.type === "tag_lead" ? "automation.tagName" : "automation.ticketTitle")}</FieldLabel><Input id={`action-value-${i}`} required maxLength={a.type === "tag_lead" ? 40 : 120} value={a.value} onChange={e => change({ actions: d.actions.map((item, j) => j === i ? { ...item, value: e.target.value } : item) })} /></Field>}
          {(a.type === "assign_lead" || a.type === "create_ticket") && <Field data-invalid={!!form.formState.errors.definition?.actions?.[i]?.ownerId}><FieldLabel htmlFor={`action-owner-${i}`}>{t(a.type === "assign_lead" ? "automation.owner" : "automation.ticketOwner")}</FieldLabel><OptionCombobox id={`action-owner-${i}`} value={a.ownerId ? String(a.ownerId) : a.type === "create_ticket" ? "0" : ""} options={[...(a.type === "create_ticket" ? [{ value: "0", label: t("automation.unassigned") }] : []), ...owners]} placeholder={t("automation.selectOwner")} preserveExternalSelection aria-invalid={!!form.formState.errors.definition?.actions?.[i]?.ownerId} aria-describedby={`action-owner-error-${i}`} onChange={v => change({ actions: d.actions.map((item, j) => j === i ? { ...item, ownerId: Number(v) } : item) })} /><FieldError id={`action-owner-error-${i}`}>{form.formState.errors.definition?.actions?.[i]?.ownerId?.message}</FieldError></Field>}
        </FieldGroup><Button type="button" variant="ghost" size="icon" disabled={d.actions.length === 1} title={t("automation.removeAction")} aria-label={t("automation.removeAction")} onClick={() => change({ actions: d.actions.filter((_, j) => i !== j) })}><Trash2Icon /></Button></div>)}
        {ownerError && <FieldError>{t("automation.ownersFailed")} <Button type="button" variant="ghost" size="icon-sm" title={t("automation.reload")} aria-label={t("automation.reload")} onClick={() => setOwnerAttempt(n => n + 1)}><RefreshCwIcon /></Button></FieldError>}
        <Button type="button" variant="outline" className="self-start" disabled={d.actions.length >= 2} onClick={() => { const type = (d.trigger === "lead_created" ? ["tag_lead", "assign_lead"] : ["extract_lead", "create_ticket"]).find(type => !d.actions.some(a => a.type === type)); if (type) change({ actions: [...d.actions, { type, value: "", ownerId: 0 }], oncePerConversation: type === "create_ticket" || d.oncePerConversation }); }}><PlusIcon data-icon="inline-start" />{t("automation.addAction")}</Button>
      </FieldGroup></FieldSet>
      <FieldSet><FieldLegend>{t("automation.testTitle")}</FieldLegend><FieldGroup>
        <Field><FieldLabel htmlFor="sample-channel">{t("automation.fields.channel")}</FieldLabel><OptionCombobox id="sample-channel" value={channel} options={options(["web", "whatsapp", "messenger"], "channels")} placeholder={t("automation.fields.channel")} onChange={setChannel} /></Field>
        <Field><FieldLabel htmlFor="sample-text">{t("automation.sampleText")}</FieldLabel><Textarea id="sample-text" maxLength={20000} value={sample} onChange={e => setSample(e.target.value)} /></Field>
        {d.trigger === "lead_created" && <Field><FieldLabel htmlFor="sample-tags">{t("automation.fields.lead_tag")}</FieldLabel><OptionCombobox id="sample-tags" multiple values={leadTags} onValuesChange={setLeadTags} options={options(["quote", "sample", "bulk", "urgent", "contact"], "tags")} placeholder={t("automation.selectValue")} /></Field>}
        <div className="flex flex-wrap items-center gap-3"><Button type="button" variant="outline" disabled={testing || saving} onClick={() => void test()}><PlayIcon data-icon="inline-start" />{t(testing ? "automation.testing" : "automation.test")}</Button>{currentResult && <Badge variant={currentResult.matched ? "secondary" : "outline"}>{t(currentResult.matched ? "automation.matched" : "automation.notMatched")}</Badge>}</div>
        {currentResult && currentResult.conditions.length > 0 && <ul className="flex flex-col gap-1 text-sm">{currentResult.conditions.map((hit, i) => <li key={i}>{t("automation.conditionResult", { number: i + 1, result: t(hit ? "automation.matched" : "automation.notMatched") })}</li>)}</ul>}
        {testError && <FieldError>{testError}</FieldError>}
      </FieldGroup></FieldSet>
      </fieldset>
      {error && <FieldError>{error}</FieldError>}
    </form>
  </ProjectDialog>;
}
