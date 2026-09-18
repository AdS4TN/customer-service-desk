import assert from "node:assert/strict"
import http from "node:http"
import { readFile, mkdir } from "node:fs/promises"
import path from "node:path"
import { createRequire } from "node:module"

const require = createRequire(import.meta.url)
const { chromium } = require(process.env.PLAYWRIGHT_MODULE || "playwright")
const root = path.resolve("out")
const shots = path.resolve("../.impeccable/review/ai-employee")
await mkdir(shots, { recursive: true })
const server = http.createServer(async (req, res) => {
  let file = path.join(root, decodeURIComponent(new URL(req.url, "http://localhost").pathname))
  if (!path.extname(file)) file += ".html"
  try {
    const data = await readFile(file)
    res.writeHead(200, { "Content-Type": { ".html": "text/html", ".js": "text/javascript", ".css": "text/css", ".svg": "image/svg+xml" }[path.extname(file)] || "application/octet-stream" })
    res.end(data)
  } catch { res.writeHead(404); res.end() }
})
await new Promise((resolve) => server.listen(0, "127.0.0.1", resolve))
const browser = await chromium.launch({ channel: "chrome", headless: true })
try {
  for (const locale of ["zh-CN", "en-US"]) {
    const labels = JSON.parse(await readFile(`messages/${locale}.json`, "utf8"))
    for (const width of [1440, 390]) {
      const context = await browser.newContext({ viewport: { width, height: width === 390 ? 844 : 1000 }, reducedMotion: "reduce" })
      const session = { accessToken: "synthetic-employee", user: { id: 1, username: "qa", nickname: "Synthetic QA", userType: "admin", roles: ["super_admin"] }, permissions: ["*"], roles: ["super_admin"] }
      await context.addInitScript((session) => localStorage.setItem("agent-desk-session", JSON.stringify(session)), session)
      let agent = { id: 1, name: "Synthetic Support", displayName: "", statusText: "", avatar: "", description: "", status: 0, statusName: labels.aiAgent.enabled, serviceMode: 2, aiConfigId: 1, aiConfigName: "QA Model", systemPrompt: "Speak for our business.", welcomeMessage: "", replyTimeoutSeconds: 180, handoffMode: 1, fallbackMode: 1, fallbackMessage: "", teams: [], skills: [], skillIds: [], knowledgeBaseIds: [1], mcpTools: [], workflowBindings: [], publishedRevisionId: 1, rolloutPercent: 12, maxSteps: 6, contextWindow: 20, knowledgePolicy: "", toolPolicy: "" }
      let failTrial = false, failSave = false, failMetadata = false, stateReads = 0, writes = 0, sends = 0
      const previews = [], errors = []
      let resumePreview = null, holdPreview = false
      const importedSkills = []
      let skills = [{ id: 5, name: "Price negotiation", instruction: "Ask about quantity before discussing a discount.", description: "", examples: [], toolWhitelist: [], remark: "", status: 0 }]
      await context.route("**/api/**", async (route) => {
        const url = new URL(route.request().url())
        const pathname = url.pathname
        let success = true, data = { results: [], page: { page: 1, limit: 20, total: 0 } }
        if (/send_message|send_ai|send_agent|delegation\/start/.test(pathname)) sends++
        if (pathname === "/api/config") data = { language: locale, companyName: "Synthetic QA", passwordLoginEnabled: true }
        else if (pathname === "/api/auth/profile") data = session
        else if (pathname.endsWith("/ai-agent/list")) data = { results: [agent], page: { page: 1, limit: 20, total: 1 } }
        else if (pathname.endsWith("/ai-agent/1")) data = agent
        else if (pathname.endsWith("/reception_state")) { stateReads++; data = { agentId: 1, serviceMode: 2, receptionEnabled: false, assistanceAvailable: true, publishedRevisionId: 1, modelName: "QA published model", knowledgeCount: 1, skillCount: 0, channels: [{ id: 3, name: "International sales WhatsApp / 国际销售", channelType: "whatsapp", receptionEnabled: true, automaticMessagesAllowed: false, outboundBlocked: true }] } }
        else if (pathname.endsWith("/ai-agent/update")) {
          writes++; success = !failSave
          const body = route.request().postDataJSON()
          assert.equal(body.capabilitiesOnly, true)
          assert.equal(body.serviceMode, 2)
          if (success) agent = { ...agent, ...body }
        } else if (pathname.endsWith("/ai-agent/preview")) {
          success = !failTrial
          const body = route.request().postDataJSON(); previews.push(body)
          if (holdPreview) await new Promise((resolve) => { resumePreview = resolve })
          data = { content: "We can help with your order. What quantity do you need?", modelName: "QA draft model", knowledgeStatus: "matched", sources: [{ knowledgeBaseId: 1, documentId: 1, chunkId: 1, title: "Synthetic product policy", content: "Discounts depend on purchase volume." }], mountedSkills: [], durationMs: 500 }
        } else if (pathname.endsWith("/revision/list")) { success = !failMetadata; data = [{ id: 1, revision: 1, publishedAt: "2026-09-15 12:00:00", publishedByName: "Synthetic QA", definitionHash: "synthetic" }] }
        else if (pathname.includes("/ai-config/") && pathname.endsWith("/list_all")) data = [{ id: 1, name: "QA Model", modelName: "test-model", status: 0 }]
        else if (pathname.includes("/knowledge-base/") && pathname.endsWith("/list_all")) data = [{ id: 1, name: "Product documentation", status: 0 }]
        else if (pathname.includes("/skill-definition/list_all")) data = skills
        else if (pathname.includes("/skill-definition/create")) { const body = route.request().postDataJSON(); data = { ...body, id: 6 + skills.length, status: 0 }; skills.push(data); importedSkills.push(body) }
        else if (pathname.includes("/sales-experience/options")) data = { skills: [{ id: "price", activeRevisionId: 4 }], models: [] }
        else if (pathname.includes("/sales-experience/revision")) data = { id: 4, skillId: "price", hash: "synthetic", note: "Synthetic experience", payload: { rules: [{ id: "price-1", condition: "Customer asks for a discount", action: "Clarify volume", exceptions: "No approved discount" }] } }
        else if (pathname.endsWith("/list_all") || pathname.endsWith("/catalog")) data = []
        await route.fulfill({ json: { success, errorCode: success ? 0 : 1, message: success ? "" : "Synthetic retryable failure", data } })
      })
      const page = await context.newPage()
      page.setDefaultTimeout(15000)
      console.log(locale, width, "opening employee")
      page.on("pageerror", (error) => errors.push(error.message))
      await page.goto(`${process.env.EMPLOYEE_TEST_ORIGIN || `http://127.0.0.1:${server.address().port}`}/dashboard/ai-agents`)
      await page.getByRole("button", { name: "Synthetic Support", exact: true }).click()
      await page.waitForURL("**/ai-agents/editor?id=1")
      await page.getByRole("textbox", { name: labels.employee.persona, exact: true }).waitFor()
      async function capture(name) {
        console.log(locale, width, "capture", name)
        const devBadge = page.getByRole("button", { name: "Collapse issues badge", exact: true })
        if (await devBadge.isVisible()) await devBadge.click()
        await page.screenshot({ path: path.join(shots, `${locale}-${width}-${name}.png`), fullPage: true, animations: "disabled", timeout: 15000 })
        assert.equal(await page.evaluate(() => document.documentElement.scrollWidth > innerWidth), false, "document overflow")
      }
      async function section(name) {
        if (width >= 768) await page.getByRole("tab", { name, exact: true }).click()
        else { await page.getByRole("combobox").first().click(); await page.getByRole("option", { name, exact: true }).click() }
      }
      async function view(name) { if (width < 1280) await page.getByRole("tab", { name, exact: true }).click() }
      await capture("persona")
      await page.getByRole("textbox", { name: labels.employee.persona, exact: true }).fill("UNSAVED: Speak in English and ask one question.")
      await view(labels.employee.trial)
      await page.getByRole("textbox", { name: labels.employee.question, exact: true }).fill("Can I get a discount?")
      await page.getByRole("button", { name: labels.employee.try, exact: true }).click()
      await page.getByText("We can help with your order. What quantity do you need?", { exact: true }).waitFor()
      assert.equal(previews[0].draft.systemPrompt, "UNSAVED: Speak in English and ask one question.")
      assert.equal(writes, 0)
      await page.getByText(labels.employee.evidence, { exact: false }).click()
      await page.getByText("Synthetic product policy", { exact: true }).click()
      await capture("trial")
      failTrial = true
      await page.getByRole("textbox", { name: labels.employee.question, exact: true }).fill("What about 500 units?")
      await page.getByRole("button", { name: labels.employee.try, exact: true }).click()
      await page.getByText("Synthetic retryable failure", { exact: true }).waitFor()
      assert.equal(await page.getByRole("textbox", { name: labels.employee.question, exact: true }).inputValue(), "What about 500 units?")
      failTrial = false
      holdPreview = true
      const heldRequest = page.waitForRequest("**/ai-agent/preview")
      await page.getByRole("button", { name: labels.employee.try, exact: true }).click()
      await heldRequest
      await page.getByRole("button", { name: labels.employee.stop, exact: true }).click()
      holdPreview = false
      resumePreview?.()
      await page.waitForFunction((value) => document.querySelector("#employee-trial-input")?.value === value, "What about 500 units?")
      assert.equal(await page.getByRole("textbox", { name: labels.employee.question, exact: true }).inputValue(), "What about 500 units?")
      await view(labels.employee.configure)
      await page.getByRole("button", { name: labels.employee.back, exact: true }).last().click()
      const discard = page.getByRole("dialog", { name: labels.aiReception.discardTitle, exact: true })
      await discard.waitFor(); await discard.getByRole("button", { name: labels.confirm.cancel, exact: true }).click()
      failSave = true
      await page.getByRole("button", { name: labels.aiAgent.saveConfig, exact: true }).click()
      await page.locator('[data-sonner-toast]').filter({ hasText: "Synthetic retryable failure" }).first().waitFor()
      assert.match(await page.getByRole("textbox", { name: labels.employee.persona, exact: true }).inputValue(), /UNSAVED/)
      failSave = false; failMetadata = true
      await page.getByRole("button", { name: labels.aiAgent.saveConfig, exact: true }).click()
      await page.getByText(labels.aiReception.metadataRefreshFailed, { exact: true }).waitFor()
      failMetadata = false
      await page.getByRole("button", { name: labels.aiReception.retryMetadata, exact: true }).click()
      await page.getByText(labels.aiReception.metadataRefreshFailed, { exact: true }).waitFor({ state: "hidden" })
      await section(labels.employee.knowledge)
      await page.getByRole("button", { name: labels.employee.addSkill, exact: true }).click()
      const skillDialog = page.getByRole("dialog", { name: labels.employee.skillContent, exact: true })
      await skillDialog.getByRole("textbox", { name: labels.aiAgent.name, exact: true }).fill("Synthetic sales skill")
      await skillDialog.getByRole("textbox", { name: labels.employee.skillContent, exact: true }).fill("Ask about purchase volume.")
      await skillDialog.getByRole("button", { name: labels.employee.saveSkill, exact: true }).click()
      await skillDialog.waitFor({ state: "hidden" })
      await page.getByText("Synthetic sales skill", { exact: true }).last().waitFor()
      await capture("knowledge")
      await section(labels.employee.assignment)
      await page.getByText(labels.aiReception.outboundBlocked, { exact: true }).waitFor()
      assert.ok(stateReads > 0)
      await capture("reception")
      await section(labels.employee.knowledge)
      await page.getByRole("button", { name: labels.employee.importExperience, exact: true }).click()
      const experienceDialog = page.getByRole("dialog", { name: labels.employee.importExperience, exact: true })
      await experienceDialog.getByRole("combobox").click()
      await page.getByRole("option", { name: "price #4", exact: true }).click()
      await experienceDialog.getByText("Customer asks for a discount", { exact: true }).waitFor()
      await experienceDialog.getByRole("button", { name: labels.employee.import, exact: true }).click()
      await experienceDialog.waitFor({ state: "hidden" })
      assert.match(importedSkills.at(-1).instruction, /Condition: Customer asks for a discount\nAction: Clarify volume/)
      assert.match(importedSkills.at(-1).remark, /revision:4 hash:synthetic/)
      await view(labels.employee.trial)
      await page.getByRole("button", { name: labels.employee.try, exact: true }).click()
      await page.getByRole("textbox", { name: labels.employee.question, exact: true }).waitFor({ state: "visible" })
      await page.getByRole("button", { name: labels.employee.stop, exact: true }).waitFor({ state: "hidden" })
      assert.ok(previews.at(-1).draft.skillIds.includes(8), "experience not linked to trial draft")
      assert.equal(sends, 0)
      assert.deepEqual(errors, [])
      for (const label of [labels.employee.back, labels.aiAgent.saveConfig, labels.aiAgent.publishAgent]) {
        const box = await page.getByRole("button", { name: label, exact: true }).last().boundingBox()
        assert.ok(box && box.x >= 0 && box.x + box.width <= width && box.y + box.height <= (width === 390 ? 844 : 1000), label + " clipped")
      }
      await view(labels.employee.configure)
      await section(labels.employee.persona)
      await page.getByRole("textbox", { name: labels.employee.persona, exact: true }).fill("BROWSER_BACK_DRAFT")
      await page.evaluate(() => history.back())
      await discard.waitFor()
      await discard.getByRole("button", { name: labels.confirm.cancel, exact: true }).click()
      assert.match(page.url(), /editor\?id=1/)
      assert.equal(await page.getByRole("textbox", { name: labels.employee.persona, exact: true }).inputValue(), "BROWSER_BACK_DRAFT")
      await page.evaluate(() => history.back())
      await discard.waitFor()
      await discard.getByRole("button", { name: labels.reception.discard, exact: true }).click()
      await page.waitForURL("**/dashboard/ai-agents")
      console.log(locale, width, "passed", { previews: previews.length, writes, sends })
      await context.close()
    }
  }
} finally { await browser.close(); server.closeAllConnections(); await new Promise((resolve) => server.close(resolve)) }
