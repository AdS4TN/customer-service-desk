---
version: 1
slug: "web-app-dashboard-dashboard-ai-agents-config-workbench-tsx"
primary_target: "web/app/(dashboard)/dashboard/ai-agents/_components/config-workbench.tsx"
related_targets: ["web/app/(dashboard)/dashboard/ai-agents/page.tsx", "web/app/(dashboard)/dashboard/ai-agents/editor/page.tsx", "web/app/(dashboard)/dashboard/ai-agents/_components/employee-preview.tsx", "web/app/(dashboard)/dashboard/ai-agents/_components/employee-resources.tsx", "web/app/(dashboard)/dashboard/ai-agents/_components/reception-state.tsx"]
---

# AI Employee Workspace

Mode: Operate. This user-approved, Mixdesk-inspired employee workspace extends the incumbent AgentDesk dashboard. Its visual authority remains the existing dashboard stylesheet and shared controls; the approved change concerns this workspace's composition and workflow.

## Direction contract

THESIS: Configure and test one AI employee from an independent editor, with draft capabilities, published capabilities, and effective reception state clearly distinguished.
OWN-WORLD: Inherit semantic dashboard colors, Geist-based typography, compact Base UI controls, restrained borders, and Lucide icons. No new global visual identity.
STORY: Select an employee, edit persona or bind knowledge and experience, test the current draft, inspect response evidence, then save and publish deliberately.
FIRST VIEWPORT: Employee name, publication state, return action, and version history precede four sections. Persona and goals open first; on wide screens a private test conversation remains alongside configuration.
FORM: A full-page workbench within the dashboard shell, with horizontal section navigation, independently scrolling configuration and trial regions, and a persistent save/publish footer. Smaller screens switch between configuration and trial.
FINISH: This brief records the implemented workspace and supplied finish evidence. Browser-history cancellation/acceptance and stopping trial requests passed all four locale/viewport runs; the finish reviewer scored those fixes resolved. This extension establishes no global design tokens or sidecar.

## Implemented layout

- The employee list opens `/dashboard/ai-agents/editor?id=<id>`; omitting the ID creates an employee. The editor is not a configuration modal.
- Persona/goals, knowledge/experience, business capabilities, and reception arrangements use shared line tabs from 768px. Below that width, the section selector is the existing OptionCombobox.
- At 1280px and above, configuration sits beside a 380px trial column, capped at 40% of the work area; the trial column becomes 440px at 1536px. Below 1280px, shared segmented tabs switch between configuration and trial without unmounting the conversation.
- Configuration has an 896px maximum inner width, with 16px padding increasing to 24px at 1024px. Form groups are unframed; borders divide major regions and repeated resource rows.
- Header, section controls, and footer stay outside the scrolling body. The trial composer stays below its own scrolling conversation. Below 640px, footer actions use two columns with publish spanning both.
- Employee name and persona lead the configuration view. Reception policy, public identity, and advanced model settings are progressively disclosed. Reception details retain a two-column definition list and wrapping channel rows.
- The local type hierarchy uses 16px employee/trial/runtime headings, 14px section headings, body and navigation, and 12px supporting text. Inputs retain shared 16px narrow-screen and 14px desktop sizing.

## Incumbent system

- Palette: semantic background, foreground, primary, muted, border, and destructive roles resolve through `web/app/(dashboard)/dashboard.css`. The plain light palette uses the existing blue primary (`#2475fc`), near-white background (`#f9f9f9`), dark text (`#151923`), and pale borders (`#dfe7f3`); other palette and dark variants remain stylesheet-owned.
- Type: the dashboard's Geist sans stack and shared weight hierarchy remain authoritative. The workspace adds no display face or decorative type treatment.
- Shape and depth: the layout relies on unframed surfaces and dividers. Existing shared controls own corner, focus, hover, disabled, and overlay treatments; the dashboard radius base remains `0.3rem`.
- Controls: Button, Badge, Input, Textarea, Field, Tabs, OptionCombobox, and ProjectDialog retain their existing APIs. Trial messages reuse Message, Bubble, and MessageScroller primitives. Lucide icons accompany commands and provide named icon-only actions.
- Scope rule: this is a surface extension. Do not infer new global tokens or a dashboard redesign from its split editor/trial composition.

## State and action rules

- Draft rule: save writes employee capabilities with `capabilitiesOnly: true`; it preserves live reception mode and rollout state, including concurrent reception changes. Publication and rollback select capability revisions and do not reactivate reception. New employees default to human-only mode.
- Reception rule: effective values come from the reception-state response. Loading, failed, empty-channel, paused, and outbound-blocked states retain their own meaning. Channel connection and reception assignment remain in their existing workflows; the current human-only conversations and WhatsApp outbound hold remain intact.
- Trial rule: the private text-only, non-streaming request uses the current unsaved draft and local trial history. It mounts selected Skill instructions and retrieves selected knowledge, without running workflows, MCP tools, handoffs, or other business actions. It creates no conversation, message, or outbox records and is not an exact production Agent Loop simulation.
- Evidence rule: responses expose the model, retrieved source excerpts, and mounted Skills. These are inputs supplied to the trial, not proof that the model used every instruction. Empty knowledge selection means no knowledge is bound.
- Trial pending state prevents duplicate submissions and exposes stop. Failure or stop restores the question for retry. Clear resets the local conversation; draft or selected Skill changes mark earlier evidence as stale for the current configuration. Trial history lasts only within the current editor.
- Resource rule: knowledge detail and document ingestion open existing pages in a separate browser tab, preserving the employee draft. Skills are created and edited from focused dialogs inside the workspace; edits to an existing shared Skill require confirmation because they affect that shared resource immediately. Binding remains an employee draft change.
- Experience import previews a selected active revision and creates a Skill from its conditions, actions, and exceptions, retaining the source revision ID/hash. The new Skill is selected in the employee draft; publishing remains explicit.
- Successful draft writes commit the saved baseline and refresh runtime independently of publication metadata reads. A metadata read failure produces an inline alert with read-only retry, without repeating the write or resetting the draft.
- Save/publish disables the editable fieldset while pending. Unsaved state stays visible in the footer. Return and app-link navigation confirm draft discard, modern-browser same-document history traversal uses the Navigation API guard, and page unload retains the native warning. Skill-dialog close separately protects unsaved Skill text.

## Evidence and limits

Source checked: the primary and related targets, `.impeccable/ai-employee-direction.md`, `PRODUCT.md`, `AI-CUSTOMER-SERVICE.md`, `internal/services/ai_employee_preview.go`, `web/app/(dashboard)/dashboard.css`, and shared Button, Input, and Tabs source files. The shared Tabs implementation was also compared with HEAD to establish that its styling predates this workspace.

Finish evidence is stored under `.impeccable/review/ai-employee/`: both `zh-CN` and `en-US`, at 1440px and 390px, covering persona, trial, knowledge, and reception. This documentation pass visually sampled `zh-CN-1440-persona.png`, `en-US-390-trial.png`, `en-US-1440-knowledge.png`, and `zh-CN-390-reception.png`. These are synthetic QA captures, not production records or shipping image assets. They show the inherited dashboard controls, responsive region changes, draft/evidence state, and reception hold presentation. They do not verify a live model provider or every palette variant.

Not canonized or repaired: shared controls already use large-radius tokens and the default segmented Tabs already adds an active shadow, while AGENTS.md calls for medium corners and no added support-platform shadows. This pre-existing primitive/rule drift is outside the documentation-only boundary and is not a new design rule. The obsolete rail-label guidance is removed with the replaced rail. Root `DESIGN.md` and `.impeccable/design.json` are absent and were not created by this surface-only extension.
