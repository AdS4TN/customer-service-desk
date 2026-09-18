"use client"

import { Suspense, useCallback, useEffect, useState } from "react"
import { useRouter, useSearchParams } from "next/navigation"
import { useConfirm } from "@/components/confirm-provider"
import { useI18n } from "@/i18n/provider"
import { AIAgentConfigWorkbench } from "../_components/config-workbench"

function EmployeeEditor() {
  const query = useSearchParams()
  const router = useRouter()
  const t = useI18n()
  const confirm = useConfirm()
  const rawId = query.get("id")
  const id = rawId ? Number(rawId) : null
  const [state, setState] = useState({ dirty: false, saving: false })
  const onState = useCallback((value: typeof state) => setState(value), [])
  useEffect(() => {
    if (!state.dirty && !state.saving) return
    let asking = false
    let approvedEntry = ""
    const interceptTraversal = (event: NavigateEvent) => {
      if (event.navigationType !== "traverse" || !event.destination.sameDocument || !event.cancelable) return
      if (approvedEntry === event.destination.key) { approvedEntry = ""; return }
      event.preventDefault()
      if (state.saving || asking) return
      asking = true
      const destination = event.destination.key
      void confirm({ title: t("aiReception.discardTitle"), description: t("aiReception.discardBody"), confirmText: t("reception.discard") }).then((accepted) => {
        if (accepted) {
          approvedEntry = destination
          void window.navigation?.traverseTo(destination).finished?.catch(() => { approvedEntry = "" })
        }
      }).finally(() => { asking = false })
    }
    const intercept = (event: MouseEvent) => {
      const anchor = event.target instanceof Element ? event.target.closest("a[href]") : null
      if (!(anchor instanceof HTMLAnchorElement) || anchor.target === "_blank" || event.metaKey || event.ctrlKey || event.shiftKey || event.altKey || event.button !== 0) return
      const url = new URL(anchor.href)
      if (url.origin !== location.origin || url.href === location.href) return
      event.preventDefault(); event.stopImmediatePropagation()
      if (state.saving || asking) return
      asking = true
      void confirm({ title: t("aiReception.discardTitle"), description: t("aiReception.discardBody"), confirmText: t("reception.discard") }).then((accepted) => {
        if (accepted) router.push(url.pathname + url.search + url.hash)
      }).finally(() => { asking = false })
    }
    document.addEventListener("click", intercept, true)
    window.navigation?.addEventListener("navigate", interceptTraversal)
    return () => {
      document.removeEventListener("click", intercept, true)
      window.navigation?.removeEventListener("navigate", interceptTraversal)
    }
  }, [state.dirty, state.saving, confirm, router, t])
  async function leave() {
    if (state.saving) return
    if (state.dirty && !await confirm({ title: t("aiReception.discardTitle"), description: t("aiReception.discardBody"), confirmText: t("reception.discard") })) return
    router.push("/dashboard/ai-agents")
  }
  if (rawId && (!Number.isSafeInteger(id) || Number(id) <= 0)) return <p role="alert" className="p-6">{t("aiAgent.loadDetailFailed")}</p>
  return <div className="flex min-h-0 flex-1 flex-col overflow-hidden">
    <AIAgentConfigWorkbench key={rawId || "new"} agentId={id} onCancel={() => void leave()} onPolicyStateChange={onState}
      onAgentCreated={(agent) => router.replace(`/dashboard/ai-agents/editor?id=${agent.id}`)} />
  </div>
}

export default function EmployeeEditorPage() {
  return <Suspense><EmployeeEditor /></Suspense>
}
