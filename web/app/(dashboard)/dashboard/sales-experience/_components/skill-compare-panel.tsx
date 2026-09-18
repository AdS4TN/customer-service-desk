"use client";

import { useEffect, useMemo, useState } from "react";
import { FlaskConicalIcon, LockKeyholeIcon } from "lucide-react";
import { useI18n } from "@/i18n/provider";
import { OptionCombobox } from "@/components/option-combobox";
import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import { Field, FieldDescription, FieldGroup, FieldLabel } from "@/components/ui/field";
import { Textarea } from "@/components/ui/textarea";
import {
  fetchAIAgentsAll,
  fetchSkillDefinitionsAll,
  type AIAgent,
  type SkillDefinition,
} from "@/lib/api/admin";
import {
  experienceAPI,
  type SkillComparePreview,
  type SkillCompareResult,
} from "@/lib/api/sales-experience";
import { ActionContent, ExperienceError, useExperienceAction } from "./shared";

function ReplyPanel({
  title,
  badge,
  preview,
}: {
  title: string;
  badge: string;
  preview: SkillComparePreview;
}) {
  const t = useI18n();
  return (
    <Card className="min-w-0" size="sm">
      <CardHeader className="border-b">
        <div className="flex min-w-0 flex-wrap items-center justify-between gap-2">
          <CardTitle className="break-words">{title}</CardTitle>
          <Badge variant="outline">{badge}</Badge>
        </div>
        <CardDescription className="flex flex-wrap gap-x-4 gap-y-1">
          <span>{t("sx.compare.model")}: {preview.modelName}</span>
          <span>{t("sx.compare.durationValue", { value: preview.durationMs })}</span>
        </CardDescription>
      </CardHeader>
      <CardContent>
        <div className="mb-2 text-xs font-medium text-muted-foreground">
          {t("sx.compare.reply")}
        </div>
        <div className="min-h-28 whitespace-pre-wrap break-words text-sm leading-6">
          {preview.content}
        </div>
      </CardContent>
    </Card>
  );
}

export function SkillComparePanel() {
  const t = useI18n();
  const action = useExperienceAction();
  const [agents, setAgents] = useState<AIAgent[]>([]);
  const [skills, setSkills] = useState<SkillDefinition[]>([]);
  const [loading, setLoading] = useState(true);
  const [loadError, setLoadError] = useState("");
  const [agentId, setAgentId] = useState("");
  const [skillId, setSkillId] = useState("");
  const [message, setMessage] = useState("");
  const [result, setResult] = useState<SkillCompareResult | null>(null);

  useEffect(() => {
    let active = true;
    Promise.all([fetchAIAgentsAll({ status: 1 }), fetchSkillDefinitionsAll()])
      .then(([agentRows, skillRows]) => {
        if (!active) return;
        setAgents(agentRows);
        setSkills(skillRows);
        setAgentId((value) => value || String(agentRows[0]?.id ?? ""));
        setSkillId((value) => value || String(skillRows[0]?.id ?? ""));
      })
      .catch((error) => {
        if (active)
          setLoadError(error instanceof Error ? error.message : t("sx.failed"));
      })
      .finally(() => {
        if (active) setLoading(false);
      });
    return () => {
      active = false;
    };
  }, [t]);

  const agentOptions = useMemo(
    () =>
      agents.map((agent) => ({
        value: String(agent.id),
        label: agent.displayName || agent.name,
        subtitle: agent.aiConfigName,
      })),
    [agents],
  );
  const skillOptions = useMemo(
    () =>
      skills.map((skill) => ({
        value: String(skill.id),
        label: skill.name,
        subtitle: skill.status === 1 ? undefined : t("sx.compare.disabledSkill"),
        description: skill.description,
      })),
    [skills, t],
  );
  const clearResult = () => setResult(null);
  const canRun =
    !loading && !action.busy && !!agentId && !!skillId && !!message.trim();

  return (
    <div className="flex min-w-0 flex-col gap-4">
      <div>
        <h2 className="text-base font-semibold">{t("sx.compare.title")}</h2>
        <p className="mt-1 text-sm text-muted-foreground">
          {t("sx.compare.description")}
        </p>
      </div>

      <Alert>
        <LockKeyholeIcon />
        <AlertTitle>{t("sx.compare.privatePreview")}</AlertTitle>
        <AlertDescription>{t("sx.compare.privatePreviewBody")}</AlertDescription>
      </Alert>

      <ExperienceError message={loadError || action.error} />

      <FieldGroup className="grid gap-4 md:grid-cols-2">
        <Field>
          <FieldLabel htmlFor="skill-compare-agent">{t("sx.compare.agent")}</FieldLabel>
          <OptionCombobox
            id="skill-compare-agent"
            value={agentId}
            onChange={(value) => {
              setAgentId(value);
              clearResult();
            }}
            options={agentOptions}
            placeholder={loading ? t("common.loading") : t("sx.compare.selectAgent")}
            emptyText={t("sx.compare.noAgents")}
            disabled={loading || action.busy || agentOptions.length === 0}
          />
        </Field>
        <Field>
          <FieldLabel htmlFor="skill-compare-skill">{t("sx.compare.skill")}</FieldLabel>
          <OptionCombobox
            id="skill-compare-skill"
            value={skillId}
            onChange={(value) => {
              setSkillId(value);
              clearResult();
            }}
            options={skillOptions}
            placeholder={loading ? t("common.loading") : t("sx.compare.selectSkill")}
            emptyText={t("sx.compare.noSkills")}
            disabled={loading || action.busy || skillOptions.length === 0}
          />
        </Field>
      </FieldGroup>

      <Field>
        <FieldLabel htmlFor="skill-compare-message">
          {t("sx.compare.customerMessage")}
        </FieldLabel>
        <Textarea
          id="skill-compare-message"
          value={message}
          onChange={(event) => {
            setMessage(event.target.value);
            clearResult();
          }}
          placeholder={t("sx.compare.customerMessagePlaceholder")}
          className="min-h-28 resize-y"
          disabled={action.busy}
        />
        <FieldDescription>{t("sx.compare.customerMessageDescription")}</FieldDescription>
      </Field>

      <div>
        <Button
          disabled={!canRun}
          onClick={() =>
            void action.run(
              () =>
                experienceAPI.compareSkill(Number(agentId), Number(skillId), [
                  { role: "user", content: message.trim() },
                ]),
              setResult,
            )
          }
        >
          <ActionContent busy={action.busy}>
            {!action.busy && <FlaskConicalIcon data-icon="inline-start" />}
            {t(action.busy ? "sx.compare.running" : "sx.compare.run")}
          </ActionContent>
        </Button>
      </div>

      {result && (
        <section className="min-w-0" aria-label={t("sx.compare.results") }>
          <div className="mb-3 flex min-w-0 flex-wrap items-center gap-2 text-sm">
            <span className="font-medium">{result.aiAgentName}</span>
            <span className="text-muted-foreground">/</span>
            <span className="break-words text-muted-foreground">{result.skillName}</span>
          </div>
          <div className="grid min-w-0 gap-4 lg:grid-cols-2">
            <ReplyPanel
              title={t("sx.compare.withoutSkill")}
              badge={t("sx.compare.baseline")}
              preview={result.withoutSkill}
            />
            <ReplyPanel
              title={t("sx.compare.withSkill")}
              badge={t("sx.compare.mounted")}
              preview={result.withSkill}
            />
          </div>
        </section>
      )}
    </div>
  );
}
