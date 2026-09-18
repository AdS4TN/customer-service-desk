# AI Customer Service Configuration

## Ownership

The AI Customer Service navigation group contains the service workbench,
knowledge bases, Skills, sales experience and run history. Provider credentials
and MCP server administration belong to System Settings. Channel connection and
human reception remain independent modules; this is a modular monolith, not a
new service deployment.

The workbench separates saved runtime state from the editing draft:

| Concern | Source of truth | Effective time |
| --- | --- | --- |
| Incoming messages | Channel status and connector session | Channel save/connect |
| Automatic reception | Live Agent status/service mode and conversation service mode | Save |
| Identity, prompts, knowledge, Skills, tools, handoff strategy | Published Agent revision | Publish/rollback |
| Translation and private reply suggestions | Published model/capability; current model status and credentials | Published behavior, live model availability |
| WhatsApp outbound hold | Existing `whatsapp.OutboundMessagesDisabled` | Unchanged by this work |

Agent disable pauses automatic reception, not private assistance or incoming
conversation creation. New conversations for a disabled Agent enter human-only
reception. Deletion still invalidates the Agent, and disabling its model still
disables model calls. Assistance never invokes business tools or dispatches a
customer message.

## Runtime Boundary

`reception.AutomaticMessagesAllowed` is shared by reply eligibility, welcomes,
service notices, delegated reception, final message persistence and linked-channel
outbox dispatch. Assignment, delegation revision, rollout and channel transport
checks remain with the existing owners. This function does not replace them.

`applyRevisionAgentSnapshot` intentionally leaves `ServiceMode` and `Status`
unchanged. Old revisions may retain the serialized service mode for audit, but
it cannot reactivate reception. Legacy API configuration saves can still update
live mode. Employee editor saves use `capabilitiesOnly: true`, preserving live
service mode, rollout and previous rollout values even after concurrent changes.
Existing human-only conversations are not migrated into automatic reception.

`GET /api/dashboard/ai-agent/:id/reception_state` reports saved runtime settings,
published model/resources and bound channel receiving/sending policy. It does
not report socket connectivity or guarantee delivery. It requires AI Agent view
permission and never returns credentials or provider failure details.

## Employee Workspace

`/dashboard/ai-agents` lists employees. Selecting a name opens the independent
`/dashboard/ai-agents/editor?id=<id>` page; omit the ID to create an employee.
Four sections group persona/goals, knowledge/experience, business capabilities
and reception arrangements. Desktop shows private trial chat alongside editing;
small screens switch between configuration and trial while retaining trial state.
App navigation and modern-browser history traversal confirm before discarding
unsaved configuration. History guarding uses the Navigation API; page unloads
also retain the native `beforeunload` warning.

Knowledge selection is explicit: an empty array unbinds all bases. Only an omitted
legacy API field retains default-base behavior. Document management opens the
existing knowledge pages in a separate tab. Skills can be created, edited or
bound here; shared edits require confirmation. Extracted experience imports the
selected active revision as a Skill, recording its source ID/hash. Binding a
Skill changes the employee draft, not its published revision.

`POST /api/dashboard/ai-agent/preview` requires Agent update permission and uses
the submitted unsaved draft and client-supplied conversation, not a live customer
conversation or published Agent snapshot. It shares foundational reply/language
instructions and retrieval with the existing service, and includes selected
Skill documents with their conditions. Returned evidence lists retrieved sources
and mounted Skills, not proof that the model used every instruction.

Trials are text-only, non-streaming model calls, with pending/stop/retry states.
They do not execute workflows, MCP tools, handoffs, or other business actions;
they create no conversation, message or outbox records. This is a capability
preview, not an exact simulation of the production Agent Loop. Provider requests
have a 90-second context timeout. Trial history exists only in the current editor.

## Verification

- `go test ./internal/services ./internal/ai/runtime ./internal/whatsapp ./internal/pkg/reception`
- `cd web && pnpm --config.verify-deps-before-run=false run typecheck`
- `cd web && pnpm --config.verify-deps-before-run=false run build`
- `cd web && node ai-employee.browser.mjs` (Playwright and Chrome required;
  `PLAYWRIGHT_MODULE` can name an installed Playwright module).

Browser tests use synthetic API responses only. Do not test customer sends on
the connected production WhatsApp account. The current human-only settings and
outbound hold must stay in place until the user explicitly changes that scope.
