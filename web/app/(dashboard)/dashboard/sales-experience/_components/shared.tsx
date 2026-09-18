"use client";

import { useEffect, useRef, useState, type ReactNode } from "react";
import { Loader2Icon } from "lucide-react";
import { useI18n } from "@/i18n/provider";
import { useConfirm } from "@/components/confirm-provider";
import { Alert, AlertDescription } from "@/components/ui/alert";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { OptionCombobox } from "@/components/option-combobox";
import type {
  ExperienceEvent,
  ExperienceOptions,
} from "@/lib/api/sales-experience";

export function ExperienceError({
  message,
  retry,
}: {
  message: string;
  retry?: () => void;
}) {
  const t = useI18n();
  if (!message) return null;
  return (
    <Alert variant="destructive">
      <AlertDescription className="flex flex-wrap items-center justify-between gap-2">
        <span role="alert">{message}</span>
        {retry && (
          <Button variant="outline" size="sm" onClick={retry}>
            {t("sx.retry")}
          </Button>
        )}
      </AlertDescription>
    </Alert>
  );
}

export function useExperienceAction() {
  const t = useI18n();
  const lock = useRef(false);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  async function run<T>(action: () => Promise<T>, done?: (value: T) => void) {
    if (lock.current) return;
    lock.current = true;
    setBusy(true);
    setError("");
    try {
      const value = await action();
      done?.(value);
    } catch (e) {
      setError(e instanceof Error ? e.message : t("sx.failed"));
    } finally {
      lock.current = false;
      setBusy(false);
    }
  }
  return { busy, error, run };
}

export function useExperienceClose(
  dirty: boolean,
  busy: boolean,
  close: () => void,
) {
  const confirm = useConfirm();
  const t = useI18n();
  useEffect(() => {
    if (!dirty) return;
    const warn = (event: BeforeUnloadEvent) => {
      event.preventDefault();
      event.returnValue = "";
    };
    window.addEventListener("beforeunload", warn);
    return () => window.removeEventListener("beforeunload", warn);
  }, [dirty]);
  return async () => {
    if (busy) return;
    if (
      dirty &&
      !(await confirm({
        title: t("sx.discardTitle"),
        description: t("sx.discardBody"),
        confirmText: t("sx.discard"),
        cancelText: t("common.cancel"),
      }))
    )
      return;
    close();
  };
}

export function ActionContent({
  busy,
  children,
}: {
  busy: boolean;
  children: ReactNode;
}) {
  return (
    <>
      {busy && (
        <Loader2Icon data-icon="inline-start" className="animate-spin" />
      )}
      {children}
    </>
  );
}
export function ModelPicker({
  options,
  value,
  onChange,
  disabled,
}: {
  options: ExperienceOptions;
  value: number;
  onChange: (id: number) => void;
  disabled?: boolean;
}) {
  const t = useI18n();
  return (
    <OptionCombobox
      id="experience-model"
      value={value ? String(value) : ""}
      onChange={(v) => onChange(Number(v))}
      disabled={disabled}
      placeholder={t("sx.model")}
      options={options.models.map((m) => ({
        value: String(m.id),
        label: m.name,
        subtitle: m.modelName,
      }))}
    />
  );
}
export function EventEvidence({ event }: { event: ExperienceEvent }) {
  const t = useI18n();
  return (
    <article className="flex min-w-0 flex-col gap-3 py-3">
      <div className="flex flex-wrap items-center gap-2">
        <h3 className="break-words font-medium">{event.title}</h3>
        {event.skillIds.map((id) => (
          <Badge key={id} variant="outline">
            {t(`sx.skills.${id}`)}
          </Badge>
        ))}
      </div>
      <dl className="grid gap-2 text-sm sm:grid-cols-[7rem_minmax(0,1fr)]">
        <dt className="text-muted-foreground">{t("sx.signal")}</dt>
        <dd className="whitespace-pre-wrap break-words">{event.signal}</dd>
        <dt className="text-muted-foreground">{t("sx.sellerAction")}</dt>
        <dd className="whitespace-pre-wrap break-words">
          {event.sellerAction}
        </dd>
        <dt className="text-muted-foreground">{t("sx.observedResult")}</dt>
        <dd className="whitespace-pre-wrap break-words">
          {event.observedResult}
        </dd>
        {event.uncertainty && (
          <>
            <dt className="text-muted-foreground">{t("sx.uncertainty")}</dt>
            <dd className="whitespace-pre-wrap break-words">
              {event.uncertainty}
            </dd>
          </>
        )}
      </dl>
      <div className="flex flex-col gap-2">
        {event.quotes.map((q, i) => (
          <blockquote
            key={`${q.messageId}-${i}`}
            className="whitespace-pre-wrap break-words border-l pl-3 text-sm"
          >
            <span className="mr-2 text-muted-foreground">#{q.messageId}</span>
            {q.text}
          </blockquote>
        ))}
      </div>
    </article>
  );
}
