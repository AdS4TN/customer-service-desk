import { request } from "@/lib/api/client"
import type { CreateAIAgentPayload } from "@/lib/api/admin"

export type PreviewTurn = { role: "user" | "assistant"; content: string }
export type EmployeePreviewResult = {
  content: string
  modelName: string
  knowledgeStatus: "not_configured" | "empty" | "matched"
  sources: { knowledgeBaseId: number; documentId: number; chunkId: number; title: string; content: string }[]
  mountedSkills: { id: number; name: string }[]
  durationMs: number
}
export function previewAIEmployee(draft: CreateAIAgentPayload, messages: PreviewTurn[], signal?: AbortSignal) {
  return request<EmployeePreviewResult>("/api/dashboard/ai-agent/preview", {
    method: "POST", body: JSON.stringify({ draft, messages }), signal,
  })
}
