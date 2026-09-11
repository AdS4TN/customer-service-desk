"use client";

import { LanguagesIcon, LoaderCircleIcon, RotateCwIcon, XIcon } from "lucide-react";
import { useEffect, useRef, useState } from "react";
import { OptionCombobox } from "@/components/option-combobox";
import { Alert, AlertDescription } from "@/components/ui/alert";
import { Button } from "@/components/ui/button";
import { Switch } from "@/components/ui/switch";
import { useI18n } from "@/i18n/provider";
import type { AgentMessage } from "@/lib/api/agent";
import { translateConversationMessage, translateConversationText, type ConversationTranslation } from "@/lib/api/conversation-translation";
import { IMMessageStatus, TranslationLanguage } from "@/lib/generated/enums";
import { useAgentConversationsStore } from "@/lib/stores/agent-conversations";
import { useTranslationPreferences } from "@/lib/stores/conversation-translation";
import { translationLanguageOptions } from "@/lib/translation";

export function TranslationToolbar({ conversationId }: { conversationId: number }) {
  const t = useI18n();
  const { preferences, update } = useTranslationPreferences(conversationId);
  return <div className="flex shrink-0 flex-wrap items-center gap-x-4 gap-y-2 border-b px-4 py-2">
    <div role="group" aria-label={t("translation.reading")} className="flex min-w-0 items-center gap-2">
      <LanguagesIcon className="size-4 shrink-0 text-muted-foreground" aria-hidden="true" />
      <span className="text-xs">{t("translation.reading")}</span>
      <OptionCombobox options={translationLanguageOptions(t("translation.auto"))} value={preferences.reading} onChange={(value) => update({ reading: value as TranslationLanguage })} placeholder={t("translation.reading")} triggerClassName="h-8 w-36" />
    </div>
    <label className="flex items-center gap-2 text-xs">
      <Switch size="sm" checked={preferences.autoRead} onCheckedChange={(autoRead) => update({ autoRead })} />{t("translation.autoRead")}
    </label>
  </div>;
}

export function MessageTranslation({ message }: { message: AgentMessage }) {
  const { preferences } = useTranslationPreferences(message.conversationId);
  const recentCustomerMessage = useAgentConversationsStore((state) => state.messages.filter((m) => m.senderType === "customer" && (m.messageType === "text" || m.messageType === "html") && !m.recalledAt).sort((a, b) => b.id - a.id).slice(0, 5).some((m) => m.id === message.id));
  if (message.recalledAt || [IMMessageStatus.Sending, IMMessageStatus.Failed, IMMessageStatus.Recalled].includes(message.sendStatus) || !["text", "html"].includes(message.messageType)) return null;
  return <PrivateTranslation key={`${message.id}:${message.content}:${preferences.reading}`} conversationId={message.conversationId} messageId={message.id} auto={preferences.autoRead && recentCustomerMessage} />;
}

export function PrivateTranslation({ conversationId, messageId, text, auto = false }: { conversationId: number; messageId?: number; text?: string; auto?: boolean }) {
  const { preferences } = useTranslationPreferences(conversationId);
  return <PrivateTranslationBody key={`${conversationId}:${messageId ?? text}:${preferences.reading}`} conversationId={conversationId} messageId={messageId} text={text} auto={auto} />;
}

function PrivateTranslationBody({ conversationId, messageId, text, auto = false }: { conversationId: number; messageId?: number; text?: string; auto?: boolean }) {
  const t = useI18n();
  const { preferences } = useTranslationPreferences(conversationId);
  const [result, setResult] = useState<ConversationTranslation | null>(null);
  const [error, setError] = useState("");
  const [loading, setLoading] = useState(false);
  const [visible, setVisible] = useState(false);
  const request = useRef<AbortController | null>(null);
  const translate = useRef<() => void>(() => {});

  function cancel() { request.current?.abort(); request.current = null; setLoading(false); }
  async function generate() {
    if (request.current) return;
    const controller = new AbortController(); request.current = controller;
    setLoading(true); setError(""); setVisible(true);
    try {
      const value = messageId
        ? await translateConversationMessage(conversationId, messageId, preferences.reading, controller.signal)
        : await translateConversationText(conversationId, text ?? "", preferences.reading, controller.signal);
      if (!controller.signal.aborted) setResult(value);
    } catch (e) {
      if (!controller.signal.aborted) setError(e instanceof Error ? e.message : t("translation.failed"));
    } finally { if (request.current === controller) { request.current = null; setLoading(false); } }
  }
  translate.current = () => { void generate(); };
  useEffect(() => () => request.current?.abort(), []);
  useEffect(() => { if (auto) translate.current(); }, [auto]);

  return <div className="mt-1 flex min-w-0 max-w-full flex-col gap-1">
    <div className="flex flex-wrap items-center gap-1">
      <Button type="button" variant="ghost" size="sm" disabled={loading} onClick={() => result ? setVisible(!visible) : void generate()}>
        {loading ? <LoaderCircleIcon data-icon="inline-start" className="motion-safe:animate-spin" /> : <LanguagesIcon data-icon="inline-start" />}
        {t(loading ? "translation.translating" : result && visible ? "translation.hide" : "translation.view")}
      </Button>
      {loading && <Button type="button" variant="ghost" size="icon-sm" aria-label={t("translation.cancel")} title={t("translation.cancel")} onClick={cancel}><XIcon /></Button>}
      {error && <Button type="button" variant="ghost" size="sm" onClick={() => void generate()}><RotateCwIcon data-icon="inline-start" />{t("translation.retry")}</Button>}
    </div>
    {visible && error && <Alert variant="destructive"><AlertDescription>{error}</AlertDescription></Alert>}
    {visible && result && <div className="flex min-w-0 flex-col gap-1 py-1" role="status">
      <span className="text-xs text-muted-foreground">{t("translation.privateResult")}</span>
      <p className="whitespace-pre-wrap break-words text-sm leading-relaxed [overflow-wrap:anywhere]" dir="auto" lang={result.targetLanguage}>{result.content}</p>
    </div>}
  </div>;
}
