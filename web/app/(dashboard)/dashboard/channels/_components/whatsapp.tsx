"use client"

import { useEffect, useRef, useState } from "react"
import { LinkIcon, UnlinkIcon, LogOutIcon } from "lucide-react"
import { toast } from "sonner"
import { ProjectDialog } from "@/components/project-dialog"
import { Button } from "@/components/ui/button"
import { Badge } from "@/components/ui/badge"
import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert"
import { useI18n } from "@/i18n/provider"
import { fetchWhatsAppConnection, updateWhatsAppConnection, type WhatsAppConnection } from "@/lib/api/admin"

export function WhatsAppDialog({ channel, onClose }: { channel: { id: number; name: string }; onClose: () => void }) {
  const t = useI18n()
  const [connection, setConnection] = useState<WhatsAppConnection | null>(null)
  const [error, setError] = useState("")
  const [busy, setBusy] = useState(false)
  const active = useRef(true)
  const mutation = useRef(0)

  useEffect(() => {
    active.current = true
    let timer: ReturnType<typeof setTimeout>
    async function poll() {
      const version = mutation.current
      try {
        const result = await fetchWhatsAppConnection(channel.id)
        if (active.current && version === mutation.current) {
          setConnection(result)
          setError("")
        }
      } catch (err) {
        if (active.current) {
          setConnection(null) // Never leave a stale QR displayed after losing access.
          setError(err instanceof Error ? err.message : t("channel.wa.requestFailed"))
        }
      } finally {
        if (active.current) timer = setTimeout(poll, 2000)
      }
    }
    void poll()
    return () => { active.current = false; clearTimeout(timer) }
  }, [channel.id, t])

  async function act(action: "connect" | "disconnect" | "logout") {
    if (action === "logout" && !window.confirm(t("channel.wa.logoutConfirm"))) return
    mutation.current += 1
    setBusy(true)
    try {
      const result = await updateWhatsAppConnection(channel.id, action)
      if (active.current) { setConnection(result); setError("") }
    } catch (err) {
      toast.error(err instanceof Error ? err.message : t("channel.wa.requestFailed"))
    } finally {
      mutation.current += 1
      if (active.current) setBusy(false)
    }
  }

  const state = connection?.state
  const qrValid = connection?.qr && connection.qrExpiresAt && new Date(connection.qrExpiresAt).getTime() > Date.now()
  return (
    <ProjectDialog open onOpenChange={(open) => { if (!open) onClose() }} title={`${channel.name} · WhatsApp`} size="md">
      <div className="flex flex-col gap-4">
        <div className="flex items-center justify-between gap-3" aria-live="polite">
          <Badge variant={state === "connected" ? "default" : "outline"}>
            {state ? t(`channel.wa.state.${state}`) : t("channel.loading")}
          </Badge>
          {connection?.account && <span className="truncate text-sm">{connection.account.split("@")[0]}</span>}
        </div>
        <Alert>
          <AlertTitle>{t("channel.wa.experimental")}</AlertTitle>
          <AlertDescription>{t("channel.wa.risk")}</AlertDescription>
        </Alert>
        {(error || connection?.error) && (
          <Alert variant="destructive">
            <AlertDescription>{error || t(`channel.wa.error.${connection?.error}`)}</AlertDescription>
          </Alert>
        )}
        {state === "qr" && (
          <div className="flex h-[310px] items-center justify-center">
            {qrValid ? (
              // A server-generated QR is deliberately not sent through an image optimizer.
              // eslint-disable-next-line @next/next/no-img-element
              <img src={connection?.qr} alt={t("channel.wa.qrAlt")} width={280} height={280} />
            ) : <span className="text-sm text-muted-foreground">{t("channel.wa.refreshingQR")}</span>}
          </div>
        )}
        <div className="flex flex-wrap gap-2">
          <Button type="button" disabled={busy || state === "connected" || state === "connecting" || state === "qr" || state === "disabled"} onClick={() => void act("connect")}>
            <LinkIcon data-icon="inline-start" />{t("channel.wa.connect")}
          </Button>
          <Button type="button" variant="outline" disabled={busy || !state || state === "disabled" || state === "logged_out" || state === "disconnected"} onClick={() => void act("disconnect")}>
            <UnlinkIcon data-icon="inline-start" />{t("channel.wa.disconnect")}
          </Button>
          {connection?.account && (
            <Button type="button" variant="outline" disabled={busy || state !== "connected"} onClick={() => void act("logout")}>
              <LogOutIcon data-icon="inline-start" />{t("channel.wa.logout")}
            </Button>
          )}
        </div>
      </div>
    </ProjectDialog>
  )
}
