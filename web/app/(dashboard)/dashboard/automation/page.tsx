"use client";

import { useState } from "react";
import Link from "next/link";
import { PlusIcon, PencilIcon, Trash2Icon, HistoryIcon, RefreshCwIcon, ExternalLinkIcon } from "lucide-react";
import { toast } from "sonner";
import { useI18n } from "@/i18n/provider";
import { useAuth } from "@/components/auth-provider";
import { useConfirm } from "@/components/confirm-provider";
import { DashboardListPage } from "@/components/dashboard/list";
import { ProjectDialog } from "@/components/project-dialog";
import { OptionCombobox } from "@/components/option-combobox";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { Switch } from "@/components/ui/switch";
import { Tabs, TabsList, TabsTrigger, TabsContent } from "@/components/ui/tabs";
import { automationTemplate, automationResultLink } from "@/lib/automation";
import { fetchAutomationRules, fetchAutomationRuns, changeAutomation, deleteAutomation, retryAutomation, type AutomationDraft, type AutomationRule, type AutomationRun } from "@/lib/api/automation";
import { formatDateTime } from "@/lib/utils";
import { AutomationEditor } from "./_components/automation-editor";

export default function AutomationPage() {
  const t = useI18n();
  const { session } = useAuth();
  const confirm = useConfirm();
  const allowed = (permission: string) => !!session && (session.roles.includes("super_admin") || session.permissions.includes("*") || session.permissions.includes(permission));
  const canEdit = allowed("aiAgent.update") && allowed("conversation.send") && allowed("conversation.view");
  const [tab, setTab] = useState("rules");
  const [reload, setReload] = useState(0);
  const [draft, setDraft] = useState<AutomationDraft | null>(null);
  const [template, setTemplate] = useState("inquiry");
  const [busy, setBusy] = useState(false);
  const [ruleFilter, setRuleFilter] = useState(0);
  const [run, setRun] = useState<AutomationRun | null>(null);
  const refresh = () => setReload(n => n + 1);
  async function mutate(action: () => Promise<void>) {
    if (busy) return;
    setBusy(true);
    try { await action(); refresh(); toast.success(t("automation.updated")); }
    catch (e) { toast.error(e instanceof Error ? e.message : t("automation.failed")); }
    finally { setBusy(false); }
  }
  async function toggle(rule: AutomationRule, enabled: boolean) {
    if (enabled && !(await confirm({ title: t("automation.enableTitle", { name: rule.name }), description: t("automation.enableBody"), confirmText: t("automation.enable"), cancelText: t("common.cancel") }))) return;
    await mutate(() => changeAutomation(rule, enabled));
  }
  async function remove(rule: AutomationRule) {
    if (!(await confirm({ title: t("automation.deleteTitle", { name: rule.name }), description: t("automation.deleteBody"), confirmText: t("automation.delete"), cancelText: t("common.cancel"), variant: "destructive" }))) return;
    await mutate(() => deleteAutomation(rule));
  }
  const labels = { loading: t("common.loading"), empty: t("automation.empty"), loadFailed: t("automation.failed") };
  const resultLabel = (result: string) => { const [code, id] = result.split(":"); return t(`automation.results.${code}`) + (id ? ` #${id}` : ""); };
  return <div className="flex min-w-0 flex-col gap-4 p-4 sm:p-6">
    <h1 className="text-xl font-semibold">{t("automation.title")}</h1>
    <Tabs value={tab} onValueChange={setTab}>
      <TabsList><TabsTrigger value="rules">{t("automation.rules")}</TabsTrigger><TabsTrigger value="runs">{t("automation.runs")}</TabsTrigger></TabsList>
      <TabsContent value="rules">
        <DashboardListPage<AutomationRule> layout="fragment" reloadKey={reload} labels={labels} getItemId={r => r.id}
          filters={[{ name: "keyword", label: t("automation.name"), placeholder: t("automation.search"), defaultValue: "", trim: true }, { name: "status", label: t("automation.status"), defaultValue: "all", type: "select", options: ["all", "enabled", "disabled"].map(value => ({ value, label: t(`automation.statuses.${value}`) })) }]}
          fetchList={async query => {
            const all = await fetchAutomationRules();
            const rows = all.filter(r => r.name.toLowerCase().includes(String(query.keyword || "").toLowerCase()) && (query.status === "enabled" ? r.enabled : query.status === "disabled" ? !r.enabled : true));
            const page = Number(query.page) || 1, limit = Number(query.limit) || 20;
            return { results: rows.slice((page - 1) * limit, page * limit), page: { page, limit, total: rows.length } };
          }}
          renderToolbarActions={() => canEdit && <div className="flex w-full flex-wrap items-center gap-2 sm:w-auto"><OptionCombobox value={template} onChange={setTemplate} options={["inquiry", "lead", ...(allowed("ticket.create") ? ["ticket"] : [])].map(value => ({ value, label: t(`automation.templates.${value}`) }))} placeholder={t("automation.template")} triggerClassName="w-full sm:w-44" /><Button disabled={busy} onClick={() => setDraft(automationTemplate(template, t(`automation.templates.${template}`), t("automation.defaultTag")))}><PlusIcon data-icon="inline-start" />{t("automation.create")}</Button></div>}
          columns={[
            { key: "name", label: t("automation.name"), render: r => <div className="min-w-40 max-w-72 break-words"><div className="font-medium">{r.name}</div><div className="text-sm text-muted-foreground">{t(`automation.triggers.${r.definition.trigger}`)}</div></div> },
            { key: "conditions", label: t("automation.conditions"), render: r => <span>{r.definition.conditions.length ? t("automation.conditionSummary", { count: r.definition.conditions.length, match: t(`automation.matches.${r.definition.match}`) }) : t("automation.allCustomers")}</span> },
            { key: "actions", label: t("automation.actionsTitle"), render: r => <div className="flex flex-col gap-1">{r.definition.actions.map(a => <span key={a.type}>{t(`automation.actions.${a.type}`)}</span>)}</div> },
            { key: "priority", label: t("automation.priority"), render: r => <span className="tabular-nums">{r.priority}</span> },
            { key: "status", label: t("automation.status"), render: r => <div className="flex items-center gap-3"><Switch checked={r.enabled} disabled={!canEdit || busy} aria-label={t(r.enabled ? "automation.disableRule" : "automation.enableRule", { name: r.name })} onCheckedChange={v => void toggle(r, v)} /><span>{t(r.enabled ? "automation.statuses.enabled" : "automation.statuses.disabled")}</span></div> },
            { key: "updated", label: t("automation.updatedAt"), render: r => formatDateTime(r.updatedAt) },
            { key: "actionsMenu", label: t("automation.actionsLabel"), render: r => <div className="flex gap-1"><Button variant="ghost" size="icon" title={t("automation.edit")} aria-label={t("automation.edit")} disabled={!canEdit || busy} onClick={() => setDraft(r)}><PencilIcon /></Button><Button variant="ghost" size="icon" title={t("automation.runs")} aria-label={t("automation.runs")} onClick={() => { setRuleFilter(r.id); setTab("runs"); }}><HistoryIcon /></Button><Button variant="ghost" size="icon" title={t("automation.delete")} aria-label={t("automation.delete")} disabled={!canEdit || busy} onClick={() => void remove(r)}><Trash2Icon /></Button></div> },
          ]} />
      </TabsContent>
      <TabsContent value="runs">
        <DashboardListPage<AutomationRun> layout="fragment" reloadKey={`${reload}:${ruleFilter}`} labels={{ ...labels, empty: t("automation.noRuns") }} getItemId={r => r.id}
          fetchList={q => fetchAutomationRuns(ruleFilter, Number(q.page), Number(q.limit))}
          renderToolbarActions={() => ruleFilter > 0 && <Button variant="outline" onClick={() => setRuleFilter(0)}>{t("automation.allRuns")}</Button>}
          columns={[
            { key: "rule", label: t("automation.name"), render: r => <div><div className="max-w-64 break-words">{r.ruleName}</div><div className="text-muted-foreground">{t("automation.version", { version: r.revision })}</div></div> },
            { key: "conversation", label: t("automation.conversation"), render: r => <Link className="underline underline-offset-4" href={`/dashboard/conversations?conversationId=${r.conversationId}`}>#{r.conversationId}</Link> },
            { key: "status", label: t("automation.status"), render: r => <Badge variant={r.status === "failed" ? "destructive" : "outline"}>{t(`automation.runStatuses.${r.status}`)}</Badge> },
            { key: "result", label: t("automation.result"), render: r => <div className="flex max-w-80 flex-col gap-1 whitespace-normal">{r.errorCode ? t(`automation.errors.${r.errorCode}`) : (r.results ?? []).map((value, i) => <span key={i}>{resultLabel(value)}</span>)}</div> },
            { key: "time", label: t("automation.executedAt"), render: r => formatDateTime(r.updatedAt) },
            { key: "detail", label: t("automation.actionsLabel"), render: r => <Button variant="ghost" onClick={() => setRun(r)}>{t("automation.details")}</Button> },
          ]} />
      </TabsContent>
    </Tabs>
    {draft && <AutomationEditor initial={draft} onClose={() => setDraft(null)} onSaved={() => { setDraft(null); refresh(); }} />}
    <ProjectDialog open={!!run} onOpenChange={open => { if (!open && !busy) setRun(null); }} title={t("automation.runDetail")} footer={run?.status === "failed" && canEdit ? <Button disabled={busy} onClick={() => { if (run) void mutate(async () => { await retryAutomation(run.id); setRun(null); }); }}><RefreshCwIcon data-icon="inline-start" />{t(busy ? "automation.retrying" : "automation.retry")}</Button> : undefined}>
      {run && <div className="flex flex-col gap-4"><dl className="grid grid-cols-[auto_1fr] gap-x-4 gap-y-2 text-sm"><dt>{t("automation.name")}</dt><dd className="break-words">{run.ruleName}</dd><dt>{t("automation.versionLabel")}</dt><dd>{run.revision}</dd><dt>{t("automation.attempts")}</dt><dd>{run.attempts}</dd><dt>{t("automation.trigger")}</dt><dd>{t(`automation.triggers.${run.definition.trigger}`)} #{run.eventId}</dd><dt>{t("automation.status")}</dt><dd>{t(`automation.runStatuses.${run.status}`)}</dd></dl>
        <ul className="flex flex-col gap-2 text-sm">{run.definition.conditions.map((c, i) => <li key={i}>{t(`automation.fields.${c.field}`)}: {c.value}</li>)}</ul>
        <h2 className="text-sm font-medium">{t("automation.actionSnapshot")}</h2>
        <ol className="flex list-inside list-decimal flex-col gap-2 text-sm">{run.definition.actions.map((a, i) => <li key={i} className="break-words">{t(`automation.actions.${a.type}`)}{a.value ? `: ${a.value}` : ""}{a.ownerId > 0 ? ` (#${a.ownerId})` : ""}</li>)}</ol>
        {run.errorCode && <p role="alert" className="text-sm text-destructive">{t(`automation.errors.${run.errorCode}`)}</p>}
        <ul className="flex flex-col gap-2">{(run.results ?? []).map((result, i) => { const link = automationResultLink(result); return <li key={i}>{link ? <Link className="inline-flex items-center gap-1 underline underline-offset-4" href={link}>{resultLabel(result)}<ExternalLinkIcon className="size-4" /></Link> : resultLabel(result)}</li>; })}</ul>
      </div>}
    </ProjectDialog>
  </div>;
}
