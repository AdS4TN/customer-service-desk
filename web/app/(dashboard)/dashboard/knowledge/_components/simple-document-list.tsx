"use client"

import { FileTextIcon, LoaderCircleIcon, RefreshCwIcon, SearchIcon, Trash2Icon } from "lucide-react"
import { useCallback, useEffect, useState } from "react"
import { toast } from "sonner"

import { useConfirm } from "@/components/confirm-provider"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { ScrollArea } from "@/components/ui/scroll-area"
import { Skeleton } from "@/components/ui/skeleton"
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table"
import { useI18n } from "@/i18n/provider"
import {
  buildKnowledgeDocumentIndex,
  deleteKnowledgeDocument,
  fetchKnowledgeDocuments,
  type KnowledgeDocumentListItem,
} from "@/lib/api/admin"
import { KnowledgeDocumentIndexStatus } from "@/lib/generated/enums"
import { formatDateTime } from "@/lib/utils"

type SimpleDocumentListProps = {
  knowledgeBaseId: number
  reloadKey: number
}

function statusVariant(status: string) {
  if (status === KnowledgeDocumentIndexStatus.Failed) return "destructive" as const
  if (status === KnowledgeDocumentIndexStatus.Indexed) return "secondary" as const
  return "outline" as const
}

export function SimpleDocumentList({ knowledgeBaseId, reloadKey }: SimpleDocumentListProps) {
  const t = useI18n()
  const confirm = useConfirm()
  const [documents, setDocuments] = useState<KnowledgeDocumentListItem[]>([])
  const [query, setQuery] = useState("")
  const [loading, setLoading] = useState(true)
  const [actionId, setActionId] = useState<number | null>(null)

  const loadDocuments = useCallback(async () => {
    setLoading(true)
    try {
      const result = await fetchKnowledgeDocuments({
        knowledgeBaseId,
        title: query.trim() || undefined,
        page: 1,
        limit: 200,
      })
      setDocuments(result.results)
    } catch (error) {
      toast.error(error instanceof Error ? error.message : t("knowledge.loadDocumentsFailed"))
    } finally {
      setLoading(false)
    }
  }, [knowledgeBaseId, query, t])

  useEffect(() => {
    void loadDocuments()
  }, [loadDocuments, reloadKey])

  useEffect(() => {
    if (!documents.some((item) => item.indexStatus === KnowledgeDocumentIndexStatus.Pending)) return
    const timer = window.setInterval(() => void loadDocuments(), 2000)
    return () => window.clearInterval(timer)
  }, [documents, loadDocuments])

  async function rebuild(item: KnowledgeDocumentListItem) {
    setActionId(item.id)
    try {
      await buildKnowledgeDocumentIndex(item.id)
      toast.success(t("knowledge.documentIndexRebuilt", { title: item.title }))
      await loadDocuments()
    } catch (error) {
      toast.error(error instanceof Error ? error.message : t("knowledge.documentIndexRebuildFailed"))
    } finally {
      setActionId(null)
    }
  }

  async function remove(item: KnowledgeDocumentListItem) {
    const confirmed = await confirm({
      title: t("knowledge.deleteDocumentTitle"),
      description: t("knowledge.deleteDocumentConfirm", { title: item.title }),
      confirmText: t("knowledge.delete"),
      variant: "destructive",
    })
    if (!confirmed) return
    setActionId(item.id)
    try {
      await deleteKnowledgeDocument(item.id)
      toast.success(t("knowledge.documentDeleted", { title: item.title }))
      await loadDocuments()
    } catch (error) {
      toast.error(error instanceof Error ? error.message : t("knowledge.documentDeleteFailed"))
    } finally {
      setActionId(null)
    }
  }

  return (
    <div className="flex h-full min-h-0 flex-col">
      <form className="flex shrink-0 items-center gap-2 border-b px-5 py-3" onSubmit={(event) => { event.preventDefault(); void loadDocuments() }}>
        <div className="relative min-w-0 flex-1">
          <SearchIcon className="pointer-events-none absolute top-1/2 left-2.5 size-4 -translate-y-1/2 text-muted-foreground" />
          <Input value={query} onChange={(event) => setQuery(event.target.value)} placeholder={t("knowledge.searchDocumentTitle")} className="pl-8" />
        </div>
        <Button type="submit" variant="outline" disabled={loading}>
          <SearchIcon data-icon="inline-start" />
          {t("knowledge.query")}
        </Button>
        <Button type="button" variant="ghost" size="icon" onClick={() => void loadDocuments()} disabled={loading} aria-label={t("knowledge.refreshDocuments")}>
          <RefreshCwIcon className={loading ? "animate-spin" : undefined} />
        </Button>
      </form>

      <ScrollArea className="min-h-0 flex-1">
        {loading && documents.length === 0 ? (
          <div className="flex flex-col gap-2 p-5">
            <Skeleton className="h-10 w-full" />
            <Skeleton className="h-14 w-full" />
            <Skeleton className="h-14 w-full" />
          </div>
        ) : documents.length === 0 ? (
          <div className="flex h-72 flex-col items-center justify-center gap-2 px-8 text-center">
            <FileTextIcon className="size-8 text-muted-foreground" />
            <p className="text-sm font-medium">{t("knowledge.noIndexedDocuments")}</p>
            <p className="text-sm text-muted-foreground">{t("knowledge.noIndexedDocumentsDescription")}</p>
          </div>
        ) : (
          <div className="p-5">
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>{t("knowledge.documentName")}</TableHead>
                  <TableHead>{t("knowledge.columnStatus")}</TableHead>
                  <TableHead className="text-right">{t("knowledge.columnActions")}</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {documents.map((item) => {
                  const busy = actionId === item.id
                  return (
                    <TableRow key={item.id}>
                      <TableCell className="max-w-0">
                        <div className="flex items-center gap-2">
                          <FileTextIcon className="size-4 shrink-0 text-muted-foreground" />
                          <div className="flex min-w-0 flex-col gap-0.5">
                            <span className="truncate font-medium">{item.title}</span>
                            <span className="truncate text-xs text-muted-foreground">
                              {item.chunkProvider || t("knowledge.useDefaultChunkConfig")} · {formatDateTime(item.updatedAt)}
                            </span>
                          </div>
                        </div>
                      </TableCell>
                      <TableCell><Badge variant={statusVariant(item.indexStatus)}>{item.indexStatusName}</Badge></TableCell>
                      <TableCell>
                        <div className="flex justify-end gap-1">
                          <Button variant="ghost" size="icon-sm" disabled={busy} onClick={() => void rebuild(item)} aria-label={t("knowledge.rebuildDocument", { title: item.title })}>
                            {busy ? <LoaderCircleIcon className="animate-spin" /> : <RefreshCwIcon />}
                          </Button>
                          <Button variant="ghost" size="icon-sm" disabled={busy} onClick={() => void remove(item)} aria-label={t("knowledge.removeDocument", { title: item.title })}>
                            <Trash2Icon />
                          </Button>
                        </div>
                      </TableCell>
                    </TableRow>
                  )
                })}
              </TableBody>
            </Table>
          </div>
        )}
      </ScrollArea>
    </div>
  )
}
