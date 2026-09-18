import assert from "node:assert/strict";
import http from "node:http";
import { mkdir, readFile } from "node:fs/promises";
import path from "node:path";
import { createRequire } from "node:module";

const require = createRequire(import.meta.url);
const { chromium } = require(process.env.PLAYWRIGHT_MODULE || "playwright");
const root = path.resolve("out");
const shots = path.resolve("../.impeccable/review/sales-experience");
await mkdir(shots, { recursive: true });

const server = http.createServer(async (req, res) => {
  let filename = path.join(
    root,
    decodeURIComponent(new URL(req.url, "http://localhost").pathname),
  );
  if (!path.extname(filename)) filename += ".html";
  try {
    const data = await readFile(filename);
    const contentType =
      {
        ".html": "text/html",
        ".js": "text/javascript",
        ".css": "text/css",
        ".png": "image/png",
        ".svg": "image/svg+xml",
      }[path.extname(filename)] || "application/octet-stream";
    res.writeHead(200, { "Content-Type": contentType });
    res.end(data);
  } catch {
    res.writeHead(404);
    res.end();
  }
});
await new Promise((resolve) => server.listen(0, "127.0.0.1", resolve));
const base = `http://127.0.0.1:${server.address().port}`;
const browser = await chromium.launch({ channel: "chrome", headless: true });

try {
  for (const locale of ["zh-CN", "en-US"]) {
    for (const width of [1440, 390]) {
      const labels = JSON.parse(
        await readFile(`messages/${locale}.json`, "utf8"),
      );
      const l = labels.sx;
      const context = await browser.newContext({
        viewport: { width, height: width === 390 ? 844 : 1000 },
        reducedMotion: "reduce",
      });
      const session = {
        accessToken: "synthetic-test",
        user: {
          id: 1,
          username: "test",
          nickname: "Synthetic QA",
          userType: "admin",
        },
        permissions: ["*"],
        roles: ["super_admin"],
      };
      await context.addInitScript(
        (value) =>
          localStorage.setItem("agent-desk-session", JSON.stringify(value)),
        session,
      );

      const stamp = "2026-09-16T02:00:00Z";
      const caseItem = {
        id: 2,
        name: "Michael Stone Haven",
        customerId: 1,
        sourceHash: "synthetic",
        messageCount: 249,
        outcome: "progress",
        outcomeNote: "",
        createdAt: stamp,
        snapshot: {
          schemaVersion: "1.0",
          customerId: 1,
          customerName: "Michael Stone Haven",
          conversationIds: [2],
          messages: [],
        },
      };
      const sourceItem = {
        id: 11,
        customerId: 1,
        customerName: "Michael Stone Haven",
        channelType: "whatsapp",
        channelName: "Synthetic WhatsApp",
        lastMessageAt: stamp,
        messageCount: 2,
        status: 1,
      };
      const importPreview = {
        conversationId: 11,
        customerId: 1,
        customerName: "Michael Stone Haven",
        messages: [
          {
            id: 101,
            conversationId: 11,
            role: "buyer",
            type: "text",
            text: "Could this work outdoors?",
            at: stamp,
          },
          {
            id: 102,
            conversationId: 11,
            role: "seller",
            type: "text",
            text: "Which weather conditions should it handle?",
            at: stamp,
          },
        ],
      };
      const skillName =
        locale === "zh-CN"
          ? "遇到结构做法差异时，先确认使用场景"
          : "Clarify the use case before resolving a configuration difference";
      const secondSkillName =
        locale === "zh-CN"
          ? "用聚焦问题确认客户的真实顾虑"
          : "Use a focused question to uncover the customer's real concern";
      const overallReason =
        locale === "zh-CN"
          ? "客户在销售追问后补充了此前没有说明的使用场景，销售动作与客户变化之间存在明确对应关系。"
          : "The customer added a previously unstated use case after the seller's focused question, creating a clear link between the action and the change.";
      const skillResult = {
        protocolVersion: "sales-skill-miner-v2",
        hasLearnableSkill: true,
        assessment: {
          conversationNature: "mixed",
          reason: overallReason,
        },
        candidates: [
          {
            sourceEpisodeId: "episode-1",
            skill: {
              name: skillName,
              description:
                locale === "zh-CN"
                  ? "客户用既有经验质疑方案时，先确认实际使用场景，再判断差异是否影响选择。"
                  : "When a customer challenges a proposal using prior experience, clarify the actual use case before judging whether the difference matters.",
              whenToUse: [
                locale === "zh-CN"
                  ? "客户拿常见做法质疑当前方案，但尚未说明实际应用环境"
                  : "The customer challenges the proposal with a familiar practice but has not stated the actual environment",
              ],
              objective:
                locale === "zh-CN"
                  ? "把笼统质疑转成影响方案判断的具体场景信息。"
                  : "Turn a broad objection into concrete context that affects the decision.",
              steps: [
                {
                  instruction:
                    locale === "zh-CN"
                      ? "先确认客户正在比较的做法。"
                      : "Confirm the practice the customer is comparing.",
                  purpose:
                    locale === "zh-CN"
                      ? "明确质疑来自哪里。"
                      : "Identify the source of the objection.",
                },
                {
                  instruction:
                    locale === "zh-CN"
                      ? "用一个聚焦问题确认实际使用场景。"
                      : "Ask one focused question about the actual use case.",
                  purpose:
                    locale === "zh-CN"
                      ? "补齐判断方案的关键信息。"
                      : "Gather the context needed to judge the proposal.",
                },
              ],
              successSignals: [
                locale === "zh-CN"
                  ? "客户明确说明实际使用场景"
                  : "The customer states the actual use case",
              ],
              whenNotToUse: [
                locale === "zh-CN"
                  ? "客户已经说明场景，只要求确定参数"
                  : "The use case is already clear and the customer only needs a fixed specification",
              ],
            },
            evidence: {
              customerBefore: [
                { messageId: 1424, quote: "The lower track only prevents sway." },
              ],
              sellerMove: [
                { messageId: 1422, quote: "Is this mainly for indoor or outdoor use?" },
              ],
              customerAfter: [
                { messageId: 1421, quote: "Outdoor." },
              ],
            },
            assessment: {
              customerChange:
                locale === "zh-CN"
                  ? "客户补充了室外使用场景。"
                  : "The customer added that the product would be used outdoors.",
              causalReason:
                locale === "zh-CN"
                  ? "新信息直接回应销售的聚焦问题。"
                  : "The new information directly answered the seller's focused question.",
              transferReason:
                locale === "zh-CN"
                  ? "这一问法可以迁移到其他由使用场景造成的方案差异。"
                  : "The question transfers to other proposal differences caused by use context.",
              confidence: "medium",
            },
          },
        ],
      };
      const noSkillResult = {
        protocolVersion: "sales-skill-miner-v2",
        hasLearnableSkill: false,
        assessment: {
          conversationNature: "routine_inquiry",
          reason:
            locale === "zh-CN"
              ? "这段对话只有资料发送和常规跟进，没有证据表明销售动作改变了客户状态。"
              : "The conversation only contains document sharing and routine follow-up, with no evidence that a seller action changed the customer state.",
        },
        candidates: [],
      };
      skillResult.candidates.push({
        ...structuredClone(skillResult.candidates[0]),
        sourceEpisodeId: "episode-2",
        skill: {
          ...structuredClone(skillResult.candidates[0].skill),
          name: secondSkillName,
        },
      });
      Object.assign(caseItem, {
        extractionStatus: "succeeded",
        extractionModel: "mock",
        extractedAt: stamp,
        extractionResult: { ok: true, model: "mock", result: skillResult },
      });
      let minerCalls = 0;
      const compareBodies = [];
      const reviewBodies = [];
      const candidateReviews = {};
      const pageResult = (rows) => ({
        results: rows,
        page: { page: 1, limit: 20, total: rows.length },
      });
      const unexpected = [];

      await context.route("**/api/**", async (route) => {
        const url = new URL(route.request().url());
        const endpoint = url.pathname.split("/").at(-1);
        let data = pageResult([]);
        if (url.pathname === "/api/config") {
          data = {
            language: locale,
            companyName: "Synthetic QA",
            passwordLoginEnabled: true,
          };
        } else if (url.pathname === "/api/auth/profile") {
          data = session;
        } else if (url.pathname === "/api/dashboard/ai-agent/list_all") {
          data = [
            {
              id: 7,
              name: "sales-agent",
              displayName: locale === "zh-CN" ? "外贸销售客服" : "Export Sales Agent",
              aiConfigName: "Synthetic model",
              status: 1,
            },
          ];
        } else if (url.pathname === "/api/dashboard/skill-definition/list_all") {
          data = [
            {
              id: 21,
              name: skillName,
              description:
                locale === "zh-CN"
                  ? "先确认客户的使用场景，再回应方案差异。"
                  : "Clarify the customer's use case before addressing proposal differences.",
              status: 0,
              statusName: "disabled",
            },
          ];
        } else if (url.pathname.includes("/sales-experience/")) {
          switch (endpoint) {
            case "options":
              data = {
                skills: [],
                models: [
                  { id: 1, name: "Synthetic model", modelName: "mock" },
                ],
              };
              break;
            case "sources":
              data = pageResult([sourceItem]);
              break;
            case "import_preview":
              data = [importPreview];
              break;
            case "cases":
              data = pageResult([caseItem]);
              break;
            case "case":
              data = caseItem;
              break;
            case "skill-miner-debug":
              minerCalls += 1;
              data = {
                ok: true,
                model: "mock",
                result: noSkillResult,
              };
              break;
            case "skill-miner-review": {
              const body = route.request().postDataJSON();
              reviewBodies.push(body);
              const candidate = skillResult.candidates.find(
                (item) => item.sourceEpisodeId === body.sourceEpisodeId,
              );
              if (body.action === "confirm" && candidate) candidate.skill = body.skill;
              candidateReviews[body.sourceEpisodeId] = {
                status: body.action === "confirm" ? "confirmed" : "dismissed",
                reviewedAt: stamp,
                ...(body.action === "confirm"
                  ? { skillDefinitionId: 20 + reviewBodies.length }
                  : {}),
              };
              data = {
                ok: true,
                model: "mock",
                result: skillResult,
                candidateReviews: structuredClone(candidateReviews),
                ...(Object.keys(candidateReviews).length === skillResult.candidates.length
                  ? { reviewStatus: "confirmed", reviewedAt: stamp }
                  : {}),
              };
              break;
            }
            case "skill-compare": {
              const body = route.request().postDataJSON();
              compareBodies.push(body);
              data = {
                aiAgentId: 7,
                aiAgentName:
                  locale === "zh-CN" ? "外贸销售客服" : "Export Sales Agent",
                skillDefinitionId: 21,
                skillName,
                withoutSkill: {
                  content:
                    locale === "zh-CN"
                      ? "我们的产品可以用于室外。"
                      : "Our product can be used outdoors.",
                  modelName: "mock-model",
                  knowledgeStatus: "not_configured",
                  mountedSkills: [],
                  durationMs: 418,
                },
                withSkill: {
                  content:
                    locale === "zh-CN"
                      ? "可以。您的项目需要应对哪些天气和安装环境？确认后我再给您对应的方案。"
                      : "It can. Which weather and installation conditions must the project handle? I can then recommend the matching configuration.",
                  modelName: "mock-model",
                  knowledgeStatus: "not_configured",
                  mountedSkills: [{ id: 21, name: skillName }],
                  durationMs: 463,
                },
              };
              break;
            }
            default:
              if (route.request().method() === "POST") unexpected.push(endpoint);
          }
        }
        await route.fulfill({
          json: { success: true, errorCode: 0, message: "", data },
        });
      });

      const page = await context.newPage();
      const errors = [];
      page.on("pageerror", (error) => errors.push(error.message));
      await page.goto(`${base}/dashboard/sales-experience`);
      await page.getByRole("checkbox").nth(1).click();
      await page.getByRole("button", { name: l.import, exact: true }).click();
      await page
        .getByRole("heading", { name: l.importDialog.title, exact: true })
        .waitFor();
      await page
        .getByRole("button", { name: l.importDialog.selected, exact: true })
        .click();
      const importDialog = page.getByRole("dialog");
      assert.equal(await importDialog.getByRole("checkbox").count(), 3);
      await importDialog.getByRole("checkbox").nth(1).click();
      await importDialog
        .getByText(
          l.importDialog.summary
            .replace("{conversations}", "1")
            .replace("{messages}", "1"),
          { exact: true },
        )
        .waitFor();
      await importDialog
        .getByRole("button", { name: labels.common.cancel, exact: true })
        .click();
      await page.getByRole("tab", { name: l.cases, exact: true }).click();
      assert.equal(
        await page.getByRole("button", { name: l.miner.run, exact: true }).count(),
        0,
        "case list still exposes extraction action",
      );
      await page.getByRole("tab", { name: l.skillMiner, exact: true }).click();
      await page.getByRole("heading", { name: skillName, exact: true }).waitFor();
      await page.screenshot({
        path: path.join(shots, `${locale}-${width}-pending.png`),
        fullPage: true,
      });
      assert.equal(
        await page.getByRole("button", { name: l.miner.directConfirm, exact: true }).count(),
        2,
        "each pending Skill should have its own direct confirm action",
      );
      await page
        .getByRole("button", { name: l.miner.directConfirm, exact: true })
        .first()
        .click();
      await page
        .getByRole("dialog")
        .getByRole("button", { name: l.miner.directConfirm, exact: true })
        .click();
      await page.getByRole("dialog").waitFor({ state: "hidden" });
      await page.getByText(l.miner.confirmedTitle, { exact: true }).waitFor();
      assert.equal(
        await page.getByRole("button", { name: l.miner.directConfirm, exact: true }).count(),
        1,
        "confirming one Skill should leave the other pending",
      );
      await page
        .getByRole("button", { name: l.miner.editSkill, exact: true })
        .click();
      await page.getByLabel(l.miner.skillName, { exact: true }).waitFor();
      await page
        .getByRole("button", { name: l.miner.cancelEdit, exact: true })
        .click();
      await page
        .getByRole("button", { name: l.miner.editSkill, exact: true })
        .click();
      await page
        .getByRole("button", { name: l.miner.saveConfirm, exact: true })
        .click();
      await page
        .getByRole("dialog")
        .getByRole("button", { name: l.miner.saveConfirm, exact: true })
        .click();
      await page.getByRole("dialog").waitFor({ state: "hidden" });
      assert.equal(
        await page.getByText(l.miner.confirmedTitle, { exact: true }).count(),
        2,
        "both Skills should show their own confirmed state",
      );
      await page.getByText(overallReason, { exact: true }).waitFor();
      assert.equal(await page.locator("pre").count(), 0, "raw JSON is visible");
      assert.equal(
        await page.getByText("protocolVersion", { exact: false }).count(),
        0,
        "protocol field is visible",
      );
      assert.equal(
        await page.evaluate(() => document.documentElement.scrollWidth > innerWidth),
        false,
        "page overflow",
      );
      await page.screenshot({
        path: path.join(shots, `${locale}-${width}-skill.png`),
        fullPage: true,
      });

      await page
        .getByRole("button", { name: l.miner.rerun, exact: true })
        .click();
      await page
        .getByRole("dialog")
        .getByRole("button", { name: l.miner.rerun, exact: true })
        .click();
      await page
        .getByText(l.miner.noSkillTitle, { exact: true })
        .waitFor();
      assert.equal(reviewBodies.length, 2);
      assert.deepEqual(
        reviewBodies.map((body) => body.sourceEpisodeId),
        ["episode-1", "episode-2"],
      );
      assert.equal(reviewBodies.every((body) => !Object.hasOwn(body, "candidates")), true);
      assert.equal(minerCalls, 1);
      await page.getByRole("tab", { name: l.skillCompare, exact: true }).click();
      await page
        .getByRole("heading", { name: l.compare.title, exact: true })
        .waitFor();
      await page
        .getByLabel(l.compare.customerMessage, { exact: true })
        .fill("Could this work outdoors?");
      await page.getByRole("button", { name: l.compare.run, exact: true }).click();
      await page.getByText(l.compare.withSkill, { exact: true }).waitFor();
      await page.getByText(l.compare.withoutSkill, { exact: true }).waitFor();
      assert.equal(compareBodies.length, 1);
      assert.deepEqual(compareBodies[0], {
        aiAgentId: 7,
        skillDefinitionId: 21,
        messages: [{ role: "user", content: "Could this work outdoors?" }],
      });
      assert.equal(
        await page.evaluate(() => document.documentElement.scrollWidth > innerWidth),
        false,
        "comparison page overflow",
      );
      await page.screenshot({
        path: path.join(shots, `${locale}-${width}-compare.png`),
        fullPage: true,
      });
      assert.deepEqual(errors, []);
      assert.deepEqual(unexpected, []);
      console.log(`PASS ${locale} ${width}: focused Skill extraction console`);
      await context.close();
    }
  }
} finally {
  await browser.close();
  await new Promise((resolve) => server.close(resolve));
}
