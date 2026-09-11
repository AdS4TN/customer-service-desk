"use client"

import { useEffect, useRef, useState } from "react"
import { LinkIcon, UnlinkIcon, LogOutIcon, LoaderCircleIcon } from "lucide-react"
import { ProjectDialog } from "@/components/project-dialog"
import { Button } from "@/components/ui/button"
import { Badge } from "@/components/ui/badge"
import { Input } from "@/components/ui/input"
import { Field, FieldGroup, FieldLabel } from "@/components/ui/field"
import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert"
import { useI18n } from "@/i18n/provider"
import { fetchMessengerConnection, updateMessengerConnection, type MessengerConnection } from "@/lib/api/admin"

export function MessengerDialog({ channel, onClose }: { channel: { id: number; name: string }; onClose: () => void }) {
  const t = useI18n()
  const [connection, setConnection] = useState<MessengerConnection | null>(null)
  const [cookie, setCookie] = useState("")
  const [error, setError] = useState("")
  const [busy, setBusy] = useState(false)
  const active = useRef(true)
  const version = useRef(0)
  const lock = useRef(false)

  useEffect(() => {
    active.current = true
    let timer: ReturnType<typeof setTimeout>
    async function poll() {
      const current = version.current
      try {
        const result = await fetchMessengerConnection(channel.id)
        if (active.current && current === version.current && !lock.current) setConnection(result)
      } catch (err) {
        if (active.current && current === version.current && !lock.current) {
          setConnection(null)
          setError(err instanceof Error ? err.message : t("channel.ms.requestFailed"))
        }
      } finally {
        if (active.current) timer = setTimeout(poll, 2500)
      }
    }
    void poll()
    return () => { active.current = false; clearTimeout(timer) }
  }, [channel.id, t])

  async function act(action: "login" | "connect" | "disconnect" | "logout") {
    if (lock.current) return
    if (action === "logout" && !window.confirm(t("channel.ms.logoutConfirm", { name: channel.name }))) return
    if (action === "login" && !window.confirm(t("channel.ms.loginConfirm", { name: channel.name }))) return
    lock.current = true
    version.current += 1
    setBusy(true)
    setError("")
    try {
      const result = await updateMessengerConnection(channel.id, action, cookie)
      if (active.current) { setConnection(result); if (action === "login" || action === "logout") setCookie("") }
    } catch (err) {
      if (active.current) setError(err instanceof Error ? err.message : t("channel.ms.requestFailed"))
    } finally {
      version.current += 1
      lock.current = false
      if (active.current) setBusy(false)
    }
  }

  const state = connection?.state
  const disabled = busy || !connection || state === "disabled"
  function close() {
    if (busy || (cookie && !window.confirm(t("channel.ms.discard")))) return
    onClose()
  }
  return (
    <ProjectDialog open onOpenChange={(open) => { if (!open) close() }} title={`${channel.name} · Messenger`} size="md"
      footer={<Button variant="outline" disabled={busy} onClick={close}>{t("common.close")}</Button>}>
      <div className="flex flex-col gap-4">
        <div className="flex items-center justify-between gap-3" aria-live="polite">
          <Badge variant={state === "connected" ? "default" : "outline"}>{state ? t(`channel.wa.state.${state}`) : t("channel.loading")}</Badge>
          {connection?.account && <span className="truncate text-sm">{connection.account}</span>}
        </div>
        <Alert>
          <AlertTitle>{t("channel.ms.experimental")}</AlertTitle>
          <AlertDescription>{t("channel.ms.limitations")}</AlertDescription>
        </Alert>
        {(error || connection?.error) && <Alert variant="destructive"><AlertDescription>{error || t(`channel.ms.error.${connection?.error}`)}</AlertDescription></Alert>}
        <form onSubmit={(event) => { event.preventDefault(); void act("login") }}>
          <FieldGroup>
            <Field data-disabled={disabled}>
              <FieldLabel htmlFor="messenger-cookie">{t("channel.ms.cookie")}</FieldLabel>
              <Input id="messenger-cookie" type="password" autoComplete="off" spellCheck={false} maxLength={32768}
                value={cookie} onChange={(event) => setCookie(event.target.value)} disabled={disabled}
                placeholder={t(connection?.hasCredentials ? "channel.ms.saved" : "channel.ms.required")} />
            </Field>
            <Button type="submit" disabled={disabled || !cookie.trim()}>
              {busy ? <LoaderCircleIcon className="animate-spin" data-icon="inline-start" /> : <LinkIcon data-icon="inline-start" />}
              {t("channel.ms.login")}
            </Button>
          </FieldGroup>
        </form>
        <div className="flex flex-wrap gap-2">
          <Button variant="outline" disabled={disabled || !connection?.hasCredentials || state === "connected" || state === "connecting"} onClick={() => void act("connect")}>
            <LinkIcon data-icon="inline-start" />{t("channel.ms.reconnect")}
          </Button>
          <Button variant="outline" disabled={disabled || state === "disconnected"} onClick={() => void act("disconnect")}>
            <UnlinkIcon data-icon="inline-start" />{t("channel.wa.disconnect")}
          </Button>
          <Button variant="outline" disabled={disabled || !connection?.hasCredentials} onClick={() => void act("logout")}>
            <LogOutIcon data-icon="inline-start" />{t("channel.ms.logout")}
          </Button>
        </div>
      </div>
    </ProjectDialog>
  )
}
