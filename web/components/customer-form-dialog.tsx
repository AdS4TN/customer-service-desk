"use client"

import { useState } from "react"

import {
  CustomerForm,
  type CustomerFormSavePayload,
} from "@/components/customer-form"
import { ProjectDialog } from "@/components/project-dialog"
import { Button } from "@/components/ui/button"
import { useI18n } from "@/i18n/provider"
import { useConfirm } from "@/components/confirm-provider"

export type CustomerFormDialogProps = {
  open: boolean
  saving: boolean
  itemId: number | null
  onOpenChange: (open: boolean) => void
  onSave: (payload: CustomerFormSavePayload) => Promise<void>
}

/** Customer create/edit dialog shared by customer management and conversation workflows. */
export function CustomerFormDialog({
  open,
  saving,
  itemId,
  onOpenChange,
  onSave,
}: CustomerFormDialogProps) {
  if (!open) return null
  return (
    <CustomerFormDialogBody
      key={itemId ? `edit-${itemId}` : "create"}
      saving={saving}
      itemId={itemId}
      onOpenChange={onOpenChange}
      onSave={onSave}
    />
  )
}

type CustomerFormDialogBodyProps = Omit<CustomerFormDialogProps, "open">

function CustomerFormDialogBody({
  saving,
  itemId,
  onOpenChange,
  onSave,
}: CustomerFormDialogBodyProps) {
  const t = useI18n()
  const formId = "customer-form-dialog"
  const [loadingDetail, setLoadingDetail] = useState(() => Boolean(itemId))
  const [dirty, setDirty] = useState(false)
  const confirm = useConfirm()
  async function close() {
    if (saving) return
    if (dirty && !await confirm({ title: t("customerForm.discardTitle"), description: t("customerForm.discardDescription") })) return
    onOpenChange(false)
  }

  return (
    <ProjectDialog
      open
      onOpenChange={(next) => { if (!next) void close() }}
      title={itemId ? t("customerForm.editTitle") : t("customerForm.createTitle")}
      allowFullscreen
      size="xl"
      footer={
        <>
          <Button
            type="button"
            variant="outline"
            onClick={() => void close()}
            disabled={saving}
          >
            {t("customerForm.cancel")}
          </Button>
          <Button
            type="submit"
            form={formId}
            disabled={saving || loadingDetail}
          >
            {saving ? t("customerForm.saving") : itemId ? t("customerForm.save") : t("customerForm.create")}
          </Button>
        </>
      }
    >
      <CustomerForm
        formId={formId}
        itemId={itemId}
        onSave={onSave}
        fieldIdPrefix="customer"
        className="space-y-4"
        onLoadingDetailChange={setLoadingDetail}
        onDirtyChange={setDirty}
      />
    </ProjectDialog>
  )
}
