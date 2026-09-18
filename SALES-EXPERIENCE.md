# Sales Experience Workbench

The dashboard at `/dashboard/sales-experience` reuses AgentDesk conversations and
enabled language-model configurations. It does not publish Skills to live agents
and has no channel-send or outbox integration.

## Workflow

1. Select conversations across channels/statuses and import. Selected conversations
   belonging to the same customer become one case. Unknown customers remain separate.
2. Inspect the immutable history snapshot and optionally annotate the actual sales
   outcome. Reimporting unchanged selections reuses the snapshot; changed history
   creates a new snapshot. Separate imports are not silently merged.
3. Extract sales events with exact source quotations. Full histories are processed
   in overlapping whole-message windows. AI replies are excluded unless selected;
   attachment contents are not inferred. Only locally synchronized history is available.
   Summaries, event analysis and newly authored Skill rules use Simplified Chinese;
   exact source quotations retain their original language. A retried task with an
   older extraction prompt version restarts extraction instead of reusing old-language
   checkpoints. Saved case snapshots and existing immutable Skill versions are retained.
4. Propose additions, revisions, exceptions or supporting evidence for negotiation,
   decision and closing Skills. Rules describe observable customer state and one
   reusable sales mechanism, not the product, quotation, certification, shipping,
   payment or document workflow seen in the source case. Every substantive proposal
   must pass the same-wording transfer test in three unrelated industries; a single
   case may produce at most two substantive changes for each Skill. Concrete facts
   remain attached as evidence instead of entering the Skill wording. The default is
   to learn nothing: most conversations should complete with no Skill proposal. A
   proposal is accepted only when the seller made a non-routine sales move, a later
   quoted buyer message shows a meaningful effect, the underlying mechanism transfers
   beyond this transaction, and it adds a new technique, boundary or strong evidence.
   Routine replies, requirement collection, quotation, scheduling, status updates,
   payment/shipping/document handling, politeness and unanswered seller messages do
   not pass this gate merely because the conversation continued or the order closed.
5. Each Skill has one current, confirmed rule set. Extraction produces pending
   additions, edits and supporting evidence, grouped by extraction time. Accept,
   edit-and-accept or ignore each item. Batch confirmation is atomic and excludes
   deletions and visible conflicts. Stale edits require refreshing; target conflicts
   require explicit editing against the displayed current rule. Export current
   knowledge as a ZIP containing `SKILL.md`, rules, evidence and provenance.
   Internal immutable snapshots remain for provenance, but are not a user-facing
   version workflow. A newer unreviewed extraction of the exact same case scope and
   confirmed baseline supersedes its older unreviewed proposal; partially reviewed
   proposals are never superseded. Subsequent extraction uses only confirmed current knowledge.
   Manual edits retain historical evidence as lineage, not validation of new text.
6. Choose one case, one Skill and a customer message cutoff. Compare the
   selected Skill against no Skill using the same conversation prefix, business
   context and model. A/B assignment is hidden until human assessment is saved.
   Future replies and recorded outcomes are excluded from the model input.

The three initial Skill categories match the independent lab. Custom categories,
automatic production mounting, standalone-lab data migration and attachment OCR
are not part of this integration. The snapshots use a channel-independent 1.0
schema so future import adapters can reuse the same extraction layer.

## Execution

Four `SalesExperience*` models are registered for startup migration. Jobs run in
the existing background scheduler. Per-window and per-reply checkpoints persist;
interrupted jobs require explicit retry. Changing model configuration resets old
checkpoints on retry to prevent mixed-model results. New pending suggestions and successful
job completion commit atomically. Large individual messages or accumulated rules
can still exceed a provider's context window; model limits are not bypassed.

Background distillation has no local completion deadline and no automatic model
retry. It uses streaming completions, persisting worker heartbeats every two
seconds independently of model activity. The task view shows the current segment,
per-call elapsed time, latest model response, phase and received output characters.
Reasoning text and incomplete JSON are never exposed or saved as Skill results.
An overdue heartbeat or failed status request means status is unconfirmed, not
that a slow model has failed. Provider errors/disconnections still fail the task;
removing the local timeout cannot override a provider's own limits.

Stopping a queued/running distillation marks it cancelled and interrupts the model
request on the next heartbeat. Completed windows remain available for explicit
resume; the interrupted call runs again. Cancelled attempts cannot publish late
results. Service restarts mark running jobs interrupted rather than silently
resuming them. Interactive evaluation/reply/translation timeout policies are
unchanged. Chinese authoring does not change the customer-language evaluation
reply policy.

Viewing requires `conversation.view` and `skillDefinition.view`; mutations require
`conversation.view` and `skillDefinition.update`. Extraction sends the selected
history to the user-selected configured model provider.

## Verification

Focused Go tests live in `internal/pkg/salesexperience/pipeline_test.go`,
`internal/services/sales_experience_service_test.go`, and
`internal/builders/sales_experience_test.go`. They use synthetic SQLite data and a
mock model, including import deduplication, histories over 50 messages, evidence
validation, checkpoint retry, immutable versions, ZIP export and blinded ablation.
`internal/services/sales_experience_progress_test.go` and
`internal/ai/llm_stream_test.go` cover streaming, real activity counters, local
deadline removal, cancellation, resume and upstream failure with synthetic data.

`web/sales-experience.browser.mjs` serves the static build on an ephemeral port,
mocks API calls and exercises Chinese/English at desktop/mobile widths. Set
`PLAYWRIGHT_MODULE` when Playwright is outside the web project's dependencies.
No real customer chats or channel sends are used by these tests.

Review tests in `internal/services/sales_experience_review_test.go` cover explicit
accept/ignore, stale edits, target conflicts, atomic batches, duplicate extraction
supersession, and no outbound messages.
`scripts/localize-sales-experience.mjs data/app.db MODEL_ID` is an optional one-time
repair for old English-authored pending results. It translates authored fields via
the configured provider, preserves quotes and immutable originals, backs up the
database, and writes only a review overlay with optimistic concurrency checks.
`scripts/generalize-sales-experience.mjs data/app.db MODEL_ID` similarly rewrites
only pending review overlays into one or two mechanism-level techniques, requires
three transfer tests per rule, preserves immutable extraction output and exact
quotes, and creates a database backup before its atomic update.
