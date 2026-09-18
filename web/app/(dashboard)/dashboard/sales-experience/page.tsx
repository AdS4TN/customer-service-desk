"use client";

import { useCallback, useState } from "react";
import Link from "next/link";
import {
  EyeIcon,
  ImportIcon,
  Trash2Icon,
  XIcon,
} from "lucide-react";
import { toast } from "sonner";
import { useI18n } from "@/i18n/provider";
import { useAuth } from "@/components/auth-provider";
import { useConfirm } from "@/components/confirm-provider";
import { DashboardPage } from "@/components/dashboard-page";
import { DashboardListPage } from "@/components/dashboard/list";
import { Checkbox } from "@/components/ui/checkbox";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import {
  experienceAPI as api,
  type ExperienceCase,
  type ExperienceSource,
} from "@/lib/api/sales-experience";
import { formatDateTime } from "@/lib/utils";
import {
  ExperienceError,
  useExperienceAction,
} from "./_components/shared";
import { CaseDetail } from "./_components/case-workspace";
import { ImportCaseDialog } from "./_components/import-case-dialog";
import { SkillMinerDebugPanel } from "./_components/skill-miner-panel";
import { SkillComparePanel } from "./_components/skill-compare-panel";

export default function SalesExperiencePage() {
  const t = useI18n();
  const { session } = useAuth();
  const confirm = useConfirm();
  const allowed = (p: string) =>
    !!session &&
    (session.roles.includes("super_admin") ||
      session.permissions.includes("*") ||
      session.permissions.includes(p));
  const canView =
    allowed("conversation.view") && allowed("skillDefinition.view");
  const canEdit = canView && allowed("skillDefinition.update");
  const canConfirm = canEdit && allowed("skillDefinition.create");
  const [tab, setTab] = useState("sources");
  const [reload, setReload] = useState(0);
  const [selectedSources, setSelectedSources] = useState<number[]>([]);
  const [caseDetail, setCaseDetail] = useState<ExperienceCase | null>(null);
  const [importOpen, setImportOpen] = useState(false);
  const action = useExperienceAction();
  const refresh = useCallback(() => setReload((n) => n + 1), []);
  const labels = {
    loading: t("common.loading"),
    empty: t("sx.empty"),
    loadFailed: t("sx.failed"),
    query: t("sx.search"),
    refresh: t("sx.refresh"),
  };
  const toggle = (ids: number[], id: number, checked: boolean) =>
    checked ? [...new Set([...ids, id])] : ids.filter((v) => v !== id);
  const openCase = (id: number) => action.run(() => api.case(id), setCaseDetail);
  const deleteCase = async (item: ExperienceCase) => {
    if (
      !(await confirm({
        title: t("sx.deleteCaseTitle", { name: item.name }),
        description: t("sx.deleteCaseBody"),
        confirmText: t("sx.deleteCase"),
        cancelText: t("common.cancel"),
        variant: "destructive",
      }))
    )
      return;
    await action.run(
      () => api.deleteCase(item.id),
      () => {
        if (caseDetail?.id === item.id) setCaseDetail(null);
        refresh();
        toast.success(t("sx.caseDeleted"));
      },
    );
  };
  if (!canView)
    return (
      <DashboardPage>
        <ExperienceError message={t("sx.noPermission")} />
      </DashboardPage>
    );
  return (
    <DashboardPage className="min-w-0">
      <h1 className="text-xl font-semibold">{t("sx.title")}</h1>
      <ExperienceError message={action.error} />
      <Tabs value={tab} onValueChange={setTab} className="min-w-0 gap-4">
        <div className="max-w-full overflow-x-auto pb-1">
          <TabsList variant="line">
            {["sources", "cases", "skillMiner", "skillCompare"].map((id) => (
              <TabsTrigger key={id} value={id}>
                {t(`sx.${id}`)}
              </TabsTrigger>
            ))}
          </TabsList>
        </div>
        <TabsContent value="sources" className="flex min-w-0 flex-col gap-3">
          <DashboardListPage<ExperienceSource>
            layout="fragment"
            enabled={tab === "sources"}
            fetchList={api.sources}
            reloadKey={reload}
            labels={labels}
            getItemId={(r) => r.id}
            filters={[
              {
                name: "keyword",
                label: t("sx.customer"),
                placeholder: t("sx.searchCustomer"),
                defaultValue: "",
                trim: true,
              },
              {
                name: "channel",
                label: t("sx.channel"),
                defaultValue: "",
                type: "select",
                options: [
                  "",
                  "web",
                  "whatsapp",
                  "messenger",
                  "wxwork_kf",
                  "telegram",
                  "zalo",
                ].map((value) => ({
                  value,
                  label: value
                    ? t(`sx.channels.${value}`)
                    : t("sx.allChannels"),
                })),
              },
            ]}
            renderToolbarActions={({ result, loading }) => (
              <div className="flex flex-wrap items-center gap-2">
                <Checkbox
                  aria-label={t("sx.selectPage")}
                  disabled={
                    loading ||
                    !canEdit ||
                    action.busy ||
                    result.results.length === 0
                  }
                  checked={
                    result.results.length > 0 &&
                    result.results.every((r) => selectedSources.includes(r.id))
                  }
                  onCheckedChange={(v) =>
                    setSelectedSources(
                      v
                        ? [
                            ...new Set([
                              ...selectedSources,
                              ...result.results.map((r) => r.id),
                            ]),
                          ]
                        : selectedSources.filter(
                            (id) => !result.results.some((r) => r.id === id),
                          ),
                    )
                  }
                />
                <span className="text-sm tabular-nums">
                  {t("sx.selected", { count: selectedSources.length })}
                </span>
                {selectedSources.length > 0 && (
                  <Button
                    variant="ghost"
                    size="icon-sm"
                    title={t("sx.clearSelection")}
                    aria-label={t("sx.clearSelection")}
                    disabled={action.busy}
                    onClick={() => setSelectedSources([])}
                  >
                    <XIcon />
                  </Button>
                )}
                <Button
                  disabled={
                    !canEdit || action.busy || selectedSources.length === 0
                  }
                  onClick={() => setImportOpen(true)}
                >
                  <ImportIcon data-icon="inline-start" />
                  {t("sx.import")}
                </Button>
              </div>
            )}
            columns={[
              {
                key: "selected",
                label: "",
                className: "w-10",
                render: (r) => (
                  <Checkbox
                    aria-label={t("sx.selectConversation", { id: r.id })}
                    checked={selectedSources.includes(r.id)}
                    disabled={!canEdit || action.busy}
                    onCheckedChange={(v) =>
                      setSelectedSources(toggle(selectedSources, r.id, v))
                    }
                  />
                ),
              },
              {
                key: "customer",
                label: t("sx.customer"),
                render: (r) => (
                  <div className="min-w-36 max-w-64">
                    <div className="break-words font-medium">
                      {r.customerName || `#${r.customerId}`}
                    </div>
                    <Link
                      className="text-sm text-muted-foreground underline underline-offset-4"
                      href={`/dashboard/conversations?conversationId=${r.id}`}
                    >
                      #{r.id}
                    </Link>
                  </div>
                ),
              },
              {
                key: "channel",
                label: t("sx.channel"),
                render: (r) => (
                  <div className="min-w-28 max-w-48 whitespace-normal break-words">
                    {r.channelType
                      ? t(`sx.channels.${r.channelType}`)
                      : t("sx.unknownChannel")}
                    <div className="text-sm text-muted-foreground">
                      {r.channelName}
                    </div>
                  </div>
                ),
              },
              {
                key: "count",
                label: t("sx.messages"),
                render: (r) => (
                  <span className="tabular-nums">{r.messageCount}</span>
                ),
              },
              {
                key: "last",
                label: t("sx.lastMessage"),
                render: (r) => formatDateTime(r.lastMessageAt),
              },
            ]}
          />
        </TabsContent>
        <TabsContent value="cases" className="flex min-w-0 flex-col gap-3">
          <DashboardListPage<ExperienceCase>
            layout="fragment"
            enabled={tab === "cases"}
            fetchList={api.cases}
            reloadKey={reload}
            labels={{ ...labels, empty: t("sx.noCases") }}
            getItemId={(r) => r.id}
            columns={[
              {
                key: "name",
                label: t("sx.case"),
                render: (r) => (
                  <button
                    className="min-w-36 max-w-64 break-words text-left font-medium underline-offset-4 hover:underline"
                    onClick={() => void openCase(r.id)}
                    disabled={action.busy}
                  >
                    {r.name}{" "}
                    <span className="text-muted-foreground">#{r.id}</span>
                  </button>
                ),
              },
              {
                key: "count",
                label: t("sx.messages"),
                render: (r) => r.messageCount,
              },
              {
                key: "outcome",
                label: t("sx.outcome"),
                render: (r) => (
                  <Badge variant="outline">
                    {t(`sx.outcomes.${r.outcome}`)}
                  </Badge>
                ),
              },
              {
                key: "extractionStatus",
                label: t("sx.extractionStatus"),
                render: (r) => (
                  <Badge
                    variant={
                      r.extractionStatus === "succeeded"
                        ? "default"
                        : r.extractionStatus === "failed"
                          ? "destructive"
                          : "outline"
                    }
                  >
                    {t(
                      `sx.extractionStatuses.${r.extractionStatus || "pending"}`,
                    )}
                  </Badge>
                ),
              },
              {
                key: "date",
                label: t("sx.importedAt"),
                render: (r) => formatDateTime(r.createdAt),
              },
              {
                key: "actions",
                label: t("sx.actions"),
                render: (r) => (
                  <div className="flex items-center gap-1">
                    <Button
                      size="icon-sm"
                      variant="ghost"
                      title={t("sx.viewCase")}
                      aria-label={t("sx.viewCase")}
                      disabled={action.busy}
                      onClick={() => void openCase(r.id)}
                    >
                      <EyeIcon />
                    </Button>
                    {canEdit && (
                      <Button
                        size="icon-sm"
                        variant="ghost"
                        title={t("sx.deleteCase")}
                        aria-label={t("sx.deleteCaseNamed", { name: r.name })}
                        disabled={action.busy || r.extractionStatus === "running"}
                        onClick={() => void deleteCase(r)}
                      >
                        <Trash2Icon />
                      </Button>
                    )}
                  </div>
                ),
              },
            ]}
          />
        </TabsContent>
        <TabsContent value="skillMiner" className="flex min-w-0 flex-col gap-3">
          <SkillMinerDebugPanel
            canEdit={canEdit}
            canConfirm={canConfirm}
            extracted={refresh}
          />
        </TabsContent>
        <TabsContent value="skillCompare" className="flex min-w-0 flex-col gap-3">
          <SkillComparePanel />
        </TabsContent>
      </Tabs>
      {importOpen && (
        <ImportCaseDialog
          sourceIds={selectedSources}
          close={() => setImportOpen(false)}
          imported={(rows) => {
            setImportOpen(false);
            setSelectedSources([]);
            setTab("cases");
            refresh();
            toast.success(t("sx.imported", { count: rows.length }));
          }}
        />
      )}
      {caseDetail && (
        <CaseDetail
          key={caseDetail.id}
          item={caseDetail}
          canEdit={canEdit}
          close={() => setCaseDetail(null)}
          saved={refresh}
        />
      )}
    </DashboardPage>
  );
}
