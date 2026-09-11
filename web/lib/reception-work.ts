import { ConversationWorkStatus, IMConversationStatus } from "@/lib/generated/enums"
import type { AgentConversation } from "@/lib/api/agent"

export function workStatusKey(status?: string) {
  if (status === ConversationWorkStatus.NeedsReply) return "reception.needsReply"
  if (status === ConversationWorkStatus.Snoozed) return "reception.snoozed"
  return "reception.waiting"
}
export function isReplyOverdue(item: AgentConversation, now: number) {
  return item.status !== IMConversationStatus.Closed && item.workStatus === ConversationWorkStatus.NeedsReply && !!item.replyDueAt && Date.parse(item.replyDueAt) <= now
}
export function shouldNotifyCustomerMessage(sender: string, status: number | undefined, assignee: number | undefined, userId: number, hidden: boolean) {
  return sender === "customer" && status === IMConversationStatus.Active && userId > 0 && assignee === userId && hidden
}
