"use client";

import { useEffect, useMemo, useState } from "react";
import { ImportIcon } from "lucide-react";
import { useI18n } from "@/i18n/provider";
import { ProjectDialog } from "@/components/project-dialog";
import { Button } from "@/components/ui/button";
import { Checkbox } from "@/components/ui/checkbox";
import { ToggleGroup, ToggleGroupItem } from "@/components/ui/toggle-group";
import {
  experienceAPI as api,
  type ExperienceCase,
  type ExperienceImportPreview,
  type ExperienceImportSelection,
} from "@/lib/api/sales-experience";
import { formatDateTime } from "@/lib/utils";
import {
  ActionContent,
  ExperienceError,
  useExperienceAction,
} from "./shared";

type ImportMode = "all" | "selected";

export function ImportCaseDialog({
  sourceIds,
  close,
  imported,
}: {
  sourceIds: number[];
  close: () => void;
  imported: (rows: ExperienceCase[]) => void;
}) {
  const t = useI18n();
  const [mode, setMode] = useState<ImportMode>("all");
  const [previews, setPreviews] = useState<ExperienceImportPreview[]>([]);
  const [selected, setSelected] = useState<Record<number, number[]>>({});
  const [loading, setLoading] = useState(true);
  const [loadError, setLoadError] = useState("");
  const action = useExperienceAction();

  useEffect(() => {
    let active = true;
    void api
      .importPreview(sourceIds)
      .then((rows) => {
        if (!active) return;
        setPreviews(rows);
        setSelected(
          Object.fromEntries(
            rows.map((row) => [
              row.conversationId,
              row.messages.map((message) => message.id),
            ]),
          ),
        );
      })
      .catch((error) => {
        if (!active) return;
        setLoadError(error instanceof Error ? error.message : t("sx.failed"));
      })
      .finally(() => {
        if (active) setLoading(false);
      });
    return () => {
      active = false;
    };
  }, [sourceIds, t]);

  const availableCount = useMemo(
    () => previews.reduce((count, row) => count + row.messages.length, 0),
    [previews],
  );
  const selectedCount = useMemo(
    () => Object.values(selected).reduce((count, ids) => count + ids.length, 0),
    [selected],
  );
  const count = mode === "all" ? availableCount : selectedCount;

  function toggleMessage(conversationId: number, messageId: number, checked: boolean) {
    setSelected((current) => {
      const ids = current[conversationId] ?? [];
      return {
        ...current,
        [conversationId]: checked
          ? [...new Set([...ids, messageId])]
          : ids.filter((id) => id !== messageId),
      };
    });
  }

  function selections(): ExperienceImportSelection[] {
    if (mode === "all") {
      return previews.map((row) => ({
        conversationId: row.conversationId,
        includeAll: true,
      }));
    }
    return previews.flatMap((row) => {
      const messageIds = selected[row.conversationId] ?? [];
      return messageIds.length
        ? [{ conversationId: row.conversationId, includeAll: false, messageIds }]
        : [];
    });
  }

  return (
    <ProjectDialog
      open
      onOpenChange={(open) => {
        if (!open && !action.busy) close();
      }}
      title={t("sx.importDialog.title")}
      description={t("sx.importDialog.description")}
      size="xxl"
      contentClassName="rounded-md"
      headerClassName="pr-12"
      footer={
        <>
          <Button variant="outline" disabled={action.busy} onClick={close}>
            {t("common.cancel")}
          </Button>
          <Button
            disabled={loading || action.busy || count === 0}
            onClick={() => void action.run(() => api.import(selections()), imported)}
          >
            <ActionContent busy={action.busy}>
              <ImportIcon data-icon="inline-start" />
              {t("sx.importDialog.confirm", { count })}
            </ActionContent>
          </Button>
        </>
      }
    >
      <ExperienceError message={loadError} />
      <ExperienceError message={action.error} />
      <div className="flex flex-wrap items-center justify-between gap-3">
        <ToggleGroup
          multiple={false}
          value={[mode]}
          onValueChange={(value) => {
            const next = value[0] as ImportMode | undefined;
            if (next) setMode(next);
          }}
          variant="outline"
          aria-label={t("sx.importDialog.range")}
        >
          <ToggleGroupItem value="all">
            {t("sx.importDialog.all")}
          </ToggleGroupItem>
          <ToggleGroupItem value="selected">
            {t("sx.importDialog.selected")}
          </ToggleGroupItem>
        </ToggleGroup>
        <p className="text-sm tabular-nums text-muted-foreground" aria-live="polite">
          {loading
            ? t("common.loading")
            : t("sx.importDialog.summary", {
                conversations: previews.length,
                messages: count,
              })}
        </p>
      </div>

      {!loading && !loadError && previews.length === 0 && (
        <p className="text-sm text-muted-foreground">
          {t("sx.importDialog.empty")}
        </p>
      )}

      {!loading && mode === "selected" && (
        <div className="flex min-w-0 flex-col divide-y border-y">
          {previews.map((preview) => {
            const chosen = selected[preview.conversationId] ?? [];
            const allSelected =
              preview.messages.length > 0 && chosen.length === preview.messages.length;
            return (
              <section
                key={preview.conversationId}
                className="flex min-w-0 flex-col gap-3 py-4"
              >
                <div className="flex flex-wrap items-center justify-between gap-3">
                  <div className="min-w-0">
                    <h3 className="break-words font-medium">
                      {preview.customerName || `#${preview.customerId}`}
                    </h3>
                    <p className="text-sm tabular-nums text-muted-foreground">
                      {t("sx.importDialog.conversation", {
                        id: preview.conversationId,
                        count: chosen.length,
                        total: preview.messages.length,
                      })}
                    </p>
                  </div>
                  <label className="flex cursor-pointer items-center gap-2 text-sm">
                    <Checkbox
                      checked={allSelected}
                      onCheckedChange={(checked) =>
                        setSelected((current) => ({
                          ...current,
                          [preview.conversationId]: checked
                            ? preview.messages.map((message) => message.id)
                            : [],
                        }))
                      }
                    />
                    {allSelected
                      ? t("sx.importDialog.clearConversation")
                      : t("sx.importDialog.selectConversation")}
                  </label>
                </div>
                <div className="flex min-w-0 flex-col divide-y rounded-md border">
                  {preview.messages.map((message) => {
                    const checked = chosen.includes(message.id);
                    const inputId = `import-message-${preview.conversationId}-${message.id}`;
                    return (
                      <label
                        key={message.id}
                        htmlFor={inputId}
                        className="grid cursor-pointer grid-cols-[auto_minmax(0,1fr)] gap-3 px-3 py-2.5 hover:bg-muted/50"
                      >
                        <Checkbox
                          id={inputId}
                          className="mt-0.5"
                          checked={checked}
                          onCheckedChange={(value) =>
                            toggleMessage(
                              preview.conversationId,
                              message.id,
                              value,
                            )
                          }
                        />
                        <span className="min-w-0">
                          <span className="flex flex-wrap gap-x-2 text-xs text-muted-foreground">
                            <span>{t(`sx.roles.${message.role}`)}</span>
                            <span>{formatDateTime(message.at)}</span>
                          </span>
                          <span className="mt-1 block whitespace-pre-wrap break-words text-sm leading-6">
                            {message.type === "text"
                              ? message.text
                              : t("sx.nonText", { type: message.type })}
                          </span>
                        </span>
                      </label>
                    );
                  })}
                </div>
              </section>
            );
          })}
        </div>
      )}
    </ProjectDialog>
  );
}
