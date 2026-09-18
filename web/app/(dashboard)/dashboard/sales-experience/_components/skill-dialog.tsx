"use client";

import { useState } from "react";
import { useFieldArray, useForm } from "react-hook-form";
import {
  CheckIcon,
  DownloadIcon,
  PencilIcon,
  PlusIcon,
  SaveIcon,
  TrashIcon,
  XIcon,
} from "lucide-react";
import { toast } from "sonner";
import { useI18n } from "@/i18n/provider";
import { ProjectDialog } from "@/components/project-dialog";
import { useConfirm } from "@/components/confirm-provider";
import { Alert, AlertDescription } from "@/components/ui/alert";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { Field, FieldGroup, FieldLabel } from "@/components/ui/field";
import { Textarea } from "@/components/ui/textarea";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import {
  experienceAPI as api,
  type ExperienceRule,
  type ExperienceWorkspace,
} from "@/lib/api/sales-experience";
import { formatDateTime } from "@/lib/utils";
import {
  ActionContent,
  EventEvidence,
  ExperienceError,
  useExperienceAction,
  useExperienceClose,
} from "./shared";

function RuleText({ rule }: { rule: ExperienceRule }) {
  const t = useI18n();
  return (
    <div className="flex min-w-0 flex-col gap-2">
      <h3 className="whitespace-pre-wrap break-words font-medium">
        {rule.condition}
      </h3>
      <p className="whitespace-pre-wrap break-words text-sm leading-relaxed">
        {rule.action}
      </p>
      {rule.exceptions && (
        <p className="whitespace-pre-wrap break-words text-sm text-muted-foreground">
          {t("sx.exceptions")}: {rule.exceptions}
        </p>
      )}
    </div>
  );
}

export function SkillDialog({
  item,
  canEdit,
  close,
  saved,
}: {
  item: ExperienceWorkspace;
  canEdit: boolean;
  close: () => void;
  saved: (w: ExperienceWorkspace) => void;
}) {
  const t = useI18n();
  const count = item.suggestions.reduce((n, s) => n + s.changes.length, 0);
  const confirm = useConfirm();
  const eligible = item.suggestions.flatMap((s) =>
    s.changes
      .filter((d) => !d.conflict && d.kind !== "remove")
      .map((d) => ({
        proposalId: s.id,
        ruleId: d.ruleId,
        decision: "accept" as const,
      })),
  );
  const [tab, setTab] = useState(count ? "pending" : "current");
  const [editing, setEditing] = useState<
    { proposalId: number; ruleId: string } | "current" | null
  >(null);
  const [busyKey, setBusyKey] = useState("");
  const [editingExpectedID, setEditingExpectedID] = useState(item.current.id);
  const action = useExperienceAction();
  const form = useForm({
    defaultValues: { rules: item.current.payload.rules },
  });
  const fields = useFieldArray({
    control: form.control,
    name: "rules",
    keyName: "fieldKey",
  });
  const tryClose = useExperienceClose(
    form.formState.isDirty,
    action.busy,
    close,
  );
  const cancelEdit = useExperienceClose(
    form.formState.isDirty,
    action.busy,
    () => {
      setEditing(null);
      form.reset({ rules: item.current.payload.rules });
    },
  );
  const reload = async () => {
    saved(await api.skill(item.skillId));
  };
  const review = (
    proposalId: number,
    ruleId: string,
    decision: "accept" | "ignore",
    rule?: ExperienceRule,
  ) => {
    setBusyKey(`${proposalId}:${ruleId}:${decision}`);
    return action.run(
      async () => {
        await api.review(
          proposalId,
          ruleId,
          decision,
          rule ? editingExpectedID : item.current.id,
          rule,
        );
        setEditing(null);
        form.reset();
        await reload();
      },
      () => {
        setEditing(null);
        form.reset();
        toast.success(
          t(decision === "accept" ? "sx.review.accepted" : "sx.review.ignored"),
        );
      },
    );
  };
  return (
    <ProjectDialog
      open
      onOpenChange={(v) => {
        if (!v) void tryClose();
      }}
      title={t(`sx.skills.${item.skillId}`)}
      size="xxl"
      contentClassName="rounded-md"
      headerClassName="pr-12"
      footer={
        <>
          <Button
            variant="outline"
            disabled={action.busy}
            onClick={() => void tryClose()}
          >
            {t("common.close")}
          </Button>
          {editing && (
            <Button
              variant="outline"
              disabled={action.busy}
              onClick={() => void cancelEdit()}
            >
              {t("common.cancel")}
            </Button>
          )}
          {editing && (
            <Button
              type="submit"
              form="skill-edit"
              disabled={
                action.busy ||
                (editing === "current" && !form.formState.isDirty)
              }
            >
              <ActionContent busy={action.busy}>
                <SaveIcon data-icon="inline-start" />
                {t(
                  editing === "current"
                    ? "sx.review.save"
                    : "sx.review.saveAccept",
                )}
              </ActionContent>
            </Button>
          )}
        </>
      }
    >
      <ExperienceError
        message={action.error}
        retry={() => void action.run(reload)}
      />
      {editing ? (
        <form
          id="skill-edit"
          className="flex flex-col gap-5"
          onSubmit={form.handleSubmit((v) => {
            if (editing !== "current")
              return review(
                editing.proposalId,
                editing.ruleId,
                "accept",
                v.rules[0],
              );
            return action.run(
              async () => {
                await api.saveSkill(item.skillId, editingExpectedID, v.rules);
                setEditing(null);
                form.reset();
                await reload();
              },
              () => {
                toast.success(t("sx.saved"));
              },
            );
          })}
        >
          {fields.fields.map((r, i) => (
            <FieldGroup key={r.fieldKey} className="gap-3">
              <div className="flex items-center justify-between gap-2">
                <h3 className="font-medium">
                  {t("sx.ruleNumber", { count: i + 1 })}
                </h3>
                {editing === "current" && (
                  <Button
                    type="button"
                    variant="ghost"
                    size="icon-sm"
                    title={t("sx.removeRule")}
                    aria-label={t("sx.removeRule")}
                    disabled={action.busy}
                    onClick={() => fields.remove(i)}
                  >
                    <TrashIcon />
                  </Button>
                )}
              </div>
              {(["condition", "action", "exceptions"] as const).map((key) => (
                <Field key={key}>
                  <FieldLabel htmlFor={`rule-${i}-${key}`}>
                    {t(`sx.${key}`)}
                  </FieldLabel>
                  <Textarea
                    id={`rule-${i}-${key}`}
                    {...form.register(`rules.${i}.${key}`, {
                      required: key !== "exceptions",
                    })}
                    required={key !== "exceptions"}
                    disabled={action.busy}
                  />
                </Field>
              ))}
            </FieldGroup>
          ))}
          {editing === "current" && (
            <Button
              type="button"
              variant="outline"
              disabled={action.busy}
              onClick={() =>
                fields.append({
                  id: crypto.randomUUID(),
                  condition: "",
                  action: "",
                  exceptions: "",
                })
              }
            >
              <PlusIcon data-icon="inline-start" />
              {t("sx.addRule")}
            </Button>
          )}
        </form>
      ) : (
        <Tabs value={tab} onValueChange={setTab} className="min-w-0 gap-4">
          <TabsList variant="line">
            <TabsTrigger value="pending">
              {t("sx.review.pending")} ({count})
            </TabsTrigger>
            <TabsTrigger value="current">
              {t("sx.review.current")} ({item.current.payload.rules.length})
            </TabsTrigger>
          </TabsList>
          <TabsContent value="pending" className="flex min-w-0 flex-col gap-6">
            {eligible.length > 0 && (
              <div>
                <Button
                  variant="outline"
                  disabled={!canEdit || action.busy}
                  onClick={() =>
                    void (async () => {
                      if (
                        !(await confirm({
                          title: t("sx.review.acceptAll", {
                            count: eligible.length,
                          }),
                          description: t("sx.review.acceptAllBody"),
                          confirmText: t("sx.review.accept"),
                          cancelText: t("common.cancel"),
                        }))
                      )
                        return;
                      setBusyKey("batch");
                      await action.run(
                        async () => {
                          await api.reviewBatch(
                            item.skillId,
                            item.current.id,
                            eligible,
                          );
                          await reload();
                        },
                        () => toast.success(t("sx.review.accepted")),
                      );
                    })()
                  }
                >
                  <ActionContent busy={action.busy && busyKey === "batch"}>
                    <CheckIcon data-icon="inline-start" />
                    {t("sx.review.acceptAll", { count: eligible.length })}
                  </ActionContent>
                </Button>
              </div>
            )}
            {!count && (
              <p className="text-sm text-muted-foreground">
                {t("sx.review.empty")}
              </p>
            )}
            {item.suggestions.map((s) => (
              <section key={s.id} className="flex min-w-0 flex-col gap-4">
                <h2 className="text-sm font-semibold">
                  {t("sx.review.extractedAt", {
                    time: formatDateTime(s.createdAt),
                  })}
                </h2>
                {s.changes.map((d) => (
                  <article
                    key={d.ruleId}
                    className="flex min-w-0 flex-col gap-3 border-b pb-5 last:border-b-0"
                  >
                    <Badge variant="outline" className="self-start">
                      {t(`sx.review.kinds.${d.kind}`)}
                    </Badge>
                    {(d.after || d.before) && (
                      <RuleText rule={(d.after || d.before)!} />
                    )}
                    {d.conflict && (
                      <Alert>
                        <AlertDescription className="flex flex-col gap-2">
                          <p>{t("sx.review.conflict")}</p>
                          {d.current ? (
                            <RuleText rule={d.current} />
                          ) : (
                            <p>{t("sx.review.removed")}</p>
                          )}
                        </AlertDescription>
                      </Alert>
                    )}
                    {d.before && d.kind === "revise" && (
                      <details className="text-sm">
                        <summary className="cursor-pointer py-1 font-medium">
                          {t("sx.review.before")}
                        </summary>
                        <div className="pt-2">
                          <RuleText rule={d.before} />
                        </div>
                      </details>
                    )}
                    {(d.rationale.length > 0 || d.evidence.length > 0) && (
                      <details className="text-sm">
                        <summary className="cursor-pointer py-1 font-medium">
                          {t("sx.review.evidence")}
                        </summary>
                        <div className="flex flex-col gap-3 pt-2">
                          {d.rationale.map((r, i) => (
                            <p
                              key={i}
                              className="whitespace-pre-wrap break-words"
                            >
                              {r}
                            </p>
                          ))}
                          {d.evidence.flatMap((e, i) =>
                            e.events.map((event) => (
                              <EventEvidence
                                key={`${i}:${event.id}`}
                                event={event}
                              />
                            )),
                          )}
                        </div>
                      </details>
                    )}
                    <div className="flex flex-wrap gap-2">
                      <Button
                        size="sm"
                        disabled={!canEdit || action.busy || d.conflict}
                        onClick={() => void review(s.id, d.ruleId, "accept")}
                      >
                        <ActionContent
                          busy={
                            action.busy &&
                            busyKey === `${s.id}:${d.ruleId}:accept`
                          }
                        >
                          <CheckIcon data-icon="inline-start" />
                          {t("sx.review.accept")}
                        </ActionContent>
                      </Button>
                      {d.after && (
                        <Button
                          size="sm"
                          variant="outline"
                          disabled={!canEdit || action.busy}
                          onClick={() => {
                            form.reset({ rules: [d.after!] });
                            setEditingExpectedID(item.current.id);
                            setEditing({ proposalId: s.id, ruleId: d.ruleId });
                          }}
                        >
                          <PencilIcon data-icon="inline-start" />
                          {t("sx.review.edit")}
                        </Button>
                      )}
                      <Button
                        size="sm"
                        variant="ghost"
                        disabled={!canEdit || action.busy}
                        onClick={() => void review(s.id, d.ruleId, "ignore")}
                      >
                        <ActionContent
                          busy={
                            action.busy &&
                            busyKey === `${s.id}:${d.ruleId}:ignore`
                          }
                        >
                          <XIcon data-icon="inline-start" />
                          {t("sx.review.ignore")}
                        </ActionContent>
                      </Button>
                    </div>
                  </article>
                ))}
              </section>
            ))}
          </TabsContent>
          <TabsContent value="current" className="flex min-w-0 flex-col gap-5">
            <div className="flex flex-wrap gap-2">
              <Button
                variant="outline"
                size="sm"
                disabled={!canEdit || action.busy}
                onClick={() => {
                  form.reset({ rules: item.current.payload.rules });
                  setEditingExpectedID(item.current.id);
                  setEditing("current");
                }}
              >
                <PencilIcon data-icon="inline-start" />
                {t("sx.edit")}
              </Button>
              <Button
                variant="outline"
                size="sm"
                disabled={action.busy}
                onClick={() =>
                  void action.run(
                    () => api.export(item.current.id),
                    ({ blob, filename }) => {
                      const url = URL.createObjectURL(blob);
                      const a = document.createElement("a");
                      a.href = url;
                      a.download = filename || `sales-${item.skillId}.zip`;
                      a.click();
                      setTimeout(() => URL.revokeObjectURL(url), 1000);
                    },
                  )
                }
              >
                <DownloadIcon data-icon="inline-start" />
                {t("sx.export")}
              </Button>
            </div>
            {!item.current.payload.rules.length && (
              <p className="text-sm text-muted-foreground">{t("sx.noRules")}</p>
            )}
            {item.current.payload.rules.map((r) => (
              <section key={r.id} className="flex flex-col gap-3">
                <RuleText rule={r} />
                {item.current.payload.evidence.some(
                  (e) => e.ruleId === r.id,
                ) && (
                  <details className="text-sm">
                    <summary className="cursor-pointer py-1 font-medium">
                      {t("sx.review.evidence")}
                    </summary>
                    <div className="flex flex-col gap-3 pt-2">
                      {item.current.payload.evidence
                        .filter((e) => e.ruleId === r.id)
                        .flatMap((e, i) =>
                          e.events.map((event) => (
                            <EventEvidence
                              key={`${i}:${event.id}`}
                              event={event}
                            />
                          )),
                        )}
                    </div>
                  </details>
                )}
              </section>
            ))}
          </TabsContent>
        </Tabs>
      )}
    </ProjectDialog>
  );
}
