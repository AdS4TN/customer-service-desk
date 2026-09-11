"use client";

import { create } from "zustand";
import { useAuth } from "@/components/auth-provider";
import { TranslationLanguage } from "@/lib/generated/enums";
import type { ConversationTranslation } from "@/lib/api/conversation-translation";
import type { TranslationContext } from "@/lib/translation";

type Preferences = { reading: TranslationLanguage; customer: TranslationLanguage; beforeSend: boolean; autoRead: boolean };
const defaults: Preferences = { reading: TranslationLanguage.Chinese, customer: TranslationLanguage.Auto, beforeSend: false, autoRead: false };

export type TranslationReplyDraft = { html: string; draft: string; context: TranslationContext; target: TranslationLanguage; result: ConversationTranslation; edited: string };

// Session memory only, scoped by operator and conversation; never localStorage or message history.
const useTranslationStore = create<{
  preferences: Record<string, Preferences>;
  drafts: Record<string, TranslationReplyDraft>;
  update: (key: string, patch: Partial<Preferences>) => void;
  saveDraft: (key: string, draft: TranslationReplyDraft | null) => void;
}>((set) => ({
  preferences: {},
  drafts: {},
  update: (key, patch) => set((state) => ({ preferences: { ...state.preferences, [key]: { ...(state.preferences[key] ?? defaults), ...patch } } })),
  saveDraft: (key, draft) => set((state) => {
    const drafts = { ...state.drafts };
    if (draft) drafts[key] = draft; else delete drafts[key];
    return { drafts };
  }),
}));

export function useTranslationPreferences(conversationId: number) {
  const { session } = useAuth();
  const key = `${session?.user.id ?? 0}:${conversationId}`;
  const preferences = useTranslationStore((state) => state.preferences[key] ?? defaults);
  return {
    preferences,
    update: (patch: Partial<Preferences>) => useTranslationStore.getState().update(key, patch),
    readDraft: () => useTranslationStore.getState().drafts[key],
    saveDraft: (draft: TranslationReplyDraft | null) => useTranslationStore.getState().saveDraft(key, draft),
  };
}
