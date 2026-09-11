"use client"

import { DatabaseIcon } from "lucide-react"
import { useRouter } from "next/navigation"
import { useEffect } from "react"
import { toast } from "sonner"

import { Skeleton } from "@/components/ui/skeleton"
import { useI18n } from "@/i18n/provider"
import { createKnowledgeBase, fetchKnowledgeBasesAll } from "@/lib/api/admin"
import { KnowledgeAnswerMode, KnowledgeBaseType, KnowledgeChunkProvider, Status } from "@/lib/generated/enums"

export default function DashboardKnowledgePage() {
  const router = useRouter()
  const t = useI18n()

  useEffect(() => {
    let cancelled = false
    async function openWorkspace() {
      try {
        const knowledgeBases = await fetchKnowledgeBasesAll({ status: Status.Ok })
        let knowledgeBase = knowledgeBases.find((item) => item.knowledgeType === KnowledgeBaseType.Document)
        if (!knowledgeBase) {
          knowledgeBase = await createKnowledgeBase({
            name: t("knowledge.defaultBaseName"),
            description: t("knowledge.defaultBaseDescription"),
            knowledgeType: KnowledgeBaseType.Document,
            defaultTopK: 5,
            defaultScoreThreshold: 0.2,
            defaultRerankLimit: 10,
            chunkProvider: KnowledgeChunkProvider.Structured,
            chunkTargetTokens: 300,
            chunkMaxTokens: 400,
            chunkOverlapTokens: 40,
            parentChunkTokens: 900,
            childChunkTokens: 200,
            answerMode: KnowledgeAnswerMode.Strict,
            remark: "",
          })
        }
        if (!cancelled) router.replace(`/dashboard/knowledge/ingest?id=${knowledgeBase.id}`)
      } catch (error) {
        if (!cancelled) toast.error(error instanceof Error ? error.message : t("knowledge.loadBaseFailed"))
      }
    }
    void openWorkspace()
    return () => {
      cancelled = true
    }
  }, [router, t])

  return (
    <div className="flex h-full flex-col gap-4 p-5">
      <div className="flex items-center gap-3">
        <div className="flex size-9 items-center justify-center rounded-md bg-muted text-muted-foreground">
          <DatabaseIcon className="size-4" />
        </div>
        <div className="flex flex-col gap-1">
          <Skeleton className="h-4 w-28" />
          <Skeleton className="h-3 w-48" />
        </div>
      </div>
      <Skeleton className="min-h-0 flex-1" />
    </div>
  )
}
