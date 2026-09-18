---
name: sales-skill-miner
description: 从一段完整的 B2B 客户对话中判断是否存在真正值得学习的销售技巧，并把有充分对话证据的技巧写成可供 AI 客服复用的候选 Skill。适用于销售聊天复盘和经验沉淀，不用于总结对话、提取业务事实或生成 CRM 标签。
---

# 销售技巧挖掘器

从真实对话中发现少量、可复用、能改变销售行为的技巧。多数对话没有值得沉淀的技巧；返回空结果是正常且优先的结论。

## 判定原则

只有下面这条证据链完整成立，才生成候选 Skill：

```text
客户面临明确的购买阻力或决策机会
→ 销售采取了并非例行服务的沟通动作
→ 客户随后出现有价值的认知、意愿或行动变化
→ 该变化与销售动作存在清楚、合理的联系
→ 该变化不是客户在销售动作前已经提出或承诺的下一步
→ 去掉产品和客户细节后，同一方法仍能指导其他销售
```

事实回答、报价、发送资料、执行客户已经提出的要求、收集标准字段、礼貌跟进和交易手续不是销售技巧。成交结果本身也不能证明某句话有效。

## 执行

1. 读取调用方提供的对话，保持消息顺序和角色不变。
2. 使用 [references/evidence-audit-prompt.md](references/evidence-audit-prompt.md) 只提取动作前、销售动作、动作后的原始证据，不撰写 Skill。
3. 校验证据引用、消息角色和先后顺序；证据无效时终止该候选。
4. 使用 [references/extraction-prompt.md](references/extraction-prompt.md) 对证据候选做反方审查，只为全部门槛都成立的候选撰写 Skill。
5. 严格按 [references/output-schema.md](references/output-schema.md) 返回 JSON，最多保留两个候选，不为填满结果降低标准。

输出中的 `skill` 是可复用技巧正文，不得出现原案例的公司、客户、产品、价格、证书、地区或交易细节。`evidence` 和 `assessment` 只用于人工审核，不属于最终挂载给客服的 Skill 正文。

需要检查正反例时再读取 [references/examples.md](references/examples.md)。
