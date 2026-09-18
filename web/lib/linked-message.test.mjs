import assert from "node:assert/strict"
import { readFile } from "node:fs/promises"
import test from "node:test"
import ts from "typescript"
import vm from "node:vm"

const source = await readFile(new URL("./linked-message.ts", import.meta.url), "utf8")
const compiled = ts.transpileModule(source, { compilerOptions: { target: ts.ScriptTarget.ES2022, module: ts.ModuleKind.CommonJS } })
const mod = { exports: {} }
vm.runInNewContext(compiled.outputText, { module: mod, exports: mod.exports, URL })
const { parseLinkedMessage, safeMediaURL, linkedKind, linkedDate } = mod.exports

test("preserve types and reject malformed metadata", () => {
  assert.equal(parseLinkedMessage("broken"), null)
  assert.equal(parseLinkedMessage('{"linkedMessage":{"kind":4}}'), null)
  for (const kind of ["text", "image", "sticker", "audio", "video", "round_video", "document", "reaction", "location", "live_location", "contact", "view_once", "poll", "poll_vote", "poll_update", "buttons", "list", "template", "interactive", "reply", "product", "order", "event", "event_response", "album", "sticker_pack", "invite", "payment", "call", "notice", "revoked"]) {
    assert.equal(parseLinkedMessage(JSON.stringify({linkedMessage: {kind}})).kind, kind)
    assert.equal(linkedKind(kind), kind)
  }
  assert.equal(linkedKind("unknown"), "unsupported")
})

test("structured data validates values and keeps usable content", () => {
  const parsed = parseLinkedMessage(JSON.stringify({ linkedMessage: {
    kind: "poll", title: "A long poll", selectable: 1, startAt: 1800000000,
    options: [null, "broken", { label: 4 }, { label: "A", count: -3 }, { label: "B", count: 0, description: "Details" }],
    fields: [null, { key: "total", value: "USD 12.000" }, { key: 1, value: "bad" }],
    url: "javascript:alert(1)", rawType: "pollCreationMessage", forwarded: "true", seconds: -10
  } }))
  assert.equal(parsed.options.length, 2)
  assert.equal(parsed.options[0].count, undefined)
  assert.equal(parsed.options[1].count, 0)
  assert.equal(parsed.fields.length, 1)
  assert.equal(parsed.url, undefined)
  assert.equal(parsed.forwarded, false)
  assert.equal(parsed.seconds, undefined)
  assert.equal(linkedDate(1e30, "en-US"), undefined)
  assert.ok(linkedDate(1800000000, "zh-CN"))
})

test("both locales cover all message types and statuses", async () => {
  for (const locale of ["en-US", "zh-CN"]) {
    const labels = JSON.parse(await readFile(new URL(`../messages/${locale}.json`, import.meta.url), "utf8")).linkedMessage
    for (const key of ["round_video", "poll", "poll_vote", "poll_update", "buttons", "list", "template", "interactive", "reply", "product", "order", "event", "event_response", "album", "sticker_pack", "invite", "payment", "call", "notice", "revoked", "encryptedUnavailable", "targetUnavailable", "voteRemoved", "partial", "edited", "forwarded"]) assert.ok(labels[key], `${locale}: ${key}`)
  }
})
test("only serve http(s) or same-origin attachment URLs", () => {
  for (const url of ["javascript:alert(1)", "data:text/html,test", "//evil.test", "/\\evil.test", "file:///tmp/secret"]) assert.equal(safeMediaURL(url), undefined)
  assert.equal(safeMediaURL("/uploads/test.webp"), "/uploads/test.webp")
  assert.equal(safeMediaURL("https://cdn.example/test.ogg"), "https://cdn.example/test.ogg")
})
