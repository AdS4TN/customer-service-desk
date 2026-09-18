---
version: 1
slug: "web-app-dashboard-dashboard-conversations-page-tsx"
primary_target: "web/app/(dashboard)/dashboard/conversations/page.tsx"
related_targets: []
---

# Reception workflow extension

## Direction contract

THESIS: Make outstanding customer work explicit in the existing unified inbox. Read state remains separate from reply obligations.

OWN-WORLD: Inherit AgentDesk's restrained neutral dashboard, semantic accents, compact Lucide controls, Base UI primitives, rounded-md corners and no new shadows.

STORY: Operators select a work queue, read a conversation, reply or snooze, and privately collaborate without sending internal text to customers.

FIRST VIEWPORT: Preserve channel filters and conversation list at left, conversation in the center and customer dossier at right. Add a compact handling-status strip above messages; expose private collaboration from that strip. Narrow screens keep the existing single-conversation layout.

FORM: Local extension of an established surface; no concept seed or visual identity change. Protected-focus dialogs hold settings and private notes with drafts retained on dismissal.

FINISH: unreviewed and undocumented is unfinished; this build ends with the finish review, the verdict, DESIGN.md, and every shipping raster carrying its provenance

Scope: no new visual system or imagery, no translation or reporting in this batch. Preserve existing design artifacts. Validate notification, work state, privacy, delivery and mobile controls.

## Contact profile extension

Preserve the existing reception layout. In the Customer tab, show the channel avatar and name, a compact sync command for WhatsApp, and an explicit edit-customer command. Reuse the customer form for local display name, notes, and customer-level manual tags. Keep these tags distinct from conversation tags and AI suggestions. Sync must never overwrite operator notes or tags. Support missing/private avatars, cached images on network failure, pending/retry feedback, long tags, both locales, and narrow screens. No new visual system or illustration is introduced.

## Selective delegation extension

Mode: Operate. Local extension; seed and comp not applicable.

THESIS: Keep the customer's salesperson visible while AI temporarily handles one conversation.

OWN-WORLD: Preserve the incumbent neutral dashboard, semantic tokens, compact Lucide controls, Base UI components, rounded-md and no additional shadows.

STORY: Inspect the owner, choose an AI employee and mandate, start a no-send trial or an explicitly permitted delegation, then reclaim without changing customer ownership. Inspect private results and return reasons.

FIRST VIEWPORT: Add one compact delegation strip above the existing work-state strip. Owner and current mode remain visible; reclaim is directly available. A focused dialog holds mandate configuration and recent private records. Narrow viewports wrap the strip and scroll the dialog body, retaining reachable actions.

FORM: Code-led extension, no visual-world change. Preserve unsaved mandate text on dismissal and failure. Trial status must explicitly say no messages are sent. Synthetic browser tests cover both locales, desktop and mobile, start/stop, pending and retry.

FINISH: unreviewed and undocumented is unfinished; this build ends with the finish review, the verdict, DESIGN.md, and every shipping raster carrying its provenance

## Copilot evidence extension

Mode: Operate. Local extension; no visual-world or design-system change.

THESIS: Keep AI reply assistance visibly private and inspectable before an operator chooses to insert it.

OWN-WORLD: Reuse the incumbent conversation-side panel, semantic text and badge variants, compact Lucide labels, separators, existing type scale, and unshadowed stacked content.

STORY: Generate a private draft, inspect the effective employee and model plus Skill routing, knowledge retrieval, bounded tool activity, sources, and timing, then insert only while the conversation and operator eligibility are still current.

FIRST VIEWPORT: Keep the draft primary. Place a compact execution trace below it in the existing Copilot region, using wrapping rows and badges rather than cards or new navigation so long names and narrow screens remain readable.

FORM: Label the output as shadow mode and draft, distinguish unavailable, empty, matched, and failed evidence states, and state the external-tool boundary explicitly. Preserve cancellation, stale-result rejection, takeover requirements, and the existing insert-only workflow. Support both locales and narrow viewports without hiding evidence or actions.
