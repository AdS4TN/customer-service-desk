import test from "node:test";
import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import ts from "typescript";
const source = readFileSync(new URL("./automation.ts", import.meta.url), "utf8");
const { outputText } = ts.transpileModule(source, { compilerOptions: { module: ts.ModuleKind.ESNext } });
const { automationTemplate, automationResultLink } = await import(`data:text/javascript;base64,${Buffer.from(outputText).toString("base64")}`);
test("templates have no outbound actions and tickets are once per conversation", () => {
  for (const kind of ["inquiry", "lead", "ticket"]) {
    const value = automationTemplate(kind, "Synthetic", "Follow-up");
    assert.equal(value.id, 0);
    assert.ok(value.definition.actions.every(a => !a.type.startsWith("send")));
    if (kind === "ticket") assert.equal(value.definition.oncePerConversation, true);
    if (kind === "lead") assert.equal(value.definition.trigger, "lead_created");
  }
});
test("audit links use only supported resource IDs", () => {
  assert.equal(automationResultLink("ticket:12"), "/dashboard/tickets?ticketId=12");
  assert.equal(automationResultLink("assign_lead:8"), "/dashboard/sales-leads?leadId=8");
  for (const value of ["ticket:javascript:alert(1)", "ticket:-1", "other:2", "tag_exists"]) assert.equal(automationResultLink(value), null);
});
test("automation dictionary parity", () => {
  const zh = JSON.parse(readFileSync(new URL("../messages/zh-CN.json", import.meta.url), "utf8"));
  const en = JSON.parse(readFileSync(new URL("../messages/en-US.json", import.meta.url), "utf8"));
  function keys(v, prefix = "") { return Object.entries(v).flatMap(([k, value]) => typeof value === "object" ? keys(value, prefix + k + ".") : [prefix + k]).sort(); }
  assert.deepEqual(keys(zh.automation), keys(en.automation));
  assert.ok(zh.nav.automation && en.nav.automation);
});
