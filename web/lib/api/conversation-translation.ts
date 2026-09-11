import { request } from "@/lib/api/client";
import type { TranslationLanguage } from "@/lib/generated/enums";

export type ConversationTranslation = {
  conversationId: number;
  messageId?: number;
  lastMessageId: number;
  sourceLanguage: TranslationLanguage | "und";
  targetLanguage: TranslationLanguage;
  content: string;
  cached: boolean;
};

export function translateConversationMessage(id: number, messageId: number, targetLanguage: TranslationLanguage, signal?: AbortSignal) {
  return request<ConversationTranslation>(`/api/dashboard/conversation/${id}/translate_message`, { method: "POST", body: JSON.stringify({ messageId, targetLanguage }), signal });
}

export function translateConversationText(id: number, text: string, targetLanguage: TranslationLanguage, signal?: AbortSignal) {
  return request<ConversationTranslation>(`/api/dashboard/conversation/${id}/translate_text`, { method: "POST", body: JSON.stringify({ text, targetLanguage }), signal });
}
