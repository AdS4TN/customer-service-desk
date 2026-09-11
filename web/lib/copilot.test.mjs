import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import test from "node:test";
import ts from "typescript";
import vm from "node:vm";

const compiled = ts.transpileModule(readFileSync(new URL("./copilot.ts", import.meta.url), "utf8"), { compilerOptions: { module: ts.ModuleKind.CommonJS } });
const module = { exports: {} };
vm.runInNewContext(compiled.outputText, { module, exports: module.exports });
const { suggestionParagraphs, isCurrentSuggestion } = module.exports;

test("suggestions use literal TipTap text, never HTML", () => {
  const content = suggestionParagraphs('<img src=x onerror=alert(1)>\nHello');
  assert.equal(content[0].content[0].type, "text");
  assert.equal(content[0].content[0].text, '<img src=x onerror=alert(1)>');
  assert.equal(content[1].content[0].text, "Hello");
});
test("only a matching conversation and message revision can consume a suggestion", () => {
  const result = { conversationId: 1, lastMessageId: 7 };
  assert.equal(isCurrentSuggestion(result, { id: 1, lastMessageId: 7 }), true);
  assert.equal(isCurrentSuggestion(result, { id: 2, lastMessageId: 7 }), false);
  assert.equal(isCurrentSuggestion(result, { id: 1, lastMessageId: 8 }), false);
  assert.equal(isCurrentSuggestion(result, null), false);
});
test("copilot translations match", () => {
  const read = (locale) => JSON.parse(readFileSync(new URL(`../messages/${locale}.json`, import.meta.url), "utf8")).copilot;
  const zh = read("zh-CN"), en = read("en-US");
  assert.deepEqual(Object.keys(zh).sort(), Object.keys(en).sort());
  assert.deepEqual(Object.keys(zh.knowledge).sort(), Object.keys(en.knowledge).sort());
  for (const key of Object.keys(zh)) if (typeof zh[key] === "string") assert.deepEqual(zh[key].match(/\{\w+\}/g), en[key].match(/\{\w+\}/g));
});
