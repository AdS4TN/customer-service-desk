export type LinkedMessage = {
  kind: string
  state?: string
  filename?: string
  mimeType?: string
  size?: number
  seconds?: number
  targetId?: string
  targetPreview?: string
  rawType?: string
  title?: string
  footer?: string
  url?: string
  mediaKind?: string
  options?: { id?: string; label: string; description?: string; count?: number }[]
  fields?: { key: string; value: string }[]
  startAt?: number
  endAt?: number
  selectable?: number
  forwarded?: boolean
  update?: string
}

const stringValue = (v: unknown) => typeof v === "string" ? v : undefined
const numberValue = (v: unknown) => typeof v === "number" && Number.isFinite(v) && v >= 0 ? v : undefined
const record = (v: unknown): v is Record<string, unknown> => !!v && typeof v === "object" && !Array.isArray(v)

export function parseLinkedMessage(payload?: string): LinkedMessage | null {
  try {
    const value = JSON.parse(payload || "{}").linkedMessage
    if (!value || typeof value.kind !== "string") return null
    return {
      kind: value.kind,
      state: typeof value.state === "string" ? value.state : undefined,
      filename: typeof value.filename === "string" ? value.filename : undefined,
      mimeType: typeof value.mimeType === "string" ? value.mimeType : undefined,
      size: numberValue(value.size),
      seconds: numberValue(value.seconds),
      targetId: typeof value.targetId === "string" ? value.targetId : undefined,
      targetPreview: typeof value.targetPreview === "string" ? value.targetPreview : undefined,
      rawType: stringValue(value.rawType), title: stringValue(value.title), footer: stringValue(value.footer),
      url: safeMediaURL(stringValue(value.url)), mediaKind: stringValue(value.mediaKind),
      startAt: numberValue(value.startAt), endAt: numberValue(value.endAt), selectable: numberValue(value.selectable),
      forwarded: value.forwarded === true, update: stringValue(value.update),
      options: Array.isArray(value.options) ? value.options.filter(record).filter((o: Record<string, unknown>) => typeof o.label === "string").map((o: Record<string, unknown>) => ({ id: stringValue(o.id), label: o.label as string, description: stringValue(o.description), count: numberValue(o.count) })) : undefined,
      fields: Array.isArray(value.fields) ? value.fields.filter(record).filter((f: Record<string, unknown>) => typeof f.key === "string" && typeof f.value === "string").map((f: Record<string, unknown>) => ({ key: f.key as string, value: f.value as string })) : undefined,
    }
  } catch { return null }
}

export function safeMediaURL(url?: string) {
  if (!url || /[\s\\]/.test(url)) return undefined
  if (url.startsWith("/") && !url.startsWith("//")) return url
  try {
    const parsed = new URL(url)
    return ["http:", "https:"].includes(parsed.protocol) ? url : undefined
  } catch { return undefined }
}

export function linkedKind(kind: string) {
  return ["text", "image", "sticker", "audio", "video", "round_video", "document", "reaction", "contact", "location", "live_location", "view_once", "poll", "poll_vote", "poll_update", "buttons", "list", "template", "interactive", "reply", "product", "order", "event", "event_response", "album", "sticker_pack", "invite", "payment", "call", "notice", "revoked"].includes(kind) ? kind : "unsupported"
}

export function linkedDate(seconds: number | undefined, locale: string) {
  if (!seconds || !Number.isFinite(seconds)) return undefined
  const date = new Date(seconds * 1000)
  return Number.isNaN(date.getTime()) ? undefined : new Intl.DateTimeFormat(locale, { dateStyle: "medium", timeStyle: "short" }).format(date)
}
