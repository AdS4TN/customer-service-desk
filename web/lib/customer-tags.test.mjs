import assert from "node:assert/strict"
import { readFile } from "node:fs/promises"
import test from "node:test"
import ts from "typescript"
import vm from "node:vm"

const source = await readFile(new URL("./customer-tags.ts", import.meta.url), "utf8")
const compiled = ts.transpileModule(source, { compilerOptions: { target: ts.ScriptTarget.ES2022, module: ts.ModuleKind.CommonJS } })
const mod = { exports: {} }
vm.runInNewContext(compiled.outputText, { module: mod, exports: mod.exports })
const { normalizeCustomerTags } = mod.exports
test("tags trim and deduplicate without losing display case or order", () => {
  assert.deepEqual(Array.from(normalizeCustomerTags([" VIP ", "vip", "", "需报价", "需报价"])), ["VIP", "需报价"])
  assert.deepEqual(Array.from(normalizeCustomerTags([])), [])
})
test("both locales cover contact sync and editing states", async () => {
  for (const locale of ["en-US", "zh-CN"]) {
    const labels = JSON.parse(await readFile(new URL(`../messages/${locale}.json`, import.meta.url), "utf8")).customerForm
    for (const key of ["manualTags", "addTag", "removeTag", "editProfile", "syncContact", "synced", "syncFailed"]) assert.ok(labels[key])
    for (const state of ["hidden", "empty", "failed"]) assert.ok(labels.avatar[state])
  }
})
