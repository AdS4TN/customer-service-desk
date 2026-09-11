import type { InboxChannel } from "@/lib/api/agent"
import { translateCurrentMessage as t } from "@/i18n/messages"

export function inboxSourceLabel(channel: InboxChannel | undefined, channelId?: number) {
  if (!channel) return t("conversation.channelNumber", { id: channelId || "-" })
  const type = channel.channelType === "web" ? t("conversation.inbox.web") : channel.channelType === "whatsapp" ? "WhatsApp" : channel.channelType === "messenger" ? "Messenger" : channel.channelType
  return `${type} / ${channel.name}`
}
