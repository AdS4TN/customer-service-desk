import { request } from "@/lib/api/client"

export type DelegationView = {
  conversationId: number; ownerId: number; ownerName: string; canManage: boolean
  active: boolean; previewOnly: boolean; revision: number; aiAgentId: number; instructions: string
  startedByName: string; startedAt?: string; expiresAt?: string; endReason: string
  agents: { id: number; name: string; liveAvailable: boolean }[]
  events: { id: number; actorName: string; sourceMessageId: number; kind: string; reason: string; content: string; previewOnly: boolean; createdAt: string }[]
}
export type DelegationInput = { revision: number; aiAgentId: number; previewOnly: boolean; durationMinutes: number; instructions: string }
export function fetchDelegation(id: number) { return request<DelegationView>(`/api/dashboard/conversation/${id}/delegation`) }
export function startDelegation(id: number, body: DelegationInput) {
  return request<void>(`/api/dashboard/conversation/${id}/delegation/start`, { method: "POST", body: JSON.stringify(body) })
}
export function stopDelegation(id: number, revision: number) {
  return request<void>(`/api/dashboard/conversation/${id}/delegation/stop`, { method: "POST", body: JSON.stringify({ revision }) })
}
export function previewDelegation(id: number, revision: number) {
  return request<void>(`/api/dashboard/conversation/${id}/delegation/preview`, { method: "POST", body: JSON.stringify({ revision }) })
}
