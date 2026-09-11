import assert from "node:assert/strict"
import { readFileSync } from "node:fs"
import test from "node:test"
import vm from "node:vm"
import ts from "typescript"

const module = { exports: {} }
const source = readFileSync(new URL("./reception.ts", import.meta.url), "utf8")
const output = ts.transpileModule(source, { compilerOptions: { module: ts.ModuleKind.CommonJS, target: ts.ScriptTarget.ES2022 } }).outputText
vm.runInNewContext(output, { module, exports: module.exports })
const { receptionPolicyError, emptyReceptionPolicy } = module.exports

test("disabled legacy agents remain valid; activation requires an explicit goal", () => {
  const policy = emptyReceptionPolicy()
  assert.equal(receptionPolicyError(policy), "")
  assert.equal(receptionPolicyError({ ...policy, enabled: true }), "goalRequired")
})

test("inquiry field identifiers and names remain unique", () => {
  const policy = { ...emptyReceptionPolicy(), enabled: true, goal: "Collect requirements", fields: [{ key: "quantity", label: "Quantity", askWhen: "Purchase intent", required: true }] }
  assert.equal(receptionPolicyError(policy), "")
  assert.equal(receptionPolicyError({ ...policy, fields: [...policy.fields, { ...policy.fields[0], key: "other" }] }), "fieldsInvalid")
  assert.equal(receptionPolicyError({ ...policy, fields: [{ ...policy.fields[0], label: " " }] }), "fieldsInvalid")
  assert.equal(receptionPolicyError({ ...policy, fields: Array(17).fill(policy.fields[0]) }), "invalid")
})

test("reception translations have matching keys and placeholders", () => {
  const zh = JSON.parse(readFileSync(new URL("../messages/zh-CN.json", import.meta.url))).reception
  const en = JSON.parse(readFileSync(new URL("../messages/en-US.json", import.meta.url))).reception
  assert.deepEqual(Object.keys(zh).sort(), Object.keys(en).sort())
  for (const key of Object.keys(zh)) assert.deepEqual(zh[key].match(/\{\w+\}/g), en[key].match(/\{\w+\}/g))
})
