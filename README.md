# AffectWeave for FreeAgent

可选的人物、关系与经历连续性扩展。当前为 **1.6.0-dev.1 Go 开发候选**，配套 FreeAgent **v8.7.0-dev.1**。产品运行时只需扩展程序和 Core 管理的受限 PostgreSQL；不需要 Python、Node 或 Go。

## 本版能力

- 每个 Core/空间/人物/参与者分别保存已确认状态。人物身份、边界、偏好、当前情绪、关系、经历和未决事项全部由扩展负责。
- 可选自动输入分析：默认关闭；每个已采用输入最多一次，默认 8 秒。临时判断只影响当前冻结片段；持久变化是待审阅草稿，必须预览和确认。超时、失败或 UNKNOWN 使用已确认片段继续普通回答，不后台补跑。
- 原回答的真实 sent 片段与后续明确 reply-to 关联，形成 L2 反馈草稿。重复反馈不再计入。用户批准保存不是业务成功证据，首版不接受自动 L3 升级。
- 人格数值驱动情绪敏感度、恢复速度和表达建议；中性值均为 0.5。可选的原因/对象内情绪交互默认关闭，确定性限幅，不靠读取放大情绪。
- 人格演进默认关闭：至少两个独立已确认反馈结果；单次参数变化最多 0.05、相对基线累计最多 0.20。每次确认产生新版本，可冻结或复制旧参数生成回退版本。身份与边界不自动改写。
- 回复方式：完整一条、自然分段、按人物表达；最多三段，无损保留正文。开启分析和表达后可选择已经授权的贴纸、反应和确认事项的固定跟进；平台协议、资源权限与发送在 Core/渠道，不在人物库。
- 可选主动对话：**安装后仍关闭**。人物页操作的是 Core 唯一的主动对话总开关，还需目标、来源和规则授权。一次模型同时决定说/不说和最终文字；用户新输入优先，UNKNOWN 不重发，未回复不追问。

详细测试及限制见 [验收记录](docs/VALIDATION_1_6_2026-11-08.md)。真人 TG 主动开场、真实设备观察和真实模型表现必须分别验收，不能以模拟测试冒充。

## 安装和使用

从 GitHub Release 下载对应系统包，解压后导入其中的 `cc.endview.affectweave` 目录。扩展默认停用，先在能力管理中初始化独立 PG，再启用并创建/选择人物。普通 Core 不含本扩展的算法、人物设置或常驻进程。

已有安装更新前核对待确认/UNKNOWN，创建 Core 与扩展的配套备份，停止 Owner 后替换程序。执行：

```text
freeagent ext storage-preflight --data-dir DATA --id cc.endview.affectweave
```

更新使用现有 schema aw、存储格式 1 和状态契约 v2，给原 inputs 添加完整冻结片段。无法还原完整片段的历史输入不会重新分析；新输入正常工作。不恢复 Python/SQLite 或旧宿主兼容层。

设置 → 能力管理 → AffectWeave：创建人物、配置空间授权、查看已确认状态/本轮分析/待审阅变化、预览确认、选择独立反馈证据及人格版本。默认人物不覆盖已有表达设置。本轮人物和表达方式在对话输入区的能力选择中设置。

开启主动对话还需已配对的 Telegram 单人私聊、该会话已选择人物、明确来源（如 persona:interests、thread:事项标识、memory:经历标识）及投递规则。先用“只生成不发送”验证，预览消耗显示的模型额度。默认每目标滚动 24 小时评估两次、主动开场一次；正常 Core 回答无这类人物主动额度限制。

## CLI / AI / 备份

```text
freeagent ext state --id cc.endview.affectweave --role maya --space local --actor owner --data-dir DATA
freeagent control catalog
freeagent delivery wake-list --session CHAT --data-dir DATA
freeagent delivery preview --session CHAT --rule RULE --key UNIQUE --confirm --data-dir DATA
freeagent ext backup --id cc.endview.affectweave --to NEW_BACKUP --data-dir DATA
freeagent ext verify-backup --from BACKUP
freeagent ext restore --from BACKUP --data-dir NEW_DATA --package SAME_VERSION_EXTENSION
```

所有持久人物/设置修改使用 Core 的具体控制提案确认，Full 不跳过。原操作回执丢失时用 `control reconcile-extension --operation ID` 只核对原结果，不再提交。恢复目标为新目录；主动生成默认关闭，旧机会不重放。备份包含私人资料，应按用户数据保管，凭据与工作区另行备份。

## 开发

[开发指南](docs/DEVELOPMENT.md) · [架构](docs/ARCHITECTURE.md) · [FreeAgent 接口](docs/FREEAGENT.md) · [规则对照](docs/REFERENCE_MAPPING.md)。纯计算和 PG/独立进程测试分开运行；产品包不含真人资料、Key 或聊天。MIT，依赖许可证随包保留。
