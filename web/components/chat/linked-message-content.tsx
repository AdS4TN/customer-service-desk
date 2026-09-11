"use client"

import { useState } from "react"
import { DownloadIcon } from "lucide-react"
import { Button } from "@/components/ui/button"
import { useI18n } from "@/i18n/provider"
import { formatFileSize, parseMessageAssetPayload } from "@/lib/im-message"
import { linkedKind, parseLinkedMessage, safeMediaURL } from "@/lib/linked-message"

export function LinkedMessageContent({ message, onImageClick, onSettled }: {
  message: { content: string; payload?: string }
  onImageClick?: (src: string, alt?: string) => void
  onSettled?: () => void
}) {
  const t = useI18n()
  const [failedURL, setFailedURL] = useState<string>()
  const meta = parseLinkedMessage(message.payload)
  if (!meta) return null
  const kind = linkedKind(meta.kind)
  const asset = parseMessageAssetPayload(message.payload)
  const url = safeMediaURL(asset?.url)
  const label = t(`linkedMessage.${kind}`)
  const mime = asset?.mimeType || ""
  const image = (kind === "image" || kind === "sticker") && /^image\/(png|jpeg|webp|gif)(;|$)/.test(mime)
  const audio = kind === "audio" && (mime.startsWith("audio/") || mime === "application/ogg" || mime === "video/mp4")
  const video = kind === "video" && mime.startsWith("video/")
  const media = ["image", "sticker", "audio", "video", "document"].includes(kind)
  const ready = meta.state === "ready" && !!url
  const preview = ready && failedURL !== url
  return (
    <div className="flex min-w-0 max-w-full flex-col gap-2 text-sm">
      <span className="font-medium">{kind === "reaction" && !message.content ? t("linkedMessage.reactionRemoved") : label}</span>
      {meta.filename ? <span className="break-all">{meta.filename}</span> : null}
      {preview && image ? (
        <button type="button" className="max-w-full rounded-md focus-visible:outline-2 focus-visible:outline-ring" aria-label={t("linkedMessage.preview")}
          onClick={() => onImageClick?.(url, label)}>
          {/* WhatsApp media is served by our existing asset storage, not Next Image. */}
          {/* eslint-disable-next-line @next/next/no-img-element */}
          <img src={url} alt={label} width={kind === "sticker" ? 160 : 280} height={kind === "sticker" ? 160 : 210}
            className="max-h-64 max-w-full rounded-md object-contain" onLoad={onSettled} onError={() => { setFailedURL(url); onSettled?.() }} />
        </button>
      ) : null}
      {preview && audio ? <audio controls preload="none" src={url} aria-label={label} className="w-64 max-w-full" onError={() => setFailedURL(url)} /> : null}
      {preview && video ? <video controls preload="metadata" src={url} aria-label={label} className="aspect-video w-72 max-w-full rounded-md" onLoadedMetadata={onSettled} onError={() => setFailedURL(url)} /> : null}
      {message.content ? <p className="whitespace-pre-wrap break-words [overflow-wrap:anywhere]">{message.content}</p> : null}
      {meta.size || meta.seconds ? <span className="text-xs">{[formatFileSize(meta.size || 0), meta.seconds ? t("linkedMessage.duration", { seconds: meta.seconds }) : ""].filter(Boolean).join(" · ")}</span> : null}
      {kind === "reaction" ? <span className="text-xs">{t("linkedMessage.reactionTarget")}</span> : null}
      {kind === "reaction" && meta.targetPreview ? <blockquote className="break-words border-s border-current ps-2 text-xs">{meta.targetPreview}</blockquote> : null}
      {media && !ready ? <p role="status">{t(meta.state === "pending" ? "linkedMessage.pending" : meta.state === "too_large" ? "linkedMessage.tooLarge" : "linkedMessage.unavailable")}</p> : null}
      {ready && failedURL === url ? <p role="status">{t("linkedMessage.previewFailed")}</p> : null}
      {kind === "view_once" ? <p>{t("linkedMessage.protected")}</p> : null}
      {kind === "unsupported" ? <p>{t("linkedMessage.unsupportedHint")}</p> : null}
      {ready ? <Button variant="outline" size="sm" className="self-start rounded-md" render={<a href={url} target="_blank" rel="noopener noreferrer" download={meta.filename || "attachment"} />} nativeButton={false}>
        <DownloadIcon data-icon="inline-start" />{t("linkedMessage.download")}
      </Button> : null}
    </div>
  )
}
