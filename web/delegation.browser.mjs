import assert from "node:assert/strict"
import http from "node:http"
import { readFile, mkdir } from "node:fs/promises"
import path from "node:path"
import { createRequire } from "node:module"

const require = createRequire(import.meta.url)
const { chromium } = require(process.env.PLAYWRIGHT_MODULE || "playwright")
const root = path.resolve("out")
const shots = path.resolve("../.impeccable/review/delegation")
await mkdir(shots, { recursive: true })
const server = http.createServer(async (req, res) => {
  let filename = path.join(root, decodeURIComponent(new URL(req.url, "http://localhost").pathname))
  if (!path.extname(filename)) filename += ".html"
  try {
    const data = await readFile(filename)
    const mime = { ".html": "text/html", ".js": "text/javascript", ".css": "text/css", ".png": "image/png", ".svg": "image/svg+xml" }[path.extname(filename)] || "application/octet-stream"
    res.writeHead(200, { "Content-Type": mime }); res.end(data)
  } catch { res.writeHead(404); res.end() }
})
await new Promise((resolve) => server.listen(0, "127.0.0.1", resolve))
const base = `http://127.0.0.1:${server.address().port}`
const browser = await chromium.launch({ channel: "chrome", headless: true })
try {
  for (const locale of ["zh-CN", "en-US"]) {
    const labels = JSON.parse(await readFile(`messages/${locale}.json`, "utf8"))
    for (const width of [1440, 390]) {
      const context = await browser.newContext({ viewport: { width, height: width === 390 ? 844 : 1000 }, reducedMotion: "reduce" })
      const session = { accessToken: "synthetic-delegation-fixture", user: { id: 1, username: "test", nickname: "Synthetic QA", userType: "admin", roles: ["admin"] }, permissions: ["*", "conversation.view", "conversation.send"], roles: ["admin"] }
      await context.addInitScript((session) => localStorage.setItem("agent-desk-session", JSON.stringify(session)), session)
      const conversation = { id: 1, channelId: 3, aiAgentId: 1, customerId: 1, customerName: "Synthetic customer / 测试联系人", status: 3, serviceMode: 2, currentAssigneeId: 1, currentAssigneeName: "Synthetic QA", agentUnreadCount: 0, customerUnreadCount: 0, lastMessageId: 1, tags: [], workStatus: "needs_reply", workRevision: 1 }
      let state = { conversationId: 1, ownerId: 1, ownerName: "Synthetic QA / International sales", canManage: true, active: false, previewOnly: true, revision: 0, aiAgentId: 1, instructions: "", startedByName: "Synthetic QA", agents: [{ id: 1, name: "Sales assistant / 销售助手", liveAvailable: false }], events: [] }
      let failStart = true, starts = 0, customerSends = 0, pausePreview = false, releasePreview
      const errors = []
      function event(kind, content = "") {
        state.events.unshift({ id: state.events.length + 1, actorName: kind === "started" || kind === "reclaimed" ? "Synthetic QA" : "", kind, content, previewOnly: true, createdAt: new Date().toISOString() })
      }
      await context.route("**/api/**", async (route) => {
        const url = new URL(route.request().url())
        let data = { results: [], page: { page: 1, limit: 50, total: 0 } }, success = true
        if (/send_message|send_ai_message|send_agent_message/.test(url.pathname)) customerSends++
        if (url.pathname === "/api/config") data = { language: locale, companyName: "Synthetic QA", passwordLoginEnabled: true }
        else if (url.pathname === "/api/auth/profile") data = session
        else if (url.pathname.endsWith("/conversations")) data = { results: [conversation], page: { page: 1, limit: 50, total: 1 } }
        else if (url.pathname.endsWith("/conversation/channels")) data = [{ id: 3, name: "WhatsApp QA", channelType: "whatsapp", status: 0, connectionState: "connected" }]
        else if (url.pathname.endsWith("/conversation/1")) data = conversation
        else if (url.pathname.endsWith("/message_list")) data = { results: [{ id: 1, conversationId: 1, senderType: "customer", messageType: "text", content: "Please reply in English. I need 200 units.", sendStatus: "sent", createdAt: "2026-09-14 10:00:00" }], hasMore: false, cursor: "" }
        else if (url.pathname.endsWith("/customer/1")) data = { id: 1, name: conversation.customerName, manualTags: [], remark: "", gender: 0 }
        else if (url.pathname.endsWith("/customer-contact/list") || url.pathname.endsWith("/tag/all") || url.pathname.endsWith("/list_all")) data = []
        else if (url.pathname.endsWith("/memory")) data = { status: "ready", entries: [], shared: [], dossier: [] }
        else if (url.pathname.endsWith("/delegation")) data = state
        else if (url.pathname.endsWith("/delegation/start")) {
          starts++
          success = !failStart; failStart = false
          const input = route.request().postDataJSON()
          assert.equal(input.previewOnly, true)
          if (success) {
            assert.equal(input.revision, state.revision)
            state = { ...state, ...input, active: true, revision: state.revision + 1, startedAt: new Date().toISOString(), expiresAt: new Date(Date.now() + 3600000).toISOString() }
            event("started")
          }
        } else if (url.pathname.endsWith("/delegation/stop")) {
          state = { ...state, active: false, revision: state.revision + 1 }
          event("reclaimed")
        } else if (url.pathname.endsWith("/delegation/preview")) {
          const revision = state.revision
          if (pausePreview) await new Promise((resolve) => { releasePreview = resolve })
          if (state.active && state.revision === revision) event("preview", "We can help with 200 units. Which delivery region should we prepare the quote for?")
        }
        await route.fulfill({ json: { success, errorCode: success ? 0 : 1, message: success ? "" : "Synthetic retryable failure", data } })
      })
      const page = await context.newPage()
      page.on("pageerror", (error) => errors.push(error.message))
      await page.goto(`${base}/dashboard/conversations?conversationId=1`)
      const open = page.getByRole("button", { name: labels.delegation.title, exact: true })
      await open.waitFor()
      await page.screenshot({ path: path.join(shots, `${locale}-${width}-overview.png`), fullPage: true })
      await open.click()
      const dialog = page.getByRole("dialog", { name: labels.delegation.title, exact: true })
      await dialog.locator("#delegation-instructions").fill("Answer product questions and collect purchase needs. Return discount requests to me.")
      assert.equal(await dialog.getByRole("switch").isDisabled(), true)
      await dialog.getByRole("button", { name: labels.delegation.close, exact: true }).click()
      await open.click()
      assert.match(await dialog.locator("#delegation-instructions").inputValue(), /Return discount/)
      const start = dialog.getByRole("button", { name: labels.delegation.startTrial, exact: true })
      await start.click()
      await dialog.getByText("Synthetic retryable failure", { exact: true }).waitFor()
      assert.match(await dialog.locator("#delegation-instructions").inputValue(), /Return discount/)
      await page.screenshot({ path: path.join(shots, `${locale}-${width}-form.png`), fullPage: true })
      await start.click()
      const preview = dialog.getByRole("button", { name: labels.delegation.preview, exact: true })
      await preview.waitFor()
      assert.equal(starts, 2)
      await preview.click()
      await dialog.getByText("We can help with 200 units. Which delivery region should we prepare the quote for?", { exact: true }).waitFor()
      await page.waitForFunction((label) => !Array.from(document.querySelectorAll("button")).find((button) => button.textContent.trim() === label)?.disabled, labels.delegation.preview)
      await dialog.evaluate(async (element) => { await Promise.all(element.getAnimations({ subtree: true }).map((animation) => animation.finished.catch(() => {}))) })
      await page.screenshot({ path: path.join(shots, `${locale}-${width}-trial.png`), fullPage: true })
      pausePreview = true
      await preview.click()
      await dialog.getByRole("button", { name: labels.delegation.generating, exact: true }).waitFor()
      await dialog.getByRole("button", { name: labels.delegation.reclaim, exact: true }).click()
      await start.waitFor()
      releasePreview()
      assert.equal(state.active, false)
      await start.click()
      await preview.waitFor()
      await dialog.getByRole("button", { name: labels.delegation.close, exact: true }).click()
      await dialog.waitFor({ state: "hidden" })
      await page.getByRole("button", { name: labels.delegation.reclaim, exact: true }).click()
      await page.getByText(labels.delegation.human, { exact: true }).waitFor()
      assert.equal(customerSends, 0, "delegation trials must never use customer send APIs")
      assert.equal(await page.evaluate(() => document.documentElement.scrollWidth > window.innerWidth), false, "page overflow")
      assert.deepEqual(errors, [], "browser errors")
      console.log(`PASS ${locale} ${width}: held sending, draft retention, retry, trial, preview, reclaim during generation, restart; no sends`)
      await context.close()
    }
  }
} finally {
  await browser.close()
  await new Promise((resolve) => server.close(resolve))
}
