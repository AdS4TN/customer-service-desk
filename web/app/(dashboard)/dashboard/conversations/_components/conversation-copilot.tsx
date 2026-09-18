"use client";

import {
  ArrowDownToLineIcon,
  BookOpenCheckIcon,
  BrainCircuitIcon,
  LoaderCircleIcon,
  SparklesIcon,
  WrenchIcon,
  XIcon,
  type LucideIcon,
} from "lucide-react";
import { useEffect, useRef, useState } from "react";
import { toast } from "sonner";
import { useAuth } from "@/components/auth-provider";
import { Alert, AlertDescription } from "@/components/ui/alert";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Separator } from "@/components/ui/separator";
import { useI18n } from "@/i18n/provider";
import type { AgentConversation } from "@/lib/api/agent";
import { suggestConversationReply } from "@/lib/api/conversation-copilot";
import { isCurrentSuggestion } from "@/lib/copilot";
import { useAgentConversationsStore } from "@/lib/stores/agent-conversations";
import { PrivateTranslation } from "./conversation-translation";

function TraceRow({
  icon: Icon,
  label,
  children,
}: {
  icon: LucideIcon;
  label: string;
  children: React.ReactNode;
}) {
  return (
    <div className="grid min-w-0 grid-cols-[1rem_minmax(0,1fr)] gap-x-2 gap-y-1 text-xs">
      <Icon className="mt-0.5 text-muted-foreground" aria-hidden />
      <div className="min-w-0">
        <div className="font-medium text-foreground">{label}</div>
        <div className="mt-1 min-w-0 text-muted-foreground">{children}</div>
      </div>
    </div>
  );
}

export function ConversationCopilot({ conversation }: { conversation: AgentConversation }) {
  return <CopilotBody key={conversation.id} conversation={conversation} />;
}

function CopilotBody({ conversation }: { conversation: AgentConversation }) {
  const t = useI18n();
  const { session } = useAuth();
  const [error, setError] = useState("");
  const [generating, setGenerating] = useState(false);
  const request = useRef<AbortController | null>(null);
  const current = useRef(conversation);
  current.current = conversation;
  const savedSuggestion = useAgentConversationsStore((s) => s.copilotSuggestions[conversation.id]);
  const suggestion = savedSuggestion?.suggestion ?? null;
  const inserted = savedSuggestion?.inserted ?? false;
  const saveSuggestion = useAgentConversationsStore((s) => s.saveCopilotSuggestion);
  const markInserted = useAgentConversationsStore((s) => s.markCopilotSuggestionInserted);
  const clearSuggestion = useAgentConversationsStore((s) => s.clearCopilotSuggestion);
  const sending = useAgentConversationsStore((s) => s.sending);
  const uploading = useAgentConversationsStore((s) => s.uploadingAsset);
  const channel = useAgentConversationsStore((s) => s.channels.find((c) => c.id === conversation.channelId));
  const canGenerate = !!session?.permissions.some((p) => p === "*" || p === "conversation.send");
  const canInsert = canGenerate && conversation.status === 3 && conversation.currentAssigneeId === session?.user.id && channel?.status === 0 && !sending && !uploading;
  const stale = !!suggestion && !isCurrentSuggestion(suggestion, conversation);

  useEffect(() => () => request.current?.abort(), []);

  useEffect(() => {
    if (suggestion && !isCurrentSuggestion(suggestion, conversation)) {
      clearSuggestion(conversation.id);
    }
  }, [clearSuggestion, conversation, suggestion]);

  async function generate() {
    if (request.current) return;
    const controller = new AbortController();
    request.current = controller;
    setGenerating(true); setError("");
    try {
      const result = await suggestConversationReply(conversation.id, controller.signal);
      if (controller.signal.aborted) return;
      if (!isCurrentSuggestion(result, current.current)) { setError(t("copilot.stale")); return; }
      saveSuggestion(result);
    } catch (e) {
      if (!controller.signal.aborted) setError(e instanceof Error ? e.message : t("copilot.failed"));
    } finally {
      if (request.current === controller) { request.current = null; setGenerating(false); }
    }
  }

  function cancel() {
    request.current?.abort(); request.current = null; setGenerating(false);
  }

  function insert() {
    if (!suggestion || !canInsert || stale || inserted) return;
    if (!useAgentConversationsStore.getState().insertSuggestion(conversation.id, suggestion.lastMessageId, suggestion.content)) {
      setError(t("copilot.stale")); return;
    }
    markInserted(conversation.id);
    toast.success(t("copilot.inserted"));
  }

  return <section className="flex min-w-0 flex-col gap-3" aria-label={t("copilot.reply")}>
    <div className="flex flex-wrap items-center justify-between gap-2">
      <h3 className="text-sm font-semibold">{t("copilot.reply")}</h3>
      {canGenerate && <Button size="sm" variant="outline" disabled={generating || conversation.status === 4} onClick={() => void generate()}>
        {generating ? <LoaderCircleIcon data-icon="inline-start" className="motion-safe:animate-spin" /> : <SparklesIcon data-icon="inline-start" />}
        {t(generating ? "copilot.generating" : suggestion ? "copilot.regenerate" : "copilot.generate")}
      </Button>}
    </div>
    {generating && <div className="flex flex-wrap items-center justify-between gap-2" role="status">
      <span className="text-xs text-muted-foreground">{t("copilot.working")}</span>
      <Button size="sm" variant="ghost" onClick={cancel}><XIcon data-icon="inline-start" />{t("copilot.cancel")}</Button>
    </div>}
    {error && <Alert variant="destructive"><AlertDescription>{error}</AlertDescription></Alert>}
    {!suggestion && !generating && <p className="text-sm text-muted-foreground">{t(conversation.status === 4 ? "copilot.closed" : "copilot.empty")}</p>}
    {suggestion && <>
      <div className="flex flex-wrap items-center gap-2">
        <Badge variant="outline">{t("copilot.shadow")}</Badge>
        <Badge variant="secondary">{t("copilot.draft")}</Badge>
      </div>
      <p className="whitespace-pre-wrap break-words text-sm leading-relaxed" dir="auto">{suggestion.content}</p>
      <PrivateTranslation conversationId={conversation.id} text={suggestion.content} />
      <Separator />
      <div className="flex flex-col gap-3" aria-label={t("copilot.trace.title")}>
        <div className="flex flex-wrap items-center justify-between gap-2">
          <h4 className="text-xs font-semibold">{t("copilot.trace.title")}</h4>
          <span className="text-xs text-muted-foreground">
            {suggestion.agentName} · {suggestion.modelName}
          </span>
        </div>
        <TraceRow icon={BrainCircuitIcon} label={t("copilot.trace.skill")}>
          {suggestion.skills.length > 0 ? (
            <div className="flex min-w-0 flex-col gap-2">
              {suggestion.skills.map((skill) => (
                <div key={skill.id} className="min-w-0">
                  <Badge variant="outline" className="max-w-full whitespace-normal text-left">
                    {skill.name}
                  </Badge>
                  {skill.reason && <p className="mt-1 break-words">{skill.reason}</p>}
                </div>
              ))}
            </div>
          ) : (
            <span>{t(`copilot.skill.${suggestion.skillStatus}`)}</span>
          )}
        </TraceRow>
        <TraceRow icon={BookOpenCheckIcon} label={t("copilot.trace.knowledge")}>
          <span>{t(`copilot.knowledge.${suggestion.knowledgeStatus}`)}</span>
        </TraceRow>
        <TraceRow icon={WrenchIcon} label={t("copilot.trace.tools")}>
          <div className="flex min-w-0 flex-col gap-1.5">
            {suggestion.tools.map((tool) => (
              <div key={tool.code} className="flex min-w-0 flex-wrap items-center gap-1.5">
                <Badge variant="outline">
                  {tool.code === "builtin/conversation_context"
                    ? t("copilot.tool.conversationContext")
                    : tool.code === "builtin/knowledge_retrieve"
                      ? t("copilot.tool.knowledgeRetrieve")
                      : tool.code}
                </Badge>
                <span>{t(`copilot.toolStatus.${tool.status}`)}</span>
              </div>
            ))}
            <span>{t("copilot.trace.externalToolsBlocked")}</span>
          </div>
        </TraceRow>
      </div>
      {!!suggestion.sources.length && <details className="text-xs">
        <summary className="cursor-pointer py-1 text-muted-foreground focus-visible:outline-ring">{t("copilot.sources", { count: suggestion.sources.length })}</summary>
        <div className="flex flex-col gap-3 py-2">
          {suggestion.sources.map((source, index) => <article key={`${source.chunkId}-${index}`} className="flex min-w-0 flex-col gap-1">
            <h4 className="break-words font-medium">{source.title || t("copilot.source")}</h4>
            <p className="max-h-48 overflow-y-auto whitespace-pre-wrap break-words text-muted-foreground">{source.content}</p>
          </article>)}
        </div>
      </details>}
      <p className="text-xs text-muted-foreground">{t("copilot.timing", { route: (suggestion.routingMs / 1000).toFixed(1), retrieve: (suggestion.retrievalMs / 1000).toFixed(1), generate: (suggestion.generationMs / 1000).toFixed(1) })}</p>
      {stale && <Alert><AlertDescription>{t("copilot.stale")}</AlertDescription></Alert>}
      <Button size="sm" variant="secondary" className="self-start" disabled={!canInsert || stale || inserted || generating} onClick={insert}>
        <ArrowDownToLineIcon data-icon="inline-start" />{t(inserted ? "copilot.insertedLabel" : "copilot.insert")}
      </Button>
      {!canInsert && conversation.status !== 4 && <p className="text-xs text-muted-foreground">{t("copilot.takeover")}</p>}
    </>}
  </section>;
}
