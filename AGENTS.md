# AGENTS.md

This file defines the mandatory working agreement for AI agents in this repository. It is intentionally based on the current codebase rather than historical conventions.

## 1. Scope and Priorities

- These rules apply to the repository root and every subdirectory.
- Explicit user instructions take precedence over this file. Mention any deliberate deviation in the final summary.
- Inspect the relevant implementation before editing. Reuse current helpers, component APIs, generated-code workflows, and neighboring patterns instead of relying on memory.
- Keep changes narrowly scoped. Preserve unrelated and user-owned worktree changes, including staged changes.
- A review, investigation, or diagnosis request is read-only unless the user also asks for implementation.

## 2. Current Architecture

The repository contains three application areas:

- Go server: `cmd/server` and `internal/*`
- Next.js application: `web/*`
- Embedded workflow editor: `flowgram-editor/*`, built into `web/public/flowgram-editor`

The main stack is:

- Go 1.26, Gin, GORM, `github.com/mlogclub/simple`
- SQLite and MySQL
- Next.js 16 App Router, React 19, TypeScript, Tailwind CSS, shadcn/Base UI
- `pnpm` for both frontend projects
- Optional LanceDB builds through CGO; Qdrant is also supported by the application

Important entry points:

- Server assembly and middleware: `internal/bootstrap/server.go`
- Explicit API routes: `internal/bootstrap/routes.go`
- Model registration: `internal/models/models.go`
- Schema/data migration startup: `internal/bootstrap/migration.go`
- CRUD generator: `cmd/generator/generator.go`
- Frontend enum generator: `cmd/enums/generator.go`
- Frontend API client: `web/lib/api/client.ts`
- Frontend i18n: `web/i18n/*` and `web/messages/*`
- Dashboard shared components: `web/components/dashboard/*`
- Project commands: `Taskfile.yml`

## 3. General Change Rules

- Read the actual type, function, or component signature before using it.
- Prefer the highest-level existing abstraction that fits the requirement. Do not duplicate query state, pagination, auth refresh, localization, or dashboard CRUD behavior.
- Do not edit generated artifacts by hand. Change their source and run the corresponding generator/build command.
- Do not add a second implementation style when the repository already has a shared path for the same concern.
- Use `log/slog` for new Go logging and structured key-value fields for relevant context.
- New Go code uses `any`, not `interface{}`. Existing generated or legacy code does not need unrelated cleanup.
- Secrets, tokens, credentials, and private customer data must never be committed or printed in logs/tests.

## 4. Go Backend

### 4.1 Layer Ownership

The normal dependency and data flow is:

`models -> repositories -> services -> handlers -> builders/response DTOs`

- `internal/models`: entity fields, GORM mappings, associations, and schema metadata only.
- `internal/repositories`: GORM/SQL access, conditions, ordering, pagination, locks, and persistence details.
- `internal/services`: business validation, state changes, authorization-independent domain rules, aggregation, transactions, and event orchestration.
- `internal/handlers/{api,dashboard,third}`: HTTP parameter parsing, authentication/permission checks, service calls, and response writing.
- `internal/builders`: pure model/aggregate-to-response mapping. Builders must not query the database.
- `internal/pkg/dto/request` and `internal/pkg/dto/response`: external request/response contracts.

Mandatory boundaries:

- Handlers must not call repositories or issue GORM queries directly.
- Models and repositories must not contain HTTP, permission, or cross-resource workflow logic.
- GORM models must not be returned directly from an API. Map them to response DTOs/builders.
- A service may return models internally, but public response shape remains owned by builders/DTOs.
- Put reusable SQL in repositories. A genuinely one-off aggregate query may stay near its domain service only when extraction would make ownership less clear.

### 4.2 Database and Transactions

- Repository methods that participate in transactions accept `db *gorm.DB`; call them with `sqls.DB()` outside a transaction and `ctx.Tx` inside one.
- Services own transaction boundaries through `sqls.WithTransaction`.
- Use a transaction for atomic multi-write workflows and consistency-sensitive read-modify-write operations.
- Do not add a transaction around a single independent SQL write.
- Every database operation inside a transaction must use the same `ctx.Tx`; never escape to `sqls.DB()` mid-transaction.
- Use `sqls.Cnd`/`sqls.NewCnd()` and repository methods for ordinary filtering and pagination.
- Preserve SQLite and MySQL compatibility. Avoid dialect-specific SQL unless both dialects are explicitly implemented and tested.
- Use portable column types and `int64` primary/foreign identifiers. Keep time handling compatible with MySQL `parseTime=True`.

### 4.3 Models, Generation, and Migrations

- Register persistent models in `internal/models/models.go` so startup `AutoMigrate(models.Models...)` includes them.
- New tables, columns, indexes, and compatible constraints are normally applied by GORM `AutoMigrate` in `internal/bootstrap/migration.go`.
- Use `internal/migration/*` only for versioned, idempotent data migration/backfill/repair work. Its version must increase monotonically.
- When a model uses the standard generated repository/service surface, register it in `cmd/generator/generator.go` and run `task generator`.
- Treat generator output as mechanical infrastructure. Put business-specific methods in handwritten files and do not manually patch generated CRUD output.
- Backend/frontend shared enums are defined in `internal/pkg/enums`, annotated using the existing enum pattern, and generated with `task enums` into `web/lib/generated/enums.ts`.
- Never create a handwritten frontend duplicate of a generated backend enum.

### 4.4 HTTP APIs

- All routes are explicit in `internal/bootstrap/routes.go`; handler names do not create endpoints.
- Public/product APIs live under `/api/*`, authenticated management APIs under `/api/dashboard/*`, callbacks under `/api/third/*`, and WebSockets under `/api/ws/*`.
- Add a resource-specific `register...Routes` function or extend the existing one, then mount it from `addRouter` under the correct group.
- Follow the existing resource contract: detail commonly uses `GET /:id`, list uses `/list`, writes use explicit POST actions such as `/create`, `/update`, `/delete`, and domain actions retain their established snake_case path names.
- Do not introduce `/api/v1`, automatic routing assumptions, or unnecessary deeply nested resource paths.
- Handler names mirror the registered method and path, for example `XxxGetBy`, `XxxAnyList`, and `XxxPostCreate`.
- Parse JSON/form/query/path values with `internal/pkg/httpx/params` and `internal/pkg/httpx` helpers.
- Dashboard permission checks use `services.AuthService.RequirePermission` or the established permission helper before domain work.
- Write responses through `httpx.WriteJSON`; preserve the shared `JsonResult` contract.
- Paginated responses use `web.PageResult` with `data.results` and `data.page`.
- Convert not-found, validation, permission, and persistence failures into stable application errors. Never expose raw SQL errors to clients.

### 4.5 Backend Internationalization

- Any new or changed user-visible backend error must support every backend locale, currently `zh-CN` and `en-US`.
- Add matching keys to both `internal/pkg/i18nx/locales/zh-CN.yml` and `internal/pkg/i18nx/locales/en-US.yml` in the same change.
- Services should return localized application errors through the `errorsx.*I18n` helpers.
- Handlers that need an immediate localized response use `httpx.JsonErrorMsg(ctx, key, args...)`; other request-context translation uses `i18nx.T`.
- Do not hard-code Chinese or English error sentences in handlers/services when the text can reach a user.
- Keep format arguments equivalent across locales and cover new reusable/error-format behavior with focused tests.

## 5. Frontend

### 5.1 Component and Data Boundaries

- Application routes live under `web/app`; reusable business components live under `web/components`; route-private components live in the route's `_components` directory.
- Reuse `web/components/ui/*` primitives. Do not edit these shadcn base components for a feature-specific requirement.
- Use the current Base UI/shadcn component API as implemented in the repository; do not assume APIs such as Radix `asChild` exist.
- Use `@/*` imports for code within `web`.
- Client components must declare `"use client"` when they use state, effects, browser APIs, or client-only hooks.
- Keep resource APIs in `web/lib/api/*` and route all normal requests through `web/lib/api/client.ts`.
- Pages, business components, and stores must not implement their own `JsonResult` parsing, auth header handling, token refresh, or login-expiry cleanup.
- Direct `fetch` is reserved for unsupported transports such as third-party calls, binary transfers, SSE, and WebSocket handshakes; explain the exception in code.
- Prefer `OptionCombobox` for standard dropdowns rather than adding shadcn Select-based business controls.
- Format displayed timestamps with `formatDateTime` from `web/lib/utils.ts` unless the product explicitly requires a different representation.

### 5.2 Dashboard Pages

Before building a dashboard page, inspect `web/components/dashboard/crud`, `web/components/dashboard/list`, and `web/components/dashboard-page.tsx`.

Use this order of preference:

1. `DashboardCrudPage` for standard create/read/update/delete resources.
2. `DashboardListPage` for read-only or custom-content paginated resources.
3. `useDashboardPagedList` when layout is bespoke but list query/filter/pagination lifecycle is standard.
4. `DashboardPage`, `DashboardToolbar`, `DashboardTableShell`, and related low-level primitives only for interactions that cannot fit the higher-level components.

Rules:

- Do not copy a resource page to recreate standard filters, query/reset/refresh actions, pagination, loading/empty states, confirmations, row actions, or dialogs.
- Configure `DashboardCrudPage` through its filters, columns, labels, service callbacks, row actions, sorting, and form/dialog extension points before adding page-local infrastructure.
- Use its schema-driven `DashboardCrudFormDialog` when supported. A genuinely custom form may live in `_components` and should normally use `react-hook-form`, `zod`, and `Field`.
- Configure `DashboardListPage` with columns or `renderContent`, and use `renderToolbarActions` for resource-specific actions.
- Business API knowledge stays in the page/service module; generic dashboard components must not import a resource-specific API.
- A change to `web/components/dashboard/*` must be generic, backward-compatible, and useful beyond one page. Otherwise keep it local to the feature.

### 5.3 Frontend Internationalization

- Every frontend feature and modification must work in all `SUPPORTED_LOCALES`, currently `zh-CN` and `en-US`.
- Add every new key to both `web/messages/zh-CN.json` and `web/messages/en-US.json` in the same change, preserving matching structure.
- React pages/components use `useI18n()`; locale-aware formatting/mapping may also use `useAppLocale()`.
- Non-React code uses `translateCurrentMessage()` or `translateMessage()` from `web/i18n/messages.ts`.
- Do not hard-code user-visible copy in JSX/TSX, toast messages, dialogs, confirmations, placeholders, validation, empty/loading/error states, tooltips, accessibility labels, or client-side fallback errors.
- Product names, protocol literals, user content, and raw business data do not need translation unless the UI already provides a display-name mapping.
- Dashboard labels supplied to shared CRUD/list components must come from translation keys.
- Centralize localized display names for backend identifiers/enums in a reusable `web/lib/*-i18n.ts` helper instead of duplicating locale switches across pages.
- Preserve the same placeholders in every locale and interpolate with `t(key, values)`; do not assemble sentences by concatenating translated fragments.
- Locale configuration belongs to `AppI18nProvider` and `web/i18n/config.ts`; features must not introduce separate locale detection or state.

### 5.4 Commercial-Grade UI and Interaction

This is a commercial product, not a prototype or component demo. UI work is complete only when it is visually coherent, interaction-complete, responsive, localized, and credible with real production data.

#### Visual Quality

- Follow the existing design language, spacing scale, typography, radius, color tokens, and component variants. A new feature must look native to the product rather than like a pasted template.
- In support platform UI, if rounded corners are needed, use `rounded-md` consistently.
- Support platform components should not add `shadow`.
- Establish a clear hierarchy: page title/primary action, filters or context, main content, and secondary information. Do not make every region a card or every action visually prominent.
- Prefer restrained, purposeful styling. Avoid decorative gradients, oversized hero text, excessive shadows, glass effects, emoji icons, random accent colors, and ornamental copy unless the product context explicitly calls for them.
- Use Lucide icons consistently. Choose icons by meaning, keep icon size and stroke weight aligned with neighboring controls, and never use icons as decoration without communicative value.
- Keep spacing and alignment deliberate at every breakpoint. Labels, inputs, table columns, action groups, dialog footers, and empty states must align cleanly without ad hoc offsets.
- Design for realistic content, not ideal sample text. Verify long names, multiline content, large counts, missing optional fields, and mixed Chinese/English values. Use wrapping, truncation, tooltips, or scroll containers intentionally.
- Preserve information density appropriate to an operations dashboard. Do not waste large areas on decoration, but do not compress controls until scanning and clicking become difficult.

#### Interaction Completeness

- Every asynchronous action must have an immediate and unambiguous state: pending/loading, success, failure, and retry or recovery when appropriate.
- Prevent duplicate submissions. Disable or lock the initiating control while a mutation is pending and show action-specific progress text or a spinner without causing layout shift.
- Keep feedback close to the action. Use inline validation for field problems, contextual error states for failed content, and toast notifications for completed background or page-level actions.
- Never silently discard user input. Warn before closing, navigating away, resetting, or switching context when there are meaningful unsaved changes.
- Destructive, irreversible, security-sensitive, or broad-impact operations require confirmation that clearly names the object and consequence. Do not use a generic “Are you sure?” message.
- Confirmation is not a substitute for good defaults: routine reversible actions should remain efficient and should not be interrupted by unnecessary modal prompts.
- After create/update/delete operations, keep list state coherent: refresh affected data, preserve useful filters/page position when possible, close dialogs only on success, and prevent stale selections or detail panels.
- Buttons and menu items must use precise verbs describing the result. Avoid vague labels such as “OK”, “Submit”, or “Process” when a specific action name is available.
- Preserve keyboard behavior and focus flow: Enter submits only where expected, Escape closes dismissible overlays, focus moves into dialogs and returns to the trigger, and destructive actions are not the accidental default.
- Interactive rows, icons, badges, and text links must look interactive only when they are interactive. Do not rely on hover-only discoverability for essential actions.

#### Forms and Dialogs

- Use the smallest suitable interaction container: inline editing for simple local changes, a dialog for focused tasks, and a full page/workbench for complex or multi-step workflows.
- Dialogs need a clear title, concise context when necessary, stable body layout, and a consistent footer with secondary action before primary action. Long content must scroll inside the dialog without pushing actions off-screen.
- Forms must have visible labels, appropriate controls, useful defaults, required/optional semantics, and examples or help text only where they reduce ambiguity.
- Validate at the right time: do not show errors before the user has interacted, clear stale errors after correction, and map backend validation failures to the relevant field when possible.
- Preserve entered values after failed submission. Do not reset or close a form until the server confirms success.
- Dependent fields must clearly reflect disabled/loading/empty states. When one field invalidates another, update it predictably and explain the dependency when it is not obvious.
- Configuration and high-impact forms should not expose a permanently editable surface when a deliberate edit mode improves safety. Saving sensitive configuration requires explicit user intent and appropriate confirmation.

#### Lists, Tables, and Operational Screens

- Use the shared Dashboard CRUD/list system and maintain consistent toolbar, filter, pagination, loading, empty, and action placement across resources.
- Filters must distinguish draft values from applied query state. Query, reset, refresh, pagination, and URL/state behavior should be predictable and must not unexpectedly erase one another.
- Tables must remain scannable: align comparable values, keep action columns stable, use badges sparingly for status, avoid dense multiline cells when a detail view is more appropriate, and provide horizontal overflow on narrow screens.
- Empty states must distinguish “no data exists” from “no results match the current filters” and offer the most relevant next action when the user can resolve the state.
- Loading states should preserve layout. Prefer skeletons for content whose shape is known and compact spinners for localized actions; avoid replacing an entire stable page with a centered spinner.
- Error states must explain what failed in user terms and provide retry when retry is meaningful. Never leave a blank table or empty panel after a request failure.
- Bulk actions must show selection count, affected scope, eligibility, and partial-failure results. Clear selection when it is no longer valid.

#### Responsive and Accessibility Requirements

- Every changed screen must work at desktop and narrow/mobile widths. Do not treat horizontal clipping, overlapping controls, wrapped action chaos, or off-screen dialog buttons as acceptable.
- Responsive behavior must preserve task priority: primary actions remain reachable, secondary actions may move into menus, filters may stack or collapse, and tables may scroll without hiding row identity/actions.
- Use semantic controls and accessible names. Icon-only buttons require localized accessible labels and tooltips where the meaning is not universally obvious.
- Maintain visible focus states, logical tab order, sufficient target sizes, and adequate text/background contrast. Do not encode status or errors using color alone.
- Respect reduced-motion preferences. Animations should explain state or continuity, remain subtle, and never delay work.

#### Product Copy and Data Credibility

- Copy must sound like a finished product: concise, specific, consistent, and action-oriented. Do not expose implementation jargon, placeholder prose, “TODO”, mock labels, debug wording, or developer instructions to users.
- Do not ship fake statistics, sample records, disabled-looking placeholder buttons, decorative charts without meaning, or interactions that only log to the console.
- Distinguish unavailable features from empty data. If a capability is not implemented, do not render a control that pretends it is functional.
- User-visible names, statuses, permissions, dates, and errors must use the established formatting and i18n mappings rather than raw backend identifiers.

#### UI Verification

- For meaningful UI changes, inspect the finished screen in a real browser at representative desktop and narrow widths. Source review and typecheck alone are not sufficient visual validation.
- Exercise the complete interaction, including initial load, populated state, empty/filtered state, validation failure, server failure when practical, pending/disabled behavior, success, cancel/close, and refresh.
- Check both `zh-CN` and `en-US`; verify that translated copy does not overflow, truncate critical meaning, or break control alignment.
- Before handoff, remove temporary data, debug UI, console output, test-only shortcuts, and visual artifacts introduced during verification.
- When browser verification cannot be performed, state that limitation explicitly; do not describe the UI as visually verified.

### 5.5 Generated and Embedded Frontend Assets

- `web/lib/generated/enums.ts` is generated by `task enums`; do not edit it manually.
- `web/public/sdk/agent-desk-sdk.min.js` is generated from the SDK source. When SDK source changes, run `cd web && pnpm build:sdk` and its focused SDK tests.
- `web/public/flowgram-editor` is produced from `flowgram-editor`; edit the source project, not the generated public output.
- When changing `flowgram-editor`, use its own `pnpm` scripts and ensure the embedding build still succeeds.
- The Next.js application is statically exported/embedded into the Go binary. Avoid runtime-only Next.js features that conflict with the current export and embedding model.

## 6. Testing and Validation

Validation must match the changed surface. Do not claim checks that were not run.

- Go formatting: run `gofmt` on every changed `.go` file.
- Go behavior: add focused tests for changed business rules, transactions, security boundaries, parsing, and reusable helpers; run the narrow package tests first, then `go test ./...` when practical.
- Frontend TypeScript: run `cd web && pnpm typecheck` for frontend changes.
- Frontend lint: run `cd web && pnpm lint` for broader component/page changes or when lint-sensitive code changed.
- Frontend logic: run relevant `node --test ...` files when changing utilities, i18n mappings, generated SDK behavior, or other modules with focused tests.
- Workflow editor: run the relevant `flowgram-editor` lint/build commands for changes in that project.
- Generation: after model CRUD or shared enum changes, run the corresponding `task generator` or `task enums` and review generated diffs.
- Build: use `task build` when changes affect frontend embedding, build configuration, generated public assets, or release assembly.
- Browser verification is expected for meaningful visual or interaction changes when a runnable environment is available; state explicitly when it was not performed.
- Documentation-only changes require at least `git diff --check` and verification that every referenced path/command exists.

## 7. Completion Checklist

Before handing off a change, confirm the applicable items:

- The implementation follows current layer/component ownership and does not create reverse dependencies.
- Transactions cover exactly the operations that must be atomic, and all transactional DB calls use the same `ctx.Tx`.
- API routes are explicitly registered and responses preserve `JsonResult`/`PageResult` contracts.
- Models, migrations, queries, and tests remain compatible with SQLite and MySQL.
- Backend and frontend user-visible text is complete in both Chinese and English.
- Dashboard pages reuse the highest-level suitable component under `web/components/dashboard/*`.
- UI changes meet the commercial-grade standard: complete states, precise feedback, safe mutations, responsive layout, accessible controls, credible copy/data, and no demo-only behavior.
- Generated files were regenerated from their source and were not manually edited.
- Relevant tests/typechecks/lint/build/browser checks were run, and any validation limitation is reported.
- `git diff --check` passes and unrelated worktree changes remain untouched.

<!-- aoci:begin -->
## AOCI 仓库认知

AOCI 为本仓库维护一个稳定、可版本化、可增量更新的仓库级认知层，供模型跨任务复用对系统的理解。

`aoci.txt` 是面向模型的结构化认知索引。它以每个受管理文件、数据库表或其他受管理对象一条独立 Entry 的方式，用符号标签与 F/R/A/S 语义表达对象的核心职责、重要关系、对外契约，以及理解或修改系统时必须知道的非显然约束和设计决策。

Header、目录段和全部 Entry 共同组成完整仓库索引，可以覆盖前端、后端、配置、数据库结构及其他受管理内容。受管理内容发生变化时，通常只需维护受影响的认知条目，不需要重新生成整个索引。

AOCI 提供系统架构、对象职责、重要关系、对外契约和关键约束的高密度视图。

### 工作原理

AOCI 采用“模型生成、模型读取”的认知闭环。

Header、Entry 和 Curation 语义的创作只按当前机器签发的 Plan 与实时 Guide 执行；由 Host 模型基于当前绑定证据独立完成。

Entry 的语义必须来自模型对真实证据的理解。不得仅依据路径、文件名、扩展名、AST、符号列表、依赖扫描、正则、固定模板或规则引擎推导、预填、拼接或改写索引语义。

对 Fresh Bootstrap，只按当前机器签发的 Plan 和实时 Guide 执行。当它们要求创作时，Host 模型创作 Root、Meta、Tag 和 F/R/A/S，提供 authoring-run 声明，并把它绑定到 Plan、Evidence 与完整 Candidate。不得要求 AOCI 填写 `origin=host_model`、制造 Receipt 或把程序生成的 Framework 当作语义。本文件不自行重建 Onboarding 流程。内部批次不是用户决策；只有遇到既有批准边界或真实的安全、漂移、CAS、Recovery 条件才停止。

### 最小使用入口

- `aoci_rules`：取得当前AOCI版本的会话运行合同。
- `aoci_overview`：建立或恢复本仓库的完整认知。
- `aoci_maintain`：受管理对象达到最终稳定状态后检查认知是否需要维护。
- `aoci_update_entry`：提交与当前证据和源码摘要绑定的完整语义更新批次。
- `aoci_report`：仅当当前布局和工具状态支持时，在证据不足、无法可靠生成语义时登记待办，不猜写。

其他MCP工具、CLI命令、参数和专项流程，以当前工具说明、Guide和 `--help` 返回内容为准，不在本文件中重复完整手册。

本区块只规定仓库接入、认知使用和收尾原则。`aoci_rules` 承载当前会话合同，Guide实时输出承载当前Plan的执行顺序与停点，工具Schema、Spec和Validator承载机器结构与判据；Prompt、Description、README和静态文档不能覆盖这些机器事实。

### 建立、生成和恢复认知

1. 每个新的 Agent Run 开始时，应先判断：

   - 本仓库是否已经存在可用的完整AOCI索引；
   - 当前上下文中是否已有与本仓库根、当前索引版本和当前AOCI服务相匹配，并且模型仍可可靠使用的完整仓库认知。

2. 仓库已经存在可用的完整索引，但当前Run没有可靠完整认知时，先调用 `aoci_rules`，再调用 `aoci_overview`。

   完整认知仍可靠时直接复用。局部不确定本身不要求机械重读系统全貌。

   本Run从已知Host上下文压缩恢复时（包括宿主注入的压缩摘要），必须把此前模型认知视为不可靠。压缩handoff不得保留或摘要正式Whole-Index，也不得保留或摘要任何Overview Header、Entry、Chunk、Challenge或Attestation正文；只能保留安全续接所需的receipt身份、未完成write或Recovery状态，以及立即重载指令。复制进handoff的Whole-Index语义或receipt不能证明恢复后模型的当前认知可靠。若当前上下文已无法可靠保留运行合同，先调用 `aoci_rules`。继续业务任务前，使用 `refresh_reasons=["context_compaction"]` 和新的 `refresh_event_id` 调用普通完整Whole-Index `aoci_overview`（不设置 `check_only` 或设为false）；不得使用 `check_only` 或认知probe。原样跟随每个 `next_cursor` 直到 `completed=true`，确认交付，并且只基于新交付正文提交一次Attestation。完成这次新的完整传输后，即使Attestation为partial或fail也消费该generation，并按既有合同继续source-bound任务，不再自动调用第二次Overview。

   AOCI可以针对 `context_compaction`、项目 `cognition_refresh_threshold` 下的机器 `semantic_threshold` 或主要 `phase_transition` 提供checkpoint与认知状态事实。只需要这些紧凑事实时使用 `check_only=true`；这些事实只向Agent提供建议，不替模型决定是否需要系统全貌。

   Agent显式调用普通 `aoci_overview`（未设置 `check_only` 或为false）时，只要能形成一致的CognitionSet，AOCI必须完整交付请求scope。不得因为已有receipt、阈值未达到或没有待处理刷新原因而抑制正文。正式认知Dirty或Stale时仍交付正文，但必须标记不可靠。存在未决恢复或无法形成一致snapshot时失败关闭，不返回混合正文。

   普通Overview返回 `continuation_required=true` 时，必须原样提交 `next_cursor` 并自动继续到 `completed=true`。不得询问用户、开始业务任务或给出阶段性系统结论。Host截断、缺块、重复、乱序、cursor失败、Index变化或`chunk_tokens`变化时停止本次认知链。Attestation完成前不得用Memory、源码、Spec、`aoci.txt`、历史会话、scope、search或Entry读取修补或补充Whole-Index认知。Challenge ordinal是正式Entry序列中的1-based位置；Header内容、注释、空行、Section/Overview/Chunk Marker、Receipt与Metadata均不计数，Chunk Receipt ordinal使用同一序列。Attestation必须原样回绑本次Challenge发布的当前`index_sha256`、`entry_sequence_sha256`与`entry_count`；旧Index、旧Entry序列、旧数量或旧Attestation均无效。完整链结束后只正式提交一次既有模型认知Attestation；同一响应只允许一次不改变语义答案的JSON Schema或字段格式修正。对象、Tag或F不匹配即失败且认知吸收不确定，不得语义重试或旁路补答。首次认知失败时还不得执行Root/Meta、Migration、全局布局或其他未重新绑定的系统级决策。上下文压缩刷新若传输完整、认知身份不变、治理对齐且没有Recovery或第三方冲突，即使Attestation为partial或fail也消耗该refresh generation，并继续原任务，不再自动重读Overview。`system_mastery_percent`只自评系统框架——架构、职责、强关系、稳定外部契约以及高熵安全和维护约束——不表示完整实现或运行实况知识；机器索引覆盖率必须分开。默认只向用户输出由本次真实覆盖率、Challenge、块数、Token和掌握度生成的规定成功或失败一句话。Host截断时提示用户把 `overview_delivery.chunk_tokens` 设置为更小的合法值后重新开始，不得自动修改。

   加法认知等级必须与严格证明字段分开解释。`delivery_verified`表示已加载Index且Host交付已确认，但完整认知验证仍未完成；应表达为“已加载且交付已验证”，不得描述为“没有认知”或“没有理解系统”。`cognition_verified`要求Attestation通过（Challenge至少80%的ordinal完全正确且对象身份至多失手一处），`cognition_governed`还要求治理对齐。通用完整读取失败句只用于真实交付故障。

   当Overview响应包含可选`cognition-state/v2`投影时，必须分别解释各维度。其Level止于`model_cognition_usable`；`strict_attestation_verified`、`governance_aligned`与`current_system_cognition_reliable`都是独立状态，绝不参与该Level。ordinal、对象身份、Tag或核心F不匹配可以导致严格Attestation失败，而模型认知仍然可用；不得仅凭这种不匹配就宣称模型没有理解系统。只有`current_system_cognition_reliable=true`允许无保留地声称当前完整系统认知可靠。投影缺失时继续使用上述Legacy解释。

   普通的只读审计、分析、检查、不修改代码或不提交、不push，不自动等于严格零写入，也不改变上述认知有效性判断。Codex Memory和历史Skill只能辅助恢复经验、用户偏好与调查方向，不能替代与当前仓库根、索引摘要、AOCI服务身份和认知范围匹配的当前认知收据；项目AGENTS和当前AOCI身份在AOCI状态上优先于历史Memory。

   只有用户明确禁止Ledger、元数据、`.aoci`运行资产及任何文件写入时，才按严格零写入处理。若必要的认知建立与该边界冲突，必须报告冲突并请求用户裁决或建议使用隔离副本，不得静默以Memory替代当前仓库认知。

3. 仓库没有可用的完整索引，或当前只有最小骨架、Header不完整、Entries未完成、必要Curation尚未裁决时，如果需要建立正式完整AOCI索引，先取得 `aoci_rules`，然后进入当前AOCI Guide。由Guide依据仓库真实状态决定下一阶段并完成必要安全步骤。

   `aoci_maintain` 不替代索引建立流程。

   不在本文件中自行重建或硬编码完整索引生成状态机。

4. 在长程任务中，模型负责保留当前认知收据并正确使用刷新门禁：

   - Host报告上下文压缩或模型已知系统全貌丢失时，执行上述强制 `context_compaction` 重载规则；AOCI不能自行推断Host事件；
   - 进入真正的主要阶段时声明 `phase_transition`，不得把函数、测试运行或小步骤当作阶段；
   - 在有用的稳定检查点通过 `check_only=true` 取得机器语义计数；
   - 除已知压缩的强制重载外，由Agent判断当前任务是否需要再次显式获取指定scope或完整Overview；
   - 在维护和对齐完成前，保留AOCI报告的Dirty或Stale可靠性状态。

### 任务收尾与认知维护

5. 纯只读问答、分析、版本核验，或没有产生受AOCI管理对象变化的任务，不需要调用维护工具。当前AOCI版本是任意`aoci_overview` check_only或`aoci_maintain`响应里的`cognition_receipt.mcp_service_version`；二进制路径是项目`.mcp.json`里的`command`，CLI不必在PATH上。

6. 发生受AOCI管理对象变化时，待其达到本次任务的最终稳定状态后，只调用一次 `aoci_maintain`。不要在每次中间修改后逐文件维护。

7. 若维护结果返回真实语义候选，Host 模型必须基于每个候选绑定的对象和必要证据，独立创作完整标签与F/R/A/S更新。通过 `aoci_update_entry` 一次提交当前机器签发批次的完整候选集合，同时原样保留每项 `source_sha256`、`candidate_id` 与对应domain批次身份。`max_entries`只限制单次请求和原子事务，不限制logical plan、Whole-Index或Managed Scope。`remaining`非零时，在当前批次成功Apply后重新调用Maintain并从新preimage继续；绝不能为满足transport上限缩减Index覆盖或自行截取返回批次。

   没有足够证据且当前布局支持 `aoci_report` 时，使用它而不猜测、套用模板或为消除待办而生成缺乏证据的认知。

8. 必须遵守工具返回的结构化状态和安全边界：

   - `repair_required`：只修复明确命中的候选，再重新提交当前机器签发的完整批次；
   - `stopped`：结束当前写入尝试并检查 `failed_step`、错误、正式写入证据与Recovery。auto模式下，已证明零写入则记录closure并重新Plan；完整Intent和可证明postimage则Resume；策略要求Rollback且preimage可证明则精确恢复后重新Plan。只有证据不足、第三方正式字节冲突、需要审批或外部动作，或命中其他真实安全边界时，才停止整个用户任务；
   - 冲突、审批、人工裁决、权限和安全信号不得忽略；
   - 已经对齐后不得重复维护或重复写入；`refresh_ready_for_overview` 是checkpoint事实，由Agent决定是否为下一阶段请求普通完整Overview。

   维护完成后如果又修改了任何受管理对象，之前的维护结果失效，应在新的最终稳定状态重新完成收尾。

9. 用户只限制业务文件范围，但没有明确禁止仓库托管资产时，AOCI托管资产可以在收尾阶段为保持认知一致而更新，并应在审计和提交中与业务文件区分。

   用户明确禁止修改 `aoci.txt`、`.aoci`、元数据或任何额外文件时，以用户限制为准，不得写入，并如实报告剩余不一致。

### 专项流程

初始化、完整索引生成、Header生成、Entries生成、数据库结构索引、Curation、人工评审和故障恢复，只按当前AOCI Guide或工具在对应阶段返回的指令、命令和安全停点执行。

不预加载、不猜测，也不自行重建这些专项流程。平台调用方式、请求格式、批次上限、审批规则、索引格式细节和恢复步骤由对应Guide、工具说明、模型Prompt和CLI帮助按需提供。
<!-- aoci:end -->
