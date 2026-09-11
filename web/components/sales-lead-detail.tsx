"use client";

import { useCallback, useEffect, useState } from "react";
import { Controller, useForm } from "react-hook-form";
import { useRouter } from "next/navigation";
import { CheckIcon, MessageSquareIcon, RefreshCwIcon } from "lucide-react";
import { toast } from "sonner";
import { useAuth } from "@/components/auth-provider";
import { useConfirm } from "@/components/confirm-provider";
import { ProjectDialog } from "@/components/project-dialog";
import {
  OptionCombobox,
  type ComboboxOption,
} from "@/components/option-combobox";
import { Alert, AlertDescription } from "@/components/ui/alert";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Checkbox } from "@/components/ui/checkbox";
import {
  Field,
  FieldGroup,
  FieldLabel,
  FieldError,
} from "@/components/ui/field";
import { Input } from "@/components/ui/input";
import { Separator } from "@/components/ui/separator";
import { Skeleton } from "@/components/ui/skeleton";
import { Textarea } from "@/components/ui/textarea";
import { Tabs, TabsList, TabsTrigger, TabsContent } from "@/components/ui/tabs";
import { LeadFollowUpForm, LeadHistory } from "@/components/sales-lead-followup";
import { useI18n } from "@/i18n/provider";
import { fetchAgentProfilesAll } from "@/lib/api/admin";
import {
  fetchSalesLead,
  updateSalesLead,
  type SalesLead,
  type LeadData,
} from "@/lib/api/sales-lead";
import { LeadStatus, LeadTag } from "@/lib/generated/enums";
import { useAgentConversationsStore } from "@/lib/stores/agent-conversations";
import { formatDateTime } from "@/lib/utils";

export const leadFields = [
  "title",
  "product",
  "quantity",
  "destination",
  "budget",
  "timeline",
  "contactName",
  "company",
  "email",
  "phone",
  "needs",
] as const;

export function LeadBadges({ lead }: { lead: SalesLead }) {
  const t = useI18n();
  return (
    <div className="flex flex-wrap gap-1">
      {(lead.data.autoTags ?? []).map((tag) => (
        <Badge key={tag} variant="secondary">
          {t(`lead.tags.${tag}`)}
        </Badge>
      ))}
      {(lead.customTags ?? []).map((tag) => (
        <Badge key={tag} variant="outline">
          {tag}
        </Badge>
      ))}
    </div>
  );
}

export function SalesLeadDetail({
  id,
  onClose,
  onSaved,
}: {
  id: number | null;
  onClose: () => void;
  onSaved: () => void;
}) {
  const t = useI18n();
  const [item, setItem] = useState<SalesLead | null>(null);
  const [error, setError] = useState("");
  const [attempt, setAttempt] = useState(0);
  const [dirty, setDirty] = useState(false);
  const [saving, setSaving] = useState(false);
  const [mode, setMode] = useState("followup");
  const { session } = useAuth();
  const canEdit = !!session?.permissions.some((p) => p === "*" || p === "conversation.send");
  const confirm = useConfirm();
  useEffect(() => {
    let active = true;
    if (id !== null)
      fetchSalesLead(id)
        .then((v) => {
          if (active) {
            setItem(v);
            setError("");
          }
        })
        .catch((e) => {
          if (active)
            setError(e instanceof Error ? e.message : t("lead.failed"));
        });
    return () => {
      active = false;
    };
  }, [id, attempt, t]);
  async function close() {
    if (saving) return;
    if (
      dirty &&
      !(await confirm({
        title: t("lead.discard"),
        description: t("lead.discardBody"),
        confirmText: t("lead.discard"),
        cancelText: t("lead.keepEditing"),
      }))
    )
      return;
    setItem(null);
    setError("");
    setDirty(false);
    setMode("followup");
    onClose();
  }
  return (
    <ProjectDialog
      open={id !== null}
      onOpenChange={(open) => {
        if (!open) void close();
      }}
      title={t("lead.detail")}
      size="lg"
      footer={<>
        <Button type="button" variant="outline" disabled={saving} onClick={() => void close()}>{t("lead.close")}</Button>
        {canEdit && item?.id === id && !error && <Button type="submit" form={mode === "followup" ? "lead-followup-form" : "lead-details-form"} disabled={saving || (mode === "details" && !item.sourceValid)}><CheckIcon data-icon="inline-start" />{t(saving ? "lead.saving" : mode === "followup" ? "leadWork.save" : "lead.confirmSave")}</Button>}
      </>}
    >
      {error ? (
        <Alert variant="destructive">
          <AlertDescription>
            {error}
            <Button
              variant="outline"
              size="sm"
              onClick={() => setAttempt((n) => n + 1)}
            >
              <RefreshCwIcon data-icon="inline-start" />
              {t("lead.retry")}
            </Button>
          </AlertDescription>
        </Alert>
      ) : item?.id === id ? (
        <Tabs value={mode} onValueChange={async (value) => {
          if (saving || value === mode) return;
          if (dirty && !(await confirm({title:t("lead.discard"), description:t("lead.discardBody"), confirmText:t("lead.discard"), cancelText:t("lead.keepEditing")}))) return;
          setDirty(false);
          setMode(String(value));
        }}>
          <TabsList variant="line">
            <TabsTrigger value="followup" disabled={saving}>{t("leadWork.followup")}</TabsTrigger>
            <TabsTrigger value="details" disabled={saving}>{t("leadWork.details")}</TabsTrigger>
          </TabsList>
          <TabsContent value="followup">
            {mode === "followup" && <LeadFollowUpForm key={`${item.id}-${item.revision}`} item={item} onDirty={setDirty} onSaving={setSaving} onClose={() => void close()} onSaved={() => {
              setDirty(false); setItem(null); setMode("followup"); onSaved(); onClose();
            }} />}
          </TabsContent>
          <TabsContent value="details">
        {mode === "details" && <LeadForm
          key={`${item.id}-${item.revision}`}
          item={item}
          onDirty={setDirty}
          onSaving={setSaving}
          onClose={() => void close()}
          onSaved={() => {
            setDirty(false);
            setItem(null);
            setMode("followup");
            onSaved();
            onClose();
          }}
        />}
          </TabsContent>
        </Tabs>
      ) : (
        <Skeleton className="h-64 w-full" />
      )}
    </ProjectDialog>
  );
}

type LeadFormValues = {
  data: LeadData;
  status: LeadStatus;
  ownerId: string;
  tags: string;
  note: string;
  followUpAt: string;
};
function localDate(value?: string) {
  if (!value) return "";
  const date = new Date(value);
  if (!Number.isFinite(date.getTime())) return "";
  return new Date(date.getTime() - date.getTimezoneOffset() * 60000)
    .toISOString()
    .slice(0, 16);
}
function LeadForm({
  item,
  onDirty,
  onSaving,
  onClose,
  onSaved,
}: {
  item: SalesLead;
  onDirty: (v: boolean) => void;
  onSaving: (v: boolean) => void;
  onClose: () => void;
  onSaved: () => void;
}) {
  const t = useI18n();
  const confirm = useConfirm();
  const router = useRouter();
  const { session } = useAuth();
  const [owners, setOwners] = useState<ComboboxOption[]>([]);
  const [proposalApplied, setProposalApplied] = useState(false);
  const [error, setError] = useState("");
  const [saving, setSaving] = useState(false);
  const canEdit = !!session?.permissions.some(
    (p) => p === "*" || p === "conversation.send",
  );
  const form = useForm<LeadFormValues>({
    defaultValues: {
      data: item.data,
      status: item.status,
      ownerId: String(item.ownerId),
      tags: (item.customTags ?? []).join(", "),
      note: item.note,
      followUpAt: localDate(item.followUpAt),
    },
  });
  const {
    register,
    control,
    handleSubmit,
    formState: { isDirty, errors },
  } = form;
  useEffect(() => {
    onDirty(isDirty);
  }, [isDirty, onDirty]);
  const loadOwners = useCallback(() => {
    fetchAgentProfilesAll()
      .then((items) =>
        setOwners(
          items.map((u) => ({
            value: String(u.userId),
            label:
              u.displayName || u.nickname || u.username || String(u.userId),
          })),
        ),
      )
      .catch(() => setError(t("lead.ownersFailed")));
  }, [t]);
  useEffect(() => {
    if (canEdit) loadOwners();
  }, [loadOwners, canEdit]);
  const ownerOptions = [{ value: "0", label: t("lead.unassigned") }, ...owners];
  if (item.ownerId && !owners.some((o) => o.value === String(item.ownerId)))
    ownerOptions.push({
      value: String(item.ownerId),
      label: item.ownerName || String(item.ownerId),
    });
  async function save(values: LeadFormValues) {
    if (saving || !canEdit) return;
    setSaving(true);
    onSaving(true);
    setError("");
    try {
      await updateSalesLead(item.id, {
        revision: item.revision,
        data: values.data,
        status: item.status,
        ownerId: item.ownerId,
        customTags: values.tags
          .split(/[,，]/)
          .map((v) => v.trim())
          .filter(Boolean),
        note: values.note,
        followUpAt: item.followUpAt ?? null,
        acceptProposal: proposalApplied,
      });
      toast.success(t("lead.saved"));
      onSaved();
    } catch (e) {
      setError(e instanceof Error ? e.message : t("lead.failed"));
    } finally {
      setSaving(false);
      onSaving(false);
    }
  }
  async function openConversation() {
    try {
      await useAgentConversationsStore
        .getState()
        .selectConversation(item.conversationId);
      router.push("/dashboard/conversations");
      onClose();
    } catch (e) {
      setError(e instanceof Error ? e.message : t("lead.failed"));
    }
  }
  async function applyProposal() {
    if (!item.proposal) return;
    if (
      isDirty &&
      !(await confirm({
        title: t("lead.discard"),
        description: t("lead.discardBody"),
        confirmText: t("lead.useProposal"),
        cancelText: t("lead.keepEditing"),
      }))
    )
      return;
    form.setValue("data", item.proposal, { shouldDirty: true });
    setProposalApplied(true);
  }
  return (
    <form id="lead-details-form" className="flex min-w-0 flex-col gap-5" onSubmit={handleSubmit(save)}>
      <div className="flex flex-wrap items-center justify-between gap-2">
        <div className="flex flex-wrap items-center gap-2">
          <span>{item.customerName || `#${item.conversationId}`}</span>
          <Badge variant="outline">
            {t(item.confirmed ? "lead.confirmed" : "lead.unconfirmed")}
          </Badge>
        </div>
        <Button
          type="button"
          variant="outline"
          size="sm"
          disabled={isDirty || saving}
          onClick={() => void openConversation()}
        >
          <MessageSquareIcon data-icon="inline-start" />
          {t("lead.conversation")}
        </Button>
      </div>
      {!item.sourceValid && (
        <Alert variant="destructive">
          <AlertDescription>{t("lead.invalidSource")}</AlertDescription>
        </Alert>
      )}
      {item.withdrawn && (
        <Alert>
          <AlertDescription>{t("lead.withdrawn")}</AlertDescription>
        </Alert>
      )}
      {error && (
        <Alert variant="destructive">
          <AlertDescription>{error}</AlertDescription>
        </Alert>
      )}
      {item.proposal && (
        <details className="text-sm">
          <summary className="cursor-pointer font-medium">
            {t("lead.proposal")}
          </summary>
          <div className="flex flex-col gap-2 py-3">
            {leadFields
              .filter((key) => item.proposal?.[key] !== item.data[key])
              .map((key) => (
                <p className="break-words" key={key}>
                  {t(`lead.fields.${key}`)}: {item.data[key] || "-"} →{" "}
                  {item.proposal?.[key] || "-"}
                </p>
              ))}
            <p>
              {t("lead.tagsLabel")}:{" "}
              {(item.proposal.autoTags ?? [])
                .map((v) => t(`lead.tags.${v}`))
                .join(", ") || "-"}
            </p>
            <Button
              type="button"
              variant="secondary"
              size="sm"
              className="self-start"
              disabled={!canEdit || saving || proposalApplied}
              onClick={() => void applyProposal()}
            >
              <CheckIcon data-icon="inline-start" />
              {t("lead.useProposal")}
            </Button>
          </div>
        </details>
      )}
      <FieldGroup className="grid grid-cols-1 gap-4 sm:grid-cols-2">
        {leadFields.map((key) => (
          <Field
            key={key}
            data-invalid={!!errors.data?.[key]}
            className={key === "needs" ? "sm:col-span-2" : undefined}
          >
            <FieldLabel htmlFor={`lead-${key}`}>
              {t(`lead.fields.${key}`)}
            </FieldLabel>
            {key === "needs" ? (
              <Textarea
                id={`lead-${key}`}
                rows={3}
                maxLength={1000}
                disabled={!canEdit || saving || !item.sourceValid}
                {...register(`data.${key}`)}
              />
            ) : (
              <Input
                id={`lead-${key}`}
                type={
                  key === "email" ? "email" : key === "phone" ? "tel" : "text"
                }
                maxLength={key === "title" ? 100 : 1000}
                disabled={!canEdit || saving || !item.sourceValid}
                aria-invalid={!!errors.data?.[key]}
                aria-describedby={
                  errors.data?.[key] ? `lead-${key}-error` : undefined
                }
                {...register(`data.${key}`, {
                  validate:
                    key === "title"
                      ? (value) => !!value.trim() || t("lead.titleRequired")
                      : undefined,
                })}
              />
            )}
            {errors.data?.[key]?.message && (
              <FieldError id={`lead-${key}-error`}>
                {errors.data[key]?.message}
              </FieldError>
            )}
          </Field>
        ))}
      </FieldGroup>
      <FieldGroup>
        <Field>
          <FieldLabel>{t("lead.autoTags")}</FieldLabel>
          <Controller
            control={control}
            name="data.autoTags"
            render={({ field }) => (
              <div className="flex flex-wrap gap-3">
                {Object.values(LeadTag).map((tag) => (
                  <label className="flex items-center gap-2 text-sm" key={tag}>
                    <Checkbox
                      checked={(field.value ?? []).includes(tag)}
                      disabled={!canEdit || saving || tag === LeadTag.Contact}
                      onCheckedChange={(checked) =>
                        field.onChange(
                          checked
                            ? [...(field.value ?? []), tag]
                            : (field.value ?? []).filter((v) => v !== tag),
                        )
                      }
                    />
                    {t(`lead.tags.${tag}`)}
                  </label>
                ))}
              </div>
            )}
          />
        </Field>
        <Field>
          <FieldLabel htmlFor="lead-custom-tags">
            {t("lead.customTags")}
          </FieldLabel>
          <Input
            id="lead-custom-tags"
            disabled={!canEdit || saving}
            placeholder={t("lead.tagsPlaceholder")}
            {...register("tags")}
          />
        </Field>
      </FieldGroup>
      <Separator />
      <FieldGroup className="grid grid-cols-1 gap-4 sm:grid-cols-2">
        <Field>
          <FieldLabel>{t("lead.status")}</FieldLabel>
          <Controller
            control={control}
            name="status"
            render={({ field }) => (
              <OptionCombobox
                value={field.value}
                onChange={field.onChange}
                disabled
                placeholder={t("lead.status")}
                options={Object.values(LeadStatus).map((value) => ({
                  value,
                  label: t(`lead.statuses.${value}`),
                }))}
              />
            )}
          />
        </Field>
        <Field>
          <FieldLabel>{t("lead.owner")}</FieldLabel>
          <Controller
            control={control}
            name="ownerId"
            render={({ field }) => (
              <OptionCombobox
                value={field.value}
                onChange={field.onChange}
                disabled
                placeholder={t("lead.owner")}
                options={ownerOptions}
              />
            )}
          />
        </Field>
        <Field>
          <FieldLabel htmlFor="lead-follow-up">{t("lead.followUp")}</FieldLabel>
          <Input
            id="lead-follow-up"
            type="datetime-local"
            disabled
            {...register("followUpAt")}
          />
        </Field>
        <Field>
          <FieldLabel htmlFor="lead-note">{t("lead.note")}</FieldLabel>
          <Textarea
            id="lead-note"
            rows={2}
            maxLength={2000}
            disabled={!canEdit || saving}
            {...register("note")}
          />
        </Field>
      </FieldGroup>
      <details className="text-sm">
        <summary className="cursor-pointer font-medium">
          {t("lead.sources", { count: item.sources.length })}
        </summary>
        <div className="flex flex-col gap-3 py-3">
          {item.sources.map((s) => (
            <blockquote className="flex flex-col gap-1" key={s.id}>
              <span className="text-xs text-muted-foreground">
                #{s.id} · {formatDateTime(s.createdAt)}
              </span>
              <p className="whitespace-pre-wrap break-words">{s.content}</p>
            </blockquote>
          ))}
        </div>
      </details>
      <LeadHistory item={item} />
    </form>
  );
}
