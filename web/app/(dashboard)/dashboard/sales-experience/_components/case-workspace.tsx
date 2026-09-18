"use client";

import { useState } from "react";
import { Controller, useForm } from "react-hook-form";
import { SaveIcon } from "lucide-react";
import { toast } from "sonner";
import { useI18n } from "@/i18n/provider";
import { ProjectDialog } from "@/components/project-dialog";
import { OptionCombobox } from "@/components/option-combobox";
import { ListPagination } from "@/components/list-pagination";
import { Button } from "@/components/ui/button";
import { Field, FieldGroup, FieldLabel } from "@/components/ui/field";
import { Textarea } from "@/components/ui/textarea";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import {
  experienceAPI as api,
  type ExperienceCase,
} from "@/lib/api/sales-experience";
import { formatDateTime } from "@/lib/utils";
import {
  ActionContent,
  ExperienceError,
  useExperienceAction,
  useExperienceClose,
} from "./shared";

export function CaseDetail({
  item,
  close,
  saved,
  canEdit,
}: {
  item: ExperienceCase;
  close: () => void;
  saved: () => void;
  canEdit: boolean;
}) {
  const t = useI18n();
  const form = useForm({
    defaultValues: { outcome: item.outcome, note: item.outcomeNote },
  });
  const action = useExperienceAction();
  const tryClose = useExperienceClose(
    form.formState.isDirty,
    action.busy,
    close,
  );
  const [page, setPage] = useState(1);
  const [limit, setLimit] = useState(20);
  const messages = item.snapshot?.messages ?? [];
  return (
    <ProjectDialog
      open
      onOpenChange={(open) => {
        if (!open) void tryClose();
      }}
      title={`${item.name} #${item.id}`}
      size="xxl"
      contentClassName="rounded-md"
      headerClassName="pr-12"
      footer={
        <Button
          variant="outline"
          disabled={action.busy}
          onClick={() => void tryClose()}
        >
          {t("common.close")}
        </Button>
      }
    >
      <ExperienceError message={action.error} />
      <div className="flex flex-wrap gap-3 text-sm text-muted-foreground">
        <span>
          {t("sx.messages")}: {item.messageCount}
        </span>
        <span>
          {t("sx.sources")}:{" "}
          {item.snapshot?.conversationIds.map((id) => `#${id}`).join(", ")}
        </span>
        <span>{formatDateTime(item.createdAt)}</span>
      </div>
      <form
        onSubmit={form.handleSubmit((values) =>
          action.run(
            () => api.annotate(item.id, values.outcome, values.note),
            () => {
              form.reset(values);
              saved();
              toast.success(t("sx.saved"));
            },
          ),
        )}
        className="flex flex-col gap-3"
      >
        <FieldGroup className="gap-3 sm:flex-row sm:items-end">
          <Field className="sm:w-48 sm:shrink-0">
            <FieldLabel htmlFor="case-outcome">{t("sx.outcome")}</FieldLabel>
            <Controller
              control={form.control}
              name="outcome"
              render={({ field }) => (
                <OptionCombobox
                  id="case-outcome"
                  value={field.value}
                  onChange={field.onChange}
                  disabled={!canEdit || action.busy}
                  placeholder={t("sx.outcome")}
                  options={["unknown", "progress", "won", "lost"].map(
                    (value) => ({ value, label: t(`sx.outcomes.${value}`) }),
                  )}
                />
              )}
            />
          </Field>
          <Field>
            <FieldLabel htmlFor="case-note">{t("sx.outcomeNote")}</FieldLabel>
            <Textarea
              id="case-note"
              {...form.register("note")}
              disabled={!canEdit || action.busy}
              rows={2}
            />
          </Field>
        </FieldGroup>
        <div>
          <Button
            variant="outline"
            type="submit"
            disabled={!canEdit || action.busy || !form.formState.isDirty}
          >
            <ActionContent busy={action.busy}>
              <SaveIcon data-icon="inline-start" />
              {t("sx.saveOutcome")}
            </ActionContent>
          </Button>
        </div>
      </form>
      <Table>
        <TableHeader>
          <TableRow>
            <TableHead>{t("sx.message")}</TableHead>
            <TableHead>{t("sx.role")}</TableHead>
            <TableHead>{t("sx.content")}</TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          {messages.slice((page - 1) * limit, page * limit).map((m) => (
            <TableRow key={m.id} id={`source-message-${m.id}`}>
              <TableCell className="align-top">
                <div>#{m.id}</div>
                <div className="text-xs text-muted-foreground">
                  {formatDateTime(m.at)}
                </div>
              </TableCell>
              <TableCell className="align-top">
                {t(`sx.roles.${m.role}`)}
              </TableCell>
              <TableCell className="min-w-52 max-w-xl whitespace-pre-wrap break-words align-top">
                {m.type === "text" ? m.text : t("sx.nonText", { type: m.type })}
              </TableCell>
            </TableRow>
          ))}
        </TableBody>
      </Table>
      <ListPagination
        page={page}
        limit={limit}
        total={messages.length}
        onPageChange={setPage}
        onLimitChange={(v) => {
          setLimit(v);
          setPage(1);
        }}
      />
    </ProjectDialog>
  );
}
