import test from "node:test"
import assert from "node:assert/strict"
import { readFileSync } from "node:fs"
import vm from "node:vm"
import ts from "typescript"

function load(path) {
  const source = readFileSync(new URL(path, import.meta.url), "utf8")
  const module = { exports: {} }
  const compiled = ts.transpileModule(source, { compilerOptions: { target: ts.ScriptTarget.ES2022, module: ts.ModuleKind.CommonJS } })
  vm.runInNewContext(compiled.outputText, { module, exports: module.exports, require: () => load("./generated/enums.ts") })
  return module.exports
}
const { shouldNotifyCustomerMessage, isReplyOverdue, workStatusKey } = load("./reception-work.ts")
test("active assigned customer messages notify, not other users, AI, pending or closed", () => {
  assert.equal(shouldNotifyCustomerMessage("customer",3,7,7,true),true)
  for (const args of [["customer",2,7,7,true],["customer",4,7,7,true],["ai",3,7,7,true],["customer",3,8,7,true],["customer",3,7,7,false],["customer",3,0,0,true]]) assert.equal(shouldNotifyCustomerMessage(...args),false)
})
test("unread is independent of handling deadlines and closed/snoozed never show overdue", () => {
  const item = {status:3,workStatus:"needs_reply",replyDueAt:"2026-09-11T01:00:00Z",agentUnreadCount:0}
  const now = Date.parse("2026-09-11T02:00:00Z")
  assert.equal(isReplyOverdue(item,now),true)
  assert.equal(isReplyOverdue({...item,status:4},now),false)
  assert.equal(isReplyOverdue({...item,workStatus:"snoozed"},now),false)
  assert.equal(workStatusKey("needs_reply"),"reception.needsReply")
})

test("locale catalogs have unique keys and preserve reception playbook and work labels", () => {
  for (const locale of ["zh-CN", "en-US"]) {
    const source = readFileSync(new URL(`../messages/${locale}.json`, import.meta.url), "utf8")
    const tree = ts.parseJsonText(`${locale}.json`, source)
    function visit(node) {
      if (ts.isObjectLiteralExpression(node)) {
        const names = new Set()
        for (const property of node.properties) {
          const name = property.name.text
          assert.ok(!names.has(name), `${locale}: duplicate key ${name}`)
          names.add(name)
        }
      }
      ts.forEachChild(node, visit)
    }
    visit(tree)
    const catalog = JSON.parse(source)
    for (const key of ["title", "instructions", "fields", "collaboration", "needsReply", "waiting", "snoozed", "overdue", "dueAt", "filterNeedsReply", "filterOverdue", "filterWaiting", "filterSnoozed", "note", "noteSaved", "reloadState"]) {
      assert.equal(typeof catalog.reception[key], "string", `${locale}: missing reception.${key}`)
    }
  }
})
