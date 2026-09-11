"use client"

import { useEffect, useState } from "react"
import { toast } from "sonner"

import {
  SharedMessageEditor,
  type UploadedMessageEditorImage,
} from "@/components/chat/shared-message-editor"
import { useI18n } from "@/i18n/provider"
import { fetchQuickReplyListAll, type AdminQuickReply } from "@/lib/api/admin"
import { useAgentConversationsStore } from "@/lib/stores/agent-conversations"
import { useTranslatedReply } from "./translated-reply"

type AgentMessageEditorProps = {
  conversationId: number
  textOnly?: boolean
  disabled?: boolean
  uploadingAsset?: boolean
  onSend: (html: string) => Promise<void>
  onUploadImage: (file: File) => Promise<UploadedMessageEditorImage | null>
  onSendAttachment: (file: File) => Promise<void>
}

export function AgentMessageEditor({
  conversationId,
  textOnly = false,
  disabled = false,
  uploadingAsset = false,
  onSend,
  onUploadImage,
  onSendAttachment,
}: AgentMessageEditorProps) {
  const t = useI18n()
  const translation = useTranslatedReply(conversationId, onSend, disabled)
  const insertion = useAgentConversationsStore((state) => state.draftInsertion)
  const consumeInsertion = useAgentConversationsStore((state) => state.consumeDraftInsertion)
  const [quickReplies, setQuickReplies] = useState<AdminQuickReply[]>([])
  const [loadingQuickReplies, setLoadingQuickReplies] = useState(true)
  const [quickReplyPickerOpen, setQuickReplyPickerOpen] = useState(false)

  useEffect(() => {
    let cancelled = false
    void fetchQuickReplyListAll()
      .then((list) => {
        if (!cancelled) {
          setQuickReplies(list)
        }
      })
      .catch((error) => {
        if (!cancelled) {
          toast.error(error instanceof Error ? error.message : t("conversation.loadQuickRepliesFailed"))
        }
      })
      .finally(() => {
        if (!cancelled) {
          setLoadingQuickReplies(false)
        }
      })
    return () => {
      cancelled = true
    }
  }, [t])

  return (
    <>
    {translation.controls}
    <SharedMessageEditor
      variant="agent"
      textOnly={textOnly}
      allowAttachments={textOnly}
      initialHTML={useAgentConversationsStore.getState().drafts[conversationId] ?? ""}
      insertText={insertion?.conversationId === conversationId ? insertion : undefined}
      onTextInserted={consumeInsertion}
      onHTMLChange={(html) => useAgentConversationsStore.getState().setDraft(conversationId, html)}
      disabled={disabled || translation.previewing}
      uploadingAsset={uploadingAsset}
      quickReplies={{
        open: quickReplyPickerOpen,
        loading: loadingQuickReplies,
        items: quickReplies,
        onOpenChange: setQuickReplyPickerOpen,
      }}
      onSend={translation.send}
      sendLabel={translation.beforeSend ? t("translation.previewAction") : undefined}
      onUploadImage={onUploadImage}
      onSendAttachment={onSendAttachment}
    />
    {translation.dialog}
    </>
  )
}
