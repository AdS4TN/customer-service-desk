export type LinkedMessage = {
  kind: string
  state?: string
  filename?: string
  mimeType?: string
  size?: number
  seconds?: number
  targetId?: string
  targetPreview?: string
}

export function parseLinkedMessage(payload?: string): LinkedMessage | null {
  try {
    const value = JSON.parse(payload || "{}").linkedMessage
    if (!value || typeof value.kind !== "string") return null
    return {
      kind: value.kind,
      state: typeof value.state === "string" ? value.state : undefined,
      filename: typeof value.filename === "string" ? value.filename : undefined,
      mimeType: typeof value.mimeType === "string" ? value.mimeType : undefined,
      size: typeof value.size === "number" ? value.size : undefined,
      seconds: typeof value.seconds === "number" ? value.seconds : undefined,
      targetId: typeof value.targetId === "string" ? value.targetId : undefined,
      targetPreview: typeof value.targetPreview === "string" ? value.targetPreview : undefined,
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
  return ["image", "sticker", "audio", "video", "document", "reaction", "contact", "location", "view_once"].includes(kind) ? kind : "unsupported"
}
