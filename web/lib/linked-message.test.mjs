import assert from "node:assert/strict"
import { readFile } from "node:fs/promises"
import test from "node:test"
import ts from "typescript"
import vm from "node:vm"

const source = await readFile(new URL("./linked-message.ts", import.meta.url), "utf8")
const compiled = ts.transpileModule(source, { compilerOptions: { target: ts.ScriptTarget.ES2022, module: ts.ModuleKind.CommonJS } })
const mod = { exports: {} }
vm.runInNewContext(compiled.outputText, { module: mod, exports: mod.exports, URL })
const { parseLinkedMessage, safeMediaURL, linkedKind } = mod.exports

test("preserve types and reject malformed metadata", () => {
  assert.equal(parseLinkedMessage("broken"), null)
  assert.equal(parseLinkedMessage('{"linkedMessage":{"kind":4}}'), null)
  for (const kind of ["image", "sticker", "audio", "video", "document", "reaction", "location", "contact", "view_once"]) {
    assert.equal(parseLinkedMessage(JSON.stringify({linkedMessage: {kind}})).kind, kind)
    assert.equal(linkedKind(kind), kind)
  }
  assert.equal(linkedKind("unknown"), "unsupported")
})
test("only serve http(s) or same-origin attachment URLs", () => {
  for (const url of ["javascript:alert(1)", "data:text/html,test", "//evil.test", "/\\evil.test", "file:///tmp/secret"]) assert.equal(safeMediaURL(url), undefined)
  assert.equal(safeMediaURL("/uploads/test.webp"), "/uploads/test.webp")
  assert.equal(safeMediaURL("https://cdn.example/test.ogg"), "https://cdn.example/test.ogg")
})
