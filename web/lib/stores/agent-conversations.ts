"use client"

import { create } from "zustand"

import {
  fetchAgentConversations,
  fetchAgentConversationDetail,
  fetchInboxChannels,
  fetchAgentMessages,
  markAgentMessageRead,
  recallAgentMessage,
  sendAgentMessage,
  uploadAgentConversationAttachment,
  uploadAgentConversationImage,
  type AgentAsset,
  type AgentConversation,
  type AgentMessage,
  type InboxChannel,
} from "@/lib/api/agent"
import type { RealtimeConnectionStatusValue } from "@/components/realtime-connection-status"
import {
  cursorFromLoadedImMessages,
  hasMoreAfterLatestImMessageMerge,
  mergeImMessagesByIdAsc,
  parseImMessageCursorId,
} from "@/lib/im-message-merge"
import {
  markMessagesReadToMessageId,
  patchConversationList,
  patchConversationListWithMessage,
  type RealtimeConversationPatch,
} from "@/lib/im-realtime-state"
import { summarizeIMMessage } from "@/lib/im-message"
import { generateUUID } from "@/lib/utils"
import { translateCurrentMessage } from "@/i18n/messages"

export const agentConversationFilterOptions = [
  { value: "all", labelKey: "conversation.inbox.all" },
  { value: "unread", labelKey: "conversation.inbox.unread" },
  { value: "mine", labelKey: "conversation.inbox.mine" },
  { value: "needs_reply", labelKey: "reception.filterNeedsReply" },
  { value: "overdue", labelKey: "reception.filterOverdue" },
  { value: "waiting_customer", labelKey: "reception.filterWaiting" },
  { value: "snoozed", labelKey: "reception.filterSnoozed" },
  { value: "pending", labelKey: "conversation.filterPending" },
  { value: "ai_serving", labelKey: "conversation.filterAiServing" },
  { value: "closed", labelKey: "conversation.filterClosed" },
] as const

export type AgentConversationFilterKey =
  (typeof agentConversationFilterOptions)[number]["value"]

export function buildConversationQuery(filter: AgentConversationFilterKey, keyword: string, channelType = "", channelId = "", page = 1) {
  const query: Record<string, string | number | undefined> = {
    filter,
    keyword: keyword.trim() || undefined,
    channelType: channelType || undefined,
    channelId: channelId || undefined,
    page,
    limit: 50,
  }

  return query
}

type LoadMessagesOptions = {
  forceLoading?: boolean
  reset?: boolean
}

function ensureArray<T>(value: T[] | null | undefined): T[] {
  return Array.isArray(value) ? value : []
}

type AgentConversationsStore = {
  channels: InboxChannel[]
  channelsError: boolean
  channelType: string
  channelId: string
  conversationsError: string
  conversationsPage: number
  conversationsTotal: number
  selectedConversationData: AgentConversation | null
  drafts: Record<number, string>
  draftInsertion: { id: string; conversationId: number; text: string } | null
  privateNoteDrafts: Record<number, { content: string; mentionIds: string[]; clientId: string }>
  setPrivateNoteDraft: (id: number, draft: { content: string; mentionIds: string[]; clientId: string }) => void
  insertSuggestion: (conversationId: number, lastMessageId: number, text: string) => boolean
  consumeDraftInsertion: (id: string) => void
  setDraft: (conversationId: number, html: string) => void
  setChannelFilter: (channelType: string, channelId?: string) => void
  loadChannels: () => Promise<void>
  searchKeyword: string
  conversationFilter: AgentConversationFilterKey
  conversations: AgentConversation[]
  conversationsLoading: boolean
  conversationsLoaded: boolean
  selectedConversationId: number | null
  messages: AgentMessage[]
  messagesLoading: boolean
  messagesLoadingMore: boolean
  messagesCursor: string
  messagesHasMore: boolean
  messagesLoadedConversationId: number | null
  sending: boolean
  uploadingAsset: boolean
  recallingMessageId: number
  readingMessageId: number
  realtimeStatus: RealtimeConnectionStatusValue
  setSearchKeyword: (keyword: string) => void
  setConversationFilter: (filter: AgentConversationFilterKey) => void
  setRealtimeStatus: (status: RealtimeConnectionStatusValue) => void
  setConversationTags: (
    conversationId: number,
    tags: AgentConversation["tags"]
  ) => void
  loadConversations: (more?: boolean) => Promise<void>
  selectConversation: (conversationId: number) => Promise<void>
  loadMessages: (conversationId: number, options?: LoadMessagesOptions) => Promise<void>
  loadOlderMessages: () => Promise<void>
  syncLatestMessages: (conversationId: number) => Promise<void>
  markSelectedConversationRead: () => Promise<void>
  sendMessage: (html: string) => Promise<AgentMessage | null>
  uploadImage: (file: File) => Promise<AgentAsset | null>
  sendAttachment: (file: File) => Promise<AgentMessage | null>
  recallMessage: (messageId: number) => Promise<AgentMessage | null>
  applyRealtimeMessageCreated: (message: AgentMessage) => void
  applyRealtimeConversationChanged: (patch: RealtimeConversationPatch) => void
  applyRealtimeMessageRecalled: (messageId: number, patch: Partial<AgentMessage>) => void
  resyncRealtimeData: (conversationId?: number) => Promise<void>
}

let conversationsRequestSeq = 0
let messagesRequestSeq = 0

export const useAgentConversationsStore = create<AgentConversationsStore>((set, get) => ({
  channels: [],
  channelsError: false,
  channelType: "",
  channelId: "",
  conversationsError: "",
  conversationsPage: 1,
  conversationsTotal: 0,
  selectedConversationData: null,
  drafts: {},
  draftInsertion: null,
  privateNoteDrafts: {},
  setPrivateNoteDraft: (id, draft) => set((state) => ({ privateNoteDrafts: { ...state.privateNoteDrafts, [id]: draft } })),
  insertSuggestion: (conversationId, lastMessageId, text) => {
    const state = get()
    const conversation = state.conversations.find((item) => item.id === conversationId) ?? state.selectedConversationData
    if (state.selectedConversationId !== conversationId || conversation?.id !== conversationId || conversation.lastMessageId !== lastMessageId || conversation.status !== 3 || state.sending || state.draftInsertion) return false
    set({ draftInsertion: { id: generateUUID(), conversationId, text } })
    return true
  },
  consumeDraftInsertion: (id) => { if (get().draftInsertion?.id === id) set({ draftInsertion: null }) },
  setDraft: (id, html) => set((state) => ({ drafts: { ...state.drafts, [id]: html } })),
  setChannelFilter: (channelType, channelId = "") => {
    if (get().channelType === channelType && get().channelId === channelId) return
    ++conversationsRequestSeq
    set({ channelType, channelId, conversationsPage: 1, conversations: [], conversationsLoaded: false, selectedConversationId: null, selectedConversationData: null, messages: [], messagesLoadedConversationId: null, draftInsertion: null })
  },
  loadChannels: async () => {
    try { set({ channels: ensureArray(await fetchInboxChannels()), channelsError: false }) }
    catch { set({ channelsError: true }) }
  },
  searchKeyword: "",
  conversationFilter: "all",
  conversations: [],
  conversationsLoading: false,
  conversationsLoaded: false,
  selectedConversationId: null,
  messages: [],
  messagesLoading: false,
  messagesLoadingMore: false,
  messagesCursor: "",
  messagesHasMore: false,
  messagesLoadedConversationId: null,
  sending: false,
  uploadingAsset: false,
  recallingMessageId: 0,
  readingMessageId: 0,
  realtimeStatus: "connecting",

  setSearchKeyword: (keyword) => {
    if (get().searchKeyword === keyword) return
    ++conversationsRequestSeq
    set({ searchKeyword: keyword, conversationsPage: 1, conversations: [], conversationsLoaded: false, selectedConversationId: null, selectedConversationData: null, messages: [], messagesLoadedConversationId: null })
  },

  setConversationFilter: (filter) => {
    if (get().conversationFilter === filter) return
    ++conversationsRequestSeq
    set({ conversationFilter: filter, conversationsPage: 1, conversations: [], conversationsLoaded: false, selectedConversationId: null, selectedConversationData: null, messages: [], messagesLoadedConversationId: null })
  },

  setRealtimeStatus: (status) => {
    set({ realtimeStatus: status })
  },

  setConversationTags: (conversationId, tags) => {
    set((state) => ({
      conversations: state.conversations.map((item) =>
        item.id === conversationId
          ? {
              ...item,
              tags: tags && tags.length > 0 ? tags : [],
            }
          : item
      ),
    }))
  },

  loadConversations: async (more = false) => {
    const requestSeq = ++conversationsRequestSeq
    const store = get()

    const pageCount = store.conversationsPage + (more ? 1 : 0)
    set({ conversationsLoading: true, conversationsError: "" })

    try {
      // Refresh the loaded window so new messages cannot shift offset pages into duplicates.
      const pages = await Promise.all(Array.from({ length: pageCount }, (_, index) => fetchAgentConversations(
        buildConversationQuery(store.conversationFilter, store.searchKeyword, store.channelType, store.channelId, index + 1)
      )))
      const conversations = [...new Map(pages.flatMap((data) => ensureArray(data.results)).map((item) => [item.id, item])).values()]

      if (requestSeq !== conversationsRequestSeq) {
        return
      }

      const currentSelectedId = get().selectedConversationId
      set({
        conversations,
        conversationsPage: pageCount,
        conversationsTotal: pages[0]?.page?.total ?? conversations.length,
        conversationsLoaded: true,
        conversationsLoading: false,
      })
      if (currentSelectedId) {
        const selected = conversations.find((item) => item.id === currentSelectedId) ?? await fetchAgentConversationDetail(currentSelectedId)
        if (requestSeq === conversationsRequestSeq && get().selectedConversationId === currentSelectedId) {
          set({ selectedConversationData: selected })
        }
      }
    } catch (error) {
      if (requestSeq === conversationsRequestSeq) {
        set({ conversationsLoading: false, conversationsError: error instanceof Error ? error.message : translateCurrentMessage("conversation.loadListFailed") })
      }
      throw error
    }
  },

  selectConversation: async (conversationId) => {
    if (get().selectedConversationId === conversationId && get().messagesLoadedConversationId === conversationId) {
      return
    }

    set({
      selectedConversationId: conversationId,
      draftInsertion: null,
      selectedConversationData: get().conversations.find((item) => item.id === conversationId) ?? null,
      messages: [],
      messagesLoading: true,
      messagesLoadingMore: false,
      messagesCursor: "",
      messagesHasMore: false,
      messagesLoadedConversationId: null,
    })

    await get().loadMessages(conversationId, {
      forceLoading: true,
      reset: true,
    })
    if (!get().selectedConversationData) {
      const detail = await fetchAgentConversationDetail(conversationId)
      if (get().selectedConversationId === conversationId) set({ selectedConversationData: detail })
    }
  },

  loadMessages: async (conversationId, options = {}) => {
    const requestSeq = ++messagesRequestSeq
    const store = get()
    const shouldShowLoading =
      options.forceLoading || store.messagesLoadedConversationId !== conversationId

    if (shouldShowLoading) {
      set({
        messagesLoading: true,
        ...(options.reset
          ? {
              messages: [],
              messagesCursor: "",
              messagesHasMore: false,
            }
          : {}),
      })
    }

    try {
      const data = await fetchAgentMessages({
        conversationId,
        limit: 50,
      })

      if (requestSeq !== messagesRequestSeq) {
        return
      }

      if (get().selectedConversationId !== conversationId) {
        return
      }

      const list = ensureArray(data.results)
      set({
        messages: list,
        messagesLoading: false,
        messagesLoadedConversationId: conversationId,
        messagesCursor:
          cursorFromLoadedImMessages(list) || (data.cursor ?? ""),
        messagesHasMore: Boolean(data.hasMore),
      })
    } catch (error) {
      if (requestSeq === messagesRequestSeq) {
        set({ messagesLoading: false })
      }
      throw error
    }
  },

  loadOlderMessages: async () => {
    const conversationId = get().selectedConversationId
    if (!conversationId || get().messagesLoadingMore || !get().messagesHasMore) {
      return
    }
    const cursorId = parseImMessageCursorId(get().messagesCursor)
    if (cursorId <= 0) {
      return
    }

    set({ messagesLoadingMore: true })
    try {
      const data = await fetchAgentMessages({
        conversationId,
        cursor: cursorId,
        limit: 50,
      })
      if (get().selectedConversationId !== conversationId) {
        return
      }
      const incoming = ensureArray(data.results)
      set((state) => {
        const merged = mergeImMessagesByIdAsc(state.messages, incoming)
        return {
          messages: merged,
          messagesCursor:
            cursorFromLoadedImMessages(merged) ||
            (data.cursor ?? state.messagesCursor),
          messagesHasMore: Boolean(data.hasMore),
          messagesLoadingMore: false,
        }
      })
    } catch (error) {
      set({ messagesLoadingMore: false })
      throw error
    }
  },

  syncLatestMessages: async (conversationId) => {
    if (conversationId <= 0) {
      return
    }
    try {
      const data = await fetchAgentMessages({
        conversationId,
        limit: 50,
      })
      if (get().selectedConversationId !== conversationId) {
        return
      }
      const batch = ensureArray(data.results)
      if (batch.length === 0) {
        return
      }
      set((state) => {
        const merged = mergeImMessagesByIdAsc(state.messages, batch)
        return {
          messages: merged,
          messagesCursor:
            cursorFromLoadedImMessages(merged) ||
            (data.cursor ?? state.messagesCursor),
          messagesHasMore: hasMoreAfterLatestImMessageMerge({
            previousMessages: state.messages,
            previousHasMore: state.messagesHasMore,
            merged,
            apiHasMore: Boolean(data.hasMore),
          }),
        }
      })
    } catch {
      // Keep realtime callback errors contained in the store.
    }
  },

  markSelectedConversationRead: async () => {
    const store = get()
    const conversationId = store.selectedConversationId
    const conversation = agentConversationSelectors.selectedConversation(store)
    const lastMessage = store.messages.at(-1)
    if (!conversationId || !conversation || !lastMessage) {
      return
    }
    if (
      conversation.agentUnreadCount <= 0 &&
      (conversation.agentLastReadMessageId ?? 0) >= lastMessage.id
    ) {
      return
    }
    if (store.readingMessageId === lastMessage.id) {
      return
    }

    set({ readingMessageId: lastMessage.id })
    try {
      await markAgentMessageRead(conversationId, lastMessage.id)
      set((current) => {
        if (current.selectedConversationId !== conversationId) {
          return { readingMessageId: 0 }
        }
        return {
          readingMessageId: 0,
          selectedConversationData: current.selectedConversationData ? { ...current.selectedConversationData, agentUnreadCount: 0, agentLastReadMessageId: lastMessage.id } : null,
          messages: current.messages.map((item) => {
            if (item.id > lastMessage.id) {
              return item
            }
            return item.agentRead ? item : { ...item, agentRead: true }
          }),
          conversations: current.conversations.map((item) =>
            item.id === conversationId
              ? {
                  ...item,
                  agentUnreadCount: 0,
                  agentLastReadMessageId: lastMessage.id,
                }
              : item
          ),
        }
      })
    } catch (error) {
      set({ readingMessageId: 0 })
      throw error
    }
  },

  applyRealtimeMessageCreated: (message) => {
    set((state) => {
      const isSelected = state.selectedConversationId === message.conversationId
      const nextMessages = isSelected
        ? mergeImMessagesByIdAsc(state.messages, [message])
        : state.messages
      return {
        messages: nextMessages,
        selectedConversationData: state.selectedConversationData ? patchConversationListWithMessage([state.selectedConversationData], message)[0] : null,
        conversations: patchConversationListWithMessage(
          state.conversations,
          message
        ),
      }
    })
  },

  applyRealtimeConversationChanged: (patch) => {
    set((state) => {
      const conversationId = patch.id ?? patch.conversationId ?? 0
      let nextMessages = state.messages
      if (
        conversationId > 0 &&
        state.selectedConversationId === conversationId
      ) {
        if ((patch.agentLastReadMessageId ?? 0) > 0) {
          nextMessages = markMessagesReadToMessageId(
            nextMessages,
            patch.agentLastReadMessageId ?? 0,
            "agent",
            patch.agentLastReadAt
          )
        }
        if ((patch.customerLastReadMessageId ?? 0) > 0) {
          nextMessages = markMessagesReadToMessageId(
            nextMessages,
            patch.customerLastReadMessageId ?? 0,
            "customer",
            patch.customerLastReadAt
          )
        }
      }
      return {
        messages: nextMessages,
        selectedConversationData: state.selectedConversationData ? patchConversationList([state.selectedConversationData], patch)[0] : null,
        conversations: patchConversationList(state.conversations, patch),
      }
    })
  },

  applyRealtimeMessageRecalled: (messageId, patch) => {
    if (messageId <= 0) {
      return
    }
    set((state) => ({
      messages: state.messages.map((item) =>
        item.id === messageId ? { ...item, ...patch, id: item.id } : item
      ),
    }))
  },

  resyncRealtimeData: async (conversationId) => {
    await get().loadConversations()
    const selectedConversationId = get().selectedConversationId
    const targetConversationId = conversationId ?? selectedConversationId
    if (targetConversationId && selectedConversationId === targetConversationId) {
      await get().syncLatestMessages(targetConversationId)
    }
  },

  sendMessage: async (html) => {
    const trimmedContent = html.trim()
    const { selectedConversationId, sending } = get()
    if (!selectedConversationId || !trimmedContent || sending) {
      return null
    }

    set({ sending: true })
    try {
      const message = await sendAgentMessage({
        conversationId: selectedConversationId,
        messageType: "html",
        content: trimmedContent,
        clientMsgId: `agent_${generateUUID()}`,
      })

      if (get().selectedConversationId === selectedConversationId) {
        set((current) => ({
          messages: current.messages.some((m) => m.id === message.id)
            ? current.messages.map((m) => (m.id === message.id ? message : m))
            : [...current.messages, message],
          conversations: patchConversationList(
            patchConversationListWithMessage(current.conversations, message),
            {
              conversationId: selectedConversationId,
              agentUnreadCount: 0,
              customerUnreadCount:
                (current.conversations.find((item) => item.id === selectedConversationId)
                  ?.customerUnreadCount ?? 0) + 1,
              agentLastReadMessageId: message.id,
            }
          ),
        }))
      }

      return message
    } finally {
      set({ sending: false })
    }
  },

  uploadImage: async (file) => {
    const { selectedConversationId, sending, uploadingAsset } = get()
    if (!selectedConversationId || sending || uploadingAsset) {
      return null
    }

    set({ uploadingAsset: true })
    try {
      return await uploadAgentConversationImage(selectedConversationId, file)
    } finally {
      set({ uploadingAsset: false })
    }
  },

  sendAttachment: async (file) => {
    const { selectedConversationId, sending, uploadingAsset } = get()
    if (!selectedConversationId || sending || uploadingAsset) {
      return null
    }

    set({ uploadingAsset: true })
    try {
      const asset = await uploadAgentConversationAttachment(selectedConversationId, file)
      const message = await sendAgentMessage({
        conversationId: selectedConversationId,
        messageType: /^(image\/jpeg|image\/png)$/.test(asset.mimeType) ? "image" : "attachment",
        content: asset.filename,
        payload: JSON.stringify({ assetId: asset.assetId }),
        clientMsgId: `agent_attachment_${generateUUID()}`,
      })

      if (get().selectedConversationId === selectedConversationId) {
        set((current) => ({
          messages: current.messages.some((m) => m.id === message.id)
            ? current.messages.map((m) => (m.id === message.id ? message : m))
            : [...current.messages, message],
          conversations: patchConversationList(
            patchConversationListWithMessage(current.conversations, message),
            {
              conversationId: selectedConversationId,
              agentUnreadCount: 0,
              customerUnreadCount:
                (current.conversations.find((item) => item.id === selectedConversationId)
                  ?.customerUnreadCount ?? 0) + 1,
              agentLastReadMessageId: message.id,
            }
          ),
        }))
      }

      return message
    } finally {
      set({ uploadingAsset: false })
    }
  },

  recallMessage: async (messageId) => {
    const { selectedConversationId, recallingMessageId } = get()
    if (!selectedConversationId || messageId <= 0 || recallingMessageId === messageId) {
      return null
    }

    set({ recallingMessageId: messageId })
    try {
      const message = await recallAgentMessage(messageId)
      if (get().selectedConversationId === selectedConversationId) {
        set((current) => {
          const nextMessages = current.messages.map((item) =>
            item.id === message.id ? message : item
          )
          const lastActiveMessage = [...nextMessages]
            .reverse()
            .find((item) => !item.recalledAt && item.sendStatus !== 6)
          return {
            recallingMessageId: 0,
            messages: nextMessages,
            conversations: current.conversations.map((item) =>
              item.id === selectedConversationId
                ? {
                    ...item,
                    lastMessageId: lastActiveMessage?.id ?? 0,
                    lastMessageAt: lastActiveMessage?.sentAt ?? "",
                    lastMessageSummary: lastActiveMessage
                      ? summarizeIMMessage(lastActiveMessage)
                      : "",
                  }
                : item
            ),
          }
        })
      } else {
        set({ recallingMessageId: 0 })
      }
      return message
    } catch (error) {
      set({ recallingMessageId: 0 })
      throw error
    }
  },
}))

export const agentConversationSelectors = {
  selectedConversation: (state: AgentConversationsStore) =>
    state.conversations.find((item) => item.id === state.selectedConversationId) ?? state.selectedConversationData,
}
