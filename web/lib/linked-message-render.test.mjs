import assert from "node:assert/strict"
import { readFileSync, existsSync, readdirSync, writeFileSync } from "node:fs"
import { createRequire } from "node:module"
import { join, resolve } from "node:path"
import { fileURLToPath } from "node:url"
import vm from "node:vm"
import test from "node:test"
import ts from "typescript"
import React from "react"
import { renderToStaticMarkup } from "react-dom/server"

const require = createRequire(import.meta.url)
const root = fileURLToPath(new URL("../", import.meta.url))
let locale = "zh-CN"
const dictionaries = Object.fromEntries(["zh-CN", "en-US"].map(l => [l, JSON.parse(readFileSync(join(root, "messages", `${l}.json`), "utf8"))]))
const t = (key, values = {}) => {
  const value = key.split(".").reduce((v, k) => v?.[k], dictionaries[locale]) || key
  return value.replace(/\{(\w+)\}/g, (_, k) => String(values[k] ?? ""))
}
const cache = new Map()
function load(id) {
  if (id === "@/i18n/provider") return { useI18n: () => t, useAppLocale: () => ({ locale }) }
  if (id === "@/i18n/messages") return { translateCurrentMessage: t }
  if (!id.startsWith("@/")) return require(id)
  if (cache.has(id)) return cache.get(id).exports
  const base = join(root, id.slice(2))
  const file = [base + ".ts", base + ".tsx"].find(existsSync)
  assert.ok(file, `Missing module: ${id}`)
  const compiled = ts.transpileModule(readFileSync(file, "utf8"), { compilerOptions: { target: ts.ScriptTarget.ES2022, module: ts.ModuleKind.CommonJS, jsx: ts.JsxEmit.ReactJSX, esModuleInterop: true } })
  const loadedModule = { exports: {} }
  cache.set(id, loadedModule)
  vm.runInNewContext(compiled.outputText, { module: loadedModule, exports: loadedModule.exports, require: load, URL, Intl }, { filename: file })
  return loadedModule.exports
}
const { LinkedMessageContent } = load("@/components/chat/linked-message-content")
const render = (meta, content = "", asset = {}) => renderToStaticMarkup(React.createElement(LinkedMessageContent, { message: { content, payload: JSON.stringify({ linkedMessage: meta, ...asset }) } }))

test("message renderer shows structured content without executable remote controls", () => {
  for (locale of ["zh-CN", "en-US"]) {
    const poll = render({ kind: "poll", title: "Choose <script>alert(1)</script>", selectable: 1, options: [{ label: "White", count: 0 }, { label: "Black", count: 4 }] })
    assert.match(poll, /White/)
    assert.match(poll, /Black/)
    assert.match(poll, /&lt;script&gt;/)
    assert.doesNotMatch(poll, /<script|<button|<input/)
    assert.match(render({ kind: "product", title: "Product A", fields: [{ key: "price", value: "USD 12.500" }] }), /USD 12.500/)
    assert.match(render({ kind: "order", fields: [{ key: "orderId", value: "ORDER-1" }] }), /ORDER-1/)
    assert.match(render({ kind: "event", title: "Meeting", state: "canceled", startAt: 1800000000 }), /Meeting/)
    const failedReaction = render({ kind: "reaction", state: "encrypted_unavailable", rawType: "encReactionMessage" })
    assert.ok(failedReaction.includes(t("linkedMessage.encryptedUnavailable")))
    assert.ok(!failedReaction.includes(t("linkedMessage.reactionRemoved")))
    assert.ok(render({ kind: "unsupported" }).includes(t("linkedMessage.unsupportedHint")))
    const ptv = render({ kind: "round_video", state: "ready" }, "", { assetId: "fixture", mimeType: "video/mp4", url: "/fixture.mp4" })
    assert.match(ptv, /<video/)
    assert.match(ptv, /rounded-full/)
    const unsafe = render({ kind: "event", url: "javascript:alert(1)" })
    assert.doesNotMatch(unsafe, /href=/)
    const link = render({ kind: "event", url: "https://example.com/event" })
    assert.match(link, /<a /)
    assert.match(link, /text-secondary-foreground/)
    assert.doesNotMatch(link, /role="button"/)
  }
})

// Optional, isolated browser fixture output. No route or records are added to the app.
if (process.env.AGENTDESK_MESSAGE_QA_DIR) {
  const output = resolve(process.env.AGENTDESK_MESSAGE_QA_DIR)
  assert.ok(existsSync(output), "Create a temporary QA directory first")
  const cssDir = join(root, "out/_next/static/chunks")
  const css = readdirSync(cssDir).filter(f => f.endsWith(".css")).map(f => readFileSync(join(cssDir, f), "utf8")).join("\n")
  const samples = [
    { kind: "poll", title: "Which option works best for your project?", selectable: 1, options: [{ label: "Standard / 标准款", description: "Available in white and black" }, { label: "Custom / 定制款", count: 0 }] },
    { kind: "list", title: "Choose a service", options: [{ label: "产品报价与交付时间咨询", description: "Quotes and delivery estimates" }, { label: "LongProductReferenceWithoutSpacesToTestNarrowMessageWrapping1234567890" }] },
    { kind: "order", title: "Order AD-2409", fields: [{ key: "orderId", value: "ORDER-REFERENCE-1234567890" }, { key: "itemCount", value: "25" }, { key: "total", value: "USD 1250.000" }] },
    { kind: "event", title: "Product consultation / 产品咨询", startAt: 1800000000, state: "canceled", url: "https://example.com/event" },
    { kind: "reaction", state: "encrypted_unavailable", rawType: "encReactionMessage" },
    { kind: "poll_vote", targetPreview: "Choose a finish", options: [{ label: "White" }] },
    { kind: "unsupported", rawType: "field_777" },
    { kind: "text", update: "edited", forwarded: true, title: "Updated message / 已编辑消息" },
  ]
  const columns = ["zh-CN", "en-US"].map(language => {
    locale = language
    return `<section style="width:280px;min-width:0"><h2 style="margin-bottom:16px">${language}</h2>${samples.map((meta, i) => `<article style="margin-bottom:16px;padding:12px;border-radius:6px;${i % 2 ? "background:var(--primary);color:var(--primary-foreground)" : "background:var(--muted);color:var(--foreground)"}">${render(meta)}</article>`).join("")}</section>`
  }).join("")
  writeFileSync(join(output, "index.html"), `<!doctype html><html><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>WhatsApp message renderer fixtures</title><style>${css}</style></head><body style="padding:24px;font-family:Arial,sans-serif"><h1 style="font-size:18px;margin-bottom:20px">WhatsApp message renderer fixtures</h1><main style="display:flex;align-items:start;flex-wrap:wrap;gap:24px">${columns}</main></body></html>`)
}
