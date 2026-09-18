# Selective Conversation Delegation

Selective delegation gives an AI employee a temporary mandate for one conversation while retaining the customer's salesperson. It extends the existing inbox and Copilot; it does not introduce automatic customer routing or a workflow canvas.

## Current Operating State

The production WhatsApp messaging hold remains in force. Conversations and agents remain human-only, and WhatsApp outbound text/media is blocked. The available workflow is a no-send trial: generated replies are private delegation records and never customer messages. This work does not enable live sending, change the production channel configuration, or authorize customer-send tests.

The implementation includes a future live path for eligible web, Messenger, and WhatsApp conversations. Eligibility requires an enabled channel and AI employee, compatible service modes, and the applicable transport checks. The WhatsApp hold still blocks that path. A selected employee's `liveAvailable` value controls the UI switch, and the backend checks eligibility again at start and before delivery.

## Operator Flow

1. Open a conversation in the existing inbox. The compact delegation strip above the work-status strip shows the customer owner and current mode.
2. Open **AI delegation**. An active conversation must be assigned to its customer owner before a mandate can start. The owner, or an administrator acting within that assignment, also needs `conversation.send` permission. Otherwise the dialog asks the operator to claim the conversation first.
3. Choose an enabled AI employee, enter the optional task, and choose a duration in minutes. The default is 60 minutes; 0 means manual ending, and the supported maximum is 43,200 minutes. Changing the employee resets the form to a no-send trial.
4. Start the trial. The current last message becomes the baseline, so starting does not automatically replay older customer messages. **Preview current reply** explicitly evaluates the current customer text, including an imported historical message. Subsequent eligible incoming text can be evaluated by the active trial.
5. Inspect **Recent delegation history** for private replies, start/stop records, and return reasons. The trial state explicitly says no messages are sent. **Take back** is available both in the dialog and directly in the conversation strip, including while a preview is generating.

The dialog preserves its unsaved mandate when dismissed and after a failed submission. Switching to another conversation creates a separate component instance. Pending mutations disable their initiating controls; errors remain close to the action, and loading failures expose retry. The dialog body scrolls within a 90dvh limit while the footer remains reachable on narrow screens. All interface copy and accessible labels use the existing Chinese and English localization system.

## Ownership And Ending

`Customer.OwnerUserID` remains the durable salesperson identity. When it is absent, delegation resolves the current assignee as owner and initializes the empty customer owner field on start. An existing customer owner is never replaced by starting, ending, or handing back a mandate. The conversation's current assignee and normal AI employee assignment are also retained; the temporary employee lives on the delegation record.

Starting requires the active conversation's current assignee to match the resolved customer owner. The owner, current assignee, or administrator can reclaim an active mandate with send permission. View access requires conversation-view permission and the existing owner/assignee/admin scope checks.

An active mandate ends on explicit reclaim, a human reply in the inbox, a live non-passive reply from the linked phone/account, conversation close, transfer, customer relinking or other ownership/assignment changes, or expiry. Historical self-message imports do not count as a new human takeover. Unsupported incoming content, generation or delivery failure, loss of sending eligibility, and an AI handoff decision also return control. Automatic returns record the reason and create an internal notification for the owner.

Stopping increments the mandate revision, cancels in-flight generation, and invalidates pending or failed delegation outbox items. A later stale result cannot revive the old mandate. A conversation that has entered delegation remains marked `DelegationManaged`, so the general automatic reply runtime cannot become an unintended fallback after reclaim or expiry.

## AI Behavior And Privacy

The selected employee's published agent/configuration snapshot supplies the model, system prompt, reception policy, and knowledge-base selection through the existing Copilot service. The input includes recent eligible messages, reception memory, retrieved knowledge, and the salesperson's mandate. Retrieval is a read operation; this path does not run the agent tool loop, MCP tools, Skills, or workflow actions.

The model must return a structured reply or handoff decision. The prompt directs it to hand back requests for a human, approval decisions such as negotiated discounts, material questions without sufficient evidence, and work beyond the mandate. A handoff contains an internal reason and no customer reply. It cannot execute refunds, orders, discounts, scheduling, or other external actions.

Trial output is stored in `ConversationDelegationEvent.Content`, outside the customer message table and channel outboxes. The latest 50 records are returned through the permission-checked delegation endpoint. Model results are rejected when relevant messages, publication, assignment, customer linkage, or reception memory change during generation. Mandate revision and delivery checks provide a second validation before persisting a preview or sending through an eligible future live path.

## Implementation Map

Paths below are relative to the repository root.

| Layer | Source | Responsibility |
| --- | --- | --- |
| Inbox UI | `web/app/(dashboard)/dashboard/conversations/_components/conversation-delegation.tsx` | Owner/mode strip, mandate form, trial preview, reclaim, and private history |
| Integration | `web/app/(dashboard)/dashboard/conversations/_components/chat-panel.tsx` | Mounts the delegation strip before the incumbent reception work bar |
| API client | `web/lib/api/conversation-delegation.ts` | Uses the shared authenticated request client |
| HTTP boundary | `internal/handlers/dashboard/conversation_delegation_handler.go` | Permissions, request parsing, localized failures, and response builder |
| Route registration | `internal/bootstrap/routes.go` | Explicit delegation routes beneath `/api/dashboard/conversation/:id` |
| Domain service | `internal/services/conversation_delegation_service.go` | Ownership, mandate lifecycle, bounded processing, revision validation, and handoff notifications |
| Persistence | `internal/models/conversation_delegation.go`, `internal/repositories/conversation_delegation.go` | Mandate state, private events, initial owner assignment, and outbox cancellation |
| AI generation | `internal/services/conversation_copilot_service.go` | Published snapshot, memory/knowledge reads, reply/handoff parsing, and stale-result rejection |
| Message boundary | `internal/services/message_service.go`, `internal/services/whatsapp_service.go` | Final delivery validation, human/phone takeover, transport hold, and delegation outbox handling |
| Background processing | `internal/services/cronx/cron.go` | Scans active mandates every 3 seconds; the service limits concurrent processing to four conversations |

The explicit endpoints are `GET /delegation`, `POST /delegation/start`, `POST /delegation/stop`, and `POST /delegation/preview` under the conversation prefix. Mutations carry the current revision. Transactions keep ownership checks, state changes, events, and cancellation consistent. A per-conversation delivery lock serializes takeover with the final send rather than holding a lock during model generation. The frontend refreshes visible delegation state every 5 seconds and resynchronizes the existing conversation store after mutations.

## Verification

The implementing agent completed these checks with synthetic data and mocked transports. No live customer messages were used for verification.

From `web`:

```sh
pnpm build
pnpm --config.verify-deps-before-run=false exec eslint 'app/(dashboard)/dashboard/conversations/_components/conversation-delegation.tsx' lib/api/conversation-delegation.ts
node delegation.browser.mjs
```

The browser command was run with `PLAYWRIGHT_MODULE` set to an external installed Playwright module. It serves the built `out` directory on a temporary loopback port and intercepts API requests with synthetic fixtures. It passed all four combinations of `zh-CN`/`en-US` and 1440px/390px viewports, including disabled live sending, draft retention, failed-start retry, preview, reclaim during generation, restart, no customer-send API calls, no browser errors, and no page overflow. The temporary browser and server close after the run.

From the repository root:

```sh
go test -race ./internal/services ./internal/ai/runtime -run 'TestDelegation|TestCopilot|TestReception|TestWhatsApp|TestMessenger|TestReplyEligibility|TestHumanOnlyConversationDoesNotSendWelcome' -count=1
go test ./internal/bootstrap ./internal/repositories -count=1
```

Both commands passed; the final race run completed the services package in 18.253s and runtime package in 2.035s. The broader services test run has one pre-existing obsolete assertion, `TestConversationHumanDispatchHumanOnlyCreateOffHoursUsesGlobalPendingPool`, which expects an automatic waiting message contrary to the production messaging hold. The full Go suite is not claimed green.

Visual and interaction evidence is in `.impeccable/review/delegation/`. The finish reviewer returned `ship` with no material fixes. The design preservation record is `.impeccable/review/delegation/documentation.md`.
