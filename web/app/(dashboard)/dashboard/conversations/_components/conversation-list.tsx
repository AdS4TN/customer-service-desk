"use client"

import { UserIcon } from "lucide-react";
import { useEffect, useState } from "react";
import { toast } from "sonner";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { inboxSourceLabel } from "@/lib/inbox-labels";
import { isReplyOverdue, workStatusKey } from "@/lib/reception-work";

import { Avatar, AvatarFallback, AvatarImage } from "@/components/ui/avatar";
import { ScrollArea } from "@/components/ui/scroll-area";
import { IMConversationStatus } from "@/lib/generated/enums";
import { useAgentConversationsStore } from "@/lib/stores/agent-conversations";
import { formatDateTime } from "@/lib/utils";
import { useI18n } from "@/i18n/provider";

type ConversationListProps = {
  onAfterSelect?: () => void
}

export function ConversationList({ onAfterSelect }: ConversationListProps) {
  const t = useI18n()
  const [now, setNow] = useState(() => Date.now())
  useEffect(() => { const timer = setInterval(() => setNow(Date.now()), 30000); return () => clearInterval(timer) }, [])
  const conversations = useAgentConversationsStore((state) => state.conversations)
  const loading = useAgentConversationsStore((state) => state.conversationsLoading)
  const selectedId = useAgentConversationsStore((state) => state.selectedConversationId)
  const selectConversation = useAgentConversationsStore((state) => state.selectConversation)
  const channels = useAgentConversationsStore((state) => state.channels)
  const error = useAgentConversationsStore((state) => state.conversationsError)
  const total = useAgentConversationsStore((state) => state.conversationsTotal)
  const load = useAgentConversationsStore((state) => state.loadConversations)
  const uploading = useAgentConversationsStore((state) => state.uploadingAsset)
  const filtered = useAgentConversationsStore((state) => state.conversationFilter !== "all" || Boolean(state.channelType || state.channelId || state.searchKeyword))

  return (
    <ScrollArea className="min-h-0 flex-1">
      {error ? <div role="alert" className="flex flex-col gap-2 p-3 text-sm text-destructive">{error}<Button variant="outline" size="sm" onClick={() => void load().catch(() => {})}>{t("conversation.inbox.retry")}</Button></div> : null}
      {loading && conversations.length === 0 ? (
        <div className="p-6 text-center text-sm text-muted-foreground">
          {t("conversation.loading")}
        </div>
      ) : conversations.length > 0 ? (
        conversations.map((conversation) => {
          const isSelected = selectedId === conversation.id
          return (
            <button
              type="button"
              key={conversation.id}
              aria-pressed={isSelected}
              disabled={uploading}
              className={`w-full min-w-0 cursor-pointer border-b border-border/80 px-3 py-2 text-left transition-colors hover:bg-muted/40 focus-visible:outline-2 focus-visible:outline-ring focus-visible:-outline-offset-2 ${
                isSelected ? "bg-accent/70" : ""
              }`}
              onClick={() => {
                void selectConversation(conversation.id).then(
                  () => {
                    onAfterSelect?.()
                  },
                  (error) => toast.error(error instanceof Error ? error.message : t("conversation.syncMessagesFailed")),
                )
              }}
            >
              <div className="overflow-hidden">
                <div className="flex items-center gap-2">
                  <Avatar className="size-7 shrink-0">
              <AvatarImage src={conversation.customerAvatar} alt={conversation.customerName} />
                    <AvatarFallback className="bg-primary/10 text-primary">
                      <UserIcon className="size-3.5 text-primary" />
                    </AvatarFallback>
                  </Avatar>
                  <div className="min-w-0 flex-1">
                    <div className="flex items-center gap-1.5">
                      <span className="min-w-0 flex-1 truncate font-medium text-sm leading-4">
                        {conversation.customerName ||
                          t("conversation.customerFallback", {
                            id: conversation.customerId || conversation.id,
                          })}
                      </span>
                      {conversation.agentUnreadCount > 0 ? (
                        <div className="flex size-4.5 shrink-0 items-center justify-center rounded-full bg-primary text-[10px] text-primary-foreground">
                          {conversation.agentUnreadCount > 99
                            ? "99+"
                            : conversation.agentUnreadCount}
                        </div>
                      ) : null}
                    </div>
                    <div className="mt-0.5 text-[11px] text-muted-foreground">
                      {conversation.lastMessageAt
                        ? formatDateTime(conversation.lastMessageAt)
                        : t("conversation.noTime")}
                    </div>
                  </div>
                </div>
                <div className="mt-0.5 truncate text-xs leading-4 text-muted-foreground">
                  {conversation.lastMessageSummary || t("conversation.noLatestMessage")}
                </div>
                <div className="mt-1 flex min-w-0 flex-wrap items-center gap-1 text-xs text-muted-foreground">
                  <span className="min-w-0 truncate" title={inboxSourceLabel(channels.find((item) => item.id === conversation.channelId), conversation.channelId)}>{inboxSourceLabel(channels.find((item) => item.id === conversation.channelId), conversation.channelId)}</span>
                  <Badge variant="secondary">{t(conversation.status === IMConversationStatus.AIServing ? "conversation.filterAiServing" : conversation.status === IMConversationStatus.Pending ? "conversation.filterPending" : conversation.status === IMConversationStatus.Closed ? "conversation.filterClosed" : "conversation.filterActive")}</Badge>
                </div>
                {conversation.status === IMConversationStatus.Active ? <p className="mt-1 truncate text-xs text-muted-foreground">{conversation.currentAssigneeName || t("conversation.inbox.agent", { id: conversation.currentAssigneeId })}</p> : null}
                {conversation.status === IMConversationStatus.Active && <p className="mt-1 text-xs text-muted-foreground">{t(isReplyOverdue(conversation, now) ? "reception.overdue" : workStatusKey(conversation.workStatus))}</p>}
                {conversation.status === IMConversationStatus.Pending &&
                conversation.currentTeamName ? (
                  <div className="mt-1 flex items-center gap-1 text-[10px] text-muted-foreground">
                    <span className="rounded-md bg-muted px-1.5 py-0.5">
                      {t("conversation.teamOnDuty", {
                        name: conversation.currentTeamName,
                      })}
                    </span>
                  </div>
                ) : null}
              </div>
            </button>
          )
        })
      ) : (
        <div className="p-6 text-center text-sm text-muted-foreground">
          {t(filtered ? "conversation.inbox.noMatches" : "conversation.empty")}
        </div>
      )}
      {conversations.length > 0 && conversations.length < total ? <div className="p-2"><Button variant="outline" className="w-full" disabled={loading} onClick={() => void load(true).catch(() => {})}>{loading ? t("conversation.loading") : t("conversation.inbox.loadMore")}</Button></div> : null}
    </ScrollArea>
  )
}
