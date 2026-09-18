"use client"

import { useState } from "react"
import { DownloadIcon, ExternalLinkIcon } from "lucide-react"
import { buttonVariants } from "@/components/ui/button"
import { Badge } from "@/components/ui/badge"
import { useAppLocale, useI18n } from "@/i18n/provider"
import { cn } from "@/lib/utils"
import { formatFileSize, parseMessageAssetPayload } from "@/lib/im-message"
import { linkedDate, linkedKind, parseLinkedMessage, safeMediaURL } from "@/lib/linked-message"

export function LinkedMessageContent({ message, onImageClick, onSettled }: {
  message: { content: string; payload?: string }
  onImageClick?: (src: string, alt?: string) => void
  onSettled?: () => void
}) {
  const t = useI18n()
  const { locale } = useAppLocale()
  const [failedURL, setFailedURL] = useState<string>()
  const meta = parseLinkedMessage(message.payload)
  if (!meta) return null
  const kind = linkedKind(meta.kind)
  const asset = parseMessageAssetPayload(message.payload)
  const url = safeMediaURL(asset?.url)
  const label = t(`linkedMessage.${kind}`)
  const mime = asset?.mimeType || ""
  const mediaKind = meta.mediaKind || kind
  const image = (mediaKind === "image" || mediaKind === "sticker") && /^image\/(png|jpeg|webp|gif)(;|$)/.test(mime)
  const audio = mediaKind === "audio" && (mime.startsWith("audio/") || mime === "application/ogg" || mime === "video/mp4")
  const video = ["video", "round_video"].includes(mediaKind) && mime.startsWith("video/")
  const media = ["image", "sticker", "audio", "video", "round_video", "document"].includes(mediaKind)
  const ready = meta.state === "ready" && !!url
  const preview = ready && failedURL !== url
  const start = linkedDate(meta.startAt, locale)
  const end = linkedDate(meta.endAt, locale)
  const fields = meta.fields?.filter(f => ["productId", "price", "orderId", "itemCount", "total", "images", "videos", "callOutcome"].includes(f.key) && f.value)
  const options = meta.options?.filter(o => o.label)
  return (
    <div className="flex min-w-0 max-w-full flex-col gap-2 text-sm [overflow-wrap:anywhere]">
      {kind !== "text" ? <span className="font-medium">{kind === "reaction" && !message.content && !meta.state ? t("linkedMessage.reactionRemoved") : label}</span> : null}
      {meta.forwarded ? <span className="text-xs">{t("linkedMessage.forwarded")}</span> : null}
      {meta.update === "edited" ? <span className="text-xs">{t("linkedMessage.edited")}</span> : null}
      {meta.targetPreview && !meta.update ? <blockquote className="whitespace-pre-wrap break-words border-s border-current ps-2 text-xs">{meta.targetPreview}</blockquote> : null}
      {meta.title ? <p className="whitespace-pre-wrap font-medium">{meta.title}</p> : null}
      {meta.state === "canceled" ? <Badge variant="secondary" className="self-start">{t("linkedMessage.canceled")}</Badge> : null}
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
      {preview && video ? <video controls playsInline preload="metadata" src={url} aria-label={label} className={cn("max-w-full", mediaKind === "round_video" ? "aspect-square w-56 rounded-full" : "aspect-video w-72 rounded-md")} onLoadedMetadata={onSettled} onError={() => setFailedURL(url)} /> : null}
      {message.content ? <p className="whitespace-pre-wrap break-words [overflow-wrap:anywhere]">{message.content}</p> : null}
      {options?.length ? <ul className="flex min-w-0 flex-col gap-2" aria-label={t("linkedMessage.options")}>
        {options.map((option, index) => <li key={`${option.id || "option"}-${index}`} className="flex min-w-0 items-start gap-2">
          <span className="shrink-0 text-xs tabular-nums">{index + 1}.</span>
          <div className="flex min-w-0 flex-1 flex-col gap-1"><span className="whitespace-pre-wrap">{option.label}</span>{option.description ? <span className="whitespace-pre-wrap text-xs">{option.description}</span> : null}</div>
          {option.count !== undefined ? <span className="shrink-0 text-xs tabular-nums">{t("linkedMessage.votes", { count: option.count })}</span> : null}
        </li>)}
      </ul> : null}
      {meta.selectable ? <span className="text-xs">{t("linkedMessage.selectable", { count: meta.selectable })}</span> : null}
      {fields?.length ? <dl className="flex flex-col gap-1">{fields.map((field, i) => <div key={`${field.key}-${i}`} className="flex flex-wrap justify-between gap-x-4 gap-y-1"><dt className="text-xs">{t(`linkedMessage.fields.${field.key}`)}</dt><dd className="min-w-0 whitespace-pre-wrap tabular-nums">{field.key === "callOutcome" ? t(`linkedMessage.callOutcomes.${["CONNECTED", "MISSED", "FAILED", "REJECTED", "ACCEPTED_ELSEWHERE", "REJECTED_ELSEWHERE", "ONGOING", "SILENCED_BY_DND", "SILENCED_UNKNOWN_CALLER"].includes(field.value) ? field.value : "UNKNOWN"}`) : field.value}</dd></div>)}</dl> : null}
      {start ? <p className="text-xs">{t("linkedMessage.starts", { time: start })}</p> : null}
      {end ? <p className="text-xs">{t("linkedMessage.ends", { time: end })}</p> : null}
      {meta.footer ? <p className="whitespace-pre-wrap text-xs">{meta.footer}</p> : null}
      {meta.size || meta.seconds ? <span className="text-xs">{[meta.size ? formatFileSize(meta.size) : "", meta.seconds ? t("linkedMessage.duration", { seconds: meta.seconds }) : ""].filter(Boolean).join(" · ")}</span> : null}
      {kind === "reaction" ? <span className="text-xs">{t("linkedMessage.reactionTarget")}</span> : null}
      {media && !ready ? <p role="status">{t(meta.state === "pending" ? "linkedMessage.pending" : meta.state === "too_large" ? "linkedMessage.tooLarge" : "linkedMessage.unavailable")}</p> : null}
      {ready && failedURL === url ? <p role="status">{t("linkedMessage.previewFailed")}</p> : null}
      {kind === "view_once" ? <p>{t("linkedMessage.protected")}</p> : null}
      {meta.state === "encrypted_unavailable" ? <p role="status">{t("linkedMessage.encryptedUnavailable")}</p> : null}
      {meta.state === "target_unavailable" ? <p role="status">{t("linkedMessage.targetUnavailable")}</p> : null}
      {meta.state === "vote_removed" ? <p>{t("linkedMessage.voteRemoved")}</p> : null}
      {meta.state === "partial" ? <p>{t("linkedMessage.partial")}</p> : null}
      {kind === "revoked" ? <p>{t("linkedMessage.revokedHint")}</p> : null}
      {kind === "unsupported" ? <p>{t("linkedMessage.unsupportedHint")}</p> : null}
      {(kind === "unsupported" || kind === "notice" || meta.state === "partial" || meta.state === "encrypted_unavailable") && meta.rawType ? <details className="text-xs"><summary className="cursor-pointer">{t("linkedMessage.typeDetails")}</summary><code className="block break-all pt-1">{meta.rawType}</code></details> : null}
      {meta.url ? <a href={meta.url} target="_blank" rel="noopener noreferrer" className={cn(buttonVariants({ variant: "secondary", size: "sm" }), "self-start")}><ExternalLinkIcon data-icon="inline-start" />{t("linkedMessage.openLink")}</a> : null}
      {ready ? <a href={url} target="_blank" rel="noopener noreferrer" download={meta.filename || "attachment"} className={cn(buttonVariants({ variant: "secondary", size: "sm" }), "self-start")}>
        <DownloadIcon data-icon="inline-start" />{t("linkedMessage.download")}
      </a> : null}
    </div>
  )
}
