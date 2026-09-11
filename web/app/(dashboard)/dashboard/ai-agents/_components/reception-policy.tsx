"use client"

import { PlusIcon, Trash2Icon } from "lucide-react"
import { useConfirm } from "@/components/confirm-provider"
import { Button } from "@/components/ui/button"
import { Field, FieldError, FieldGroup, FieldLabel, FieldLegend, FieldSet } from "@/components/ui/field"
import { Input } from "@/components/ui/input"
import { Separator } from "@/components/ui/separator"
import { Switch } from "@/components/ui/switch"
import { Textarea } from "@/components/ui/textarea"
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip"
import { useI18n } from "@/i18n/provider"
import type { ReceptionField, ReceptionPolicy } from "@/lib/reception"

export function ReceptionPolicyEditor({ value, onChange, disabled, error }: {
  value: ReceptionPolicy; onChange: (value: ReceptionPolicy) => void; disabled: boolean; error: string
}) {
  const t = useI18n()
  const confirm = useConfirm()
  const set = (patch: Partial<ReceptionPolicy>) => onChange({ ...value, ...patch })
  const updateField = (key: string, patch: Partial<ReceptionField>) => set({ fields: value.fields.map((f) => f.key === key ? { ...f, ...patch } : f) })
  async function removeField(field: ReceptionField) {
    if (field.label && !await confirm({ title: t("reception.removeTitle"), description: t("reception.removeDescription", { label: field.label }), confirmText: t("reception.remove"), variant: "destructive" })) return
    set({ fields: value.fields.filter((f) => f.key !== field.key) })
  }
  return <FieldSet disabled={disabled} className="min-w-0 gap-6">
    <FieldLegend>{t("reception.title")}</FieldLegend>
    <FieldGroup>
      <Field orientation="horizontal">
        <FieldLabel htmlFor="reception-enabled">{t("reception.enabled")}</FieldLabel>
        <Switch id="reception-enabled" checked={value.enabled} onCheckedChange={(enabled) => set({ enabled })} disabled={disabled} />
      </Field>
      <Field data-invalid={error === "goalRequired"}>
        <FieldLabel htmlFor="reception-goal">{t("reception.goal")}</FieldLabel>
        <Textarea id="reception-goal" rows={2} maxLength={500} value={value.goal} aria-invalid={error === "goalRequired"} onChange={(e) => set({ goal: e.target.value })} />
      </Field>
      <Field>
        <FieldLabel htmlFor="reception-instructions">{t("reception.instructions")}</FieldLabel>
        <Textarea id="reception-instructions" rows={4} maxLength={3000} value={value.instructions} onChange={(e) => set({ instructions: e.target.value })} />
      </Field>
      <Field>
        <FieldLabel htmlFor="reception-handoff">{t("reception.handoffConditions")}</FieldLabel>
        <Textarea id="reception-handoff" rows={3} maxLength={1500} value={value.handoffConditions} onChange={(e) => set({ handoffConditions: e.target.value })} />
      </Field>
    </FieldGroup>
    <Separator />
    <FieldSet>
      <FieldLegend>{t("reception.fields")}</FieldLegend>
      {value.fields.map((field) => <FieldGroup key={field.key} className="pb-4">
        <Field data-invalid={error === "fieldsInvalid" && !field.label.trim()}>
          <div className="flex items-center justify-between gap-2">
            <FieldLabel htmlFor={`field-label-${field.key}`}>{t("reception.fieldName")}</FieldLabel>
            <Tooltip>
              <TooltipTrigger render={<Button type="button" variant="ghost" size="icon-sm" disabled={disabled} aria-label={t("reception.removeField", { label: field.label })} onClick={() => void removeField(field)} />}><Trash2Icon /></TooltipTrigger>
              <TooltipContent>{t("reception.remove")}</TooltipContent>
            </Tooltip>
          </div>
          <Input id={`field-label-${field.key}`} value={field.label} maxLength={100} aria-invalid={error === "fieldsInvalid" && !field.label.trim()} onChange={(e) => updateField(field.key, { label: e.target.value })} />
        </Field>
        <Field>
          <FieldLabel htmlFor={`field-when-${field.key}`}>{t("reception.askWhen")}</FieldLabel>
          <Textarea id={`field-when-${field.key}`} rows={2} maxLength={500} value={field.askWhen} onChange={(e) => updateField(field.key, { askWhen: e.target.value })} />
        </Field>
        <Field orientation="horizontal">
          <FieldLabel htmlFor={`field-required-${field.key}`}>{t("reception.required")}</FieldLabel>
          <Switch id={`field-required-${field.key}`} checked={field.required} disabled={disabled} onCheckedChange={(required) => updateField(field.key, { required })} />
        </Field>
        <Separator />
      </FieldGroup>)}
      <div><Button type="button" variant="outline" disabled={disabled || value.fields.length >= 16} onClick={() => set({ fields: [...value.fields, { key: `field_${crypto.randomUUID().replaceAll("-", "")}`, label: "", askWhen: "", required: false }] })}>
        <PlusIcon data-icon="inline-start" />{t("reception.addField")}
      </Button></div>
    </FieldSet>
    {error && <FieldError>{t(`reception.${error}`)}</FieldError>}
  </FieldSet>
}
