import { parseMessageAssetPayload } from "@/lib/im-message"

export type MergeableImMessage = {
  id: number
  conversationId: number
  clientMsgId?: string
  senderType: string
  senderId: number
  senderName?: string
  senderAvatar?: string
  messageType: string
  content: string
  payload?: string
  sendStatus: number
  sentAt?: string
  deliveredAt?: string
  readAt?: string
  customerRead: boolean
  customerReadAt?: string
  agentRead: boolean
  agentReadAt?: string
  recalledAt?: string
  recallableUntil?: string
  quotedMessageId?: number
}

export function mergeImMessagesByIdAsc<T extends MergeableImMessage>(
  a: T[],
  b: T[]
): T[] {
  const byId = new Map<number, T>()
  for (const message of a) {
    byId.set(message.id, message)
  }
  for (const message of b) {
    const existing = byId.get(message.id)
    byId.set(message.id, existing ? mergeImMessage(existing, message) : message)
  }
  const values = Array.from(byId.values())
  return values.sort(messageComparator(values))
}

type MessagePosition = { id: number; clientMsgId?: string; sentAt?: string }

function isLinkedChannelMessage(message: MessagePosition) {
  return message.clientMsgId?.startsWith("wa_") || message.clientMsgId?.startsWith("ms_")
}

function messageComparator(messages: MessagePosition[]) {
  const chronological = messages.some(isLinkedChannelMessage)
  return (a: MessagePosition, b: MessagePosition) => {
    if (chronological) {
      const first = Date.parse(a.sentAt ?? "")
      const second = Date.parse(b.sentAt ?? "")
      if (Number.isFinite(first) && Number.isFinite(second) && first !== second) return first - second
    }
    return a.id - b.id
  }
}

export function parseImMessageCursorId(cursor: string): number {
  const value = Number.parseInt(cursor, 10)
  return Number.isFinite(value) && value > 0 ? value : 0
}

export function cursorFromLoadedImMessages<T extends MessagePosition>(
  messages: T[]
): string {
  if (messages.length === 0) {
    return ""
  }
  const compare = messageComparator(messages)
  return String(messages.reduce((first, message) => compare(message, first) < 0 ? message : first).id)
}

export function hasMoreAfterLatestImMessageMerge<
  T extends MessagePosition,
>(args: {
  previousMessages: T[]
  previousHasMore: boolean
  merged: T[]
  apiHasMore: boolean
}): boolean {
  // A history batch can arrive after we previously reached the oldest record.
  if (args.merged.some(isLinkedChannelMessage)) {
    return args.previousHasMore || Boolean(args.apiHasMore)
  }
  const prevMin = minImMessageId(args.previousMessages)
  const mergedMin = minImMessageId(args.merged)

  if (mergedMin === null) {
    return Boolean(args.apiHasMore)
  }

  if (!args.previousHasMore && prevMin !== null && mergedMin >= prevMin) {
    return false
  }

  return args.previousHasMore || Boolean(args.apiHasMore)
}

function minImMessageId<T extends Pick<MergeableImMessage, "id">>(
  messages: T[]
): number | null {
  if (messages.length === 0) {
    return null
  }
  return Math.min(...messages.map((message) => message.id))
}

export function mergeImMessage<T extends MergeableImMessage>(
  existing: T,
  incoming: T
): T {
  const normalizedIncoming = normalizeDynamicImageContent(
    existing,
    normalizeDynamicImagePayload(existing, incoming)
  )
  return isSameImMessage(existing, normalizedIncoming) ? existing : normalizedIncoming
}

function normalizeDynamicImagePayload<T extends MergeableImMessage>(
  existing: T,
  incoming: T
): T {
  if (
    existing.messageType !== "image" ||
    incoming.messageType !== "image" ||
    !existing.payload ||
    !incoming.payload
  ) {
    return incoming
  }

  const existingAsset = parseMessageAssetPayload(existing.payload)
  const incomingAsset = parseMessageAssetPayload(incoming.payload)
  if (
    !existingAsset?.assetId ||
    existingAsset.assetId !== incomingAsset?.assetId ||
    existing.payload === incoming.payload
  ) {
    return incoming
  }

  return {
    ...incoming,
    payload: existing.payload,
  }
}

function normalizeDynamicImageContent<T extends MergeableImMessage>(
  existing: T,
  incoming: T
): T {
  if (
    existing.content === incoming.content ||
    !existing.content.includes("data-asset-id") ||
    !incoming.content.includes("data-asset-id")
  ) {
    return incoming
  }

  if (getStableHTMLContentKey(existing.content) !== getStableHTMLContentKey(incoming.content)) {
    return incoming
  }

  return {
    ...incoming,
    content: existing.content,
  }
}

function getStableHTMLContentKey(html: string): string {
  if (typeof document === "undefined") {
    return html.replace(
      /(<img\b[^>]*\bdata-asset-id=(["'])[^"']+\2[^>]*?)\s+(?:src|srcset)=(["'])[^"']*\3/gi,
      "$1"
    )
  }

  const template = document.createElement("template")
  template.innerHTML = html
  for (const image of Array.from(template.content.querySelectorAll("img"))) {
    if (image.getAttribute("data-asset-id")) {
      image.removeAttribute("src")
      image.removeAttribute("srcset")
    }
  }
  return template.innerHTML
}

function isSameImMessage(a: MergeableImMessage, b: MergeableImMessage): boolean {
  const aKeys = Object.keys(a)
  const bKeys = Object.keys(b)
  if (aKeys.length !== bKeys.length) {
    return false
  }
  return aKeys.every((key) => {
    const field = key as keyof MergeableImMessage
    return Object.prototype.hasOwnProperty.call(b, key) && a[field] === b[field]
  })
}
