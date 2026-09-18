"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import { RefreshCwIcon, SparklesIcon } from "lucide-react";
import { useI18n } from "@/i18n/provider";
import { useAuth } from "@/components/auth-provider";
import { useConfirm } from "@/components/confirm-provider";
import { OptionCombobox } from "@/components/option-combobox";
import { Button } from "@/components/ui/button";
import {
  Field,
  FieldDescription,
  FieldGroup,
  FieldLabel,
} from "@/components/ui/field";
import {
  experienceAPI as api,
  type ExperienceCase,
  type ExperienceOptions,
  type SkillMinerDebugResult,
} from "@/lib/api/sales-experience";
import { formatDateTime } from "@/lib/utils";
import {
  ActionContent,
  ExperienceError,
  ModelPicker,
  useExperienceAction,
} from "./shared";
import { SkillMinerResultView } from "./skill-miner-result";

export function SkillMinerDebugPanel({
  canEdit: canEditProp,
  canConfirm: canConfirmProp,
  extracted = () => undefined,
}: {
  canEdit?: boolean;
  canConfirm?: boolean;
  extracted?: () => void;
}) {
  const t = useI18n();
  const { session } = useAuth();
  const allowed = useMemo(
    () =>
      !!session &&
      (session.roles.includes("super_admin") ||
        session.permissions.includes("*") ||
        (session.permissions.includes("conversation.view") &&
          session.permissions.includes("skillDefinition.view"))),
    [session],
  );
  const canEdit =
    canEditProp ??
    Boolean(
      session &&
        (session.roles.includes("super_admin") ||
          session.permissions.includes("*") ||
          session.permissions.includes("skillDefinition.update")),
    );
  const canConfirm =
    canConfirmProp ??
    Boolean(
      canEdit &&
        session &&
        (session.roles.includes("super_admin") ||
          session.permissions.includes("*") ||
          session.permissions.includes("skillDefinition.create")),
    );
  const [options, setOptions] = useState<ExperienceOptions>({
    skills: [],
    models: [],
  });
  const [cases, setCases] = useState<ExperienceCase[]>([]);
  const [caseId, setCaseId] = useState(0);
  const [modelConfigId, setModelConfigId] = useState(0);
  const [output, setOutput] = useState<SkillMinerDebugResult | null>(null);
  const [extractionStatus, setExtractionStatus] = useState<
    ExperienceCase["extractionStatus"]
  >("");
  const [extractedAt, setExtractedAt] = useState("");
  const [resultLoading, setResultLoading] = useState(true);
  const [resultError, setResultError] = useState("");
  const [loadError, setLoadError] = useState("");
  const action = useExperienceAction();
  const confirm = useConfirm();

  const runExtraction = useCallback(async () => {
    if (!caseId || !modelConfigId || action.busy || resultLoading) return;
    if (
      output?.result &&
      !(await confirm({
        title: t("sx.miner.rerunTitle"),
        description: t("sx.miner.rerunBody"),
        confirmText: t("sx.miner.rerun"),
        cancelText: t("common.cancel"),
      }))
    )
      return;
    setOutput(null);
    setExtractionStatus("running");
    setExtractedAt("");
    void action.run(
      () => api.skillMinerDebug(caseId, modelConfigId),
      (result) => {
        setOutput(result);
        setExtractionStatus(
          result.ok
            ? result.result?.hasLearnableSkill
              ? "succeeded"
              : "no_skill"
            : "failed",
        );
        setExtractedAt(new Date().toISOString());
        extracted();
      },
    );
  }, [action, caseId, confirm, extracted, modelConfigId, output, resultLoading, t]);

  const load = useCallback(() => {
    if (!allowed) return;
    void Promise.all([api.options(), api.cases({ page: 1, limit: 100 })])
      .then(([opts, page]) => {
        setLoadError("");
        setOptions(opts);
        setCases(page.results);
        if (page.results[0]) setCaseId(page.results[0].id);
        else setResultLoading(false);
        if (opts.models[0]) setModelConfigId(opts.models[0].id);
      })
      .catch((error) => {
        setLoadError(error instanceof Error ? error.message : t("sx.failed"));
      });
  }, [allowed, t]);

  useEffect(() => {
    load();
  }, [load]);

  useEffect(() => {
    if (!caseId) return;
    let active = true;
    void api
      .case(caseId)
      .then((item) => {
        if (!active) return;
        setOutput(item.extractionResult ?? null);
        setExtractionStatus(item.extractionStatus);
        setExtractedAt(item.extractedAt ?? "");
      })
      .catch((error) => {
        if (!active) return;
        setResultError(error instanceof Error ? error.message : t("sx.failed"));
      })
      .finally(() => {
        if (active) setResultLoading(false);
      });
    return () => {
      active = false;
    };
  }, [caseId, t]);

  if (!allowed) {
    return <ExperienceError message={t("sx.noPermission")} />;
  }

  return (
    <section className="flex min-w-0 flex-col gap-6">
      <header className="flex max-w-3xl flex-col gap-1.5">
        <h2 className="text-lg font-semibold">{t("sx.miner.title")}</h2>
        <p className="text-sm leading-6 text-muted-foreground">
          {t("sx.miner.description")}
        </p>
      </header>
      <ExperienceError message={loadError} retry={load} />
      <ExperienceError message={action.error} />
      <ExperienceError message={resultError} />
      {!cases.length && !loadError ? (
        <p className="text-sm text-muted-foreground">{t("sx.noCases")}</p>
      ) : (
        <FieldGroup className="grid gap-4 sm:grid-cols-2 lg:grid-cols-[minmax(16rem,1fr)_minmax(14rem,20rem)_auto] lg:items-end">
          <Field>
            <FieldLabel htmlFor="skill-miner-case">{t("sx.case")}</FieldLabel>
            <OptionCombobox
              id="skill-miner-case"
              value={caseId ? String(caseId) : ""}
              onChange={(value) => {
                setResultLoading(true);
                setResultError("");
                setCaseId(Number(value));
                setOutput(null);
                setExtractionStatus("");
                setExtractedAt("");
              }}
              disabled={action.busy || resultLoading}
              placeholder={t("sx.miner.selectCase")}
              options={cases.map((item) => ({
                value: String(item.id),
                label: item.name,
                subtitle: t("sx.miner.caseMessages", {
                  id: item.id,
                  count: item.messageCount,
                }),
              }))}
            />
            <FieldDescription>
              {resultLoading
                ? t("common.loading")
                : t(
                    `sx.extractionStatuses.${extractionStatus || "pending"}`,
                  )}
              {!resultLoading && extractedAt
                ? ` · ${formatDateTime(extractedAt)}`
                : ""}
            </FieldDescription>
          </Field>
          <Field>
            <FieldLabel htmlFor="experience-model">{t("sx.model")}</FieldLabel>
            <ModelPicker
              options={options}
              value={modelConfigId}
              onChange={setModelConfigId}
              disabled={action.busy}
            />
            {!options.models.length && (
              <FieldDescription>{t("sx.noModel")}</FieldDescription>
            )}
          </Field>
          <Button
            className="sm:col-span-2 lg:col-span-1"
            disabled={!caseId || !modelConfigId || action.busy || resultLoading}
            onClick={() => void runExtraction()}
          >
            <ActionContent busy={action.busy}>
              {output ? (
                <RefreshCwIcon data-icon="inline-start" />
              ) : (
                <SparklesIcon data-icon="inline-start" />
              )}
              {action.busy
                ? t("sx.miner.running")
                : output
                  ? t("sx.miner.rerun")
                  : t("sx.miner.run")}
            </ActionContent>
          </Button>
        </FieldGroup>
      )}
      {output && (
        <SkillMinerResultView
          key={`${caseId}-${extractedAt}`}
          output={output}
          canEdit={canEdit}
          canConfirm={canConfirm}
          busy={action.busy}
          onConfirm={(candidate) =>
            action.run(
              () => api.reviewSkillMiner(caseId, "confirm", candidate.sourceEpisodeId, candidate.skill),
              (result) => {
                setOutput(result);
                extracted();
              },
            )
          }
          onDismiss={(sourceEpisodeId) =>
            action.run(
              () => api.reviewSkillMiner(caseId, "dismiss", sourceEpisodeId),
              (result) => {
                setOutput(result);
                extracted();
              },
            )
          }
        />
      )}
    </section>
  );
}
