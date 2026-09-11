"use client";
import { useCallback, useEffect, useState } from "react";
import Link from "next/link";
import { RefreshCwIcon, SparklesIcon } from "lucide-react";
import { toast } from "sonner";
import { useAuth } from "@/components/auth-provider";
import { LeadBadges, SalesLeadDetail } from "@/components/sales-lead-detail";
import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { Skeleton } from "@/components/ui/skeleton";
import { Separator } from "@/components/ui/separator";
import { useI18n } from "@/i18n/provider";
import { fetchSalesLeads, type SalesLead } from "@/lib/api/sales-lead";
import {
  fetchConversationMemory,
  refreshConversationMemory,
  type ConversationMemory,
} from "@/lib/api/conversation-memory";
import { leadAnalysisState } from "@/lib/lead-analysis";
import { MemoryStatus } from "@/lib/generated/enums";

export function ConversationLeads({
  conversationId,
}: {
  conversationId: number;
}) {
  return <ConversationLeadsPanel key={conversationId} conversationId={conversationId} />;
}

function ConversationLeadsPanel({ conversationId }: { conversationId: number }) {
  const t = useI18n();
  const { session } = useAuth();
  const [rows, setRows] = useState<SalesLead[] | null>(null);
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);
  const [selected, setSelected] = useState<number | null>(null);
  const [version, setVersion] = useState(0);
  const [analysis, setAnalysis] = useState<ConversationMemory | null>(null);
  const state = leadAnalysisState(analysis);
  const refresh = useCallback(() => setVersion((v) => v + 1), []);
  useEffect(() => {
    let active = true;
    let loading = false;
    const load = async () => {
      if (loading) return;
      loading = true;
      try {
        const [data, memory] = await Promise.all([
          fetchSalesLeads({ conversationId, limit: 50 }),
          fetchConversationMemory(conversationId),
        ]);
        if (active) {
          setRows(data.results);
          setAnalysis(memory);
          setError("");
        }
      } catch (e) {
        if (active) setError(e instanceof Error ? e.message : t("lead.failed"));
      } finally {
        loading = false;
      }
    };
    void load();
    const timer = setInterval(() => {
      if (document.visibilityState === "visible") void load();
    }, 8000);
    return () => {
      active = false;
      clearInterval(timer);
    };
  }, [conversationId, version, t]);
  async function analyze() {
    setBusy(true);
    try {
      await refreshConversationMemory(conversationId);
      setAnalysis((previous) => previous ? { ...previous, status: MemoryStatus.Queued, errorCode: "", attemptCount: 0, nextRetryAt: undefined } : previous);
      toast.success(t("lead.queued"));
      refresh();
    } catch (e) {
      setError(e instanceof Error ? e.message : t("lead.failed"));
    } finally {
      setBusy(false);
    }
  }
  const canEdit = session?.permissions.some(
    (p) => p === "*" || p === "conversation.send",
  );
  return (
    <section className="flex min-w-0 flex-col gap-4">
      <div className="flex flex-wrap justify-between gap-2">
        <Link
          className="text-sm underline underline-offset-4"
          href="/dashboard/sales-leads"
        >
          {t("lead.allLeads")}
        </Link>
        <Button
          size="icon-sm"
          variant="ghost"
          title={t("lead.refresh")}
          aria-label={t("lead.refresh")}
          onClick={refresh}
        >
          <RefreshCwIcon />
        </Button>
      </div>
      {error && (
        <Alert variant="destructive">
          <AlertDescription>{error}</AlertDescription>
        </Alert>
      )}
      {!error && state.failed && (
        <Alert variant="destructive">
          <AlertTitle>{t("leadAnalysis.failed")}</AlertTitle>
          <AlertDescription className="flex flex-col gap-1">
            <span>{t(state.errorKey)}</span>
            <span>{t("leadAnalysis.preserved")}</span>
          </AlertDescription>
        </Alert>
      )}
      {!error && state.retrying ? (
        <Alert role="status">
          <AlertTitle>{t(analysis?.status === "processing" ? "leadAnalysis.retryRunning" : "leadAnalysis.retryWaiting")}</AlertTitle>
          <AlertDescription className="flex flex-col gap-1">
            <span>{t(state.errorKey)}</span>
            <span>{t("leadAnalysis.attempt", {
              current: (analysis?.attemptCount ?? 0) + (analysis?.status === "queued" ? 1 : 0),
              max: analysis?.maxAttempts ?? 3,
            })}</span>
          </AlertDescription>
        </Alert>
      ) : !error && state.pending && (
        <p className="text-sm text-muted-foreground" role="status">
          {t("lead.analysisPending")}
        </p>
      )}
      {!rows ? (
        !error && <Skeleton className="h-24 w-full" />
      ) : rows.length ? (
        rows.map((item) => (
          <div key={item.id} className="flex min-w-0 flex-col gap-2">
            <Button
              variant="link"
              className="h-auto justify-start whitespace-normal p-0 text-left"
              onClick={() => setSelected(item.id)}
            >
              {item.sourceValid
                ? item.data.title
                : t("lead.invalidSourceShort")}
            </Button>
            <div className="flex flex-wrap gap-1">
              <Badge variant="outline">
                {t(`lead.statuses.${item.status}`)}
              </Badge>
              <Badge variant="outline">
                {t(item.confirmed ? "lead.confirmed" : "lead.unconfirmed")}
              </Badge>
            </div>
            <p className="break-words text-sm">
              {item.data.needs || item.data.product}
            </p>
            <LeadBadges lead={item} />
            {(item.proposal || item.withdrawn) && (
              <p className="text-xs text-muted-foreground">
                {t("lead.needsReview")}
              </p>
            )}
            <Separator />
          </div>
        ))
      ) : !error && state.emptyKey ? (
        <p className="text-sm text-muted-foreground">{t(state.emptyKey)}</p>
      ) : null}
      {canEdit && (
        <Button
          variant="outline"
          size="sm"
          className="self-start"
          disabled={busy || state.pending || !analysis}
          onClick={() => void analyze()}
        >
          <SparklesIcon data-icon="inline-start" />
          {t(busy ? "lead.loading" : state.failed ? "leadAnalysis.retry" : "lead.analyze")}
        </Button>
      )}
      <SalesLeadDetail
        id={selected}
        onClose={() => setSelected(null)}
        onSaved={refresh}
      />
    </section>
  );
}
