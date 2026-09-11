import assert from "node:assert/strict"
import { readFileSync } from "node:fs"
import { createRequire } from "node:module"
import test from "node:test"
import vm from "node:vm"
import ts from "typescript"

const require = createRequire(import.meta.url)
function loadStore(api = {}) {
  const cache = new Map()
  const stubs = {
    "zustand": { create: require("zustand/vanilla").createStore },
    "@/lib/api/agent": { fetchAgentMessages: async () => ({results: []}), ...api },
    "@/lib/im-message": { summarizeIMMessage: (message) => message.content },
    "@/lib/utils": { generateUUID: () => "test-request" },
    "@/i18n/messages": { translateCurrentMessage: (key) => key },
  }
  function load(path) {
    if (stubs[path]) return stubs[path]
    if (cache.has(path)) return cache.get(path)
    const file = new URL(`../${path.replace("@/", "")}.ts`, import.meta.url)
    const source = readFileSync(file, "utf8")
    const compiledModule = { exports: {} }
    cache.set(path, compiledModule.exports)
    const compiled = ts.transpileModule(source, { compilerOptions: {target: ts.ScriptTarget.ES2022, module: ts.ModuleKind.CommonJS} })
    vm.runInNewContext(compiled.outputText, { module: compiledModule, exports: compiledModule.exports, require: load })
    return compiledModule.exports
  }
  return load("@/lib/stores/agent-conversations")
}
const conversation = (id) => ({ id, customerName:`Buyer ${id}`, channelId: id, status: 3, currentAssigneeId: 7, agentUnreadCount: 0 })
const page = (results, total = results.length) => ({results, page: {page:1, limit:50, total}})

test("copilot insertion preserves drafts, never sends, and rejects stale or duplicate requests", async () => {
  let sends = 0
  const {useAgentConversationsStore: store} = loadStore({sendAgentMessage: async () => { sends++ }})
  store.setState({selectedConversationId:1, selectedConversationData:{...conversation(1),lastMessageId:9}})
  store.getState().setDraft(1,"Existing draft")
  assert.equal(store.getState().insertSuggestion(2,9,"Wrong customer"),false)
  assert.equal(store.getState().insertSuggestion(1,8,"Stale"),false)
  assert.equal(store.getState().insertSuggestion(1,9,"Suggestion"),true)
  assert.equal(store.getState().insertSuggestion(1,9,"Duplicate"),false)
  assert.equal(store.getState().drafts[1],"Existing draft")
  assert.equal(sends,0)
  store.getState().consumeDraftInsertion("wrong-id")
  assert.notEqual(store.getState().draftInsertion,null)
  store.getState().consumeDraftInsertion(store.getState().draftInsertion.id)
  assert.equal(store.getState().draftInsertion,null)
  store.setState({selectedConversationData:{...conversation(1),status:1,lastMessageId:9}})
  assert.equal(store.getState().insertSuggestion(1,9,"AI serving"),false)
})

test("inbox defaults to all and queries channel, account, search and pagination together", () => {
  const {useAgentConversationsStore: store, buildConversationQuery} = loadStore()
  assert.equal(store.getState().conversationFilter, "all")
  assert.deepEqual(JSON.parse(JSON.stringify(buildConversationQuery("unread", " Buyer ", "whatsapp", "3", 2))), {filter:"unread", keyword:"Buyer", channelType:"whatsapp", channelId:"3", page:2, limit:50})
})

test("loading the inbox never auto-opens or marks another customer as read", async () => {
  const {useAgentConversationsStore: store} = loadStore({fetchAgentConversations: async () => page([conversation(1)])})
  await store.getState().loadConversations()
  assert.equal(store.getState().conversations.length, 1)
  assert.equal(store.getState().selectedConversationId, null)
})

test("selected conversation remains available after leaving the unread filter", async () => {
  let results = [conversation(1)]
  const {useAgentConversationsStore: store, agentConversationSelectors: selectors} = loadStore({fetchAgentConversations: async () => page(results), fetchAgentConversationDetail: async () => conversation(1)})
  store.getState().setConversationFilter("unread")
  await store.getState().loadConversations()
  await store.getState().selectConversation(1)
  results = []
  await store.getState().loadConversations()
  assert.equal(store.getState().conversations.length, 0)
  assert.equal(selectors.selectedConversation(store.getState()).id, 1)
})

test("an older query cannot overwrite a newer channel filter", async () => {
  let resolveOld
  const {useAgentConversationsStore: store} = loadStore({fetchAgentConversations: (query) => query.channelType ? Promise.resolve(page([conversation(2)])) : new Promise((resolve) => { resolveOld = resolve })})
  const old = store.getState().loadConversations()
  store.getState().setChannelFilter("whatsapp")
  await store.getState().loadConversations()
  resolveOld(page([conversation(1)]))
  await old
  assert.equal(store.getState().conversations[0].id, 2)
})

test("explicit channel changes clear selection but preserve its draft", async () => {
  const {useAgentConversationsStore: store} = loadStore({fetchAgentConversations: async () => page([conversation(1)])})
  await store.getState().loadConversations()
  await store.getState().selectConversation(1)
  store.getState().setDraft(1, "Saved draft")
  store.getState().setChannelFilter("whatsapp")
  assert.equal(store.getState().selectedConversationId, null)
  assert.equal(store.getState().selectedConversationData, null)
  assert.equal(store.getState().drafts[1], "Saved draft")
})

test("load more refreshes the loaded window and deduplicates shifting pages", async () => {
  const calls = []
  const {useAgentConversationsStore: store} = loadStore({fetchAgentConversations: async (query) => { calls.push(query.page); return page(query.page === 1 ? [conversation(1)] : [conversation(1), conversation(2)], 60) }})
  await store.getState().loadConversations()
  await store.getState().loadConversations(true)
  assert.deepEqual(calls, [1,1,2])
  assert.equal(store.getState().conversations.length, 2)
  assert.equal(store.getState().conversationsTotal, 60)
})

test("failed sends preserve separate drafts and release the send lock", async () => {
  const {useAgentConversationsStore: store} = loadStore({sendAgentMessage: async () => { throw Error("offline") }})
  store.setState({selectedConversationId:1})
  store.getState().setDraft(1, "First buyer draft")
  store.getState().setDraft(2, "Second buyer draft")
  await assert.rejects(store.getState().sendMessage("First buyer draft"), /offline/)
  assert.equal(store.getState().drafts[1], "First buyer draft")
  assert.equal(store.getState().drafts[2], "Second buyer draft")
  assert.equal(store.getState().sending, false)
})

test("reapplying the same filters does not clear the inbox without a new request", async () => {
  const {useAgentConversationsStore: store} = loadStore({fetchAgentConversations: async () => page([conversation(1)])})
  await store.getState().loadConversations()
  store.getState().setSearchKeyword("")
  store.getState().setChannelFilter("")
  store.getState().setConversationFilter("all")
  assert.equal(store.getState().conversations.length, 1)
})
