import { request } from "@/lib/api/client";
import { MemoryKind, type MemoryStatus } from "@/lib/generated/enums";
import type { ReceptionPolicy } from "@/lib/reception";

export type MemoryEntry = {
  id: number;
  conversationId: number;
  kind: MemoryKind;
  fieldKey?: string;
  topic: string;
  label: string;
  value: string;
  confirmed: boolean;
  revision: number;
  updatedAt: string;
  sources: { id: number; conversationId: number; senderType: string; content: string; createdAt: string }[];
};

export type ConversationMemory = {
  receptionPolicy?: ReceptionPolicy;
  status: MemoryStatus;
  errorCode: string;
  attemptCount: number;
  maxAttempts: number;
  nextRetryAt?: string;
  processedMessageId: number;
  updatedAt?: string;
  handoffReason: string;
  entries: MemoryEntry[];
  shared: MemoryEntry[];
  dossier?: MemoryEntry[];
};

export function fetchConversationMemory(id: number) {
  return request<ConversationMemory>(`/api/dashboard/conversation/${id}/memory`);
}

export function refreshConversationMemory(id: number) {
  return request<void>(`/api/dashboard/conversation/${id}/memory/refresh`, { method: "POST" });
}

export function updateConversationMemory(conversationId: number, entry: MemoryEntry, value: string, remove = false) {
  const action = entry.kind === MemoryKind.CustomerProfile ? "profile/update" : "update";
  return request<void>(`/api/dashboard/conversation/${conversationId}/memory/${action}`, {
    method: "POST", body: JSON.stringify({ id: entry.id, revision: entry.revision, value, delete: remove }),
  });
}
