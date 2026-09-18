"use client"

import { useEffect, useRef, useState } from "react"
import { ArrowDownIcon, RotateCcwIcon, SendIcon, SquareIcon } from "lucide-react"
import { Button } from "@/components/ui/button"
import { Textarea } from "@/components/ui/textarea"
import { Field, FieldLabel } from "@/components/ui/field"
import { Alert, AlertDescription } from "@/components/ui/alert"
import { Message, MessageContent, MessageHeader } from "@/components/ui/message"
import { Bubble, BubbleContent } from "@/components/ui/bubble"
import { MessageScroller, MessageScrollerProvider, MessageScrollerViewport, MessageScrollerContent, MessageScrollerItem, MessageScrollerButton } from "@/components/ui/message-scroller"
import { useI18n } from "@/i18n/provider"
import { previewAIEmployee, type PreviewTurn, type EmployeePreviewResult } from "@/lib/api/ai-employee-preview"
import type { CreateAIAgentPayload } from "@/lib/api/admin"

type TrialMessage = PreviewTurn & { id: number; evidence?: EmployeePreviewResult; draftKey: string }

export function EmployeePreview({ draft, resourceKey }: { draft: CreateAIAgentPayload; resourceKey: string }) {
  const t = useI18n()
  const [messages, setMessages] = useState<TrialMessage[]>([])
  const [text, setText] = useState("")
  const [error, setError] = useState("")
  const [busy, setBusy] = useState(false)
  const controller = useRef<AbortController | null>(null)
  const draftKey = JSON.stringify([draft, resourceKey])
  useEffect(() => () => controller.current?.abort(), [])
  const lastReply = messages.findLast((message) => message.role === "assistant")
  async function submit() {
    if (controller.current || !text.trim()) return
    if (messages.length >= 99) { setError(t("employee.historyLimit")); return }
    const abort = new AbortController()
    controller.current = abort
    const question = text.trim()
    const next: TrialMessage[] = [...messages, { id: Date.now(), role: "user", content: question, draftKey }]
    setMessages(next)
    setText("")
    setError("")
    setBusy(true)
    try {
      const result = await previewAIEmployee(draft, next.map(({ role, content }) => ({ role, content })), abort.signal)
      if (!abort.signal.aborted) setMessages([...next, { id: Date.now() + 1, role: "assistant", content: result.content, evidence: result, draftKey }])
    } catch (cause) {
      if (!abort.signal.aborted) {
        setMessages(messages)
        setText(question)
        setError(cause instanceof Error ? cause.message : t("aiAgent.loadFailed"))
      }
    } finally {
      if (controller.current === abort) { controller.current = null; setBusy(false) }
    }
  }
  function stop() {
    controller.current?.abort()
    controller.current = null
    setBusy(false)
    const last = messages.at(-1)
    if (last?.role === "user") { setText(last.content); setMessages(messages.slice(0, -1)) }
  }
  return <section className="flex h-full min-h-0 min-w-0 flex-col" aria-label={t("employee.trial")}>
    <header className="flex shrink-0 items-center justify-between gap-2 border-b p-4">
      <div className="min-w-0"><h2 className="text-base font-semibold">{t("employee.trial")}</h2><p className="mt-1 text-xs text-muted-foreground">{t("employee.trialScope")}</p></div>
      <Button variant="ghost" size="icon" title={t("employee.clear")} aria-label={t("employee.clear")} disabled={busy || messages.length === 0} onClick={() => { setMessages([]); setError("") }}><RotateCcwIcon /></Button>
    </header>
    {lastReply && lastReply.draftKey !== draftKey ? <p role="status" className="border-b px-4 py-2 text-xs text-muted-foreground">{t("employee.changed")}</p> : null}
    <div className="min-h-0 flex-1">
      <MessageScrollerProvider autoScroll defaultScrollPosition="end"><MessageScroller><MessageScrollerViewport>
        <MessageScrollerContent className="p-4">
          {messages.length === 0 ? <p className="py-12 text-center text-sm text-muted-foreground">{t("employee.empty")}</p> : messages.map((message) => <MessageScrollerItem key={message.id}>
            <Message align={message.role === "user" ? "end" : "start"}><MessageContent>
              <MessageHeader>{message.role === "user" ? t("employee.customer") : draft.displayName || draft.name || t("employee.title")}</MessageHeader>
              <Bubble variant={message.role === "user" ? "secondary" : "outline"}><BubbleContent className="whitespace-pre-wrap">{message.content}</BubbleContent></Bubble>
              {message.evidence ? <details className="text-xs text-muted-foreground">
                <summary className="cursor-pointer py-2">{t("employee.evidence")} · {message.evidence.modelName}</summary>
                <div className="flex flex-col gap-3 py-2">
                  <p className="font-medium">{t("employee.sources")}</p>
                  {message.evidence.sources.map((source, index) => <details key={`${source.documentId}-${source.chunkId}-${index}`}><summary className="cursor-pointer py-1">{source.title}</summary><p className="whitespace-pre-wrap py-2">{source.content}</p></details>)}
                  {!message.evidence.sources.length ? <p>{t(message.evidence.knowledgeStatus === "not_configured" ? "employee.noKnowledge" : "employee.noMatches")}</p> : null}
                  <p className="font-medium">{t("employee.mounted")}</p>
                  <p>{message.evidence.mountedSkills.map((skill) => skill.name).join(", ") || t("employee.noSkills")}</p>
                </div>
              </details> : null}
            </MessageContent></Message>
          </MessageScrollerItem>)}
          {busy ? <p role="status" className="text-sm text-muted-foreground">{t("employee.waiting")}</p> : null}
        </MessageScrollerContent>
      </MessageScrollerViewport><MessageScrollerButton aria-label={t("employee.latest")}><ArrowDownIcon /></MessageScrollerButton></MessageScroller></MessageScrollerProvider>
    </div>
    <form className="flex shrink-0 flex-col gap-3 border-t p-4" onSubmit={(event) => { event.preventDefault(); void submit() }}>
      {error ? <Alert variant="destructive"><AlertDescription>{error}</AlertDescription></Alert> : null}
      <Field><FieldLabel htmlFor="employee-trial-input">{t("employee.question")}</FieldLabel><Textarea id="employee-trial-input" rows={3} value={text} disabled={busy} placeholder={t("employee.questionPlaceholder")} onChange={(event) => setText(event.target.value)} /></Field>
      <div className="flex flex-wrap items-center justify-between gap-2"><p className="min-w-0 flex-1 text-xs text-muted-foreground">{t("employee.configurationOnly")}</p>
        {busy ? <Button key="stop" type="button" variant="outline" onClick={(event) => { event.preventDefault(); stop() }}><SquareIcon data-icon="inline-start" />{t("employee.stop")}</Button> : <Button key="submit" type="submit" disabled={!text.trim() || !draft.aiConfigId}><SendIcon data-icon="inline-start" />{t("employee.try")}</Button>}
      </div>
    </form>
  </section>
}
