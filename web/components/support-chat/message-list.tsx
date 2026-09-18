"use client"

import {
  forwardRef,
  memo,
  useCallback,
  useEffect,
  useImperativeHandle,
  useRef,
} from "react"

import { ConversationMessageBubble } from "@/components/chat/conversation-message-bubble"
import { ConversationMessageRow } from "@/components/chat/conversation-message-row"
import {
  ConversationMessageScroller,
  ConversationMessageScrollerItem,
  type ConversationMessageScrollerHandle,
} from "@/components/chat/conversation-message-scroller"
import { ImMessageHTML } from "@/components/im-message-html"
import { useImageLightbox } from "@/components/image-lightbox"
import { useMessageRecallWindow } from "@/hooks/use-message-recall-window"
import { Avatar, AvatarFallback, AvatarImage } from "@/components/ui/avatar"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import type { ImMessage } from "@/lib/api/im"
import type { StreamingReply } from "@/lib/stores/support-chat"
import { renderIMMessageHTML } from "@/lib/im-message"
import { cn, formatDateTime } from "@/lib/utils"
import { useI18n } from "@/i18n/provider"

type SupportChatMessageListProps = {
  messages?: ImMessage[] | null
  onNearBottomVisible?: () => void
  hasMoreOlder?: boolean
  loadingOlder?: boolean
  onLoadOlder?: () => Promise<void>
  streamingReply?: StreamingReply | null
  assistantName?: string
  assistantAvatar?: string
  recallingMessageId?: number
  onRecall?: (messageId: number) => Promise<ImMessage | null>
}

export type SupportChatMessageListHandle = {
  scrollToBottom: () => void
}

function getDayKey(value?: string) {
  if (!value) {
    return "unknown"
  }
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) {
    return value.slice(0, 10)
  }
  return `${date.getFullYear()}-${String(date.getMonth() + 1).padStart(2, "0")}-${String(
    date.getDate()
  ).padStart(2, "0")}`
}

function getTimelineLabel(
  value: string | undefined,
  t: (key: string, values?: Record<string, string | number>) => string
) {
  if (!value) {
    return t("supportChat.justNow")
  }
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) {
    return value
  }
  const currentDayKey = getDayKey(value)
  const todayDayKey = getDayKey(new Date().toISOString())
  const timeText = `${String(date.getHours()).padStart(2, "0")}:${String(
    date.getMinutes()
  ).padStart(2, "0")}`
  if (currentDayKey === todayDayKey) {
    return t("supportChat.todayAt", { time: timeText })
  }
  return `${currentDayKey} ${timeText}`
}

export const SupportChatMessageList = forwardRef<SupportChatMessageListHandle, SupportChatMessageListProps>(
  function SupportChatMessageList(
    {
      messages,
      onNearBottomVisible,
      hasMoreOlder = false,
      loadingOlder = false,
      onLoadOlder,
      streamingReply,
      assistantName,
      assistantAvatar,
      recallingMessageId = 0,
      onRecall,
    },
    ref
  ) {
    const t = useI18n()
    const scrollerRef = useRef<ConversationMessageScrollerHandle | null>(null)
    const shouldStickToBottomRef = useRef(true)
    const onNearBottomVisibleRef = useRef(onNearBottomVisible)
    const safeMessages = Array.isArray(messages) ? messages : []

    useEffect(() => {
      onNearBottomVisibleRef.current = onNearBottomVisible
    }, [onNearBottomVisible])

    const scrollToBottom = useCallback(() => {
      scrollerRef.current?.scrollToBottom()
    }, [])

    useEffect(() => {
      if (streamingReply && shouldStickToBottomRef.current) {
        scrollToBottom()
      }
    }, [scrollToBottom, streamingReply])

    const handleImageSettled = useCallback(() => {
      if (shouldStickToBottomRef.current) {
        scrollToBottom()
        onNearBottomVisibleRef.current?.()
      }
    }, [scrollToBottom])

    useImperativeHandle(ref, () => ({
      scrollToBottom,
    }))

    const handleLoadOlder = useCallback(async () => {
      if (!onLoadOlder || loadingOlder || !hasMoreOlder) {
        return
      }
      await onLoadOlder()
    }, [hasMoreOlder, loadingOlder, onLoadOlder])

    return (
      <ConversationMessageScroller
        ref={scrollerRef}
        className="flex min-h-0 flex-1"
        viewportClassName="bg-transparent"
        contentClassName="gap-4 px-4 py-4"
        hasMoreOlder={hasMoreOlder}
        loadingOlder={loadingOlder}
        onLoadOlder={onLoadOlder}
        onNearBottomChange={(nearBottom) => {
          shouldStickToBottomRef.current = nearBottom
        }}
        onNearBottomVisible={onNearBottomVisible}
        topSlot={
          hasMoreOlder && onLoadOlder ? (
            <div className="flex justify-center py-1">
              <Button
                type="button"
                variant="outline"
                size="sm"
                disabled={loadingOlder}
                onClick={() => void handleLoadOlder()}
                className="h-7 rounded-full bg-background/90 text-xs text-muted-foreground shadow-sm hover:bg-background hover:text-sky-700 dark:hover:text-sky-400"
              >
                {loadingOlder ? t("supportChat.loadingOlder") : t("supportChat.loadOlder")}
              </Button>
            </div>
          ) : null
        }
      >
        {safeMessages.length === 0 && !streamingReply ? (
          <div className="flex min-h-32 items-center justify-center px-3 py-6 text-center text-sm leading-6 text-muted-foreground">
            {t("supportChat.emptyPrompt")}
          </div>
        ) : null}

        {safeMessages.map((message, index) => {
          const previousMessage = index > 0 ? safeMessages[index - 1] : null
          const showTimeline =
            index === 0 ||
            getDayKey(previousMessage?.sentAt) !== getDayKey(message.sentAt)

          return (
            <ConversationMessageScrollerItem
              key={message.id}
              messageId={`${message.id}`}
            >
              <MessageItem
                message={message}
                showTimeline={showTimeline}
                onImageSettled={handleImageSettled}
                timelineLabel={getTimelineLabel(message.sentAt, t)}
                recalling={recallingMessageId === message.id}
                onRecall={onRecall}
              />
            </ConversationMessageScrollerItem>
          )
        })}

        {streamingReply ? (
          <ConversationMessageScrollerItem messageId={`stream-${streamingReply.requestId}`}>
            <StreamingMessageItem
              content={streamingReply.content}
              assistantName={assistantName}
              assistantAvatar={assistantAvatar}
            />
          </ConversationMessageScrollerItem>
        ) : null}
      </ConversationMessageScroller>
    )
  }
)

function StreamingMessageItem({
  content,
  assistantName,
  assistantAvatar,
}: {
  content: string
  assistantName?: string
  assistantAvatar?: string
}) {
  const t = useI18n()
  const senderName = assistantName?.trim() || t("supportChat.agentLabel")
  const htmlContent = renderIMMessageHTML({ messageType: "text", content })

  return (
    <ConversationMessageRow
      align="start"
      avatar={
        <Avatar>
          {assistantAvatar?.trim() ? <AvatarImage src={assistantAvatar.trim()} alt="" /> : null}
          <AvatarFallback className="bg-muted text-muted-foreground">
            {senderName.slice(0, 1).toUpperCase()}
          </AvatarFallback>
        </Avatar>
      }
      contentClassName="max-w-[86%] gap-1.5"
      headerClassName="flex-wrap gap-x-2 gap-y-1 px-1 text-[11px]"
      header={
        <>
          <span className="font-medium">{senderName}</span>
          <span>{t("supportChat.justNow")}</span>
        </>
      }
    >
      <ConversationMessageBubble
        variant="system"
        className="rounded-lg border-0 !border-border !bg-card px-3 py-2 text-sm leading-normal !text-card-foreground shadow-[0_10px_22px_rgba(15,23,42,0.06)] dark:!bg-background"
      >
        {content ? (
          <ImMessageHTML
            html={htmlContent}
            className="[&_a]:text-card-foreground [&_a]:underline"
          />
        ) : (
          <span className="flex h-5 items-center gap-1" aria-hidden="true">
            <span className="size-1.5 animate-pulse rounded-full bg-muted-foreground/70" />
            <span className="size-1.5 animate-pulse rounded-full bg-muted-foreground/70 [animation-delay:150ms]" />
            <span className="size-1.5 animate-pulse rounded-full bg-muted-foreground/70 [animation-delay:300ms]" />
          </span>
        )}
      </ConversationMessageBubble>
    </ConversationMessageRow>
  )
}

type MessageItemProps = {
  message: ImMessage
  showTimeline: boolean
  onImageSettled: () => void
  timelineLabel: string
  recalling: boolean
  onRecall?: (messageId: number) => Promise<ImMessage | null>
}

const MessageItem = memo(
  function MessageItem({ message, showTimeline, onImageSettled, timelineLabel, recalling, onRecall }: MessageItemProps) {
    const t = useI18n()
    const { open } = useImageLightbox()
    const isCustomer = message.senderType === "customer"
    const isRecalled = Boolean(message.recalledAt) || message.sendStatus === 6
    const recallWindowOpen = useMessageRecallWindow(
      isCustomer && !isRecalled ? message.recallableUntil : undefined
    )
    const senderName = isCustomer ? t("supportChat.customerSelf") : message.senderName?.trim() || t("supportChat.agentLabel")
    const avatarSrc =
      !isCustomer && message.senderAvatar?.trim() ? message.senderAvatar.trim() : undefined
    const htmlContent = isRecalled
      ? `<p>${t("supportChat.messageRecalledBody")}</p>`
      : renderIMMessageHTML(message)
    const fallbackName = senderName.slice(0, 1).toUpperCase()

    return (
      <div>
        {showTimeline ? (
          <div className="mb-3 flex items-center justify-center">
            <Badge
              variant="outline"
              className="border-border bg-background/85 text-[11px] font-medium text-muted-foreground shadow-sm"
            >
              {timelineLabel}
            </Badge>
          </div>
        ) : null}

        <ConversationMessageRow
          align={isCustomer ? "end" : "start"}
          avatar={
            !isCustomer ? (
              <Avatar>
                {avatarSrc ? <AvatarImage src={avatarSrc} alt="" /> : null}
                <AvatarFallback className="bg-muted text-muted-foreground">
                  {fallbackName || t("supportChat.customerFallback")}
                </AvatarFallback>
              </Avatar>
            ) : null
          }
          contentClassName="max-w-[86%] gap-1.5"
          headerClassName="flex-wrap gap-x-2 gap-y-1 px-1 text-[11px]"
          header={
            <>
              <span className="font-medium">{senderName}</span>
              <span>{formatDateTime(message.sentAt)}</span>
              {isCustomer ? (
                <span>{message.agentRead ? t("supportChat.agentRead") : t("supportChat.agentUnread")}</span>
              ) : null}
              {isRecalled ? <span>{t("supportChat.messageRecalled")}</span> : null}
              {isCustomer && !isRecalled && recallWindowOpen && onRecall ? (
                <Button
                  type="button"
                  variant="ghost"
                  size="sm"
                  className="h-auto px-1 py-0 text-[11px] text-muted-foreground shadow-none"
                  disabled={recalling}
                  onClick={() => void onRecall(message.id)}
                >
                  {recalling ? t("supportChat.recalling") : t("supportChat.recall")}
                </Button>
              ) : null}
            </>
          }
        >
          <ConversationMessageBubble
            variant={isRecalled ? "recalled" : isCustomer ? "customer" : "system"}
            className={cn(
              "rounded-lg border-0 px-3 py-2 text-sm leading-normal shadow-[0_10px_22px_rgba(15,23,42,0.06)]",
              isRecalled
                ? "border border-dashed border-border bg-muted/50 text-muted-foreground shadow-none"
                : isCustomer
                ? "!bg-[#a9ea7a] !text-[#161616] dark:!bg-emerald-500 dark:!text-emerald-950"
                : "!border-border !bg-card !text-card-foreground dark:!bg-background"
            )}
          >
            <ImMessageHTML
              html={htmlContent}
              className={cn(
                isRecalled
                  ? "[&_p]:text-muted-foreground"
                  : isCustomer
                  ? "[&_p]:text-[#161616] dark:[&_p]:text-emerald-950 [&_a]:text-[#161616] dark:[&_a]:text-emerald-950 [&_a]:underline [&_img]:cursor-zoom-in"
                  : "[&_a]:text-card-foreground [&_a]:underline [&_img]:cursor-zoom-in"
              )}
              onImageSettled={onImageSettled}
              onImageClick={open}
            />
          </ConversationMessageBubble>
        </ConversationMessageRow>
      </div>
    )
  },
  (prevProps, nextProps) =>
    isSameMessageItemRender(prevProps.message, nextProps.message) &&
    prevProps.showTimeline === nextProps.showTimeline &&
    prevProps.timelineLabel === nextProps.timelineLabel &&
    prevProps.recalling === nextProps.recalling &&
    prevProps.onRecall === nextProps.onRecall &&
    prevProps.onImageSettled === nextProps.onImageSettled
)

function isSameMessageItemRender(prev: ImMessage, next: ImMessage) {
  return (
    prev.id === next.id &&
    prev.senderType === next.senderType &&
    prev.senderName === next.senderName &&
    prev.senderAvatar === next.senderAvatar &&
    prev.messageType === next.messageType &&
    prev.content === next.content &&
    prev.payload === next.payload &&
    prev.sentAt === next.sentAt &&
    prev.sendStatus === next.sendStatus &&
    prev.recalledAt === next.recalledAt &&
    prev.recallableUntil === next.recallableUntil &&
    prev.agentRead === next.agentRead
  )
}
