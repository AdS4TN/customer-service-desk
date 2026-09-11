"use client";

import { ArrowDownToLineIcon, LoaderCircleIcon, SparklesIcon, XIcon } from "lucide-react";
import { useEffect, useRef, useState } from "react";
import { toast } from "sonner";
import { useAuth } from "@/components/auth-provider";
import { Alert, AlertDescription } from "@/components/ui/alert";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { useI18n } from "@/i18n/provider";
import type { AgentConversation } from "@/lib/api/agent";
import { suggestConversationReply, type ReplySuggestion } from "@/lib/api/conversation-copilot";
import { isCurrentSuggestion } from "@/lib/copilot";
import { useAgentConversationsStore } from "@/lib/stores/agent-conversations";
import { PrivateTranslation } from "./conversation-translation";

export function ConversationCopilot({ conversation }: { conversation: AgentConversation }) {
  return <CopilotBody key={conversation.id} conversation={conversation} />;
}

function CopilotBody({ conversation }: { conversation: AgentConversation }) {
  const t = useI18n();
  const { session } = useAuth();
  const [suggestion, setSuggestion] = useState<ReplySuggestion | null>(null);
  const [error, setError] = useState("");
  const [generating, setGenerating] = useState(false);
  const [inserted, setInserted] = useState(false);
  const request = useRef<AbortController | null>(null);
  const current = useRef(conversation);
  current.current = conversation;
  const sending = useAgentConversationsStore((s) => s.sending);
  const uploading = useAgentConversationsStore((s) => s.uploadingAsset);
  const channel = useAgentConversationsStore((s) => s.channels.find((c) => c.id === conversation.channelId));
  const canGenerate = !!session?.permissions.some((p) => p === "*" || p === "conversation.send");
  const canInsert = canGenerate && conversation.status === 3 && conversation.currentAssigneeId === session?.user.id && channel?.status === 0 && !sending && !uploading;
  const stale = !!suggestion && !isCurrentSuggestion(suggestion, conversation);

  useEffect(() => () => request.current?.abort(), []);

  async function generate() {
    if (request.current) return;
    const controller = new AbortController();
    request.current = controller;
    setGenerating(true); setError("");
    try {
      const result = await suggestConversationReply(conversation.id, controller.signal);
      if (controller.signal.aborted) return;
      if (!isCurrentSuggestion(result, current.current)) { setError(t("copilot.stale")); return; }
      setSuggestion(result); setInserted(false);
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
    setInserted(true);
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
        <Badge variant="outline">{t("copilot.draft")}</Badge>
        <span className="text-xs text-muted-foreground">{t(`copilot.knowledge.${suggestion.knowledgeStatus}`)}</span>
      </div>
      <p className="whitespace-pre-wrap break-words text-sm leading-relaxed" dir="auto">{suggestion.content}</p>
      <PrivateTranslation conversationId={conversation.id} text={suggestion.content} />
      {!!suggestion.sources.length && <details className="text-xs">
        <summary className="cursor-pointer py-1 text-muted-foreground focus-visible:outline-ring">{t("copilot.sources", { count: suggestion.sources.length })}</summary>
        <div className="flex flex-col gap-3 py-2">
          {suggestion.sources.map((source, index) => <article key={`${source.chunkId}-${index}`} className="flex min-w-0 flex-col gap-1">
            <h4 className="break-words font-medium">{source.title || t("copilot.source")}</h4>
            <p className="max-h-48 overflow-y-auto whitespace-pre-wrap break-words text-muted-foreground">{source.content}</p>
          </article>)}
        </div>
      </details>}
      <p className="text-xs text-muted-foreground">{t("copilot.timing", { retrieve: (suggestion.retrievalMs / 1000).toFixed(1), generate: (suggestion.generationMs / 1000).toFixed(1) })}</p>
      {stale && <Alert><AlertDescription>{t("copilot.stale")}</AlertDescription></Alert>}
      <Button size="sm" variant="secondary" className="self-start" disabled={!canInsert || stale || inserted || generating} onClick={insert}>
        <ArrowDownToLineIcon data-icon="inline-start" />{t(inserted ? "copilot.insertedLabel" : "copilot.insert")}
      </Button>
      {!canInsert && conversation.status !== 4 && <p className="text-xs text-muted-foreground">{t("copilot.takeover")}</p>}
    </>}
  </section>;
}
