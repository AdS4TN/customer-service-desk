import assert from "node:assert/strict"
import http from "node:http"
import { readFile, mkdir } from "node:fs/promises"
import path from "node:path"
import { createRequire } from "node:module"

const require = createRequire(import.meta.url)
const { chromium } = require(process.env.PLAYWRIGHT_MODULE || "playwright")
const root = path.resolve("out")
const shots = path.resolve("../.impeccable/review/contact-profiles")
await mkdir(shots, { recursive: true })
const server = http.createServer(async (req, res) => {
  const pathname = decodeURIComponent(new URL(req.url, "http://localhost").pathname)
  let filename = path.join(root, pathname)
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
      const session = { accessToken: "synthetic-browser-fixture", user: { id: 1, username: "test", nickname: "Synthetic QA", userType: "admin", roles: ["admin"] }, permissions: ["*", "conversation.view", "customer.update", "conversation.send"], roles: ["admin"] }
      await context.addInitScript((session) => localStorage.setItem("agent-desk-session", JSON.stringify(session)), session)
      let customer = { id: 1, name: "Synthetic customer / 测试联系人", gender: 0, companyId: 0, primaryEmail: "qa@example.test", primaryMobile: "", remark: "Synthetic QA note", avatar: "/images/logo.png", avatarState: "ready", channelName: "Channel profile", manualTags: ["VIP", "Needs quote / 待报价"], createdAt: "2026-09-14 10:00:00", updatedAt: "2026-09-14 10:00:00" }
      const conversation = () => ({ id: 1, channelId: 3, aiAgentId: 1, customerId: 1, customerName: customer.name, customerAvatar: customer.avatar, status: 3, serviceMode: 1, currentAssigneeId: 1, currentAssigneeName: "Synthetic QA", agentUnreadCount: 0, customerUnreadCount: 0, lastMessageId: 0, tags: [], workStatus: "needs_reply", workRevision: 1 })
      let failSave = true, failSync = true, saves = 0, sends = 0
      const errors = []
      await context.route("**/api/**", async (route) => {
        const url = new URL(route.request().url())
        let data = { results: [], page: { page: 1, limit: 50, total: 0 } }
        let success = true
        if (url.pathname === "/api/config") data = { language: locale, companyName: "Synthetic QA", passwordLoginEnabled: true }
        else if (url.pathname === "/api/auth/profile") data = session
        else if (url.pathname.endsWith("/conversations")) data = { results: [conversation()], page: { page: 1, limit: 50, total: 1 } }
        else if (url.pathname.endsWith("/conversation/channels")) data = [{ id: 3, name: "WhatsApp QA", channelType: "whatsapp", status: 0, connectionState: "connected" }]
        else if (url.pathname.endsWith("/conversation/1")) data = conversation()
        else if (url.pathname.endsWith("/conversation/1/delegation")) data = { conversationId: 1, ownerId: 1, ownerName: "Synthetic QA", canManage: true, active: false, previewOnly: true, revision: 1, aiAgentId: 1, instructions: "", startedByName: "", endReason: "", agents: [{ id: 1, name: "Synthetic Agent", liveAvailable: false }], events: [] }
        else if (url.pathname.endsWith("/message_list")) data = { results: [], hasMore: false, cursor: "" }
        else if (url.pathname.endsWith("/customer/1")) data = customer
        else if (url.pathname.endsWith("/customer-contact/list") || url.pathname.endsWith("/tag/all") || url.pathname.endsWith("/list_all")) data = []
        else if (url.pathname.endsWith("/memory")) data = { status: "ready", entries: [], shared: [], dossier: [] }
        else if (url.pathname.endsWith("/suggest_reply")) data = {
          conversationId: 1,
          lastMessageId: 0,
          agentName: locale === "zh-CN" ? "外贸销售助手" : "Export Sales Assistant",
          modelName: "synthetic-model",
          content: locale === "zh-CN" ? "可以。您的项目需要应对哪些天气和安装环境？" : "It can. Which weather and installation conditions must the project handle?",
          skillStatus: "matched",
          skills: [{ id: 7, name: locale === "zh-CN" ? "先确认使用场景" : "Clarify the use case first", reason: locale === "zh-CN" ? "客户询问是否可用，但尚未说明具体环境。" : "The customer asked about suitability without specifying the environment." }],
          tools: [{ code: "builtin/conversation_context", status: "used" }, { code: "builtin/knowledge_retrieve", status: "matched" }],
          knowledgeStatus: "matched",
          sources: [{ knowledgeBaseId: 1, documentId: 2, chunkId: 3, title: locale === "zh-CN" ? "户外安装指南" : "Outdoor installation guide", content: locale === "zh-CN" ? "户外方案需根据风压、雨水与安装基础确认配置。" : "Outdoor configurations depend on wind load, rain exposure, and the mounting substrate." }],
          routingMs: 240,
          retrievalMs: 180,
          generationMs: 460,
          durationMs: 880,
        }
        else if (url.pathname.endsWith("/send_message")) { sends++; data = {} }
        else if (url.pathname.endsWith("/sync_contact")) {
          await new Promise((resolve) => setTimeout(resolve, 300))
          success = !failSync; failSync = false
          customer = { ...customer, avatarState: success ? "hidden" : "failed", avatar: success ? "" : customer.avatar }
          data = customer
        } else if (url.pathname.endsWith("/save_profile")) {
          saves++
          await new Promise((resolve) => setTimeout(resolve, 300))
          success = !failSave; failSave = false
          if (success) customer = { ...customer, ...route.request().postDataJSON() }
          data = customer
        }
        await route.fulfill({ json: { success, errorCode: success ? 0 : 1, message: success ? "" : "Synthetic retryable failure", data } })
      })
      const page = await context.newPage()
      page.on("pageerror", (error) => errors.push(error.message))
      await page.goto(`${base}/dashboard/conversations?conversationId=1`)
      await page.waitForTimeout(1500)
      if (width !== 390) {
        await page.getByRole("button").filter({ hasText: customer.name }).first().evaluate((button) => button.click())
        await page.waitForTimeout(300)
      }
      if (width === 390) {
        await page.getByRole("button", { name: labels.conversation.conversationInfo, exact: true }).click()
        await page.waitForTimeout(600)
      }
      await page.getByRole("button", { name: labels.copilot.generate, exact: true }).evaluate((button) => button.click())
      await page.getByText(labels.copilot.shadow, { exact: true }).waitFor()
      await page.getByText(labels.copilot.trace.externalToolsBlocked, { exact: true }).waitFor()
      await page.getByText(locale === "zh-CN" ? "先确认使用场景" : "Clarify the use case first", { exact: true }).waitFor()
      await page.waitForTimeout(300)
      assert.equal(sends, 0, "shadow suggestion sent a customer message")
      await page.screenshot({ path: path.join(shots, `${locale}-${width}-shadow.png`), fullPage: true })
      try { await page.getByRole("tab", { name: labels.copilot.customer, exact: true }).click({ timeout: 8000 }) }
      catch (error) { console.log({ errors, text: await page.locator("body").innerText() }); throw error }
      const edit = page.getByRole("button", { name: labels.customerForm.editProfile, exact: true })
      await edit.waitFor()
      await page.getByText(customer.remark, { exact: true }).waitFor()
      await page.screenshot({ path: path.join(shots, `${locale}-${width}-profile.png`), fullPage: true })
      await edit.click()
      await page.getByLabel(labels.customerForm.manualTags, { exact: true }).waitFor()
      await page.getByLabel(labels.customerForm.manualTags, { exact: true }).fill("高意向 / High intent")
      await page.getByLabel(labels.customerForm.manualTags, { exact: true }).press("Enter")
      assert.equal(saves, 0, "Enter in tag input must not save the form")
      await page.getByRole("button", { name: labels.customerForm.removeTag.replace("{tag}", "VIP"), exact: true }).click()
      await page.getByLabel(labels.customerForm.manualTags, { exact: true }).fill("Pending draft tag")
      await page.locator("#customer-remark").fill("Local edited note")
      await page.getByRole("dialog", { name: labels.customerForm.editTitle, exact: true }).evaluate((dialog) => {
        for (const element of dialog.querySelectorAll("*")) if (element.scrollTop) element.scrollTop = 0
      })
      await page.screenshot({ path: path.join(shots, `${locale}-${width}-edit.png`), fullPage: true })
      const save = page.getByRole("button", { name: labels.customerForm.save, exact: true })
      await save.click()
      await page.getByText("Synthetic retryable failure").waitFor()
      assert.equal(await page.locator("#customer-remark").inputValue(), "Local edited note")
      await save.click()
      await page.getByRole("dialog", { name: labels.customerForm.editTitle, exact: true }).waitFor({ state: "hidden" })
      assert.deepEqual(customer.manualTags, ["Needs quote / 待报价", "高意向 / High intent", "Pending draft tag"])
      const sync = page.getByRole("button", { name: labels.customerForm.syncContact, exact: true })
      await sync.click()
      await page.getByText("Synthetic retryable failure").last().waitFor()
      await sync.waitFor()
      await page.waitForFunction((label) => !Array.from(document.querySelectorAll("button")).find((el) => el.getAttribute("aria-label") === label)?.disabled, labels.customerForm.syncContact)
      await sync.click()
      await page.getByText(labels.customerForm.avatar.hidden, { exact: true }).waitFor()
      assert.equal(customer.remark, "Local edited note")
      assert.equal(await page.evaluate(() => document.documentElement.scrollWidth > window.innerWidth), false, "page overflow")
      assert.deepEqual(errors, [], "browser errors")
      console.log(`PASS ${locale} ${width}: avatar, notes, tags, pending draft, failed save/retry, sync failure/private avatar`)
      await context.close()
    }
  }
} finally {
  await browser.close()
  await new Promise((resolve) => server.close(resolve))
}
