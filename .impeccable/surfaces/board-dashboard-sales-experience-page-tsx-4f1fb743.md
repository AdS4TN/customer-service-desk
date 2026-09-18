---
version: 1
slug: "board-dashboard-sales-experience-page-tsx-4f1fb743"
primary_target: "web/app/(dashboard)/dashboard/sales-experience/page.tsx"
related_targets: []
---

# Sales Experience

Mode: Operate. Target: web/app/(dashboard)/dashboard/sales-experience/page.tsx.
The user requests the standalone Sales Skill Lab workflow natively inside AgentDesk, using selected internal conversations. Confirmed: no automatic publication to live agents.

## Direction contract

THESIS: Conversation evidence becomes editable, testable sales methods. Selection and source provenance lead, not metrics or decorative overview cards.

OWN-WORLD: Inherit AgentDesk's neutral light operational shell, semantic tokens, system UI typography, compact tables, Lucide icons and rounded-md controls. No new visual identity or global token changes.

STORY: Select conversations; import customer-grouped snapshots; inspect cases; extract pending rule changes; confirm, edit or ignore them; compare one current Skill against no Skill; retain a human assessment. One row per Skill, no revision picker or baseline concepts.

FIRST VIEWPORT: Compact title followed by four line tabs. Conversation sources open first, with search/channel filters, selection count and import action above a paginated table. Dense stable rows show customer, channel, message count and date. Mobile keeps primary action reachable and tables scroll locally.

FORM: User-specified extension of the existing dashboard and lab workflow. No concept tournament or seed: precise incumbent-world feature. Code-led. Signature interaction: importing keeps the selected cases and moves directly into the cases tab; starting work opens its durable progress/result view. Motion follows existing focus, tabs and loading conventions.

FINISH: unreviewed and undocumented is unfinished; this build ends with the finish review, the verdict, DESIGN.md, and every shipping raster carrying its provenance

Verification: bilingual desktop/mobile, selection, import, extraction, evidence, editing, ablation and error/retry states. Ordinary extension preserves existing design artifacts; no shipping raster required.

Long-running extraction extension: preserve the task dialog and dashboard shell.
Show real worker heartbeat separately from model activity, current segment, call
duration and received characters. Waiting is not failure; stale heartbeats or a
failed status request explicitly leave health unconfirmed. Stop/resume remains
available without discarding completed windows. No fabricated completion estimate.
Chinese Skill authoring preserves original-language quotes and does not alter
customer-facing reply language. Verify waiting, generating, stale/disconnected,
stop failure, stopped and resumed states in both locales at 1440 and 390 pixels.

Current knowledge and pending changes share a focused Skill dialog. Pending items
show additions, edits, evidence, and conflicts against current rules. Batch approval
is scoped to visible non-conflicting non-removal changes and is atomic. Manual edits
and approval never publish to live agents. Internal history remains out of the UI.
