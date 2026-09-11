import { request } from "@/lib/api/client";

export type ReplySuggestion = {
  conversationId: number;
  lastMessageId: number;
  content: string;
  knowledgeStatus: "matched" | "empty" | "not_configured";
  sources: { knowledgeBaseId: number; documentId: number; chunkId: number; title: string; content: string }[];
  retrievalMs: number;
  generationMs: number;
  durationMs: number;
};

export function suggestConversationReply(id: number, signal?: AbortSignal) {
  return request<ReplySuggestion>(`/api/dashboard/conversation/${id}/suggest_reply`, { method: "POST", signal });
}
