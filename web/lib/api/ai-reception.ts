import { request } from "./client"

export type AIReceptionState = {
  agentId: number
  serviceMode: number
  receptionEnabled: boolean
  assistanceAvailable: boolean
  publishedRevisionId: number
  modelName: string
  knowledgeCount: number
  skillCount: number
  channels: {
    id: number
    name: string
    channelType: string
    receptionEnabled: boolean
    automaticMessagesAllowed: boolean
    outboundBlocked: boolean
  }[]
}

export function fetchAIReceptionState(id: number) {
  return request<AIReceptionState>(`/api/dashboard/ai-agent/${id}/reception_state`)
}
