"use client"

import { useState } from "react"
import { Code2Icon, MessageCircleIcon, LinkIcon } from "lucide-react"
import { Button } from "@/components/ui/button"

import {
  createDashboardStatusColumn,
  DashboardCrudPage,
} from "@/components/dashboard/crud"
import {
  createChannel,
  deleteChannel,
  fetchChannels,
  updateChannel,
  updateChannelStatus,
  type AdminChannel,
  type CreateAdminChannelPayload,
} from "@/lib/api/admin"
import { getEnumOptions } from "@/lib/enums"
import { Status, StatusLabels } from "@/lib/generated/enums"
import { useI18n } from "@/i18n/provider"
import { EditDialog } from "./edit"
import { MessengerDialog } from "./messenger"
import { WhatsAppDialog } from "./whatsapp"

function getStatusLabel(status: Status, t: (key: string) => string) {
  if (status === Status.Disabled) {
    return t("status.disabled")
  }
  if (status === Status.Deleted) {
    return t("status.deleted")
  }
  return t("status.ok")
}

export function ChannelList({ channelType }: { channelType: "web" | "whatsapp" | "messenger" }) {
  const t = useI18n()
  const [connecting, setConnecting] = useState<AdminChannel | null>(null)
  const statusOptions = [
    { value: "all", label: t("status.all") },
    ...getEnumOptions(StatusLabels).map((option) => ({
      value: String(option.value),
      label: getStatusLabel(option.value as Status, t),
    })),
  ]

  return (
    <>
    <DashboardCrudPage<AdminChannel, CreateAdminChannelPayload>
      filters={[
        {
          name: "name",
          label: t("channel.filterName"),
          placeholder: t("channel.filterName"),
          defaultValue: "",
          trim: true,
          className: "w-full sm:w-56",
        },
        {
          name: "channelId",
          label: t("channel.filterChannelId"),
          placeholder: t("channel.filterChannelId"),
          defaultValue: "",
          trim: true,
          className: "w-full sm:w-72",
        },
        {
          name: "status",
          label: t("status.all"),
          type: "select",
          defaultValue: "all",
          allValue: "all",
          options: statusOptions,
          className: "w-full sm:w-36",
        },
      ]}
      columns={[
        {
          key: "channel",
          label: t("channel.columnChannel"),
          render: (item) => (
            <div className="flex items-center gap-3">
              <div className="flex size-10 items-center justify-center rounded-md bg-muted">
                {item.channelType !== "web" ? <MessageCircleIcon className="size-4" /> : <Code2Icon className="size-4" />}
              </div>
              <div>
                <div className="font-medium">{item.name}</div>
                <div className="text-xs text-muted-foreground">{item.channelType === "messenger" ? "Messenger" : item.channelType === "whatsapp" ? "WhatsApp" : t("channel.typeWeb")}</div>
              </div>
            </div>
          ),
        },
        {
          key: "channelId",
          label: "ChannelID",
          render: (item) => (
            <span className="font-mono text-xs">{item.channelId || "-"}</span>
          ),
        },
        {
          key: "agent",
          label: t("channel.columnAgent"),
          render: (item) => item.aiAgentName || "-",
        },
        ...(channelType !== "web" ? [{
          key: "connection",
          label: t("channel.wa.manage"),
          render: (item: AdminChannel) => (
            <Button variant="outline" size="sm" onClick={() => setConnecting(item)}>
              <LinkIcon data-icon="inline-start" />{t("channel.wa.manage")}
            </Button>
          ),
        }] : []),
        createDashboardStatusColumn<AdminChannel, Status>({
          label: t("channel.columnStatus"),
          getStatus: (item) => item.status as Status,
          getLabel: (status) => getStatusLabel(status, t),
          getBadgeVariant: (status) => (status === Status.Ok ? "default" : "outline"),
          isEnabled: (status) => status === Status.Ok,
          toggle: {
            getNextStatus: (item) =>
              item.status === Status.Ok ? Status.Disabled : Status.Ok,
            updateStatus: (item, nextStatus) =>
              updateChannelStatus(item.id, nextStatus),
            successMessage: (item, nextStatus) =>
              t(
                nextStatus === Status.Ok
                  ? "channel.statusEnabled"
                  : "channel.statusDisabled",
                { name: item.name }
              ),
            errorMessage: t("channel.statusUpdateFailed"),
            ariaLabel: (item) => t("channel.toggleStatus", { name: item.name }),
          },
        }),
      ]}
      fetchList={(query) => fetchChannels({ ...query, channelType })}
      getItemId={(item) => item.id}
      createItem={createChannel}
      updateItem={(item, payload) => updateChannel({ id: item.id, ...payload })}
      deleteItem={(item) => deleteChannel(item.id)}
      renderEditDialog={({ open, saving, itemId, onOpenChange, onSubmit }) => (
        <EditDialog
          fixedChannelType={channelType}
          open={open}
          saving={saving}
          itemId={itemId}
          onOpenChange={onOpenChange}
          onSubmit={onSubmit}
        />
      )}
      labels={{
        refresh: t("channel.refresh"),
        create: t(channelType === "web" ? "channel.newWeb" : channelType === "messenger" ? "channel.newMessenger" : "channel.newWhatsApp"),
        query: t("channel.query"),
        loading: t("channel.loading"),
        empty: t("channel.empty"),
        actions: t("channel.columnActions"),
        edit: t("channel.edit"),
        delete: t("channel.delete"),
        processing: t("channel.processing"),
        moreActions: (item) => t("channel.moreActions", { name: item.name }),
        loadFailed: t("channel.loadFailed"),
        saveFailed: t("channel.saveFailed"),
        deleteFailed: t("channel.deleteFailed"),
        created: (payload) => t("channel.created", { name: payload.name }),
        updated: (_item, payload) => t("channel.updated", { name: payload.name }),
        deleted: (item) => t("channel.deleted", { name: item.name }),
      }}
    />
    {connecting && (channelType === "messenger" ? <MessengerDialog key={connecting.id} channel={connecting} onClose={() => setConnecting(null)} /> : <WhatsAppDialog key={connecting.id} channel={connecting} onClose={() => setConnecting(null)} />)}
    </>
  )
}
