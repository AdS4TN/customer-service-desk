import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import test from "node:test";
import vm from "node:vm";
import ts from "typescript";

function compile(file, require, extra = {}) {
  const compiled = { exports: {} };
  const source = readFileSync(new URL(file, import.meta.url), "utf8");
  const output = ts.transpileModule(source, { compilerOptions: { target: ts.ScriptTarget.ES2022, module: ts.ModuleKind.CommonJS, jsx: ts.JsxEmit.ReactJSX } });
  vm.runInNewContext(output.outputText, { module: compiled, exports: compiled.exports, require, AbortController, ...extra });
  return compiled.exports;
}
const enums = compile("./generated/enums.ts");
const logic = compile("./translation.ts", () => enums);

test("translation sends literal text with preserved line breaks, never model HTML", () => {
  assert.equal(logic.translatedTextHTML('<img src=x onerror="alert(1)">\nA & B'), '<p>&lt;img src=x onerror=&quot;alert(1)&quot;&gt;</p><p>A &amp; B</p>');
});

test("preview is bound to customer, channel, assignment, agent and message revision", () => {
  const c = { id: 1, lastMessageId: 8, customerId: 7, channelId: 3, aiAgentId: 2, currentAssigneeId: 9, status: 3 };
  assert.equal(logic.isTranslationContextCurrent(c, { ...c }, 1), true);
  for (const key of Object.keys(c)) assert.equal(logic.isTranslationContextCurrent(c, { ...c, [key]: c[key] + 1 }, 1), false, key);
  assert.equal(logic.isTranslationContextCurrent(c, c, 2), false);
  assert.equal(logic.isTranslationContextCurrent(c, null, 1), false);
});

test("supported reading languages exclude automatic detection and locale keys match", () => {
  assert.equal(logic.translationLanguageOptions("Auto").some((l) => l.value === "auto"), false);
  assert.equal(logic.translationLanguageOptions("Auto", true)[0].label, "Auto");
  const read = (locale) => JSON.parse(readFileSync(new URL(`../messages/${locale}.json`, import.meta.url), "utf8")).translation;
  const zh = read("zh-CN"), en = read("en-US");
  assert.deepEqual(Object.keys(zh).sort(), Object.keys(en).sort());
  for (const key of Object.keys(zh)) assert.deepEqual(zh[key].match(/\{\w+\}/g), en[key].match(/\{\w+\}/g));
});

// Small hook harness exercises the real composer interception without browser credentials or customer sends.
function harness(sendImpl = async () => {}, apiImpl, storage = {}) {
  let cursor = 0;
  const slots = [], effects = [], listeners = new Set(), sends = [], requests = [];
  const state = { selectedConversationId: 1, drafts: { 1: "<p>draft</p>" }, sending: false, uploadingAsset: false, conversation: { id: 1, lastMessageId: 8, customerId: 7, channelId: 3, aiAgentId: 2, currentAssigneeId: 9, status: 3, customerName: "Synthetic" } };
  const t = (key) => key;
  const preferences = { customer: "auto", beforeSend: true };
  const confirmations = [];
  const react = {
    useRef(value) { const i = cursor++; return slots[i] ??= { current: value }; },
    useState(value) { const i = cursor++; if (!(i in slots)) slots[i] = value; return [slots[i], (next) => { slots[i] = next; }]; },
    useCallback(fn) { cursor++; return fn; },
    useEffect(fn) { const i = cursor++; if (!(i in slots)) { slots[i] = true; effects.push(fn()); } },
  };
  const hook = compile("../app/(dashboard)/dashboard/conversations/_components/translated-reply.tsx", (name) => {
    if (name === "react") return react;
    if (name === "react/jsx-runtime") return { jsx: (type, props) => ({ type, props }), jsxs: (type, props) => ({ type, props }) };
    if (name === "sonner") return { toast: { error() {} } };
    if (name.endsWith("auth-provider")) return { useAuth: () => ({ session: { user: { id: 9 } } }) };
    if (name.endsWith("confirm-provider")) return { useConfirm: () => (options) => new Promise((resolve) => confirmations.push({ options, resolve })) };
    if (name.endsWith("i18n/provider")) return { useI18n: () => t };
    if (name.endsWith("generated/enums")) return enums;
    if (name.endsWith("stores/conversation-translation")) return { useTranslationPreferences: () => ({ preferences, update: (patch) => Object.assign(preferences, patch), readDraft: () => storage.draft, saveDraft: (draft) => { storage.draft = draft; } }) };
    if (name.endsWith("stores/agent-conversations")) return { useAgentConversationsStore: { getState: () => state, subscribe: (fn) => { listeners.add(fn); return () => listeners.delete(fn); } }, agentConversationSelectors: { selectedConversation: (s) => s.conversation } };
    if (name.endsWith("lib/translation")) return { ...logic, translationDraftText: (html) => html.includes("<img") ? null : "draft" };
    if (name.endsWith("api/conversation-translation")) return { translateConversationText: async (...args) => { requests.push(args); return apiImpl ? apiImpl(...args) : { conversationId: 1, lastMessageId: 8, targetLanguage: "en", content: "Translated <draft>" }; } };
    return new Proxy({}, { get: (_, key) => String(key) });
  }).useTranslatedReply;
  function render() { cursor = 0; return hook(1, async (html) => { sends.push(html); await sendImpl(html); }, false); }
  function find(node, predicate) {
    if (!node || typeof node !== "object") return undefined;
    if (predicate(node)) return node;
    for (const child of [node.props?.children].flat(Infinity)) { const result = find(child, predicate); if (result) return result; }
  }
  const button = (label) => find(render().dialog, (node) => node.type === "Button" && [node.props.children].flat().includes(label));
  const field = () => find(render().dialog, (node) => node.type === "Textarea");
  const language = () => find(render().dialog, (node) => node.type === "OptionCombobox");
  return { render, sends, requests, state, button, field, language, confirmations, storage, change() { for (const fn of listeners) fn(state); }, unmount() { for (const fn of effects) fn?.(); } };
}
const tick = () => new Promise((resolve) => setImmediate(resolve));

test("cancel keeps the draft and never sends; successful confirmation escapes editable text", async () => {
  const h = harness();
  const canceled = h.render().send("<p>draft</p>");
  const rejected = assert.rejects(canceled);
  await tick();
  assert.equal(h.sends.length, 0);
  h.button("translation.cancel").props.onClick();
  await rejected;
  assert.equal(h.state.drafts[1], "<p>draft</p>");
  const successful = h.render().send("<p>draft</p>");
  await tick();
  h.button("translation.confirm").props.onClick();
  await successful;
  assert.deepEqual(h.sends, ["<p>Translated &lt;draft&gt;</p>"]);
  h.unmount();
});

test("new customer activity, draft changes and switching conversations cancel previews", async () => {
  for (const kind of ["activity", "draft", "switch", "unmount"]) {
    const h = harness();
    const rejected = assert.rejects(h.render().send("<p>draft</p>"));
    await tick();
    if (kind === "activity") h.state.conversation.lastMessageId++;
    if (kind === "draft") h.state.drafts[1] = "edited";
    if (kind === "switch") h.state.selectedConversationId = 2;
    if (kind === "unmount") h.unmount(); else h.change();
    await rejected;
    assert.equal(h.sends.length, 0);
  }
});

test("double confirmation is locked and failed sends retain edited translation for retry", async () => {
  let fail;
  let attempts = 0;
  const h = harness(() => ++attempts === 1 ? new Promise((_, reject) => { fail = reject; }) : Promise.resolve());
  const completed = h.render().send("<p>draft</p>");
  await tick();
  h.field().props.onChange({ target: { value: "Corrected translation" } });
  const confirm = h.button("translation.confirm").props.onClick;
  confirm(); confirm();
  assert.equal(h.sends.length, 1);
  fail(new Error("offline"));
  await tick();
  assert.equal(h.render().previewing, true);
  assert.equal(h.field().props.value, "Corrected translation");
  assert.equal(h.state.drafts[1], "<p>draft</p>");
  h.button("translation.confirm").props.onClick();
  await completed;
  assert.equal(h.sends[1], "<p>Corrected translation</p>");
  assert.equal(h.storage.draft, null);
  h.unmount();
});

test("edited translations survive cancel, Escape, unmount and stale context without sending", async () => {
  for (const kind of ["cancel", "escape", "unmount", "activity", "draft", "switch"]) {
    const h = harness();
    const rejected = assert.rejects(h.render().send("<p>draft</p>"));
    await tick();
    h.field().props.onChange({ target: { value: "Manual correction" } });
    if (kind === "cancel") h.button("translation.cancel").props.onClick();
    if (kind === "escape") h.render().dialog.props.onOpenChange(false);
    if (kind === "unmount") h.unmount();
    if (kind === "activity") h.state.conversation.lastMessageId++;
    if (kind === "draft") h.state.drafts[1] = "new draft";
    if (kind === "switch") h.state.selectedConversationId = 2;
    h.change();
    await rejected;
    assert.equal(h.storage.draft.edited, "Manual correction", kind);
    h.state.selectedConversationId = 1;
    const restored = kind === "unmount" ? harness(undefined, undefined, h.storage) : h;
    const closed = assert.rejects(restored.render().send("<p>draft</p>"));
    await tick();
    assert.equal(restored.field().props.value, "Manual correction", kind);
    if (kind === "activity" || kind === "draft") {
      assert.equal(restored.button("translation.confirm").props.disabled, true);
      restored.button("translation.confirm").props.onClick();
    }
    assert.equal(restored.sends.length, 0);
    restored.button("translation.cancel").props.onClick();
    await closed;
    restored.unmount();
  }
});

test("regeneration and target changes require approval before replacing manual edits", async () => {
  for (const kind of ["regenerate", "language"]) {
    const h = harness();
    const closed = assert.rejects(h.render().send("<p>draft</p>"));
    await tick();
    h.field().props.onChange({ target: { value: "Do not discard" } });
    const replace = () => kind === "regenerate" ? h.button("translation.retry").props.onClick() : h.language().props.onChange("fr");
    replace();
    assert.equal(h.requests.length, 1);
    assert.equal(h.confirmations.length, 1);
    assert.equal(h.button("translation.confirm").props.disabled, true);
    h.confirmations[0].resolve(false);
    await tick();
    assert.equal(h.field().props.value, "Do not discard");
    assert.equal(h.language().props.value, "auto");
    replace();
    h.confirmations[1].resolve(true);
    await tick();
    assert.equal(h.requests.length, 2);
    assert.equal(h.requests[1][2], kind === "language" ? "fr" : "auto");
    assert.equal(h.storage.draft.edited, "Translated <draft>");
    h.button("translation.cancel").props.onClick();
    await closed;
    h.unmount();
  }
});

test("failed regeneration preserves corrected text and cannot send a stale preview", async () => {
  let calls = 0;
  const h = harness(undefined, async () => {
    if (++calls > 1) throw new Error("offline");
    return { content: "Original translation", targetLanguage: "en", lastMessageId: 8 };
  });
  const closed = assert.rejects(h.render().send("<p>draft</p>"));
  await tick();
  h.field().props.onChange({ target: { value: "Manual correction" } });
  h.language().props.onChange("fr");
  h.confirmations[0].resolve(true);
  await tick();
  assert.equal(h.field().props.value, "Manual correction");
  assert.equal(h.storage.draft.edited, "Manual correction");
  assert.equal(h.button("translation.confirm").props.disabled, true);
  h.button("translation.cancel").props.onClick();
  await closed;
  h.unmount();
});

test("cancel aborts in-flight translation and mixed media never silently loses attachments", async () => {
  let finish;
  const h = harness(undefined, () => new Promise((resolve) => { finish = resolve; }));
  const rejected = assert.rejects(h.render().send("<p>draft</p>"));
  h.button("translation.cancel").props.onClick();
  assert.equal(h.requests[0][3].aborted, true);
  finish({ content: "stale", targetLanguage: "en", lastMessageId: 8 });
  await rejected;
  await assert.rejects(h.render().send('<p>draft<img src="test"></p>'));
  assert.equal(h.sends.length, 0);
  h.unmount();
});
