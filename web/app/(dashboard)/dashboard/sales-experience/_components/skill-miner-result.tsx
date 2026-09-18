"use client";

import Link from "next/link";
import { useState } from "react";
import { CheckCircle2Icon, CheckIcon, CircleAlertIcon, LightbulbIcon, PencilIcon, XIcon } from "lucide-react";
import { useI18n } from "@/i18n/provider";
import { useConfirm } from "@/components/confirm-provider";
import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Field, FieldDescription, FieldGroup, FieldLabel, FieldLegend, FieldSet } from "@/components/ui/field";
import { Input } from "@/components/ui/input";
import { Separator } from "@/components/ui/separator";
import { Textarea } from "@/components/ui/textarea";
import type { SkillMinerCandidate, SkillMinerDebugResult, SkillMinerEvidenceQuote, SkillMinerSkill } from "@/lib/api/sales-experience";

function TextList({ items }: { items: string[] }) {
  return <ul className="flex list-disc flex-col gap-1.5 pl-5 text-sm leading-6">{items.map((item, index) => <li key={`${index}-${item}`} className="pl-1">{item}</li>)}</ul>;
}

function EvidenceGroup({ title, quotes }: { title: string; quotes: SkillMinerEvidenceQuote[] }) {
  return <div className="flex min-w-0 flex-col gap-2"><h5 className="text-sm font-medium">{title}</h5><div className="flex flex-col gap-2">{quotes.map((quote) => <blockquote key={`${quote.messageId}-${quote.quote}`} className="whitespace-pre-wrap break-words border-l pl-3 text-sm leading-6 text-muted-foreground"><span className="mr-2 tabular-nums text-foreground/70">#{quote.messageId}</span>{quote.quote}</blockquote>)}</div></div>;
}

function LinesField({ id, label, value, onChange }: { id: string; label: string; value: string[]; onChange: (value: string[]) => void }) {
  const t = useI18n();
  return <Field><FieldLabel htmlFor={id}>{label}</FieldLabel><Textarea id={id} rows={4} value={value.join("\n")} onChange={(event) => onChange(event.target.value.split("\n"))} /><FieldDescription>{t("sx.miner.onePerLine")}</FieldDescription></Field>;
}

function SkillEditor({ skill, candidateIndex, onChange }: { skill: SkillMinerSkill; candidateIndex: number; onChange: (value: SkillMinerSkill) => void }) {
  const t = useI18n();
  const id = `mined-skill-${candidateIndex}`;
  return <FieldGroup className="p-4 sm:p-5">
    <div className="grid gap-4 lg:grid-cols-2">
      <Field><FieldLabel htmlFor={`${id}-name`}>{t("sx.miner.skillName")}</FieldLabel><Input id={`${id}-name`} maxLength={100} value={skill.name} onChange={(event) => onChange({ ...skill, name: event.target.value })} /></Field>
      <Field><FieldLabel htmlFor={`${id}-description`}>{t("sx.miner.skillDescription")}</FieldLabel><Textarea id={`${id}-description`} maxLength={255} rows={3} value={skill.description} onChange={(event) => onChange({ ...skill, description: event.target.value })} /></Field>
      <LinesField id={`${id}-when`} label={t("sx.miner.whenToUse")} value={skill.whenToUse} onChange={(whenToUse) => onChange({ ...skill, whenToUse })} />
      <Field><FieldLabel htmlFor={`${id}-objective`}>{t("sx.miner.objective")}</FieldLabel><Textarea id={`${id}-objective`} rows={4} value={skill.objective} onChange={(event) => onChange({ ...skill, objective: event.target.value })} /></Field>
    </div>
    <FieldSet><FieldLegend>{t("sx.miner.steps")}</FieldLegend><FieldGroup className="grid gap-4 lg:grid-cols-2">
      {skill.steps.map((step, stepIndex) => <FieldSet key={stepIndex} className="rounded-md border p-4"><FieldLegend variant="label">{t("sx.miner.stepNumber", { count: stepIndex + 1 })}</FieldLegend>
        <Field><FieldLabel htmlFor={`${id}-step-${stepIndex}`}>{t("sx.miner.stepInstruction")}</FieldLabel><Textarea id={`${id}-step-${stepIndex}`} rows={3} value={step.instruction} onChange={(event) => onChange({ ...skill, steps: skill.steps.map((item, index) => index === stepIndex ? { ...item, instruction: event.target.value } : item) })} /></Field>
        <Field><FieldLabel htmlFor={`${id}-purpose-${stepIndex}`}>{t("sx.miner.stepPurpose")}</FieldLabel><Textarea id={`${id}-purpose-${stepIndex}`} rows={2} value={step.purpose} onChange={(event) => onChange({ ...skill, steps: skill.steps.map((item, index) => index === stepIndex ? { ...item, purpose: event.target.value } : item) })} /></Field>
      </FieldSet>)}
    </FieldGroup></FieldSet>
    <div className="grid gap-4 lg:grid-cols-2"><LinesField id={`${id}-success`} label={t("sx.miner.successSignals")} value={skill.successSignals} onChange={(successSignals) => onChange({ ...skill, successSignals })} /><LinesField id={`${id}-exceptions`} label={t("sx.miner.whenNotToUse")} value={skill.whenNotToUse} onChange={(whenNotToUse) => onChange({ ...skill, whenNotToUse })} /></div>
  </FieldGroup>;
}

type CandidateReview = NonNullable<SkillMinerDebugResult["candidateReviews"]>[string];

function CandidateResult({ candidate, index, review, editing, canEdit, canConfirm, busy, onStartEdit, onCancelEdit, onConfirm, onDismiss, onSkillChange }: {
  candidate: SkillMinerCandidate;
  index: number;
  review?: CandidateReview;
  editing: boolean;
  canEdit: boolean;
  canConfirm: boolean;
  busy: boolean;
  onStartEdit: () => void;
  onCancelEdit: () => void;
  onConfirm: () => void;
  onDismiss: () => void;
  onSkillChange: (skill: SkillMinerSkill) => void;
}) {
  const t = useI18n();
  const skill = candidate.skill;
  return <article className="overflow-hidden rounded-md border bg-background">
    <header className="flex flex-col gap-3 p-4 sm:p-5"><div className="flex flex-wrap items-center gap-2"><Badge variant="secondary">{t("sx.miner.skillNumber", { count: index + 1 })}</Badge><Badge variant="outline">{t(`sx.miner.confidence.${candidate.assessment.confidence}`)}</Badge></div>{!editing && <div className="flex max-w-3xl flex-col gap-1.5"><h3 className="text-lg font-semibold leading-snug">{skill.name}</h3><p className="text-sm leading-6 text-muted-foreground">{skill.description}</p></div>}</header>
    <Separator />
    {editing ? <SkillEditor skill={skill} candidateIndex={index} onChange={onSkillChange} /> : <div className="grid gap-6 p-4 sm:p-5 lg:grid-cols-2">
      <section className="flex min-w-0 flex-col gap-2"><h4 className="font-medium">{t("sx.miner.whenToUse")}</h4><TextList items={skill.whenToUse} /></section>
      <section className="flex min-w-0 flex-col gap-2"><h4 className="font-medium">{t("sx.miner.objective")}</h4><p className="text-sm leading-6">{skill.objective}</p></section>
      <section className="flex min-w-0 flex-col gap-3 lg:col-span-2"><h4 className="font-medium">{t("sx.miner.steps")}</h4><ol className="grid gap-3 lg:grid-cols-2">{skill.steps.map((step, stepIndex) => <li key={`${stepIndex}-${step.instruction}`} className="grid min-w-0 grid-cols-[1.75rem_minmax(0,1fr)] gap-2 text-sm leading-6"><span className="flex size-7 items-center justify-center rounded-full bg-muted text-xs font-medium tabular-nums">{stepIndex + 1}</span><div className="min-w-0"><p>{step.instruction}</p><p className="text-muted-foreground">{step.purpose}</p></div></li>)}</ol></section>
      <section className="flex min-w-0 flex-col gap-2"><h4 className="font-medium">{t("sx.miner.successSignals")}</h4><TextList items={skill.successSignals} /></section>
      <section className="flex min-w-0 flex-col gap-2"><h4 className="font-medium">{t("sx.miner.whenNotToUse")}</h4><TextList items={skill.whenNotToUse} /></section>
    </div>}
    <div className="flex flex-col gap-5 bg-muted/40 p-4 sm:p-5"><section className="flex max-w-4xl flex-col gap-3"><div className="flex items-center gap-2"><LightbulbIcon aria-hidden="true" /><h4 className="font-medium">{t("sx.miner.whyLearn")}</h4></div><dl className="grid gap-x-5 gap-y-3 text-sm sm:grid-cols-[8rem_minmax(0,1fr)]"><dt className="text-muted-foreground">{t("sx.miner.customerChange")}</dt><dd className="leading-6">{candidate.assessment.customerChange}</dd><dt className="text-muted-foreground">{t("sx.miner.causalReason")}</dt><dd className="leading-6">{candidate.assessment.causalReason}</dd><dt className="text-muted-foreground">{t("sx.miner.transferReason")}</dt><dd className="leading-6">{candidate.assessment.transferReason}</dd></dl></section><Separator /><section className="flex flex-col gap-4"><h4 className="font-medium">{t("sx.miner.evidence")}</h4><div className="grid gap-5 lg:grid-cols-3"><EvidenceGroup title={t("sx.miner.customerBefore")} quotes={candidate.evidence.customerBefore} /><EvidenceGroup title={t("sx.miner.sellerMove")} quotes={candidate.evidence.sellerMove} /><EvidenceGroup title={t("sx.miner.customerAfter")} quotes={candidate.evidence.customerAfter} /></div></section></div>
    <footer className="border-t p-4 sm:p-5">
      {review?.status === "confirmed" ? <Alert><CheckCircle2Icon aria-hidden="true" /><AlertTitle>{t("sx.miner.confirmedTitle")}</AlertTitle><AlertDescription className="flex flex-wrap items-center gap-2"><span>{t("sx.miner.confirmedBody")}</span><Button variant="outline" size="sm" render={<Link href="/dashboard/skill-definition" />}>{t("sx.miner.openLibrary")}</Button></AlertDescription></Alert> : review?.status === "dismissed" ? <Alert><XIcon aria-hidden="true" /><AlertTitle>{t("sx.miner.dismissedTitle")}</AlertTitle><AlertDescription>{t("sx.miner.dismissedBody")}</AlertDescription></Alert> : <div className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
        <p className="max-w-2xl text-sm leading-6 text-muted-foreground">{editing ? t("sx.miner.editHint") : t("sx.miner.reviewHint")}</p>
        <div className="flex flex-wrap gap-2 sm:justify-end">
          {editing ? <><Button variant="outline" disabled={busy} onClick={onCancelEdit}><XIcon data-icon="inline-start" />{t("sx.miner.cancelEdit")}</Button><Button disabled={!canConfirm || busy || !isValidSkill(skill)} onClick={onConfirm}><CheckIcon data-icon="inline-start" />{t("sx.miner.saveConfirm")}</Button></> : <><Button variant="outline" disabled={!canEdit || busy} onClick={onDismiss}><XIcon data-icon="inline-start" />{t("sx.miner.dismiss")}</Button><Button variant="outline" disabled={!canEdit || busy} onClick={onStartEdit}><PencilIcon data-icon="inline-start" />{t("sx.miner.editSkill")}</Button><Button disabled={!canConfirm || busy} onClick={onConfirm}><CheckIcon data-icon="inline-start" />{t("sx.miner.directConfirm")}</Button></>}
        </div>
      </div>}
    </footer>
  </article>;
}

function isValidSkill(skill: SkillMinerSkill) {
  return Boolean(skill.name.trim() && skill.description.trim() && skill.objective.trim() && skill.whenToUse.some((item) => item.trim()) && skill.successSignals.some((item) => item.trim()) && skill.whenNotToUse.some((item) => item.trim()) && skill.steps.length > 0 && skill.steps.every((step) => step.instruction.trim() && step.purpose.trim()));
}

export function SkillMinerResultView({ output, canEdit, canConfirm, busy, onConfirm, onDismiss }: { output: SkillMinerDebugResult; canEdit: boolean; canConfirm: boolean; busy: boolean; onConfirm: (candidate: { sourceEpisodeId: string; skill: SkillMinerSkill }) => void; onDismiss: (sourceEpisodeId: string) => void }) {
  const t = useI18n();
  const confirm = useConfirm();
  const [editingEpisodeId, setEditingEpisodeId] = useState<string | null>(null);
  const [drafts, setDrafts] = useState<SkillMinerCandidate[]>(() =>
    output.result?.candidates.map((candidate) => ({
      ...candidate,
      skill: {
        ...candidate.skill,
        whenToUse: [...candidate.skill.whenToUse],
        steps: candidate.skill.steps.map((step) => ({ ...step })),
        successSignals: [...candidate.skill.successSignals],
        whenNotToUse: [...candidate.skill.whenNotToUse],
      },
    })) ?? [],
  );
  if (!output.ok) return <Alert variant="destructive"><CircleAlertIcon aria-hidden="true" /><AlertTitle>{t("sx.miner.failed")}</AlertTitle><AlertDescription>{output.error || t("sx.failed")}</AlertDescription></Alert>;
  if (!output.result) return <Alert variant="destructive"><CircleAlertIcon aria-hidden="true" /><AlertTitle>{t("sx.miner.failed")}</AlertTitle><AlertDescription>{t("sx.miner.missingResult")}</AlertDescription></Alert>;
  const { result } = output;
  const candidates = drafts;
  return <section aria-live="polite" className="flex min-w-0 flex-col gap-5">
    <div className="flex flex-col gap-3 border-b pb-5"><div className="flex flex-wrap items-center gap-2"><Badge variant={candidates.length ? "default" : "secondary"}>{candidates.length ? t("sx.miner.found", { count: candidates.length }) : t("sx.miner.notFound")}</Badge>{output.model && <span className="text-sm text-muted-foreground">{t("sx.miner.modelUsed", { model: output.model })}</span>}</div><div className="flex max-w-3xl flex-col gap-1.5"><h3 className="font-medium">{t("sx.miner.reason")}</h3><p className="text-sm leading-6 text-muted-foreground">{result.assessment?.reason || t("sx.miner.noReason")}</p></div></div>
    {candidates.length === 0 ? <Alert><CheckCircle2Icon aria-hidden="true" /><AlertTitle>{t("sx.miner.noSkillTitle")}</AlertTitle><AlertDescription>{t("sx.miner.noSkillBody")}</AlertDescription></Alert> : <div className="flex min-w-0 flex-col gap-5">
      {candidates.map((candidate, index) => <CandidateResult
        key={candidate.sourceEpisodeId}
        candidate={candidate}
        index={index}
        review={output.candidateReviews?.[candidate.sourceEpisodeId] ?? (!output.candidateReviews && (output.reviewStatus === "confirmed" || output.reviewStatus === "dismissed") ? { status: output.reviewStatus, reviewedAt: output.reviewedAt ?? "" } : undefined)}
        editing={editingEpisodeId === candidate.sourceEpisodeId}
        canEdit={canEdit}
        canConfirm={canConfirm}
        busy={busy}
        onStartEdit={() => setEditingEpisodeId(candidate.sourceEpisodeId)}
        onCancelEdit={() => {
          setDrafts((current) => current.map((item) => item.sourceEpisodeId === candidate.sourceEpisodeId ? result.candidates.find((stored) => stored.sourceEpisodeId === candidate.sourceEpisodeId) ?? item : item));
          setEditingEpisodeId(null);
        }}
        onConfirm={() => {
          void confirm({ title: t("sx.miner.confirmTitle"), description: t("sx.miner.confirmBody"), confirmText: editingEpisodeId === candidate.sourceEpisodeId ? t("sx.miner.saveConfirm") : t("sx.miner.directConfirm"), cancelText: t("common.cancel") }).then((accepted) => {
            if (!accepted) return;
            setEditingEpisodeId(null);
            onConfirm({ sourceEpisodeId: candidate.sourceEpisodeId, skill: candidate.skill });
          });
        }}
        onDismiss={() => {
          void confirm({ title: t("sx.miner.dismissTitle"), description: t("sx.miner.dismissBody"), confirmText: t("sx.miner.dismiss"), cancelText: t("common.cancel") }).then((accepted) => accepted && onDismiss(candidate.sourceEpisodeId));
        }}
        onSkillChange={(skill) => setDrafts((current) => current.map((item) => item.sourceEpisodeId === candidate.sourceEpisodeId ? { ...item, skill } : item))}
      />)}
    </div>}
  </section>;
}
