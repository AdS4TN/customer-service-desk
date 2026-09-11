"use client"

import { RefreshCwIcon, SearchIcon } from "lucide-react"
import { useState } from "react"
import { OptionCombobox } from "@/components/option-combobox"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { useI18n } from "@/i18n/provider"
import { useAgentConversationsStore } from "@/lib/stores/agent-conversations"

export function InboxFilters() {
  const t = useI18n()
  const channels = useAgentConversationsStore((s) => s.channels)
  const channelType = useAgentConversationsStore((s) => s.channelType)
  const channelId = useAgentConversationsStore((s) => s.channelId)
  const keyword = useAgentConversationsStore((s) => s.searchKeyword)
  const loading = useAgentConversationsStore((s) => s.conversationsLoading)
  const channelError = useAgentConversationsStore((s) => s.channelsError)
  const setChannelFilter = useAgentConversationsStore((s) => s.setChannelFilter)
  const setKeyword = useAgentConversationsStore((s) => s.setSearchKeyword)
  const load = useAgentConversationsStore((s) => s.loadConversations)
  const loadChannels = useAgentConversationsStore((s) => s.loadChannels)
  const [draft, setDraft] = useState(keyword)

  return (
    <div className="flex shrink-0 flex-col gap-2 border-b p-2">
      <form className="flex min-w-0 gap-1" onSubmit={(event) => { event.preventDefault(); setKeyword(draft) }}>
        <Input value={draft} onChange={(event) => setDraft(event.target.value)} placeholder={t("conversation.inbox.searchShort")} aria-label={t("conversation.inbox.search")} className="min-w-0" />
        <Button type="submit" variant="ghost" size="icon" aria-label={t("conversation.inbox.searchAction")} title={t("conversation.inbox.searchAction")}><SearchIcon /></Button>
        <Button type="button" variant="ghost" size="icon" disabled={loading} aria-label={t("conversation.inbox.refresh")} title={t("conversation.inbox.refresh")} onClick={() => { void load().catch(() => {}); void loadChannels() }}><RefreshCwIcon /></Button>
      </form>
      <OptionCombobox value={channelType} onChange={(value) => setChannelFilter(value)} placeholder={t("conversation.inbox.allChannels")} options={[
        { value: "", label: t("conversation.inbox.allChannels") },
        { value: "web", label: t("conversation.inbox.web") },
        { value: "whatsapp", label: "WhatsApp" },
 { value: "messenger", label: "Messenger" },
      ]} />
      <OptionCombobox value={channelId} onChange={(value) => setChannelFilter(channelType, value)} placeholder={t("conversation.inbox.allAccounts")} options={[
        { value: "", label: t("conversation.inbox.allAccounts") },
        ...channels.filter((channel) => !channelType || channel.channelType === channelType).map((channel) => ({ value: String(channel.id), label: channel.name })),
      ]} />
      {channelError ? <p role="alert" className="text-xs text-destructive">{t("conversation.inbox.channelLoadFailed")}</p> : null}
    </div>
  )
}
