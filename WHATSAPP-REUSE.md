# WhatsApp 渠道模块复用指南

更新日期：2026-09-14。本文对应本地 AgentDesk 的当前实现，不等同于上游 AgentDesk 的功能说明。

目标：让另一个项目复用扫码连接、消息收发、附件处理和历史同步，不必搬走整个客服平台。本文只记录现状和迁移方案，没有创建独立 SDK，也没有改变现有服务。

## 1. 先明确接入方式和边界

当前使用 Go 开源库 [whatsmeow](https://github.com/tulir/whatsmeow)，以 WhatsApp「关联设备」方式扫码登录，不是 Meta Cloud API，也不是浏览器自动点击方案。

手机进入 WhatsApp 的关联设备页面，扫描服务端生成的二维码。服务端保存自己的设备会话，之后通过协议连接收发消息。它不是读取手机文件系统，也不能读取 iCloud/Google Drive 聊天备份。

关键限制：

- 当前仅接入个人聊天，过滤群聊和广播。
- 能展示通话记录，不支持在平台内拨打、接听语音或视频电话。
- 能展示多种结构化消息，不代表可以操作投票、支付、表单或活动报名。
- 历史消息是尽力同步，不能保证取回手机已经删除、未恢复或服务端已不可用的内容。
- 非官方关联设备协议存在上游变更、掉线及账号限制风险。生产使用需要评估平台条款、客户授权和数据合规，不能承诺永不封号。
- 当前不是经过多租户隔离及多实例抢占验证的通用连接平台。

## 2. 推荐复用边界

```text
WhatsApp
   |
   v
internal/whatsapp                 协议、会话、解析、下载、历史请求
   |
   v
internal/pkg/linkedchat           与业务无关的消息契约
   |
   v
目标项目适配层                    客户身份、会话、幂等、附件、发送队列
   |
   +--> 目标项目数据库和对象存储
   +--> WebSocket / 消息列表
   +--> 人工客服 / AI 路由
```

优先复制协议层和消息契约，重新适配业务层。不要为了 WhatsApp 一起引入 AgentDesk 的知识库、工作流、销售自动化和完整客服数据模型。

Go 的 `internal` 有导入边界：另一独立 Go 模块不能随意直接导入本仓库的 `internal/whatsapp`。需要复制到目标模块内部并修改导入前缀，或者另行提取成公开 Go 模块。当前尚未完成独立发布封装。

## 3. 文件清单

以下路径均相对本仓库根目录。

### 可以优先整体迁移

| 文件 | 职责 |
| --- | --- |
| `internal/whatsapp/client.go` | 创建会话、扫码、连接状态、实时和历史事件入口、文本发送 |
| `internal/whatsapp/content.go` | 各类 WhatsApp 消息转统一结构 |
| `internal/whatsapp/decrypt.go` | 使用协议库解密投票、反应、评论和编辑等内容 |
| `internal/whatsapp/media.go` | 附件下载、队列、临时文件和状态回调 |
| `internal/whatsapp/outbound_media.go` | 出站附件上传与发送 |
| `internal/whatsapp/history.go` | 历史请求、状态、自动调度 |
| `internal/whatsapp/history_recovery.go` | 根据本地协议引用尝试找回缺失消息 |
| `internal/whatsapp/*_test.go` | 协议解析、附件和历史相关测试，迁移时一起保留 |
| `internal/pkg/linkedchat/types.go` | Incoming、Message、Status、OutboundMedia 契约 |

协议目录对本项目的本地依赖集中在 `internal/pkg/linkedchat`，适合作为第一阶段迁移边界。

### 需要按新项目重写或适配

| 文件 | 需要保留的逻辑 |
| --- | --- |
| `internal/services/whatsapp_service.go` | 会话管理、入站幂等、身份映射、发送队列、AI 和人工分流 |
| `internal/services/linked_message_media.go` | 附件落地和原消息更新 |
| `internal/services/whatsapp_history.go` | 业务会话到历史请求的转换 |
| `internal/repositories/whatsapp_history.go` | 查找待补历史的会话和缺失消息 |
| `internal/repositories/whatsapp_outbox.go` | 出站任务领取、恢复及重试 |
| `internal/handlers/dashboard/whatsapp_handler.go` | 权限、状态查询、连接操作 |
| `internal/models/models.go` | 查阅渠道、消息、客户、外部身份及 outbox 数据模型 |
| `internal/bootstrap/server.go` | 服务启动与路由挂载 |
| `internal/bootstrap/routes.go` | HTTP 路由注册 |
| `internal/services/cronx/cron.go` | 定时出站投递 |

注意：`whatsapp_service.go` 中的服务已经抽为与 Messenger 共用的 `linkedChatService`，直接复制该文件会引入 Messenger 和大量 AgentDesk 业务依赖。迁移时应提取 WhatsApp 所需行为，不要把这个文件误认为独立连接器。

### 可选前端

- `web/app/(dashboard)/dashboard/channels/whatsapp/page.tsx`：渠道入口。
- `web/app/(dashboard)/dashboard/channels/_components/whatsapp.tsx`：扫码和连接管理弹窗。
- `web/lib/api/admin.ts`：`fetchWhatsAppConnection`、`updateWhatsAppConnection`。
- `web/components/chat/linked-message-content.tsx`：结构化消息展示。
- `web/lib/linked-message.ts`、`web/lib/im-message.ts`：消息解析与附件辅助逻辑。
- `web/lib/linked-message.test.mjs`、`web/lib/linked-message-render.test.mjs`、`web/lib/whatsapp-history.test.mjs`：前端契约回归测试。
- `web/messages/zh-CN.json`、`web/messages/en-US.json`：相关文案。

这些组件依赖 React、现有 shadcn/Base UI、Lucide、项目弹窗、API 客户端、i18n 和 `@/` 路径别名。不能只复制一个 TSX 文件就认为迁移完成；使用其他 UI 框架时，保留交互契约并重新实现界面即可。

## 4. 依赖与许可

当前 `go.mod` 使用 Go 1.26.0，关键依赖版本如下：

| 依赖 | 当前版本 |
| --- | --- |
| `go.mau.fi/whatsmeow` | `v0.0.0-20260908082135-57796d3d6b41` |
| `github.com/glebarez/go-sqlite` | `v1.21.2` |
| `github.com/skip2/go-qrcode` | `v0.0.0-20200617195104-da1b6568686e` |
| `google.golang.org/protobuf` | `v1.36.12` |

初次迁移应固定这些版本并保留所需的 `go.sum` 校验信息，再执行目标项目的依赖整理和测试。不要同时升级协议库、改变消息结构和替换存储，否则问题很难定位。

根 `go.mod` 另有 `github.com/imroc/req/v3` 到 Beeper fork 的 replace，注释说明它服务于 mautrix-meta 的传输依赖。不要把整仓 replace 自动当作 WhatsApp 的必需依赖；按提取后的依赖图判断。

本仓库 `LICENSE` 为 Apache-2.0；whatsmeow 使用 MPL-2.0。复用、修改和分发时分别检查对应许可、保留必要声明；协议库许可不等于 WhatsApp 平台授权。

## 5. Go 接入生命周期

现有协议入口：

```go
Open(dir string, channelID int64, onMessage func(Incoming) error) (*Session, error)
```

`channelID` 必须大于 0。`dir` 使用稳定的绝对路径，不使用临时目录。会话数据库命名为 `channel-<channelID>.db`。

最小连接示例，适用于复制进目标 Go 项目后。将导入前缀替换为目标 `go.mod` 的 module 值；`persist` 由目标项目实现，并非已有的通用持久化函数。

```go
package connector

import (
    "context"

    "your-module/internal/whatsapp"
)

func Run(ctx context.Context, dir string, channelID int64,
    persist func(whatsapp.Incoming) error) error {
    session, err := whatsapp.Open(dir, channelID, persist)
    if err != nil {
        return err
    }
    defer session.Close()

    if err := session.Connect(); err != nil {
        return err
    }
    // 宿主通过 session.Status() 向已鉴权的管理界面提供二维码和状态。
    <-ctx.Done()
    return nil
}
```

此示例只展示生命周期，不包含 HTTP 服务、持久化发送队列或完整错误恢复。`Connect()` 成功返回不表示已经扫码上线，必须继续观察 `Status()`。

| 方法 | 语义 |
| --- | --- |
| `Status()` | 返回连接状态及有效二维码 |
| `Connect()` | 使用已存会话连接，或启动扫码流程 |
| `Disconnect()` | 停止当前连接，保留设备凭据 |
| `Logout(ctx)` | 尝试注销关联设备，失败时应向用户报告，不应伪装成解绑成功 |
| `Close()` | 停止连接和媒体工作线程，关闭本地数据库 |
| `Send(ctx, account, chat, id, text)` | 向指定账号下的个人聊天发送文本 |
| `SendMedia(ctx, account, chat, id, media)` | 发送 OutboundMedia 附件 |

连接状态包括 `disconnected`、`connecting`、`qr`、`connected`、`logged_out`、`error`；业务适配层还可能返回 `disabled`。状态字段包括 `account`、`qr`、`qrExpiresAt` 和 `error`。

二维码是短期凭证，过期应从界面移除；当前前端每 2 秒查询一次状态。不要缓存到 CDN 或日志。现有连接握手超时约 30 秒，初始自动重连被关闭，不能只调用一次 Connect 就承诺任意故障都能自动恢复。宿主启动会恢复启用且配置了 `autoConnect` 的渠道。

## 6. 收消息契约：迁移时最容易出错的部分

`Incoming` 的关键字段：

| 字段 | 含义 |
| --- | --- |
| `ID` | WhatsApp 消息 ID，不是跨渠道全局业务主键 |
| `Account`、`Chat`、`ChatAliases` | 当前登录账号、聊天标识及 PN/LID 别名 |
| `Name`、`Text` | 展示名和文本；Text 为空仍可能是有效消息 |
| `SentAt` | 原消息时间，不应以导入时间覆盖 |
| `History`、`FromMe` | 历史消息、本人发送标识 |
| `EchoID` | 出站消息回流关联信息 |
| `Message` | 统一结构化消息 |
| `MediaPath` | 回调期间有效的解密附件临时文件，不可作为持久化 URL |

`Message` 的 JSON 字段包括 `kind`、`state`、`title`、`footer`、`url`、`filename`、`mimeType`、`size`、`seconds`、`mediaKind`、`targetId`、`targetPreview`、`rawType`、`options`、`fields`、`startAt`、`endAt`、`selectable`、`forwarded`、`update`、`revisionMs`。完整类型以 `internal/pkg/linkedchat/types.go` 为准。`startAt/endAt` 为秒，`revisionMs` 为毫秒。

必须保留的处理规则：

1. 回调支持并发；实时事件、历史事件和媒体线程可能同时调用。目标项目需用事务、唯一约束和必要的同步控制保障一致性。
2. 按渠道、登录账号、聊天、协议消息 ID 共同幂等。本项目 clientMsgID 为 `wa_` 加 `SHA256(channelID|account|chat|incomingID)` 的十六进制值。
3. PN 和 LID 可能指向同一客户，需要别名归一化；不要只依赖手机号码或显示名关联客户。
4. 附件通常先回调 `pending`，再以相同 ID 回调完成或 `unavailable`。应更新同一条记录，不能重复插入。
5. `MediaPath` 在回调返回后会删除。必须在返回前复制或上传到目标存储；不能只把路径放进异步任务。
6. 编辑和撤回按受渠道/账号/聊天隔离的目标 ID 更新原记录，校验方向和版本；旧编辑不得覆盖新编辑，已撤回内容不得被恢复。
7. 历史消息不应触发 AI、欢迎语、自动发送或未读数增长，避免导入后批量回复旧客户。
8. reaction、poll_vote 等 passive 消息不应被当作新的客服问题，不能触发 AI 或人工接管。
9. 当前正常实时文本/引用文本可进入 AI 链路；其他实时非文本交给人工待处理，不代表已有多模态 AI。
10. 手机本人发出的实时普通消息会映射为客服消息并触发人工接管；平台发送回流则通过 outbox 关联去重。

不认识的消息仅保留类型字段或未知数字标签诊断，不保存/显示未经处理的加密载荷。未知消息也要有可见状态，不要静默丢弃。

## 7. 附件和消息类型支持范围

| 类别 | 当前能力 |
| --- | --- |
| 文本、引用、转发 | 展示文本及相关元信息 |
| 图片、贴纸、音频、视频、圆视频、文件 | 解密下载、存储、展示；受媒体可用性和大小限制 |
| 联系人、位置、实时位置 | 联系人信息及位置快照，不是持续地图追踪 |
| 投票及更新 | 展示选项；可解密且找到原投票时解析票选标签，不支持平台内投票 |
| 按钮、列表、模板、native flow、轮播 | 结构化或部分文本展示，不保证原生完整交互/全部轮播媒体 |
| 商品、订单 | 商品图、价格和订单基础字段，不执行交易 |
| 活动、相册、贴纸包 | 基础信息或计数展示；部分加密活动回应缺少公共解码能力 |
| 反应、评论、编辑、撤回 | 可解密时处理，依赖原消息及本地消息密钥 |
| 通话、预约通话、支付 | 部分记录展示；不接打电话、不执行支付 |
| 一次性查看及受保护内容 | 保留保护状态，不绕过查看限制 |

当前媒体下载上限为 64 MiB，2 个工作线程、128 个排队任务，单次下载超时 45 秒。队列满、过大、过期或下载失败会标记不可用。下载使用协议库 `DownloadToFile`，不要自己重写加密算法。

业务层将临时文件交给 `AssetService.Upload`，检查 MIME 并产生自己的附件地址。迁移时不能将上游临时媒体 URL 或媒体密钥直接暴露给前端。带图商品的 `kind` 仍为 `product`，图片类型在 `mediaKind`，渲染不能只看 kind。

服务重启后，遗留 pending 媒体目前被标记不可用，并没有可持久重试的完整下载任务。需要可靠重试时应另外设计，不能宣称当前已具备。

## 8. 历史同步：可用，但不是完整备份恢复

接口位于 `internal/whatsapp/history.go`：

```go
SetHistoryTargets(load func(string) ([]HistoryRequest, error), missing func(Incoming) bool)
RequestHistory(ctx context.Context, req HistoryRequest) error
HistoryStatus() HistoryStatus
```

`HistoryRequest` 包含 `ConversationID`、`Account`、`Chat`、`MessageID`、`FromMe`、`Before`。不接入业务目标加载函数，就不会自动知道哪些会话需要回补。

当前逻辑：连接后等待约 5 秒，按聊天顺序请求；自动重跑冷却 5 分钟，成功目标之间间隔约 10 秒。请求最近 50 条，首次可用空消息 ID 和时间游标，不伪造消息 ID。手机版本和可用历史决定实际返回内容；约 2 分钟没有进度会超时。

补救逻辑读取协议库本地消息引用，每个身份最多检查 500 个引用，每个目标最多尝试 50 个重发请求。它依赖仍可用的协议引用和消息密钥，不能恢复所有旧消息，也不是读取云备份。

### 已知问题，迁移时优先处理

当前自动同步循环遇到首个失败、断线或超时目标会返回，后续聊天可能没有被尝试。建议目标项目改为每聊天独立状态和重试，单个失败不阻断其他聊天，并展示已处理数、失败数及游标。这个改进尚未在本文任务中实现。

旧版本如果只存了 `kind=unsupported`，没有原始类型和有效内容，升级解析器不会凭空恢复内容。新适配层支持在原消息重放时原位升级，但仍需手机历史重放或对方重新发送。

## 9. 发送可靠性和队列

协议层 `Send` 不提供持久发送队列。生产业务必须保留适配层 outbox 行为：

- 业务消息和 outbox 在同一事务写入，避免界面显示已发但任务丢失。
- 协议 ID 稳定：当前取 `SHA256(channelPublicID + ":" + messageID)` 的前 16 字节，转大写十六进制；重试复用 ID。
- 调度器每 5 秒投递，每批 20 条，领取时控制并发。
- 离线等待约 10 秒再试，不消耗发送次数；文本发送超时 25 秒，附件 90 秒。
- 失败按 `(retryCount + 1) * 15s` 退避，当前最多尝试 5 次；终态需要可见，不能无限重发。
- 启动恢复被中断的领取状态；禁用渠道、账号切换、消息撤回或 AI 已不再接待时取消不合适的投递。

稳定 ID 和幂等不能承诺远端严格 exactly-once。当前发送成功表示提交被接受，不应直接在 UI 标成对方已读或已送达。

## 10. 当前管理 HTTP API

这些是 AgentDesk 已有路由，不是一个独立 WhatsApp REST SDK：

| 方法 | 路径 | 用途 |
| --- | --- | --- |
| GET | `/api/dashboard/channel/:id/whatsapp` | 读取状态、二维码 |
| POST | `/api/dashboard/channel/:id/whatsapp/connect` | 启动连接 |
| POST | `/api/dashboard/channel/:id/whatsapp/disconnect` | 断开并关闭业务层 autoConnect |
| POST | `/api/dashboard/channel/:id/whatsapp/logout` | 解绑设备 |

`:id` 是内部渠道数字 ID。接口需要管理端登录和 `PermissionChannelUpdate`，包括查看二维码。响应通过 `httpx.WriteJSON` 包装为项目 `JsonResult`，不是裸 Status；前端复用 `web/lib/api/client.ts` 解包。接口设置 `Cache-Control: no-store`。

文本和附件发送走现有客服消息业务链路，不由上面四个接口直接发送。历史请求是 Go Session 能力，不能由此推断已经有独立的 HTTP 历史同步端点。

## 11. 迁移顺序

1. 复制协议目录、linkedchat 契约和测试到目标项目，替换 Go module 导入前缀，固定依赖版本。
2. 为新项目分配独立设备数据目录和测试渠道，重新扫码；不要让两个运行中的项目共用同一设备数据库。
3. 实现已鉴权的连接状态和操作接口，先验收扫码、断开、重启恢复和解绑。
4. 实现客户身份、会话及消息持久化，给 scoped 消息键加唯一约束，验证重复事件和账号切换隔离。
5. 实现附件存储及同 ID 状态更新，验证回调返回后临时文件删除也不影响页面访问。
6. 实现事务 outbox 和重试，再接入工作台发送，最后连接 AI；历史和 passive 事件始终绕过 AI。
7. 接入历史目标查询和同步状态，处理上文的单聊天失败阻塞问题，再逐步开放历史导入。
8. 按目标技术栈复用或重写扫码弹窗和消息渲染，完成真实手机验收。

如果目标项目不是 Go，推荐把前两层放入一个独立 Go 连接服务，由业务项目通过经过鉴权的内部接口接收规范化事件和下发命令。这是建议架构，当前仓库没有现成服务包。需要另外定义事件版本、幂等键、持久投递/确认机制、重试、附件访问权限和渠道鉴权，不能只用一次 HTTP 回调替代可靠消息队列。

## 12. 数据安全和部署

设备 SQLite 保存登录凭据及消息密钥，业务数据库保存客户和消息，附件由资产存储持久化，三者用途不同。只迁移源码不会带走历史业务记录，也不应默认带走登录凭据。

- 设备目录当前权限为 0700，数据库文件为 0600；部署仍需限制运行用户和备份访问。
- 不提交会话 DB、二维码、密钥或客户聊天内容，不把它们放进示例配置或诊断日志。
- 单个设备会话只交给一个活跃进程管理，多实例需要另行实现会话归属和故障接管。
- 备份应一致、加密且可验证恢复，不要直接复制一个正在写入的 SQLite 主文件就认定备份完整。
- 不将个人消息附件公开给任意租户；下载和消息查询都要做权限隔离。
- 日志优先记录状态、耗时、计数和脱敏关联标识，不记录正文、解密字节及访问凭据。

## 13. 验证清单

协议目录迁移后的基础命令，在目标 Go 项目根运行：

```sh
go test ./internal/whatsapp ./internal/pkg/linkedchat
go test -race ./internal/whatsapp
```

完整适配还需要业务层测试：重复消息、媒体二次回调、编辑版本、撤回、账号切换、历史不触发 AI、出站回流去重、事务失败和重启恢复。可以参考本仓库 `internal/services/linked_message_media_test.go` 及相关 WhatsApp 测试。

真实手机验收至少覆盖：

- 首次扫码、二维码过期、断线重连、服务重启、手机解绑。
- 双向文本、图片、语音、视频、文件，附件失败及大文件。
- 引用、编辑、撤回、反应、投票、商品和不支持类型的可见提示。
- 手机发送和平台发送的回流不重复，历史导入不自动回复。
- 原聊天有历史、手机没有历史、一个聊天超时但其他聊天仍需同步。
- 无权限用户不能查看二维码、操作其他渠道或下载其他租户附件。

本地已有自动化解析测试，不等于所有消息类型均经过真实手机端到端验证。迁移项目必须单独验收，尤其是历史补回、加密内容及平台协议变更。

## 14. 给接手开发者的简短任务说明

> 以本指南为迁移基线，先复制 internal/whatsapp 和 internal/pkg/linkedchat，保持当前协议库版本。将 AgentDesk 业务适配替换为目标项目的客户、会话、消息、附件和 outbox 实现。保留 scoped 幂等、媒体二次更新、历史不触发 AI 和出站回流去重。不要引入整个 AgentDesk，不要复制现有登录凭据，不要把通话记录展示说成支持打电话。先用独立测试账号完成扫码与双向消息验收，再接 AI 和历史回补。
