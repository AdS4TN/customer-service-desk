import assert from "node:assert/strict"
import { readFile } from "node:fs/promises"
import test from "node:test"
import ts from "typescript"
import vm from "node:vm"

async function load(name, dependencies = {}) {
  const source = await readFile(new URL(`./${name}.ts`, import.meta.url), "utf8")
  const compiled = ts.transpileModule(source, { compilerOptions: { target: ts.ScriptTarget.ES2022, module: ts.ModuleKind.CommonJS } })
  const compiledModule = { exports: {} }
  vm.runInNewContext(compiled.outputText, { module: compiledModule, exports: compiledModule.exports, require: (id) => dependencies[id] ?? {} })
  return compiledModule.exports
}

for (const prefix of ["wa_", "ms_"]) test(`${prefix} backfill merges and paginates by original timestamp, with ID ties`, async () => {
  const api = await load("im-message-merge")
  const messages = [
    { id: 2, clientMsgId: "reply", sentAt: "2026-09-09T12:00:00Z" },
    { id: 4, clientMsgId: `${prefix}backfill`, sentAt: "2026-09-01T12:00:00Z" },
    { id: 3, clientMsgId: `${prefix}backfill2`, sentAt: "2026-09-01T12:00:00Z" },
  ]
  const merged = api.mergeImMessagesByIdAsc(messages, [messages[1]])
  assert.equal(JSON.stringify(merged.map((m) => m.id)), "[3,4,2]")
  assert.equal(api.cursorFromLoadedImMessages(merged), "3")
  assert.equal(api.hasMoreAfterLatestImMessageMerge({ previousMessages: messages, previousHasMore: false, merged, apiHasMore: true }), true)
})

test("non-WhatsApp ID cursor behavior is preserved", async () => {
  const api = await load("im-message-merge")
  const merged = api.mergeImMessagesByIdAsc([{ id: 8 }], [{ id: 2 }, { id: 3 }])
  assert.equal(JSON.stringify(merged.map((m) => m.id)), "[2,3,8]")
  assert.equal(api.cursorFromLoadedImMessages(merged), "2")
})

test("backfill cannot replace a newer conversation summary", async () => {
  const api = await load("im-realtime-state", { "@/lib/im-message": { summarizeIMMessage: (m) => m.content } })
  const conversation = { id: 1, lastMessageAt: "2026-09-09T12:00:00Z", lastMessageSummary: "new" }
  assert.equal(api.patchConversationWithMessage(conversation, { conversationId: 1, id: 99, sentAt: "2026-09-01T12:00:00Z", content: "old" }), conversation)
})
