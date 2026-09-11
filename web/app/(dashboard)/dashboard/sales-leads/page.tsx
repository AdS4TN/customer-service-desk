"use client";

import { Suspense, useState } from "react";
import { useRouter, useSearchParams } from "next/navigation";
import { DashboardListPage } from "@/components/dashboard/list";
import { LeadBadges, SalesLeadDetail } from "@/components/sales-lead-detail";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { useI18n } from "@/i18n/provider";
import { fetchSalesLeads, type SalesLead } from "@/lib/api/sales-lead";
import { LeadStatus, LeadTag } from "@/lib/generated/enums";
import { formatDateTime } from "@/lib/utils";

export default function SalesLeadsPage() {
  return <Suspense><SalesLeadsContent /></Suspense>;
}

function SalesLeadsContent() {
  const t = useI18n();
  const router = useRouter();
  const params = useSearchParams();
  const linkedId = Number(params.get("leadId"));
  const [selected, setSelected] = useState<number | null>(null);
  const [version, setVersion] = useState(0);
  return (
    <>
      <DashboardListPage<SalesLead>
        getItemId={(item) => item.id}
        reloadKey={version}
        fetchList={(q) => fetchSalesLeads(q)}
        filters={[
          { name: "queue", label: t("leadWork.queue"), type: "select", defaultValue: "", options: [
            {value:"", label:t("leadWork.all")},
            ...["unassigned", "unscheduled", "overdue", "upcoming"].map((value) => ({value, label:t(`leadWork.${value}`)})),
          ] },
          { name: "mine", label: t("lead.owner"), type: "select", defaultValue: "", options: [
            {value:"", label:t("leadWork.everyone")}, {value:"true", label:t("leadWork.mine")},
          ] },
          {
            name: "keyword",
            label: t("lead.search"),
            placeholder: t("lead.search"),
            defaultValue: "",
            trim: true,
          },
          {
            name: "status",
            label: t("lead.status"),
            type: "select",
            defaultValue: "",
            options: [
              { value: "", label: t("lead.allStatuses") },
              ...Object.values(LeadStatus).map((value) => ({
                value,
                label: t(`lead.statuses.${value}`),
              })),
            ],
          },
          {
            name: "tag",
            label: t("lead.tagsLabel"),
            type: "select",
            defaultValue: "",
            options: [
              { value: "", label: t("lead.allTags") },
              ...Object.values(LeadTag).map((value) => ({
                value,
                label: t(`lead.tags.${value}`),
              })),
            ],
          },
        ]}
        columns={[
          {
            key: "purchase",
            label: t("lead.title"),
            className: "min-w-56",
            render: (item) => (
              <div className="flex max-w-80 flex-col gap-1">
                <Button
                  variant="link"
                  className="h-auto justify-start whitespace-normal p-0 text-left"
                  onClick={() => setSelected(item.id)}
                >
                  {item.sourceValid
                    ? item.data.title
                    : t("lead.invalidSourceShort")}
                </Button>
                <span className="text-xs text-muted-foreground">
                  {item.customerName} · #{item.conversationId}
                </span>
                <LeadBadges lead={item} />
              </div>
            ),
          },
          {
            key: "requirements",
            label: t("lead.requirements"),
            className: "min-w-44",
            render: (item) => (
              <div className="flex max-w-64 flex-col gap-1 break-words">
                <span>{item.data.product || "-"}</span>
                <span className="text-xs text-muted-foreground">
                  {[item.data.quantity, item.data.destination]
                    .filter(Boolean)
                    .join(" · ") || "-"}
                </span>
              </div>
            ),
          },
          {
            key: "review",
            label: t("lead.review"),
            className: "min-w-32",
            render: (item) => (
              <div className="flex flex-wrap gap-1">
                <Badge variant="outline">
                  {t(item.confirmed ? "lead.confirmed" : "lead.unconfirmed")}
                </Badge>
                {(item.proposal || item.withdrawn) && (
                  <Badge variant="secondary">{t("lead.needsReview")}</Badge>
                )}
              </div>
            ),
          },
          {
            key: "status",
            label: t("lead.status"),
            render: (item) => t(`lead.statuses.${item.status}`),
          },
          {
            key: "owner",
            label: t("lead.owner"),
            render: (item) => item.ownerName || t("lead.unassigned"),
          },
          {
            key: "follow",
            label: t("lead.followUp"),
            className: "min-w-40",
            render: (item) =>
              <div className="flex max-w-64 flex-col gap-1 break-words"><span>{item.followUpAt ? formatDateTime(item.followUpAt) : "-"}</span><span className="text-xs text-muted-foreground">{item.nextAction || "-"}</span></div>,
          },
          {
            key: "updated",
            label: t("lead.updated"),
            className: "min-w-40",
            render: (item) => formatDateTime(item.updatedAt),
          },
        ]}
        labels={{
          refresh: t("lead.refresh"),
          query: t("lead.searchAction"),
          loading: t("lead.loading"),
          empty: t("lead.emptyList"),
          loadFailed: t("lead.failed"),
        }}
      />
      <SalesLeadDetail
        id={selected ?? (Number.isSafeInteger(linkedId) && linkedId > 0 ? linkedId : null)}
        onClose={() => { setSelected(null); if (linkedId > 0) router.replace("/dashboard/sales-leads"); }}
        onSaved={() => setVersion((v) => v + 1)}
      />
    </>
  );
}
