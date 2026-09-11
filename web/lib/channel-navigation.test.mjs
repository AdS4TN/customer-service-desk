import assert from "node:assert/strict"
import { readFile } from "node:fs/promises"
import test from "node:test"
import ts from "typescript"
import vm from "node:vm"

async function loadNavigation() {
  const source = await readFile(new URL("./navigation.tsx", import.meta.url), "utf8")
  const compiled = ts.transpileModule(source, {
    compilerOptions: { target: ts.ScriptTarget.ES2022, module: ts.ModuleKind.CommonJS, jsx: ts.JsxEmit.ReactJSX },
    fileName: "navigation.tsx",
  })
  const result = { exports: {} }
  vm.runInNewContext(compiled.outputText, {
    module: result, exports: result.exports,
    require: () => ({ jsx: () => null }),
  })
  return result.exports
}

test("channels are a separate section with permission-protected website, Messenger and WhatsApp entries", async () => {
  const nav = await loadNavigation()
  const visible = nav.filterDashboardNavForSession(["channel.view"], [])
  const channels = visible.find((section) => section.titleKey === "nav.channels")
  assert.equal(JSON.stringify(channels.items.map((item) => item.url)), JSON.stringify([
    "/dashboard/channels/web", "/dashboard/channels/messenger", "/dashboard/channels/whatsapp",
  ]))
  assert.equal(nav.filterDashboardNavForSession([], []).some((section) => section.titleKey === "nav.channels"), false)
  assert.equal(nav.dashboardNavSections.find((section) => section.titleKey === "nav.agentConfig").items.some((item) => item.url.startsWith("/dashboard/channels")), false)
  assert.equal(nav.getPageTitleKey("/dashboard/channels/web"), "nav.webChannels")
  assert.equal(nav.getPageTitleKey("/dashboard/channels/whatsapp"), "nav.whatsappChannels")
  assert.equal(nav.getPageTitleKey("/dashboard/channels/messenger"), "nav.messengerChannels")
})
