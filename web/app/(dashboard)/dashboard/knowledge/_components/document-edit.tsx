"use client"

import { useEffect, useMemo, useState } from "react"
import { zodResolver } from "@hookform/resolvers/zod"
import { Controller, type Resolver, useForm } from "react-hook-form"
import { z } from "zod/v4"

import { ProjectDialog } from "@/components/project-dialog"
import { ContentEditor } from "@/components/content-editor"
import { OptionCombobox } from "@/components/option-combobox"
import { Button } from "@/components/ui/button"
import {
  Field,
  FieldContent,
  FieldError,
  FieldLabel,
} from "@/components/ui/field"
import { Input } from "@/components/ui/input"
import {
  type KnowledgeDocument,
  type CreateKnowledgeDocumentPayload,
  type KnowledgeDirectory,
  fetchKnowledgeDocument,
  fetchKnowledgeDirectories,
} from "@/lib/api/admin"
import {
  KnowledgeDocumentContentType,
} from "@/lib/generated/enums"
import { useI18n } from "@/i18n/provider"
import { toast } from "sonner"

type DocumentEditDialogProps = {
  open: boolean
  saving: boolean
  itemId: number | null
  knowledgeBaseId: number | null
  initialDirectoryId?: number
  onOpenChange: (open: boolean) => void
  onSubmit: (payload: CreateKnowledgeDocumentPayload) => Promise<void>
}

const emptyForm: EditForm = {
  directoryId: "0",
  title: "",
  contentType: KnowledgeDocumentContentType.Markdown,
  content: "",
}

type TFunction = (key: string, values?: Record<string, string | number>) => string

function createKnowledgeDocumentFormSchema(t: TFunction) {
  return z.object({
  directoryId: z.string().trim(),
  title: z.string().trim().min(1, t("knowledge.documentTitleRequired")).max(255, t("knowledge.documentTitleMax")),
  contentType: z.string().trim().min(1, t("knowledge.contentTypeRequired")),
  content: z.string().trim().min(1, t("knowledge.contentRequired")),
  })
}

type EditForm = {
  directoryId: string
  title: string
  contentType: string
  content: string
}

type DirectoryOption = { value: string; label: string }

function flattenDirectoryOptions(items: KnowledgeDirectory[], depth = 0): DirectoryOption[] {
  return items.flatMap((item) => [
    { value: String(item.id), label: `${depth > 0 ? "  " : ""}${item.name}` },
    ...flattenDirectoryOptions(item.children || [], depth + 1),
  ])
}

function buildForm(item: KnowledgeDocument | null, initialDirectoryId = 0): EditForm {
  if (!item) {
    return { ...emptyForm, directoryId: String(initialDirectoryId) }
  }

  return {
    directoryId: String(item.directoryId || 0),
    title: item.title,
    contentType: item.contentType || KnowledgeDocumentContentType.Markdown,
    content: item.content || "",
  }
}

function buildPayload(form: EditForm, knowledgeBaseId: number, chunkConfig?: Partial<CreateKnowledgeDocumentPayload>): CreateKnowledgeDocumentPayload {
  return {
    knowledgeBaseId,
    directoryId: Number(form.directoryId),
    title: form.title.trim(),
    contentType: form.contentType,
    content: form.content.trim(),
    ...chunkConfig,
  }
}

export function DocumentEditDialog({
  open,
  saving,
  itemId,
  knowledgeBaseId,
  initialDirectoryId = 0,
  onOpenChange,
  onSubmit,
}: DocumentEditDialogProps) {
  if (!open || !knowledgeBaseId) {
    return null
  }

  return (
    <DocumentFormDialogBody
      key={itemId ? `edit-${itemId}` : "create"}
      itemId={itemId}
      knowledgeBaseId={knowledgeBaseId}
      initialDirectoryId={initialDirectoryId}
      saving={saving}
      onOpenChange={onOpenChange}
      onSubmit={onSubmit}
    />
  )
}

type DocumentFormDialogBodyProps = {
  saving: boolean
  itemId: number | null
  knowledgeBaseId: number
  initialDirectoryId: number
  onOpenChange: (open: boolean) => void
  onSubmit: (payload: CreateKnowledgeDocumentPayload) => Promise<void>
}

function DocumentFormDialogBody({
  saving,
  itemId,
  knowledgeBaseId,
  initialDirectoryId,
  onOpenChange,
  onSubmit,
}: DocumentFormDialogBodyProps) {
  const t = useI18n()
  const formId = "knowledge-document-edit-form"
  const [loading, setLoading] = useState(false)
  const [parsingFile, setParsingFile] = useState(false)
  const [directories, setDirectories] = useState<KnowledgeDirectory[]>([])
  const [savedChunkConfig, setSavedChunkConfig] = useState<Partial<CreateKnowledgeDocumentPayload>>({})
  const knowledgeDocumentFormSchema = useMemo(() => createKnowledgeDocumentFormSchema(t), [t])
  const editFormResolver = useMemo(
    () => zodResolver(knowledgeDocumentFormSchema) as Resolver<EditForm>,
    [knowledgeDocumentFormSchema],
  )
  const form = useForm<EditForm>({
    resolver: editFormResolver,
    defaultValues: emptyForm,
  })
  const {
    control,
    handleSubmit,
    reset,
    register,
    setValue,
    watch,
    formState: { errors },
  } = form

  const contentType = watch("contentType")
  const content = watch("content")
  const directoryOptions = useMemo(
    () => [
      { value: "0", label: t("knowledge.rootContent") },
      ...flattenDirectoryOptions(directories),
    ],
    [directories, t],
  )

  useEffect(() => {
    async function loadDetail() {
      if (!itemId) {
        setSavedChunkConfig({ chunkConfigOverride: false })
        reset(buildForm(null, initialDirectoryId))
        return
      }
      setLoading(true)
      try {
        const data = await fetchKnowledgeDocument(itemId)
        setSavedChunkConfig({
          chunkConfigOverride: data.chunkConfigOverride,
          chunkProvider: data.chunkProvider,
          chunkTargetTokens: data.chunkTargetTokens,
          chunkMaxTokens: data.chunkMaxTokens,
          chunkOverlapTokens: data.chunkOverlapTokens,
          parentChunkTokens: data.parentChunkTokens,
          childChunkTokens: data.childChunkTokens,
        })
        reset(buildForm(data))
      } catch (error) {
        console.error("Failed to load knowledge document:", error)
      } finally {
        setLoading(false)
      }
    }
    void loadDetail()
  }, [itemId, initialDirectoryId, reset])

  useEffect(() => {
    let cancelled = false
    async function loadDirectories() {
      try {
        const data = await fetchKnowledgeDirectories(knowledgeBaseId)
        if (!cancelled) {
          setDirectories(data)
        }
      } catch (error) {
        console.error("Failed to load knowledge directories:", error)
      }
    }
    void loadDirectories()
    return () => {
      cancelled = true
    }
  }, [knowledgeBaseId])

  async function onFormSubmit(values: EditForm) {
    const payload = buildPayload({ ...values, contentType, content }, knowledgeBaseId, savedChunkConfig)
    await onSubmit(payload)
  }

  async function handleFileImport(file: File | undefined) {
    if (!file) return
    const extension = file.name.split(".").pop()?.toLowerCase()
    const textExtensions = ["txt", "md", "markdown", "html", "htm"]
    const mineruExtensions = ["pdf", "doc", "docx", "ppt", "pptx", "png", "jpg", "jpeg"]
    if (!extension || ![...textExtensions, ...mineruExtensions].includes(extension)) {
      toast.error("不支持该文件格式")
      return
    }
    setParsingFile(true)
    try {
      let importedContent: string
      let inferredType: string
      if (textExtensions.includes(extension)) {
        importedContent = await file.text()
        inferredType = ["html", "htm"].includes(extension)
          ? KnowledgeDocumentContentType.HTML
          : KnowledgeDocumentContentType.Markdown
      } else {
        const formData = new FormData()
        formData.set("file", file)
        const parserURL = `http://${window.location.hostname}:8090/v1/parse-document`
        const response = await fetch(parserURL, { method: "POST", body: formData })
        const payload = await response.json() as { content?: string; detail?: string }
        if (!response.ok || !payload.content) throw new Error(payload.detail || "MinerU 解析失败")
        importedContent = payload.content
        inferredType = KnowledgeDocumentContentType.Markdown
      }
      if (!importedContent.trim()) {
        toast.error("文件内容为空")
        return
      }
      setValue("title", file.name.replace(/\.[^.]+$/, ""), { shouldDirty: true, shouldValidate: true })
      setValue("content", importedContent, { shouldDirty: true, shouldValidate: true })
      setValue("contentType", inferredType, { shouldDirty: true, shouldValidate: true })
      toast.success("文件内容已导入，请确认后创建文档")
    } catch (error) {
      toast.error(error instanceof Error ? error.message : "文件解析失败")
    } finally {
      setParsingFile(false)
    }
  }

  return (
    <ProjectDialog
      open={true}
      onOpenChange={onOpenChange}
      title={itemId ? t("knowledge.editDocumentTitle") : t("knowledge.createDocumentTitle")}
      size="xl"
      allowFullscreen
      footer={
        <>
          <Button
            type="button"
            variant="outline"
            onClick={() => onOpenChange(false)}
            disabled={saving}
          >
            {t("knowledge.cancel")}
          </Button>
          <Button type="submit" form={formId} disabled={saving || loading}>
            {saving ? t("knowledge.saving") : itemId ? t("knowledge.save") : t("knowledge.create")}
          </Button>
        </>
      }
    >
      {loading ? (
        <div className="flex items-center justify-center py-12">
          <div className="text-muted-foreground">{t("knowledge.loading")}</div>
        </div>
      ) : (
        <form id={formId} onSubmit={handleSubmit(onFormSubmit)} className="flex flex-col gap-4">
          {!itemId ? (
            <Field>
              <FieldLabel htmlFor="doc-file">从文件导入</FieldLabel>
              <FieldContent>
                <Input
                  id="doc-file"
                  type="file"
                  accept=".txt,.md,.markdown,.html,.htm,.pdf,.doc,.docx,.ppt,.pptx,.png,.jpg,.jpeg"
                  disabled={saving || parsingFile}
                  onChange={(event) => {
                    void handleFileImport(event.target.files?.[0])
                    event.target.value = ""
                  }}
                />
                <p className="text-sm text-muted-foreground">支持 TXT、Markdown、HTML、PDF、Office 文档和图片；复杂文件由 MinerU 解析。</p>
              </FieldContent>
            </Field>
          ) : null}
          <Field data-invalid={!!errors.directoryId}>
            <FieldLabel>{t("knowledge.directory")}</FieldLabel>
            <FieldContent>
              <Controller
                control={control}
                name="directoryId"
                render={({ field }) => (
                  <OptionCombobox
                    value={field.value}
                    onChange={(value) => field.onChange(value ?? "0")}
                    options={directoryOptions}
                    placeholder={t("knowledge.selectDirectory")}
                    searchPlaceholder={t("knowledge.searchDirectory")}
                    emptyText={t("knowledge.emptyDirectory")}
                  />
                )}
              />
              <FieldError errors={[errors.directoryId]} />
            </FieldContent>
          </Field>

          <Field data-invalid={!!errors.title}>
            <FieldLabel htmlFor="doc-title">{t("knowledge.documentTitle")}</FieldLabel>
            <FieldContent>
              <Input
                id="doc-title"
                placeholder={t("knowledge.documentTitlePlaceholder")}
                aria-invalid={!!errors.title}
                {...register("title")}
              />
              <FieldError errors={[errors.title]} />
            </FieldContent>
          </Field>

          <Field data-invalid={!!errors.content}>
            <FieldLabel htmlFor="doc-content">{t("knowledge.content")}</FieldLabel>
            <FieldContent>
              <Controller
                control={control}
                name="content"
                render={({ field }) => (
                  <ContentEditor
                    value={{
                      mode:
                        contentType === KnowledgeDocumentContentType.HTML
                          ? KnowledgeDocumentContentType.HTML
                          : KnowledgeDocumentContentType.Markdown,
                      raw: field.value ?? "",
                    }}
                    onChange={(next) => {
                      field.onChange(next.raw)
                      setValue("contentType", next.mode, {
                        shouldDirty: true,
                        shouldValidate: true,
                      })
                    }}
                    placeholder={t("knowledge.contentPlaceholder")}
                    disabled={saving}
                  />
                )}
              />
              <FieldError errors={[errors.content]} />
            </FieldContent>
          </Field>

        </form>
      )}
    </ProjectDialog>
  )
}
