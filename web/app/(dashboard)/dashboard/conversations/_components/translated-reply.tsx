"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import { LoaderCircleIcon, RotateCwIcon, SendIcon, XIcon } from "lucide-react";
import { toast } from "sonner";
import { useAuth } from "@/components/auth-provider";
import { useConfirm } from "@/components/confirm-provider";
import { OptionCombobox } from "@/components/option-combobox";
import { Alert, AlertDescription } from "@/components/ui/alert";
import { Button } from "@/components/ui/button";
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from "@/components/ui/dialog";
import { Field, FieldGroup, FieldLabel } from "@/components/ui/field";
import { Switch } from "@/components/ui/switch";
import { Textarea } from "@/components/ui/textarea";
import { useI18n } from "@/i18n/provider";
import type { AgentConversation } from "@/lib/api/agent";
import { translateConversationText, type ConversationTranslation } from "@/lib/api/conversation-translation";
import { TranslationLanguage, TranslationLanguageLabels } from "@/lib/generated/enums";
import { agentConversationSelectors, useAgentConversationsStore } from "@/lib/stores/agent-conversations";
import { useTranslationPreferences } from "@/lib/stores/conversation-translation";
import { isTranslationContextCurrent, translatedTextHTML, translationDraftText, translationLanguageOptions } from "@/lib/translation";

type PendingReply = { html: string; text: string; conversation: AgentConversation; draft: string; resolve: () => void; reject: (reason: Error) => void };

export function useTranslatedReply(conversationId: number, onSend: (html: string) => Promise<void>, disabled: boolean) {
  const t = useI18n();
  const { session } = useAuth();
  const { preferences, update, readDraft, saveDraft } = useTranslationPreferences(conversationId);
  const confirmReplacement = useConfirm();
  const [pending, setPending] = useState<PendingReply | null>(null);
  const pendingRef = useRef<PendingReply | null>(null);
  const request = useRef<AbortController | null>(null);
  const sending = useRef(false);
  const replacing = useRef(false);
  const [busy, setBusy] = useState(false);
  const [sendingPreview, setSendingPreview] = useState(false);
  const [result, setResult] = useState<ConversationTranslation | null>(null);
  const [translated, setTranslated] = useState("");
  const [target, setTarget] = useState(preferences.customer);
  const [error, setError] = useState("");
  const [stale, setStale] = useState(false);
  const [confirmingReplacement, setConfirmingReplacement] = useState(false);

  const cancel = useCallback(() => {
    request.current?.abort(); request.current = null;
    pendingRef.current?.reject(new Error("Translation preview canceled")); pendingRef.current = null;
    setPending(null); setBusy(false);
  }, []);

  function isCurrent(value: PendingReply) {
    const state = useAgentConversationsStore.getState();
    return isTranslationContextCurrent(value.conversation, agentConversationSelectors.selectedConversation(state), state.selectedConversationId) &&
      (state.drafts[conversationId] ?? "") === value.draft;
  }

  useEffect(() => {
    const unsubscribe = useAgentConversationsStore.subscribe((state) => {
      const value = pendingRef.current;
      if (!value || sending.current) return;
      if (!isTranslationContextCurrent(value.conversation, agentConversationSelectors.selectedConversation(state), state.selectedConversationId) ||
        (state.drafts[conversationId] ?? "") !== value.draft) {
        cancel(); toast.error(t("translation.stale"));
      }
    });
    return () => { unsubscribe(); request.current?.abort(); if (!sending.current) pendingRef.current?.reject(new Error("Translation preview closed")); pendingRef.current = null; };
  }, [cancel, conversationId, t]);

  async function generate(value: PendingReply, language: TranslationLanguage) {
    request.current?.abort();
    const controller = new AbortController(); request.current = controller;
    setBusy(true); setStale(true); setError("");
    try {
      const translation = await translateConversationText(conversationId, value.text, language, controller.signal);
      if (controller.signal.aborted || pendingRef.current !== value) return;
      if (!isCurrent(value) || translation.lastMessageId !== value.conversation.lastMessageId) { cancel(); toast.error(t("translation.stale")); return; }
      setResult(translation); setTranslated(translation.content); setStale(false);
      const { id, lastMessageId, currentAssigneeId, status, customerId, channelId, aiAgentId } = value.conversation;
      saveDraft({ html: value.html, draft: value.draft, context: { id, lastMessageId, currentAssigneeId, status, customerId, channelId, aiAgentId }, target: language, result: translation, edited: translation.content });
    } catch (e) { if (!controller.signal.aborted) setError(e instanceof Error ? e.message : t("translation.failed")); }
    finally { if (request.current === controller) { request.current = null; setBusy(false); } }
  }

  async function regenerate(language: TranslationLanguage) {
    const value = pendingRef.current;
    if (!value || sending.current || replacing.current) return;
    const saved = readDraft();
    if (saved && saved.edited !== saved.result.content) {
      replacing.current = true; setConfirmingReplacement(true);
      const replace = await confirmReplacement({ title: t("translation.replaceTitle"), description: t("translation.replaceBody"), confirmText: t("translation.replace"), cancelText: t("translation.keepEditing") });
      replacing.current = false; setConfirmingReplacement(false);
      if (!replace || pendingRef.current !== value) return;
    }
    if (!isCurrent(value)) { cancel(); toast.error(t("translation.stale")); return; }
    setTarget(language); update({ customer: language });
    await generate(value, language);
  }

  async function send(html: string) {
    if (!preferences.beforeSend) return onSend(html);
    const state = useAgentConversationsStore.getState();
    const conversation = agentConversationSelectors.selectedConversation(state);
    if (disabled || pendingRef.current || !conversation || conversation.id !== conversationId || conversation.currentAssigneeId !== session?.user.id || state.sending || state.uploadingAsset) throw new Error(t("translation.stale"));
    const text = translationDraftText(html);
    if (text === null || !text || Array.from(text).length > 6000) {
      const message = t(text === null ? "translation.mixedMedia" : "translation.length");
      toast.error(message); throw new Error(message);
    }
    return new Promise<void>((resolve, reject) => {
      const value = { html, text, conversation: { ...conversation }, draft: state.drafts[conversationId] ?? "", resolve, reject };
      pendingRef.current = value; setPending(value); setTarget(preferences.customer);
      setError(""); setBusy(false);
      const saved = readDraft();
      if (saved) {
        setResult(saved.result); setTranslated(saved.edited);
        setStale(saved.html !== html || saved.draft !== value.draft || saved.target !== preferences.customer || !isTranslationContextCurrent(saved.context, conversation, state.selectedConversationId));
      } else {
        setResult(null); setTranslated("");
        void generate(value, preferences.customer);
      }
    });
  }

  async function confirm() {
    const value = pendingRef.current;
    if (!value || !result || busy || stale || replacing.current || sending.current || !translated.trim()) return;
    if (disabled || !isCurrent(value) || value.conversation.currentAssigneeId !== session?.user.id || useAgentConversationsStore.getState().sending) { cancel(); toast.error(t("translation.stale")); return; }
    sending.current = true; setSendingPreview(true);
    try {
      await onSend(translatedTextHTML(translated.trim()));
      saveDraft(null); value.resolve(); pendingRef.current = null; setPending(null);
    } catch (e) {
      if (pendingRef.current !== value) value.reject(e instanceof Error ? e : new Error(t("translation.sendFailed")));
      else { setError(t("translation.sendFailed")); setStale(!isCurrent(value)); }
    } finally { sending.current = false; setSendingPreview(false); }
  }

  const controls = <div className="flex shrink-0 flex-wrap items-center gap-x-4 gap-y-2 px-3 py-2">
    <label className="flex items-center gap-2 text-xs"><Switch size="sm" checked={preferences.beforeSend} disabled={!!pending || disabled} onCheckedChange={(beforeSend) => update({ beforeSend })} />{t("translation.beforeSend")}</label>
    {preferences.beforeSend && <div role="group" aria-label={t("translation.customer")} className="flex min-w-0 items-center gap-2">
      <span className="text-xs">{t("translation.customer")}</span>
      <OptionCombobox options={translationLanguageOptions(t("translation.auto"), true)} value={preferences.customer} disabled={!!pending || disabled} onChange={(value) => update({ customer: value as TranslationLanguage })} placeholder={t("translation.customer")} triggerClassName="h-8 w-36" />
    </div>}
  </div>;

  const dialog = <Dialog open={!!pending} onOpenChange={(open) => { if (!open && !sending.current) cancel(); }}>
    <DialogContent className="max-h-[90dvh] overflow-y-auto rounded-md sm:max-w-xl" showCloseButton={false}>
      <DialogHeader><DialogTitle>{t("translation.preview")}</DialogTitle><DialogDescription>{t("translation.recipient", { name: pending?.conversation.customerName ?? "" })}</DialogDescription></DialogHeader>
      <FieldGroup>
        <Field><FieldLabel>{t("translation.original")}</FieldLabel><p className="max-h-32 overflow-y-auto whitespace-pre-wrap break-words text-sm [overflow-wrap:anywhere]" dir="auto">{pending?.text}</p></Field>
        <Field><FieldLabel>{t("translation.customer")}</FieldLabel>
          <OptionCombobox options={translationLanguageOptions(t("translation.auto"), true)} value={target} disabled={busy || sendingPreview || confirmingReplacement} onChange={(value) => { void regenerate(value as TranslationLanguage); }} placeholder={t("translation.customer")} />
        </Field>
        {busy && <div role="status" className="flex items-center gap-2 text-sm text-muted-foreground"><LoaderCircleIcon className="size-4 motion-safe:animate-spin" />{t("translation.translating")}</div>}
        {error && <Alert variant="destructive"><AlertDescription>{error}</AlertDescription></Alert>}
        {stale && result && !busy && <Alert><AlertDescription>{t("translation.savedStale")}</AlertDescription></Alert>}
        {result && <Field><FieldLabel htmlFor="translated-reply">{t("translation.outgoing", { language: TranslationLanguageLabels[result.targetLanguage] })}</FieldLabel><Textarea id="translated-reply" className="min-h-32 max-h-64" dir="auto" lang={result.targetLanguage} value={translated} disabled={busy || sendingPreview || confirmingReplacement} onChange={(event) => {
          setTranslated(event.target.value);
          const saved = readDraft();
          if (saved) saveDraft({ ...saved, edited: event.target.value });
        }} /></Field>}
      </FieldGroup>
      <DialogFooter className="rounded-b-md">
        <Button type="button" variant="ghost" disabled={sendingPreview} onClick={cancel}><XIcon data-icon="inline-start" />{t("translation.cancel")}</Button>
        <Button type="button" variant="outline" disabled={busy || sendingPreview || confirmingReplacement} onClick={() => { void regenerate(target); }}><RotateCwIcon data-icon="inline-start" />{t("translation.retry")}</Button>
        <Button type="button" disabled={busy || stale || sendingPreview || confirmingReplacement || !result || !translated.trim()} onClick={() => void confirm()}>{sendingPreview ? <LoaderCircleIcon data-icon="inline-start" className="motion-safe:animate-spin" /> : <SendIcon data-icon="inline-start" />}{t(sendingPreview ? "translation.sending" : "translation.confirm")}</Button>
      </DialogFooter>
    </DialogContent>
  </Dialog>;
  return { send, controls, dialog, beforeSend: preferences.beforeSend, previewing: !!pending };
}
