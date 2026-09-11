---
version: 1
slug: "web-app-dashboard-dashboard-sales-leads-page-tsx"
primary_target: "web/app/(dashboard)/dashboard/sales-leads/page.tsx"
related_targets: ["web/components/sales-lead-detail.tsx","web/components/sales-lead-followup.tsx","web/app/(dashboard)/dashboard/conversations/_components/conversation-leads.tsx"]
---

# Sales Leads

## Direction contract

MODE: Operate.

THESIS: Turn conversation evidence into actionable sales leads with explicit AI versus human ownership.

OWN-WORLD: Inherit AgentDesk's existing light/dark semantic tokens, system typography, compact shadcn/Base UI controls and restrained borders. Do not change the conversation workbench layout or add decorative metrics.

STORY: Operators filter unassigned, unscheduled and due leads, inspect evidence, then record a result with an accountable owner and a next action or terminal outcome. Internal reminders return to the same lead. No automatic customer messages.

FIRST VIEWPORT: Preserve the shared paginated list and add owner/work queues. The focused detail dialog opens on Follow-up, with Details as a second tab. A compact form places result, owner, status and next action above the immutable recent history. Unsaved input is protected across tabs and closing; no visual system changes.

FORM: Scoped extension using the existing DashboardListPage and ProjectDialog composition; no concept seed or visual identity replacement.

FINISH: unreviewed and undocumented is unfinished; this build ends with the finish review, the verdict, DESIGN.md, and every shipping raster carrying its provenance

No new visual system or raster assets are required; existing design files are preserved.
