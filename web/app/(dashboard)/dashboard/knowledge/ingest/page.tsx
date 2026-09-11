"use client"

import {
  CheckCircle2Icon,
  DatabaseIcon,
  FileTextIcon,
  Globe2Icon,
  LoaderCircleIcon,
  RotateCcwIcon,
  SearchIcon,
  UploadCloudIcon,
  XIcon,
} from "lucide-react"
import { useSearchParams } from "next/navigation"
import {
  useCallback,
  useEffect,
  useMemo,
  useRef,
  useState,
  type DragEvent,
} from "react"
import { toast } from "sonner"

import { OptionCombobox } from "@/components/option-combobox"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import {
  Field,
  FieldContent,
  FieldDescription,
  FieldGroup,
  FieldLabel,
} from "@/components/ui/field"
import { Input } from "@/components/ui/input"
import { ScrollArea } from "@/components/ui/scroll-area"
import { Separator } from "@/components/ui/separator"
import { Skeleton } from "@/components/ui/skeleton"
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs"
import { Textarea } from "@/components/ui/textarea"
import { useI18n } from "@/i18n/provider"
import {
  debugKnowledgeSearch,
  crawlKnowledgeWebsite,
  fetchKnowledgeBase,
  fetchKnowledgeDocument,
  fetchKnowledgeIngestionJob,
  previewKnowledgeDocumentChunks,
  retryKnowledgeIngestionJob,
  uploadKnowledgeDocument,
  type KnowledgeBase,
  type KnowledgeDocumentChunkPreview,
  type KnowledgeIngestionJob,
  type KnowledgeSearchResponse,
} from "@/lib/api/admin"
import { KnowledgeChunkProvider } from "@/lib/generated/enums"
import { cn } from "@/lib/utils"
import { SimpleDocumentList } from "../_components/simple-document-list"

const ACCEPTED_FILE_TYPES = [
  ".txt",
  ".md",
  ".markdown",
  ".html",
  ".htm",
  ".pdf",
  ".doc",
  ".docx",
  ".ppt",
  ".pptx",
  ".png",
  ".jpg",
  ".jpeg",
]
type ImportStatus =
  | "uploading"
  | "queued"
  | "parsing"
  | "chunking"
  | "embedding"
  | "indexing"
  | "indexed"
  | "failed"

type ImportItem = {
  id: string
  file: File
  title: string
  status: ImportStatus
  jobId?: number
  progress?: number
  parser?: string
  contentType?: "markdown" | "html"
  content?: string
  preview?: KnowledgeDocumentChunkPreview
  documentId?: number
  error?: string
  parseMs?: number
  chunkMs?: number
  embeddingMs?: number
  indexMs?: number
  sourceType?: "file" | "website"
}

type TFunction = (key: string, values?: Record<string, string | number>) => string

function getExtension(filename: string) {
  const index = filename.lastIndexOf(".")
  return index >= 0 ? filename.slice(index).toLowerCase() : ""
}

function titleFromFilename(filename: string) {
  const extension = getExtension(filename)
  return (extension ? filename.slice(0, -extension.length) : filename).trim() || filename
}

function formatBytes(bytes: number) {
  if (bytes < 1024) return `${bytes} B`
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`
  return `${(bytes / 1024 / 1024).toFixed(1)} MB`
}

function formatDuration(milliseconds?: number) {
  if (milliseconds === undefined) return ""
  if (milliseconds < 1000) return `${milliseconds} ms`
  return `${(milliseconds / 1000).toFixed(1)} s`
}

function parserLabel(parser: string | undefined, t: TFunction) {
  switch (parser) {
    case "browser":
    case "plain-text":
      return t("knowledge.parserBrowser")
    case "pymupdf":
      return t("knowledge.parserPyMuPDF")
    case "docx":
      return t("knowledge.parserDocx")
    case "mineru-txt":
      return t("knowledge.parserMineruText")
    case "mineru-ocr":
      return t("knowledge.parserMineruOCR")
    case "mineru-auto":
    case "mineru":
      return t("knowledge.parserMineruAuto")
    case "website":
      return t("knowledge.parserWebsite")
    default:
      return parser || "-"
  }
}

function statusLabel(item: ImportItem, t: TFunction) {
  switch (item.status) {
    case "uploading":
      return t("knowledge.importUploading")
    case "queued":
      return t("knowledge.importWaiting")
    case "parsing":
      return t("knowledge.importParsing")
    case "chunking":
      return t("knowledge.importChunking")
    case "embedding":
      return t("knowledge.importEmbedding")
    case "indexing":
      return t("knowledge.importIndexing")
    case "indexed":
      return t("knowledge.importIndexed")
    case "failed":
      return t("knowledge.importFailed")
  }
}

function statusVariant(item: ImportItem) {
  if (item.status === "failed") return "destructive" as const
  if (item.status === "indexed") return "secondary" as const
  return "outline" as const
}

function statusFromJob(job: KnowledgeIngestionJob): ImportStatus {
  if (job.status === "completed") return "indexed"
  if (job.status === "failed") return "failed"
  if (job.stage === "parsing" || job.stage === "chunking" || job.stage === "embedding" || job.stage === "indexing") {
    return job.stage
  }
  return "queued"
}

export default function KnowledgeIngestionPage() {
  const t = useI18n()
  const searchParams = useSearchParams()
  const knowledgeBaseId = Number(searchParams.get("id"))
  const fileInputRef = useRef<HTMLInputElement>(null)
  const [knowledgeBase, setKnowledgeBase] = useState<KnowledgeBase | null>(null)
  const [loading, setLoading] = useState(true)
  const [dragging, setDragging] = useState(false)
  const [items, setItems] = useState<ImportItem[]>([])
  const [importSource, setImportSource] = useState("file")
  const [websiteURL, setWebsiteURL] = useState("")
  const [websiteMaxPages, setWebsiteMaxPages] = useState("30")
  const [websiteMaxDepth, setWebsiteMaxDepth] = useState("2")
  const [websiteSubmitting, setWebsiteSubmitting] = useState(false)
  const itemsRef = useRef<ImportItem[]>([])
  const [selectedItemId, setSelectedItemId] = useState<string | null>(null)
  const [question, setQuestion] = useState("")
  const [topK, setTopK] = useState("5")
  const [scoreThreshold, setScoreThreshold] = useState("0.2")
  const [rerankLimit, setRerankLimit] = useState("5")
  const [searching, setSearching] = useState(false)
  const [searchResult, setSearchResult] = useState<KnowledgeSearchResponse | null>(null)
  const [documentsVersion, setDocumentsVersion] = useState(0)
  const overrideChunkConfig = true
  const [chunkProvider, setChunkProvider] = useState<string>(KnowledgeChunkProvider.Structured)
  const [chunkTargetTokens, setChunkTargetTokens] = useState("300")
  const [chunkMaxTokens, setChunkMaxTokens] = useState("400")
  const [chunkOverlapTokens, setChunkOverlapTokens] = useState("40")
  const [parentChunkTokens, setParentChunkTokens] = useState("900")
  const [childChunkTokens, setChildChunkTokens] = useState("200")

  const updateItem = useCallback((id: string, patch: Partial<ImportItem>) => {
    setItems((current) =>
      current.map((item) => (item.id === id ? { ...item, ...patch } : item)),
    )
  }, [])

  useEffect(() => {
    itemsRef.current = items
  }, [items])

  useEffect(() => {
    if (!Number.isInteger(knowledgeBaseId) || knowledgeBaseId <= 0) {
      setLoading(false)
      return
    }
    let cancelled = false
    setLoading(true)
    fetchKnowledgeBase(knowledgeBaseId)
      .then((base) => {
        if (cancelled) return
        setKnowledgeBase(base)
        setTopK(String(base.defaultTopK || 5))
        setScoreThreshold(String(base.defaultScoreThreshold || 0.2))
        setRerankLimit(String(base.defaultRerankLimit || 5))
        setChunkProvider(base.chunkProvider === "semantic"
          ? KnowledgeChunkProvider.Structured
          : base.chunkProvider === "fixed"
            ? KnowledgeChunkProvider.Recursive
            : base.chunkProvider || KnowledgeChunkProvider.Structured)
        setChunkTargetTokens(String(base.chunkTargetTokens || 300))
        setChunkMaxTokens(String(base.chunkMaxTokens || 400))
        setChunkOverlapTokens(String(base.chunkOverlapTokens ?? 40))
        setParentChunkTokens(String(base.parentChunkTokens || 900))
        setChildChunkTokens(String(base.childChunkTokens || 200))
      })
      .catch((error) => {
        if (cancelled) return
        toast.error(error instanceof Error ? error.message : t("knowledge.loadBaseFailed"))
      })
      .finally(() => {
        if (!cancelled) setLoading(false)
      })
    return () => {
      cancelled = true
    }
  }, [knowledgeBaseId, t])

  const uploadItem = useCallback(async (item: ImportItem) => {
    if (!ACCEPTED_FILE_TYPES.includes(getExtension(item.file.name))) {
      updateItem(item.id, { status: "failed", error: t("knowledge.unsupportedFileType") })
      return
    }
    updateItem(item.id, { status: "uploading", error: undefined })
    try {
      const job = await uploadKnowledgeDocument({
        file: item.file,
        knowledgeBaseId,
        directoryId: 0,
        title: item.title,
        chunkConfigOverride: overrideChunkConfig,
        chunkProvider,
        chunkTargetTokens: Number(chunkTargetTokens),
        chunkMaxTokens: Number(chunkMaxTokens),
        chunkOverlapTokens: Number(chunkOverlapTokens),
        parentChunkTokens: Number(parentChunkTokens),
        childChunkTokens: Number(childChunkTokens),
      })
      updateItem(item.id, {
        jobId: job.id,
        documentId: job.documentId,
        status: statusFromJob(job),
        progress: job.progress,
      })
    } catch (error) {
      updateItem(item.id, {
        status: "failed",
        error: error instanceof Error ? error.message : t("knowledge.documentUploadFailed"),
      })
    }
  }, [childChunkTokens, chunkMaxTokens, chunkOverlapTokens, chunkProvider, chunkTargetTokens, knowledgeBaseId, overrideChunkConfig, parentChunkTokens, t, updateItem])

  const addFiles = useCallback(
    async (files: File[]) => {
      if (!files.length) return
      const existing = new Set(
        items.map((item) => `${item.file.name}:${item.file.size}:${item.file.lastModified}`),
      )
      const nextItems = files
        .filter((file) => {
          const fingerprint = `${file.name}:${file.size}:${file.lastModified}`
          if (existing.has(fingerprint)) return false
          existing.add(fingerprint)
          return true
        })
        .map<ImportItem>((file) => ({
          id: crypto.randomUUID(),
          file,
          title: titleFromFilename(file.name),
          status: "uploading",
        }))

      if (!nextItems.length) {
        toast(t("knowledge.filesAlreadyQueued"))
        return
      }
      setItems((current) => [...current, ...nextItems])
      setSelectedItemId((current) => current ?? nextItems[0].id)
      await Promise.all(nextItems.map((item) => uploadItem(item)))
    },
    [items, t, uploadItem],
  )

  async function addWebsite() {
    const value = websiteURL.trim()
    if (!/^https?:\/\//i.test(value)) {
      toast.error(t("knowledge.websiteURLInvalid"))
      return
    }
    setWebsiteSubmitting(true)
    try {
      const job = await crawlKnowledgeWebsite({
        url: value,
        maxPages: Math.max(1, Number(websiteMaxPages) || 30),
        maxDepth: Math.max(1, Number(websiteMaxDepth) || 2),
        knowledgeBaseId,
        directoryId: 0,
        chunkConfigOverride: overrideChunkConfig,
        chunkProvider,
        chunkTargetTokens: Number(chunkTargetTokens),
        chunkMaxTokens: Number(chunkMaxTokens),
        chunkOverlapTokens: Number(chunkOverlapTokens),
        parentChunkTokens: Number(parentChunkTokens),
        childChunkTokens: Number(childChunkTokens),
      })
      const title = new URL(value).hostname.replace(/^www\./, "")
      const item: ImportItem = {
        id: crypto.randomUUID(),
        file: new File([], `${title}.website`),
        title,
        status: statusFromJob(job),
        jobId: job.id,
        documentId: job.documentId,
        progress: job.progress,
        sourceType: "website",
      }
      setItems((current) => [...current, item])
      setSelectedItemId(item.id)
      setWebsiteURL("")
      toast.success(t("knowledge.websiteQueued"))
    } catch (error) {
      toast.error(error instanceof Error ? error.message : t("knowledge.websiteQueueFailed"))
    } finally {
      setWebsiteSubmitting(false)
    }
  }

  const activeJobKey = items
    .filter((item) => item.jobId && item.status !== "indexed" && item.status !== "failed")
    .map((item) => item.jobId)
    .sort((a, b) => Number(a) - Number(b))
    .join(",")

  useEffect(() => {
    if (!activeJobKey) return
    const activeItems = itemsRef.current.filter((item) => item.jobId && activeJobKey.split(",").includes(String(item.jobId)))
    let cancelled = false
    let polling = false
    const poll = async () => {
      if (polling || cancelled) return
      polling = true
      await Promise.all(activeItems.map(async (item) => {
        try {
          const job = await fetchKnowledgeIngestionJob(item.jobId as number)
          if (cancelled) return
          const status = statusFromJob(job)
          updateItem(item.id, {
            status,
            progress: job.progress,
            parser: job.parser,
            documentId: job.documentId,
            error: job.error || undefined,
            parseMs: job.parseMs,
            chunkMs: job.chunkMs,
            embeddingMs: job.embeddingMs,
            indexMs: job.indexMs,
          })
          if (status === "indexed") {
            setDocumentsVersion((current) => current + 1)
            const document = await fetchKnowledgeDocument(job.documentId)
            const preview = await previewKnowledgeDocumentChunks({
              knowledgeBaseId: document.knowledgeBaseId,
              title: document.title,
              contentType: document.contentType,
              content: document.content,
              chunkConfigOverride: document.chunkConfigOverride,
              chunkProvider: document.chunkProvider,
              chunkTargetTokens: document.chunkTargetTokens,
              chunkMaxTokens: document.chunkMaxTokens,
              chunkOverlapTokens: document.chunkOverlapTokens,
              parentChunkTokens: document.parentChunkTokens,
              childChunkTokens: document.childChunkTokens,
            })
            if (!cancelled) {
              updateItem(item.id, {
                content: document.content,
                contentType: document.contentType === "html" ? "html" : "markdown",
                preview,
              })
            }
          }
        } catch {
          // A transient polling failure should not turn a running server job into a failed job.
        }
      }))
      polling = false
    }
    void poll()
    const timer = window.setInterval(() => void poll(), 1200)
    return () => {
      cancelled = true
      window.clearInterval(timer)
    }
  }, [activeJobKey, knowledgeBaseId, updateItem])

  function removeItem(id: string) {
    setItems((current) => current.filter((item) => item.id !== id))
    setSelectedItemId((current) => {
      if (current !== id) return current
      return items.find((item) => item.id !== id)?.id ?? null
    })
  }

  async function retryItem(item: ImportItem) {
    if (!item.jobId) {
      await uploadItem(item)
      return
    }
    updateItem(item.id, { status: "queued", progress: 0, error: undefined })
    try {
      const job = await retryKnowledgeIngestionJob(item.jobId)
      updateItem(item.id, { status: statusFromJob(job), progress: job.progress })
    } catch (error) {
      updateItem(item.id, {
        status: "failed",
        error: error instanceof Error ? error.message : t("knowledge.importRetryFailed"),
      })
    }
  }

  async function runRetrievalTest() {
    if (!question.trim()) {
      toast.error(t("knowledge.enterRetrievalQuestion"))
      return
    }
    setSearching(true)
    setSearchResult(null)
    try {
      const result = await debugKnowledgeSearch({
        knowledgeBaseIds: [knowledgeBaseId],
        question: question.trim(),
        topK: Math.max(1, Number(topK) || 5),
        scoreThreshold: Math.max(0, Number(scoreThreshold) || 0),
        rerankLimit: Math.max(0, Number(rerankLimit) || 0),
      })
      setSearchResult(result)
    } catch (error) {
      toast.error(error instanceof Error ? error.message : t("knowledge.retrievalTestFailed"))
    } finally {
      setSearching(false)
    }
  }

  const chunkProviderOptions = useMemo(() => [
    { value: KnowledgeChunkProvider.Structured, label: t("knowledge.chunkStructured") },
    { value: KnowledgeChunkProvider.Recursive, label: t("knowledge.chunkRecursive") },
    { value: KnowledgeChunkProvider.ParentChild, label: t("knowledge.chunkParentChild") },
  ], [t])

  const selectedItem = items.find((item) => item.id === selectedItemId) ?? null
  const processingCount = items.filter((item) => !["indexed", "failed"].includes(item.status)).length
  const indexedCount = items.filter((item) => item.status === "indexed").length
  const failedCount = items.filter((item) => item.status === "failed").length

  if (loading) {
    return (
      <div className="flex h-full flex-col gap-4 p-5">
        <Skeleton className="h-9 w-72" />
        <div className="grid min-h-0 flex-1 grid-cols-[340px_minmax(0,1fr)] gap-4">
          <Skeleton className="h-full" />
          <Skeleton className="h-full" />
        </div>
      </div>
    )
  }

  if (!knowledgeBase || knowledgeBase.knowledgeType !== "document") {
    return (
      <div className="flex h-full flex-col items-center justify-center gap-4 p-6 text-center">
        <DatabaseIcon className="size-10 text-muted-foreground" />
        <div className="flex flex-col gap-1">
          <p className="text-sm font-medium">{t("knowledge.workspaceUnavailable")}</p>
          <p className="text-sm text-muted-foreground">{t("knowledge.workspaceUnavailableDescription")}</p>
        </div>
      </div>
    )
  }

  return (
    <div className="flex h-full min-h-0 flex-col overflow-hidden bg-background">
      <header className="flex shrink-0 items-center gap-3 border-b px-4 py-2.5">
        <div className="min-w-0 flex-1">
          <h1 className="truncate text-sm font-semibold">{t("knowledge.workspaceTitle")}</h1>
          <p className="truncate text-xs text-muted-foreground">{t("knowledge.workspaceDescription")}</p>
        </div>
        <Badge variant="secondary">{t("knowledge.vectorServiceConnected")}</Badge>
        <Badge variant="outline">BGE-M3</Badge>
      </header>

      <main className="grid min-h-0 flex-1 grid-cols-[340px_minmax(0,1fr)]">
        <aside className="flex min-h-0 flex-col border-r">
          <div className="flex flex-col gap-4 p-4">
            <Tabs value={importSource} onValueChange={setImportSource}>
              <TabsList className="w-full">
                <TabsTrigger value="file">{t("knowledge.importFiles")}</TabsTrigger>
                <TabsTrigger value="website">{t("knowledge.importWebsite")}</TabsTrigger>
              </TabsList>
              <TabsContent value="file">
            <div
              className={cn(
                "flex min-h-32 flex-col items-center justify-center gap-2 rounded-lg border border-dashed px-5 py-4 text-center transition-colors",
                dragging && "border-primary bg-primary/5",
              )}
              onDragEnter={(event) => {
                event.preventDefault()
                setDragging(true)
              }}
              onDragOver={(event) => event.preventDefault()}
              onDragLeave={() => setDragging(false)}
              onDrop={(event: DragEvent<HTMLDivElement>) => {
                event.preventDefault()
                setDragging(false)
                void addFiles(Array.from(event.dataTransfer.files))
              }}
            >
              <UploadCloudIcon className="size-7 text-muted-foreground" />
              <div className="flex flex-col gap-0.5">
                <p className="text-sm font-medium">{t("knowledge.dropFiles")}</p>
                <p className="text-xs text-muted-foreground">{t("knowledge.supportedImportTypes")}</p>
              </div>
              <Button variant="outline" size="sm" onClick={() => fileInputRef.current?.click()}>
                <UploadCloudIcon data-icon="inline-start" />
                {t("knowledge.chooseFiles")}
              </Button>
              <input
                ref={fileInputRef}
                type="file"
                className="hidden"
                multiple
                accept={ACCEPTED_FILE_TYPES.join(",")}
                onChange={(event) => {
                  void addFiles(Array.from(event.target.files ?? []))
                  event.target.value = ""
                }}
              />
            </div>
              </TabsContent>
              <TabsContent value="website">
                <FieldGroup>
                  <Field>
                    <FieldLabel htmlFor="website-url">{t("knowledge.websiteURL")}</FieldLabel>
                    <Input id="website-url" type="url" value={websiteURL} onChange={(event) => setWebsiteURL(event.target.value)} placeholder="https://example.com" />
                    <FieldDescription>{t("knowledge.websiteURLDescription")}</FieldDescription>
                  </Field>
                  <div className="grid grid-cols-2 gap-2">
                    <Field>
                      <FieldLabel htmlFor="website-max-pages">{t("knowledge.websiteMaxPages")}</FieldLabel>
                      <Input id="website-max-pages" type="number" min="1" max="200" value={websiteMaxPages} onChange={(event) => setWebsiteMaxPages(event.target.value)} />
                    </Field>
                    <Field>
                      <FieldLabel htmlFor="website-max-depth">{t("knowledge.websiteMaxDepth")}</FieldLabel>
                      <Input id="website-max-depth" type="number" min="1" max="5" value={websiteMaxDepth} onChange={(event) => setWebsiteMaxDepth(event.target.value)} />
                    </Field>
                  </div>
                  <Button onClick={() => void addWebsite()} disabled={websiteSubmitting || !websiteURL.trim()}>
                    {websiteSubmitting ? <LoaderCircleIcon data-icon="inline-start" className="animate-spin" /> : <Globe2Icon data-icon="inline-start" />}
                    {websiteSubmitting ? t("knowledge.websiteQueuing") : t("knowledge.startWebsiteImport")}
                  </Button>
                </FieldGroup>
              </TabsContent>
            </Tabs>

            <FieldGroup>
              <Field>
                <FieldLabel>{t("knowledge.chunkProvider")}</FieldLabel>
                <OptionCombobox
                  value={chunkProvider}
                  onChange={(value) => setChunkProvider(value ?? KnowledgeChunkProvider.Structured)}
                  options={chunkProviderOptions}
                  placeholder={t("knowledge.selectChunkProvider")}
                  searchPlaceholder={t("knowledge.searchChunkProvider")}
                  emptyText={t("knowledge.emptyChunkProvider")}
                />
                <FieldDescription>{t(`knowledge.chunkDescription.${chunkProvider}`)}</FieldDescription>
              </Field>
              <div className="grid grid-cols-3 gap-2">
                <Field>
                  <FieldLabel htmlFor="import-target-tokens">{t("knowledge.targetToken")}</FieldLabel>
                  <Input id="import-target-tokens" type="number" min="1" value={chunkTargetTokens} onChange={(event) => setChunkTargetTokens(event.target.value)} />
                </Field>
                <Field>
                  <FieldLabel htmlFor="import-max-tokens">{t("knowledge.maxToken")}</FieldLabel>
                  <Input id="import-max-tokens" type="number" min="1" value={chunkMaxTokens} onChange={(event) => setChunkMaxTokens(event.target.value)} />
                </Field>
                <Field>
                  <FieldLabel htmlFor="import-overlap-tokens">{t("knowledge.overlapToken")}</FieldLabel>
                  <Input id="import-overlap-tokens" type="number" min="0" value={chunkOverlapTokens} onChange={(event) => setChunkOverlapTokens(event.target.value)} />
                </Field>
              </div>
              {chunkProvider === KnowledgeChunkProvider.ParentChild ? (
                <div className="grid grid-cols-2 gap-2">
                  <Field>
                    <FieldLabel htmlFor="import-parent-tokens">{t("knowledge.parentChunkToken")}</FieldLabel>
                    <Input id="import-parent-tokens" type="number" min="1" value={parentChunkTokens} onChange={(event) => setParentChunkTokens(event.target.value)} />
                  </Field>
                  <Field>
                    <FieldLabel htmlFor="import-child-tokens">{t("knowledge.childChunkToken")}</FieldLabel>
                    <Input id="import-child-tokens" type="number" min="1" value={childChunkTokens} onChange={(event) => setChildChunkTokens(event.target.value)} />
                  </Field>
                </div>
              ) : null}
              <FieldDescription>{t("knowledge.chunkSettingsApplyOnUpload")}</FieldDescription>
            </FieldGroup>
          </div>

          <Separator />

          <div className="flex items-center justify-between gap-3 px-4 py-3">
            <div className="flex items-center gap-2">
              <p className="text-sm font-medium">{t("knowledge.importQueue")}</p>
              <Badge variant="secondary">{items.length}</Badge>
            </div>
            {indexedCount > 0 ? (
              <Button
                variant="ghost"
                size="sm"
                onClick={() => {
                  const remaining = items.filter((item) => item.status !== "indexed")
                  setItems(remaining)
                  setSelectedItemId((current) =>
                    remaining.some((item) => item.id === current)
                      ? current
                      : remaining[0]?.id ?? null,
                  )
                }}
              >
                {t("knowledge.clearCompleted")}
              </Button>
            ) : null}
          </div>

          <ScrollArea className="min-h-0 flex-1">
            {items.length ? (
              <div className="flex flex-col px-2 pb-2">
                {items.map((item) => {
                  const busy = !["indexed", "failed"].includes(item.status)
                  return (
                    <div
                      key={item.id}
                      className={cn(
                        "group flex items-start gap-1 rounded-md px-2 py-2",
                        selectedItemId === item.id && "bg-muted",
                      )}
                    >
                      <button
                        type="button"
                        className="flex min-w-0 flex-1 items-start gap-2 text-left"
                        onClick={() => setSelectedItemId(item.id)}
                      >
                        {busy ? (
                          <LoaderCircleIcon className="mt-0.5 size-4 shrink-0 animate-spin text-muted-foreground" />
                        ) : item.status === "indexed" ? (
                          <CheckCircle2Icon className="mt-0.5 size-4 shrink-0 text-muted-foreground" />
                        ) : (
                          <FileTextIcon className="mt-0.5 size-4 shrink-0 text-muted-foreground" />
                        )}
                        <span className="flex min-w-0 flex-1 flex-col gap-1">
                          <span className="truncate text-sm font-medium">{item.title}</span>
                          <span className="flex flex-wrap items-center gap-1.5 text-xs text-muted-foreground">
                            <span>{formatBytes(item.file.size)}</span>
                            <Badge variant={statusVariant(item)}>{statusLabel(item, t)}</Badge>
                            {item.progress !== undefined && busy ? <span>{item.progress}%</span> : null}
                            {item.preview ? <span>{t("knowledge.chunkCount", { count: item.preview.chunkCount })}</span> : null}
                          </span>
                          {item.error ? (
                            <span className="line-clamp-2 text-xs text-destructive">{item.error}</span>
                          ) : null}
                        </span>
                      </button>
                      {item.status === "failed" ? (
                        <Button
                          variant="ghost"
                          size="icon-sm"
                          onClick={() => void retryItem(item)}
                          aria-label={t("knowledge.retryDocument", { title: item.title })}
                        >
                          <RotateCcwIcon />
                        </Button>
                      ) : null}
                      <Button
                        variant="ghost"
                        size="icon-sm"
                        disabled={busy}
                        onClick={() => removeItem(item.id)}
                        aria-label={t("knowledge.removeDocument", { title: item.title })}
                      >
                        <XIcon />
                      </Button>
                    </div>
                  )
                })}
              </div>
            ) : (
              <div className="flex h-40 flex-col items-center justify-center gap-2 px-6 text-center">
                <FileTextIcon className="size-6 text-muted-foreground" />
                <p className="text-sm text-muted-foreground">{t("knowledge.importQueueEmpty")}</p>
              </div>
            )}
          </ScrollArea>

          <Separator />
          <div className="flex flex-col gap-3 p-4">
            <div className="flex items-center justify-between text-xs text-muted-foreground">
              <span>{t("knowledge.processingCount", { count: processingCount })}</span>
              <span>{t("knowledge.completedCount", { count: indexedCount })}</span>
              <span>{t("knowledge.failedCount", { count: failedCount })}</span>
            </div>
            <p className="text-xs leading-5 text-muted-foreground">{t("knowledge.backgroundIngestionDescription")}</p>
          </div>
        </aside>

        <section className="min-h-0 min-w-0">
          <Tabs defaultValue="documents" className="h-full min-h-0 gap-0">
            <div className="flex shrink-0 items-center border-b px-4 py-2.5">
              <TabsList variant="line">
                <TabsTrigger value="documents">{t("knowledge.indexedDocuments")}</TabsTrigger>
                <TabsTrigger value="chunks">{t("knowledge.chunkPreview")}</TabsTrigger>
                <TabsTrigger value="source">{t("knowledge.parsedSource")}</TabsTrigger>
                <TabsTrigger value="retrieve">{t("knowledge.retrievalTest")}</TabsTrigger>
              </TabsList>
            </div>

            <TabsContent value="documents" className="min-h-0">
              <SimpleDocumentList knowledgeBaseId={knowledgeBaseId} reloadKey={documentsVersion} />
            </TabsContent>

            <TabsContent value="chunks" className="min-h-0">
              {selectedItem?.preview ? (
                <div className="flex h-full min-h-0 flex-col">
                  <div className="flex shrink-0 items-center gap-2 border-b px-5 py-3">
                    <Badge variant="secondary">{t("knowledge.chunkCount", { count: selectedItem.preview.chunkCount })}</Badge>
                    <Badge variant="outline">{t("knowledge.actualChunkProvider", { provider: selectedItem.preview.provider })}</Badge>
                    <span className="text-xs text-muted-foreground">
                      {t("knowledge.chunkMetrics", {
                        target: selectedItem.preview.targetTokens,
                        max: selectedItem.preview.maxTokens,
                        overlap: selectedItem.preview.overlapTokens,
                      })}
                    </span>
                    <span className="ml-auto text-xs text-muted-foreground">
                      {parserLabel(selectedItem.parser, t)}
                    </span>
                  </div>
                  <div className="grid shrink-0 grid-cols-4 border-b bg-muted/30">
                    {[
                      [t("knowledge.importParsing"), selectedItem.parseMs],
                      [t("knowledge.importChunking"), selectedItem.chunkMs],
                      [t("knowledge.importEmbedding"), selectedItem.embeddingMs],
                      [t("knowledge.importIndexing"), selectedItem.indexMs],
                    ].map(([label, duration]) => (
                      <div key={String(label)} className="border-r px-4 py-2 last:border-r-0">
                        <p className="text-xs text-muted-foreground">{label}</p>
                        <p className="text-sm font-medium tabular-nums">{formatDuration(duration as number | undefined) || "-"}</p>
                      </div>
                    ))}
                  </div>
                  <ScrollArea className="min-h-0 flex-1">
                    <div className="mx-auto flex max-w-4xl flex-col px-6 py-4">
                      {selectedItem.preview.chunks.map((chunk) => (
                        <article key={chunk.chunkNo} className="flex flex-col gap-2 border-b py-4 last:border-b-0">
                          <div className="flex items-center gap-2">
                            <Badge variant="outline">#{chunk.chunkNo + 1}</Badge>
                            <Badge variant="secondary">{chunk.chunkType}</Badge>
                            <span className="min-w-0 flex-1 truncate text-xs text-muted-foreground">
                              {chunk.sectionPath || chunk.title || selectedItem.title}
                            </span>
                            <span className="text-xs text-muted-foreground">
                              {t("knowledge.chunkSize", { tokens: chunk.tokenCount, chars: chunk.charCount })}
                            </span>
                          </div>
                          {chunk.contextContent && chunk.contextContent !== chunk.content ? (
                            <div className="flex flex-col gap-3">
                              <div>
                                <p className="mb-1 text-xs font-medium text-muted-foreground">{t("knowledge.matchedChildChunk")}</p>
                                <p className="whitespace-pre-wrap text-sm leading-6">{chunk.content}</p>
                              </div>
                              <div>
                                <p className="mb-1 text-xs font-medium text-muted-foreground">{t("knowledge.modelContextChunk")}</p>
                                <p className="whitespace-pre-wrap text-sm leading-6 text-muted-foreground">{chunk.contextContent}</p>
                              </div>
                            </div>
                          ) : (
                            <p className="whitespace-pre-wrap text-sm leading-6">{chunk.content}</p>
                          )}
                        </article>
                      ))}
                    </div>
                  </ScrollArea>
                </div>
              ) : selectedItem && !["indexed", "failed"].includes(selectedItem.status) ? (
                <div className="flex h-full flex-col items-center justify-center gap-4 px-8 text-center">
                  <LoaderCircleIcon className="size-8 animate-spin text-muted-foreground" />
                  <div className="flex flex-col gap-1">
                    <p className="text-sm font-medium">{statusLabel(selectedItem, t)}</p>
                    <p className="text-sm text-muted-foreground">{t("knowledge.backgroundProcessing")}</p>
                  </div>
                  <div className="h-1.5 w-72 overflow-hidden rounded-full bg-muted">
                    <div className="h-full bg-primary transition-[width]" style={{ width: `${selectedItem.progress ?? 0}%` }} />
                  </div>
                  <span className="text-xs tabular-nums text-muted-foreground">{selectedItem.progress ?? 0}%</span>
                </div>
              ) : (
                <div className="flex h-full flex-col items-center justify-center gap-2 px-8 text-center">
                  <FileTextIcon className="size-8 text-muted-foreground" />
                  <p className="text-sm font-medium">{t("knowledge.selectParsedDocument")}</p>
                  <p className="max-w-md text-sm text-muted-foreground">{t("knowledge.chunkPreviewDescription")}</p>
                </div>
              )}
            </TabsContent>

            <TabsContent value="source" className="min-h-0 p-5">
              {selectedItem?.content ? (
                <Textarea
                  value={selectedItem.content}
                  readOnly
                  aria-label={t("knowledge.parsedSource")}
                  className="h-full min-h-0 resize-none font-mono text-xs leading-5"
                />
              ) : (
                <div className="flex h-full items-center justify-center text-sm text-muted-foreground">{t("knowledge.noParsedSource")}</div>
              )}
            </TabsContent>

            <TabsContent value="retrieve" className="min-h-0">
              <div className="flex h-full min-h-0 flex-col">
                <form
                  className="flex shrink-0 flex-col gap-4 border-b px-5 py-4"
                  onSubmit={(event) => {
                    event.preventDefault()
                    void runRetrievalTest()
                  }}
                >
                  <div className="flex flex-col gap-1">
                    <h2 className="text-sm font-semibold">{t("knowledge.retrievalTest")}</h2>
                    <p className="text-xs leading-5 text-muted-foreground">{t("knowledge.retrievalTestDescription")}</p>
                  </div>
                  <div className="grid gap-3">
                    <Field className="min-w-0">
                      <FieldLabel htmlFor="retrieval-question">{t("knowledge.retrievalQuestion")}</FieldLabel>
                      <FieldContent>
                        <Input
                          id="retrieval-question"
                          value={question}
                          onChange={(event) => setQuestion(event.target.value)}
                          placeholder={t("knowledge.retrievalQuestionPlaceholder")}
                        />
                      </FieldContent>
                    </Field>
                    <div className="grid grid-cols-[repeat(3,minmax(0,1fr))_auto] items-end gap-3">
                      <Field className="min-w-0">
                        <FieldLabel htmlFor="retrieval-top-k">Top K</FieldLabel>
                        <Input
                          id="retrieval-top-k"
                          type="number"
                          min={1}
                          max={50}
                          value={topK}
                          onChange={(event) => setTopK(event.target.value)}
                        />
                      </Field>
                      <Field className="min-w-0">
                        <FieldLabel htmlFor="retrieval-threshold">{t("knowledge.defaultScoreThreshold")}</FieldLabel>
                        <Input
                          id="retrieval-threshold"
                          type="number"
                          min={0}
                          max={1}
                          step={0.01}
                          value={scoreThreshold}
                          onChange={(event) => setScoreThreshold(event.target.value)}
                        />
                      </Field>
                      <Field className="min-w-0">
                        <FieldLabel htmlFor="retrieval-rerank">{t("knowledge.rerankLimit")}</FieldLabel>
                        <Input
                          id="retrieval-rerank"
                          type="number"
                          min={0}
                          max={50}
                          value={rerankLimit}
                          onChange={(event) => setRerankLimit(event.target.value)}
                        />
                      </Field>
                      <Button type="submit" disabled={searching}>
                        {searching ? (
                          <LoaderCircleIcon data-icon="inline-start" className="animate-spin" />
                        ) : (
                          <SearchIcon data-icon="inline-start" />
                        )}
                        {searching ? t("knowledge.retrieving") : t("knowledge.runRetrievalTest")}
                      </Button>
                    </div>
                  </div>
                </form>

                <div className="flex min-h-0 flex-1 flex-col">
                  <div className="flex shrink-0 items-center justify-between gap-3 border-b px-5 py-3">
                    <p className="text-sm font-medium">{t("knowledge.retrievalResults")}</p>
                    {searchResult ? (
                      <div className="flex items-center gap-2 text-xs text-muted-foreground">
                        <Badge variant="secondary">{t("knowledge.resultCount", { count: searchResult.hitCount })}</Badge>
                        <span>{searchResult.latencyMs} ms</span>
                      </div>
                    ) : null}
                  </div>
                  <ScrollArea className="min-h-0 flex-1">
                    {searchResult ? (
                      searchResult.results.length ? (
                        <div className="mx-auto flex max-w-4xl flex-col px-6 py-3">
                          {searchResult.results.map((result, index) => (
                            <article key={`${result.chunkId}-${index}`} className="flex flex-col gap-2 border-b py-4 last:border-b-0">
                              <div className="flex items-center gap-2">
                                <Badge variant="outline">#{index + 1}</Badge>
                                <span className="min-w-0 flex-1 truncate text-sm font-medium">
                                  {result.documentTitle || result.title || t("knowledge.documentFallbackName", { id: result.documentId })}
                                </span>
                                <Badge variant="secondary">{result.score.toFixed(4)}</Badge>
                              </div>
                              <p className="text-xs text-muted-foreground">
                                {result.sectionPath || `Chunk #${result.chunkNo + 1}`}
                              </p>
                              {result.matchedContent && result.matchedContent !== result.content ? (
                                <div className="flex flex-col gap-3">
                                  <div>
                                    <p className="mb-1 text-xs font-medium text-muted-foreground">{t("knowledge.matchedChildChunk")}</p>
                                    <p className="whitespace-pre-wrap text-sm leading-6">{result.matchedContent}</p>
                                  </div>
                                  <div>
                                    <p className="mb-1 text-xs font-medium text-muted-foreground">{t("knowledge.modelContextChunk")}</p>
                                    <p className="whitespace-pre-wrap text-sm leading-6 text-muted-foreground">{result.content}</p>
                                  </div>
                                </div>
                              ) : (
                                <p className="whitespace-pre-wrap text-sm leading-6">{result.content}</p>
                              )}
                            </article>
                          ))}
                        </div>
                      ) : (
                        <div className="flex h-full flex-col items-center justify-center gap-2 px-8 text-center">
                          <SearchIcon className="size-8 text-muted-foreground" />
                          <p className="text-sm font-medium">{t("knowledge.noResultsAboveThreshold")}</p>
                          <p className="text-sm text-muted-foreground">{t("knowledge.noResultsAboveThresholdDescription")}</p>
                        </div>
                      )
                    ) : (
                      <div className="flex h-full flex-col items-center justify-center gap-2 px-8 text-center">
                        <SearchIcon className="size-8 text-muted-foreground" />
                        <p className="text-sm font-medium">{t("knowledge.retrievalNotRun")}</p>
                        <p className="text-sm text-muted-foreground">{t("knowledge.retrievalNotRunDescription")}</p>
                      </div>
                    )}
                  </ScrollArea>
                </div>
              </div>
            </TabsContent>
          </Tabs>
        </section>
      </main>
    </div>
  )
}
