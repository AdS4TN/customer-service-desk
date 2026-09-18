"use client"

import { RefreshCwIcon } from "lucide-react"
import { useCallback, useEffect, useState } from "react"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { useI18n } from "@/i18n/provider"
import { fetchAIReceptionState, type AIReceptionState } from "@/lib/api/ai-reception"

export function ReceptionState({ agentId, refreshKey }: { agentId: number; refreshKey: number }) {
  const t = useI18n()
  const [result, setResult] = useState<{ key: string; state: AIReceptionState | null; error: boolean } | null>(null)
  const [refresh, setRefresh] = useState(0)
  const key = `${agentId}:${refreshKey}:${refresh}`
  const loading = result?.key !== key
  const state = loading ? null : result.state
  const error = !loading && result.error
  const onRefresh = useCallback(() => setRefresh((value) => value + 1), [])
  useEffect(() => {
    let cancelled = false
    fetchAIReceptionState(agentId).then((state) => { if (!cancelled) setResult({ key, state, error: false }) }).catch(() => { if (!cancelled) setResult({ key, state: null, error: true }) })
    return () => { cancelled = true }
  }, [agentId, key])
  return <section className="flex flex-col gap-4" aria-busy={loading}>
    <div className="flex items-center justify-between gap-3">
      <h2 className="text-base font-semibold">{t("aiReception.effective")}</h2>
      <Button type="button" variant="ghost" size="sm" onClick={onRefresh} disabled={loading}>
        <RefreshCwIcon data-icon="inline-start" />{t("aiAgent.refresh")}
      </Button>
    </div>
    {error ? <p role="alert" className="text-sm text-destructive">{t("aiReception.loadFailed")}</p> : !state ?
      <p role="status" className="text-sm text-muted-foreground">{t("common.loading")}</p> : <>
      <dl className="grid grid-cols-[minmax(0,1fr)_minmax(0,1fr)] gap-x-5 gap-y-4 border-b pb-5 text-sm">
        <dt className="text-muted-foreground">{t("aiReception.autoReception")}</dt>
        <dd>{t(state.receptionEnabled ? "aiReception.enabled" : "aiReception.paused")}</dd>
        <dt className="text-muted-foreground">{t("aiReception.assistance")}</dt>
        <dd>{t(state.assistanceAvailable ? "aiReception.available" : "aiReception.modelUnavailable")}</dd>
        <dt className="text-muted-foreground">{t("aiReception.publishedModel")}</dt>
        <dd className="break-words">{state.modelName || "-"}</dd>
        <dt className="text-muted-foreground">{t("aiReception.publishedKnowledge")}</dt>
        <dd>{state.assistanceAvailable ? t("aiReception.resources", { knowledge: state.knowledgeCount, skills: state.skillCount }) : "-"}</dd>
        <dt className="text-muted-foreground">{t("aiReception.language")}</dt>
        <dd>{t("aiReception.customerLanguage")}</dd>
      </dl>
      <section className="flex flex-col gap-3">
        <h3 className="text-sm font-medium">{t("aiReception.channels")}</h3>
        {state.channels.length === 0 ? <p className="text-sm text-muted-foreground">{t("aiReception.noChannels")}</p> :
          <ul className="divide-y">
            {state.channels.map((channel) => <li key={channel.id} className="flex flex-wrap items-center justify-between gap-3 py-3">
              <div className="min-w-0 flex-1 basis-full sm:basis-0"><p className="break-words text-sm font-medium">{channel.name}</p><p className="text-xs text-muted-foreground">{channel.channelType}</p></div>
              <div className="flex flex-wrap gap-2">
                <Badge variant="secondary">{t(channel.receptionEnabled ? "aiReception.receiveEnabled" : "aiReception.receiveDisabled")}</Badge>
                <Badge variant="outline">{t(channel.outboundBlocked ? "aiReception.outboundBlocked" : channel.automaticMessagesAllowed ? "aiReception.autoAllowed" : "aiReception.autoPaused")}</Badge>
              </div>
            </li>)}
          </ul>}
      </section>
    </>}
  </section>
}
