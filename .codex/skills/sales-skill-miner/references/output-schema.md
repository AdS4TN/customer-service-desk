# 输出协议

只返回一个 JSON 对象。

```json
{
  "protocolVersion": "sales-skill-miner-v2",
  "hasLearnableSkill": true,
  "assessment": {
    "conversationNature": "sales_episode",
    "reason": "这段对话为什么具备足够的学习价值"
  },
  "candidates": [
    {
      "sourceEpisodeId": "episode-1",
      "skill": {
        "name": "先校准比较口径，再处理价格异议",
        "description": "客户认为价格偏高但比较标准尚不清楚时，先找出真正影响选择的比较项，再回应异议。",
        "whenToUse": [
          "客户表示价格高于其他方案，但没有说明比较范围或最在意的条件"
        ],
        "objective": "把笼统的价格异议转成可以回应的具体决策标准。",
        "steps": [
          {
            "instruction": "先承认客户确实看到了价格差异，不立即辩解或降价。",
            "purpose": "让客户愿意继续说明比较依据。"
          },
          {
            "instruction": "用一个聚焦问题确认客户比较的是哪些条件，以及哪个条件最影响决定。",
            "purpose": "找到真正需要处理的异议。"
          },
          {
            "instruction": "只回应已确认的关键差异，并提出一个便于客户继续评估的下一步。",
            "purpose": "推动客户从泛泛拒绝转向具体判断或行动。"
          }
        ],
        "successSignals": [
          "客户说出更具体的比较标准或真实顾虑",
          "客户接受一个明确的下一步"
        ],
        "whenNotToUse": [
          "客户已经明确说明唯一限制是不可调整的预算上限"
        ]
      },
      "evidence": {
        "customerBefore": [
          {"messageId": 101, "quote": "exact source quote"}
        ],
        "sellerMove": [
          {"messageId": 102, "quote": "exact source quote"}
        ],
        "customerAfter": [
          {"messageId": 103, "quote": "exact source quote"}
        ]
      },
      "assessment": {
        "customerChange": "客户发生了什么有价值的变化",
        "causalReason": "为什么销售动作很可能对该变化作出了贡献",
        "transferReason": "为什么去掉案例细节后仍可用于其他相似销售情境",
        "confidence": "medium"
      }
    }
  ]
}
```

## 字段约束

- `protocolVersion` 固定为 `sales-skill-miner-v2`。
- `hasLearnableSkill` 必须与 `candidates` 是否非空一致。
- `conversationNature` 只能是 `routine_inquiry`、`sales_episode` 或 `mixed`。
- `candidates` 最多两个；每项只表达一种销售技巧。
- `sourceEpisodeId` 必须引用证据审查阶段实际返回的候选片段；不得在第二阶段发现新片段。
- `skill` 是未来可挂载的正文，禁止出现原案例专属事实。
- `steps` 使用二到四个按顺序执行的动作。
- `successSignals` 描述客户可观察的回应，不写“提高成交率”等无法从对话确认的结果。
- `whenNotToUse` 只写防止错误套用所需的条件，不扩写成合规手册。
- `evidence` 必须保留案例原文；三个证据组都不能为空。
- `confidence` 只能是 `medium` 或 `high`。不足以达到 `medium` 时，丢弃候选。
