"use client"

import { useState } from "react"
import { ExternalLinkIcon, PlusIcon, RefreshCwIcon, PencilIcon, SaveIcon } from "lucide-react"
import { toast } from "sonner"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Textarea } from "@/components/ui/textarea"
import { Field, FieldGroup, FieldLabel } from "@/components/ui/field"
import { ProjectDialog } from "@/components/project-dialog"
import { OptionCombobox } from "@/components/option-combobox"
import { useConfirm } from "@/components/confirm-provider"
import { useI18n } from "@/i18n/provider"
import { Status } from "@/lib/generated/enums"
import { fetchKnowledgeBasesAll, fetchSkillDefinitionsAll, createSkillDefinition, updateSkillDefinition, type SkillDefinition, type KnowledgeBase } from "@/lib/api/admin"
import { experienceAPI, type ExperienceRevision } from "@/lib/api/sales-experience"

export function EmployeeResources({ knowledgeIds, skillIds, knowledgeBases, skills, onResourcesChange, onSkillAdded }: {
  knowledgeIds: number[]; skillIds: number[]; knowledgeBases: KnowledgeBase[]; skills: SkillDefinition[]
  onResourcesChange: (bases: KnowledgeBase[], skills: SkillDefinition[]) => void
  onSkillAdded: (skill: SkillDefinition) => void
}) {
  const t = useI18n()
  const confirm = useConfirm()
  const [busy, setBusy] = useState(false)
  const [editor, setEditor] = useState<{ id: number; name: string; instruction: string } | null>(null)
  const [baseline, setBaseline] = useState("")
  const [experienceOpen, setExperienceOpen] = useState(false)
  const [revisions, setRevisions] = useState<ExperienceRevision[]>([])
  const [revisionId, setRevisionId] = useState("")
  async function refresh() {
    setBusy(true)
    try { const [bases, definitions] = await Promise.all([fetchKnowledgeBasesAll({ status: Status.Ok }), fetchSkillDefinitionsAll({ status: Status.Ok })]); onResourcesChange(bases, definitions) }
    catch { toast.error(t("employee.resourceFailed")) }
    finally { setBusy(false) }
  }
  function edit(skill?: SkillDefinition) {
    const value = { id: skill?.id || 0, name: skill?.name || "", instruction: skill?.instruction || "" }
    setEditor(value); setBaseline(JSON.stringify(value))
  }
  async function closeEditor() {
    if (busy) return
    if (editor && JSON.stringify(editor) !== baseline && !await confirm({ title: t("aiReception.discardTitle"), description: t("aiReception.discardBody"), confirmText: t("reception.discard") })) return
    setEditor(null)
  }
  async function save() {
    if (!editor || !editor.name.trim() || !editor.instruction.trim()) return
    if (editor.id && !await confirm({ title: t("employee.saveSkill"), description: t("employee.sharedSkill"), confirmText: t("common.save") })) return
    setBusy(true)
    try {
      const existing = skills.find((item) => item.id === editor.id)
      const payload = { name: editor.name.trim(), instruction: editor.instruction, description: existing?.description || "", examples: existing?.examples || [], toolWhitelist: existing?.toolWhitelist || [], remark: existing?.remark || "" }
      if (existing) { await updateSkillDefinition({ id: existing.id, ...payload }); onResourcesChange(knowledgeBases, skills.map((item) => item.id === existing.id ? { ...item, ...payload } : item)) }
      else { const result = await createSkillDefinition(payload); onSkillAdded(result) }
      setEditor(null); toast.success(t("employee.skillSaved"))
    } catch (cause) { toast.error(cause instanceof Error ? cause.message : t("aiAgent.saveFailed")) }
    finally { setBusy(false) }
  }
  async function openExperience() {
    setBusy(true)
    try {
      const options = await experienceAPI.options()
      const available = await Promise.all(options.skills.filter((skill) => skill.activeRevisionId > 0).map((skill) => experienceAPI.revision(skill.activeRevisionId)))
      setRevisions(available.filter((revision) => revision.payload.rules.length > 0)); setRevisionId(""); setExperienceOpen(true)
    } catch (cause) { toast.error(cause instanceof Error ? cause.message : t("employee.resourceFailed")) }
    finally { setBusy(false) }
  }
  async function importExperience() {
    const revision = revisions.find((item) => item.id === Number(revisionId))
    if (!revision) return
    setBusy(true)
    try {
      const instruction = revision.payload.rules.map((rule) => `Condition: ${rule.condition}\nAction: ${rule.action}\nExceptions: ${rule.exceptions || "None specified"}`).join("\n\n")
      const skill = await createSkillDefinition({ name: `${t("employee.experienceName")} ${revision.skillId} #${revision.id}`, description: revision.note || revision.skillId, instruction, examples: [], toolWhitelist: [], remark: `sales-experience revision:${revision.id} hash:${revision.hash}` })
      onSkillAdded(skill); setExperienceOpen(false); toast.success(t("employee.imported"))
    } catch (cause) { toast.error(cause instanceof Error ? cause.message : t("aiAgent.saveFailed")) }
    finally { setBusy(false) }
  }
  return <section className="flex flex-col gap-4">
    <div className="flex flex-wrap items-center justify-between gap-2"><h2 className="text-sm font-semibold">{t("employee.resources")}</h2><Button variant="ghost" size="sm" disabled={busy} onClick={() => void refresh()}><RefreshCwIcon data-icon="inline-start" />{t("employee.refreshResources")}</Button></div>
    <div className="divide-y">
      {knowledgeIds.map((id) => <div key={id} className="flex flex-wrap items-center justify-between gap-2 py-3"><span className="min-w-0 break-words text-sm">{knowledgeBases.find((item) => item.id === id)?.name || `#${id}`}</span><div className="flex flex-wrap gap-2"><Button size="sm" variant="ghost" render={<a href={`/dashboard/knowledge/detail?id=${id}`} target="_blank" rel="noreferrer" />}><ExternalLinkIcon />{t("employee.openKnowledge")}</Button><Button size="sm" variant="outline" render={<a href={`/dashboard/knowledge/ingest?id=${id}`} target="_blank" rel="noreferrer" />}><PlusIcon />{t("employee.addDocuments")}</Button></div></div>)}
      {skillIds.map((id) => { const skill = skills.find((item) => item.id === id); return <div key={id} className="flex flex-wrap items-center justify-between gap-2 py-3"><span className="min-w-0 break-words text-sm">{skill?.name || `#${id}`}</span><Button size="sm" variant="ghost" disabled={!skill || busy} onClick={() => edit(skill)}><PencilIcon />{t("employee.viewSkill")}</Button></div> })}
    </div>
    <div className="flex flex-wrap gap-2">
      <Button variant="outline" render={<a href="/dashboard/knowledge" target="_blank" rel="noreferrer" />}><ExternalLinkIcon />{t("employee.manageKnowledge")}</Button>
      <Button variant="outline" disabled={busy} onClick={() => edit()}><PlusIcon />{t("employee.addSkill")}</Button>
      <Button variant="outline" disabled={busy} onClick={() => void openExperience()}><PlusIcon />{t("employee.importExperience")}</Button>
      <Button variant="link" render={<a href="/dashboard/sales-experience" target="_blank" rel="noreferrer" />}><ExternalLinkIcon />{t("employee.manageExperience")}</Button>
    </div>
    <ProjectDialog open={!!editor} onOpenChange={(open) => { if (!open) void closeEditor() }} title={t("employee.skillContent")} size="lg">
      {editor ? <form className="flex flex-col gap-4" onSubmit={(event) => { event.preventDefault(); void save() }}><FieldGroup>
        <Field><FieldLabel htmlFor="employee-skill-name">{t("aiAgent.name")}</FieldLabel><Input id="employee-skill-name" required disabled={busy} value={editor.name} onChange={(event) => setEditor({ ...editor, name: event.target.value })} /></Field>
        <Field><FieldLabel htmlFor="employee-skill-content">{t("employee.skillContent")}</FieldLabel><Textarea id="employee-skill-content" required disabled={busy} rows={12} value={editor.instruction} onChange={(event) => setEditor({ ...editor, instruction: event.target.value })} /></Field>
      </FieldGroup><div className="flex justify-end gap-2"><Button type="button" variant="outline" disabled={busy} onClick={() => void closeEditor()}>{t("common.cancel")}</Button><Button type="submit" disabled={busy}><SaveIcon />{t("employee.saveSkill")}</Button></div></form> : null}
    </ProjectDialog>
    <ProjectDialog open={experienceOpen} onOpenChange={(open) => { if (!busy) setExperienceOpen(open) }} title={t("employee.importExperience")} size="lg">
      <div className="flex flex-col gap-4"><OptionCombobox value={revisionId} onChange={setRevisionId} placeholder={t("employee.experienceSelect")} emptyText={t("employee.experienceEmpty")} options={revisions.map((item) => ({ value: String(item.id), label: `${item.skillId} #${item.id}` }))} />
        {revisions.find((item) => item.id === Number(revisionId))?.payload.rules.map((rule) => <div key={rule.id} className="flex flex-col gap-1 border-b pb-3 text-sm"><p className="font-medium">{rule.condition}</p><p>{rule.action}</p><p className="text-muted-foreground">{rule.exceptions}</p></div>)}
        <Button disabled={!revisionId || busy} onClick={() => void importExperience()}><PlusIcon />{t("employee.import")}</Button>
      </div>
    </ProjectDialog>
  </section>
}
