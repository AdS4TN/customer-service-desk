import { request } from "@/lib/api/client";

export type ReplySuggestion = {
  conversationId: number;
  lastMessageId: number;
  agentName: string;
  modelName: string;
  content: string;
  skillStatus: "matched" | "none" | "failed" | "not_configured";
  skills: { id: number; name: string; reason: string }[];
  tools: { code: string; status: "used" | "matched" | "empty" }[];
  knowledgeStatus: "matched" | "empty" | "not_configured";
  sources: { knowledgeBaseId: number; documentId: number; chunkId: number; title: string; content: string }[];
  routingMs: number;
  retrievalMs: number;
  generationMs: number;
  durationMs: number;
};

export function suggestConversationReply(id: number, signal?: AbortSignal) {
  return request<ReplySuggestion>(`/api/dashboard/conversation/${id}/suggest_reply`, { method: "POST", signal });
}
