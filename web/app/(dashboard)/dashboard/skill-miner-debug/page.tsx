"use client";

import { DashboardPage } from "@/components/dashboard-page";
import { useI18n } from "@/i18n/provider";
import { SkillMinerDebugPanel } from "../sales-experience/_components/skill-miner-panel";

export default function SkillMinerDebugPage() {
  const t = useI18n();
  return (
    <DashboardPage className="min-w-0">
      <h1 className="text-xl font-semibold">{t("sx.title")}</h1>
      <SkillMinerDebugPanel />
    </DashboardPage>
  );
}
