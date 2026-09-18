"use client";

import { useCallback, useEffect, useState } from "react";
import { useForm, Controller, useWatch } from "react-hook-form";
import { SaveIcon, SquareIcon, RotateCcwIcon } from "lucide-react";
import { useI18n } from "@/i18n/provider";
import { ProjectDialog } from "@/components/project-dialog";
import { OptionCombobox } from "@/components/option-combobox";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { Alert, AlertDescription } from "@/components/ui/alert";
import { Field, FieldGroup, FieldLabel } from "@/components/ui/field";
import { Textarea } from "@/components/ui/textarea";
import {
  experienceAPI as api,
  type ExperienceJob,
} from "@/lib/api/sales-experience";
import {
  ActionContent,
  EventEvidence,
  ExperienceError,
  useExperienceAction,
  useExperienceClose,
} from "./shared";

function DistillProgress({
  job,
  disconnected,
}: {
  job: ExperienceJob;
  disconnected: boolean;
}) {
  const t = useI18n();
  const [now, setNow] = useState(() => Date.now());
  useEffect(() => {
    const timer = window.setInterval(() => setNow(Date.now()), 1000);
    return () => window.clearInterval(timer);
  }, []);
  const age = (value?: string) => {
    const stamp = value ? Date.parse(value) : NaN;
    return Number.isFinite(stamp)
      ? Math.max(0, Math.floor((now - stamp) / 1000))
      : null;
  };
  const duration = (seconds: number) =>
    [Math.floor(seconds / 3600), Math.floor(seconds / 60) % 60, seconds % 60]
      .map((n) => String(n).padStart(2, "0"))
      .join(":");
  const heartbeat = age(job.heartbeatAt);
  const response = age(job.lastResponseAt);
  const elapsed = age(job.callStartedAt);
  const stale = heartbeat !== null && heartbeat > 15;
  const phase = ["waiting", "thinking", "generating", "validating"].includes(
    job.callPhase || "",
  )
    ? job.callPhase
    : "waiting";
  const stage =
    /^case:(\d+)\/(\d+):(window:(\d+)\/(\d+)|proposals|complete)$/.exec(
      job.stage,
    );
  return (
    <section
      className="flex min-w-0 flex-col gap-3"
      aria-label={t("sx.progress.title")}
    >
      <h2 className="font-semibold" aria-live="polite">
        {t(
          disconnected
            ? "sx.progress.disconnected"
            : stale
              ? "sx.progress.stale"
              : job.state === "queued"
                ? "sx.states.queued"
                : `sx.progress.${phase}`,
        )}
      </h2>
      {stage && job.state === "running" && (
        <p className="text-sm">
          {t(stage[4] ? "sx.progress.window" : "sx.progress.proposals", {
            current: stage[1],
            total: stage[2],
            window: stage[4] || "",
            windows: stage[5] || "",
          })}
        </p>
      )}
      {job.state === "running" && (
        <dl className="grid grid-cols-1 gap-x-8 gap-y-3 text-sm sm:grid-cols-2">
          <div className="flex min-w-0 flex-col gap-1">
            <dt className="text-muted-foreground">
              {t("sx.progress.backend")}
            </dt>
            <dd>
              {disconnected
                ? t("sx.progress.unconfirmed")
                : heartbeat === null
                  ? t("sx.progress.starting")
                  : t(
                      stale
                        ? "sx.progress.heartbeatStale"
                        : "sx.progress.heartbeat",
                      { seconds: heartbeat },
                    )}
            </dd>
          </div>
          <div className="flex min-w-0 flex-col gap-1">
            <dt className="text-muted-foreground">
              {t("sx.progress.lastResponse")}
            </dt>
            <dd>
              {response === null
                ? t("sx.progress.noResponse")
                : t("sx.progress.ago", { seconds: response })}
            </dd>
          </div>
          <div className="flex min-w-0 flex-col gap-1">
            <dt className="text-muted-foreground">
              {t("sx.progress.elapsed")}
            </dt>
            <dd className="tabular-nums">
              {elapsed === null ? t("sx.progress.starting") : duration(elapsed)}
            </dd>
          </div>
          <div className="flex min-w-0 flex-col gap-1">
            <dt className="text-muted-foreground">{t("sx.progress.output")}</dt>
            <dd className="tabular-nums">
              {t("sx.progress.chars", { count: job.outputChars || 0 })}
            </dd>
          </div>
        </dl>
      )}
      {stale && !disconnected && (
        <Alert>
          <AlertDescription>{t("sx.progress.staleHint")}</AlertDescription>
        </Alert>
      )}
    </section>
  );
}

export function JobDialog({
  id,
  canEdit,
  close,
  openChanges,
}: {
  id: number;
  canEdit: boolean;
  close: () => void;
  openChanges: () => void;
}) {
  const t = useI18n();
  const [job, setJob] = useState<ExperienceJob | null>(null);
  const state = job?.state;
  const [error, setError] = useState("");
  const action = useExperienceAction();
  const form = useForm({ defaultValues: { rating: "", note: "" } });
  const rating = useWatch({ control: form.control, name: "rating" });
  const tryClose = useExperienceClose(
    form.formState.isDirty,
    action.busy,
    close,
  );
  const load = useCallback(async () => {
    try {
      setJob(await api.job(id));
      setError("");
    } catch (e) {
      setError(e instanceof Error ? e.message : t("sx.failed"));
    }
  }, [id, t]);
  useEffect(() => {
    if (state && !["queued", "running"].includes(state)) return;
    let cancelled = false;
    let timer: number | undefined;
    const fetchJob = async () => {
      try {
        const next = await api.job(id);
        if (!cancelled) {
          setJob(next);
          setError("");
        }
      } catch (e) {
        if (!cancelled)
          setError(e instanceof Error ? e.message : t("sx.failed"));
      }
      if (!cancelled) timer = window.setTimeout(() => void fetchJob(), 4000);
    };
    void fetchJob();
    return () => {
      cancelled = true;
      window.clearTimeout(timer);
    };
  }, [id, t, state]);
  return (
    <ProjectDialog
      open
      onOpenChange={(v) => {
        if (!v) void tryClose();
      }}
      title={`${t("sx.task")} #${id}`}
      size="xxl"
      contentClassName="rounded-md"
      headerClassName="pr-12"
      footer={
        <>
          {canEdit &&
            job?.kind === "distill" &&
            ["queued", "running"].includes(job.state) && (
              <Button
                variant="outline"
                disabled={action.busy}
                onClick={() =>
                  void action.run(
                    () => api.cancel(id),
                    () => {
                      void load();
                    },
                  )
                }
              >
                <ActionContent busy={action.busy}>
                  <SquareIcon data-icon="inline-start" />
                  {t("sx.stop")}
                </ActionContent>
              </Button>
            )}
          <Button
            variant="outline"
            disabled={action.busy}
            onClick={() => void tryClose()}
          >
            {t("common.close")}
          </Button>
        </>
      }
    >
      <ExperienceError message={error} retry={() => void load()} />
      <ExperienceError message={action.error} />
      {!job && !error && <p role="status">{t("common.loading")}</p>}
      {job && (
        <>
          <div className="flex flex-wrap items-center gap-2">
            <Badge variant={job.state === "failed" ? "destructive" : "outline"}>
              {t(`sx.states.${job.state}`)}
            </Badge>
            <span className="text-sm">
              {t(`sx.kinds.${job.kind}`)} · {t("sx.attempts")}: {job.attempts}
            </span>
            {job.input?.skillId && (
              <span className="text-sm">
                {t(`sx.skills.${job.input.skillId}`)}
              </span>
            )}
          </div>
          {job.kind === "distill" &&
            ["queued", "running"].includes(job.state) && (
              <DistillProgress job={job} disconnected={Boolean(error)} />
            )}
          {job.state === "cancelled" && (
            <div className="flex flex-col items-start gap-3">
              <p className="text-sm text-muted-foreground">
                {t("sx.stoppedHint")}
              </p>
              {canEdit && (
                <Button
                  variant="outline"
                  disabled={action.busy}
                  onClick={() =>
                    void action.run(
                      () => api.retry(id),
                      () => {
                        void load();
                      },
                    )
                  }
                >
                  <ActionContent busy={action.busy}>
                    <RotateCcwIcon data-icon="inline-start" />
                    {t("sx.resume")}
                  </ActionContent>
                </Button>
              )}
            </div>
          )}
          {job.errorCode && (
            <ExperienceError
              message={t(`sx.errors.${job.errorCode}`)}
              retry={
                canEdit && !action.busy
                  ? () =>
                      void action.run(
                        () => api.retry(id),
                        () => {
                          void load();
                        },
                      )
                  : undefined
              }
            />
          )}
          {job.output?.cases?.map((c) => (
            <section key={c.caseId} className="flex min-w-0 flex-col gap-3">
              <h2 className="font-semibold">
                {t("sx.case")} #{c.caseId}
              </h2>
              <p role="status" className="text-sm text-muted-foreground">
                {t("sx.windowProgress", {
                  done: c.windowDone,
                  total: c.windowTotal,
                })}
              </p>
              <p className="whitespace-pre-wrap break-words text-sm">
                {c.summary}
              </p>
              {c.events.map((e) => (
                <EventEvidence key={e.id} event={e} />
              ))}
            </section>
          ))}
          {!!job.output?.revisionIds?.length && (
            <Button variant="link" className="self-start" onClick={openChanges}>
              {t("sx.review.viewChanges")}
            </Button>
          )}
          {job.kind === "distill" &&
            job.state === "succeeded" &&
            !job.output?.revisionIds?.length && (
              <p className="text-sm text-muted-foreground">
                {job.output?.cases?.some((c) => (c.skipped ?? 0) > 0)
                  ? t("sx.noLearning")
                  : t("sx.noChanges")}
              </p>
            )}
          {job.kind === "evaluate" && (
            <>
              <div className="grid min-w-0 gap-6 md:grid-cols-2">
                {["A", "B"].map((label) => (
                  <section key={label} className="flex min-w-0 flex-col gap-3">
                    <h2 className="font-semibold">
                      {t("sx.reply")} {label}
                      {job.rating &&
                        ` · ${t(job.input?.mountedLabel === label ? "sx.mounted" : "sx.unmounted")}`}
                    </h2>
                    <p className="whitespace-pre-wrap break-words text-sm leading-relaxed">
                      {job.output?.replies?.[label] || t("sx.waitingReply")}
                    </p>
                  </section>
                ))}
              </div>
              {job.state === "succeeded" &&
                (job.rating ? (
                  <div className="flex flex-col gap-2 text-sm">
                    <p>
                      {t("sx.rating")}: {t(`sx.ratings.${job.rating}`)}
                    </p>
                    <p className="whitespace-pre-wrap break-words">
                      {job.ratingNote}
                    </p>
                  </div>
                ) : (
                  <form
                    className="flex flex-col gap-3"
                    onSubmit={form.handleSubmit((v) =>
                      action.run(
                        () => api.rate(id, v.rating, v.note),
                        () => {
                          form.reset(v);
                          void load();
                        },
                      ),
                    )}
                  >
                    <FieldGroup>
                      <Field>
                        <FieldLabel htmlFor="job-rating">
                          {t("sx.rating")}
                        </FieldLabel>
                        <Controller
                          control={form.control}
                          name="rating"
                          rules={{ required: true }}
                          render={({ field }) => (
                            <OptionCombobox
                              id="job-rating"
                              value={field.value}
                              onChange={field.onChange}
                              disabled={!canEdit || action.busy}
                              placeholder={t("sx.selectRating")}
                              options={["A", "B", "tie", "neither"].map(
                                (value) => ({
                                  value,
                                  label: t(`sx.ratings.${value}`),
                                }),
                              )}
                            />
                          )}
                        />
                      </Field>
                      <Field>
                        <FieldLabel htmlFor="job-rating-note">
                          {t("sx.ratingNote")}
                        </FieldLabel>
                        <Textarea
                          id="job-rating-note"
                          {...form.register("note")}
                          disabled={!canEdit || action.busy}
                        />
                      </Field>
                    </FieldGroup>
                    <Button
                      type="submit"
                      className="self-start"
                      disabled={!canEdit || action.busy || !rating}
                    >
                      <ActionContent busy={action.busy}>
                        <SaveIcon data-icon="inline-start" />
                        {t("sx.saveRating")}
                      </ActionContent>
                    </Button>
                  </form>
                ))}
            </>
          )}
        </>
      )}
    </ProjectDialog>
  );
}
