import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import test from "node:test";
import ts from "typescript";
import vm from "node:vm";

test("lead translations cover the same fields, states and placeholders", () => {
  const read = (locale) => JSON.parse(readFileSync(new URL(`../messages/${locale}.json`, import.meta.url), "utf8")).lead;
  const compare = (zh, en) => {
    assert.deepEqual(Object.keys(zh).sort(), Object.keys(en).sort());
    for (const key of Object.keys(zh)) {
      if (typeof zh[key] === "object") compare(zh[key], en[key]);
      else assert.deepEqual(zh[key].match(/\{\w+\}/g), en[key].match(/\{\w+\}/g));
    }
  };
  compare(read("zh-CN"), read("en-US"));
});

test("lead API encodes filters and preserves review revision and proposal intent", async () => {
  const calls = [];
  const compiled = ts.transpileModule(readFileSync(new URL("./api/sales-lead.ts", import.meta.url), "utf8"), { compilerOptions: { module: ts.ModuleKind.CommonJS } });
  const testModule = { exports: {} };
  vm.runInNewContext(compiled.outputText, { module: testModule, exports: testModule.exports, URLSearchParams, require: () => ({ request: async (...args) => { calls.push(args); } }) });
  await testModule.exports.fetchSalesLeads({ keyword: "a&b", status: "", conversationId: 7 });
  assert.equal(calls[0][0], "/api/dashboard/conversation/sales_leads?keyword=a%26b&conversationId=7");
  const payload = { revision: 3, acceptProposal: true, data: { quantity: "300" }, customTags: ["priority"] };
  await testModule.exports.updateSalesLead(5, payload);
  assert.equal(calls[1][0], "/api/dashboard/conversation/sales_leads/5/update");
  assert.equal(calls[1][1].method, "POST");
  assert.deepEqual(JSON.parse(calls[1][1].body), payload);
  const follow = {revision:4, status:"following", ownerId:2, result:"Sent quote", nextAction:"Confirm specification", followUpAt:"2026-10-01T12:00:00Z"};
  await testModule.exports.recordLeadFollowUp(5, follow);
  assert.equal(calls[2][0], "/api/dashboard/conversation/sales_leads/5/follow_up");
  assert.deepEqual(JSON.parse(calls[2][1].body), follow);
  await testModule.exports.fetchSalesLeads({queue:"overdue", mine:"true"});
  assert.equal(calls[3][0], "/api/dashboard/conversation/sales_leads?queue=overdue&mine=true");
});

test("sales follow-up translations cover both locales", () => {
  const zh = JSON.parse(readFileSync(new URL("../messages/zh-CN.json", import.meta.url), "utf8")).leadWork;
  const en = JSON.parse(readFileSync(new URL("../messages/en-US.json", import.meta.url), "utf8")).leadWork;
  assert.deepEqual(Object.keys(zh).sort(), Object.keys(en).sort());
  for (const key of Object.keys(zh)) assert.deepEqual(zh[key].match(/\{\w+\}/g), en[key].match(/\{\w+\}/g));
});

test("lead analysis never reports empty while loading, failing or retrying", () => {
  const compiled = ts.transpileModule(readFileSync(new URL("./lead-analysis.ts", import.meta.url), "utf8"), { compilerOptions: { module: ts.ModuleKind.CommonJS } });
  const mod = { exports: {} };
  vm.runInNewContext(compiled.outputText, { module: mod, exports: mod.exports });
  const state = mod.exports.leadAnalysisState;
  for (const status of ["queued", "processing", "failed"]) assert.equal(state({ status }).emptyKey, null);
  assert.equal(state(null).emptyKey, null);
  assert.equal(state({ status: "ready" }).emptyKey, "leadAnalysis.noIntent");
  assert.equal(state({ status: "empty" }).emptyKey, "leadAnalysis.notAnalyzed");
  assert.equal(state({ status: "queued", errorCode: "lead_validation_failed" }).retrying, true);
  assert.equal(state({ status: "processing", errorCode: "lead_validation_failed" }).retrying, true);
  assert.equal(state({ status: "failed", errorCode: "unknown-private-error" }).errorKey, "leadAnalysis.errors.unknown");
  const zh = JSON.parse(readFileSync(new URL("../messages/zh-CN.json", import.meta.url), "utf8")).leadAnalysis;
  const en = JSON.parse(readFileSync(new URL("../messages/en-US.json", import.meta.url), "utf8")).leadAnalysis;
  assert.deepEqual(Object.keys(zh).sort(), Object.keys(en).sort());
  assert.deepEqual(Object.keys(zh.errors).sort(), Object.keys(en.errors).sort());
  assert.deepEqual(zh.attempt.match(/\{\w+\}/g), en.attempt.match(/\{\w+\}/g));
});
