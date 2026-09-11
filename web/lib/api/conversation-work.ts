import { request } from "@/lib/api/client"
import type { ConversationWorkStatus } from "@/lib/generated/enums"

export type Colleague = { id: number; name: string }
export type PrivateNote = { id: number; authorId: number; authorName: string; content: string; mentions: Colleague[]; createdAt: string }
export type Collaboration = {
  notes: PrivateNote[]; hasMore: boolean
  handoffs: { id: number; from: string; to: string; reason: string; createdAt: string }[]
}
export function updateConversationWork(id: number, body: { revision: number; status: ConversationWorkStatus; snoozeMinutes: number; replyTargetMinutes: number }) {
  return request<void>(`/api/dashboard/conversation/${id}/work`, { method: "POST", body: JSON.stringify(body) })
}
export function fetchCollaboration(id: number, before = 0) {
  return request<Collaboration>(`/api/dashboard/conversation/${id}/collaboration?before=${before}`)
}
export function fetchColleagues() { return request<Colleague[]>("/api/dashboard/conversation/colleagues") }
export function addPrivateNote(id: number, body: { content: string; mentionIds: number[]; clientId: string }) {
  return request<void>(`/api/dashboard/conversation/${id}/notes`, { method: "POST", body: JSON.stringify(body) })
}
