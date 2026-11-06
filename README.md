# AffectWeave for FreeAgent

可选的人物、关系与经历连续性扩展。当前为 **1.0.0-dev.1 Go 开发候选**，面向 FreeAgent v8.3；实际验收与未运行项见 FreeAgent 的版本报告。生产只运行 Go 程序，不安装 Python、Node 或 Go。

- Core 负责回答、工具、权限、上下文总预算和确认；AffectWeave 只提供有界人物数据。
- 人物基线、心境、共存的原因情绪、关系与未决事件保存在独立 PostgreSQL 数据库和受限角色中。Core 的数据库和模型凭据不交给扩展。
- 同一已采用输入只推进一次；工具后调用、模型替代与重连读取同一冻结版本。按 Core、空间、人物和参与者隔离，同空间跨会话连续。
- 模型提出的长期变化先预览、再确认；未批准不改变关系或经历。发送成功、部分、失败和 UNKNOWN 分开，生成不等于送达或已读。
- 未启用时没有扩展进程或分析请求。显式启用的被动人物片段可用于 None 聊天，仍不授予模型、工具或工作区 Host API。

## 安装

1. 从同次 Release 下载系统对应的 AffectWeave 归档，解压。
2. 在 FreeAgent 的“设置 → 能力 → 扩展”导入其中的 `cc.endview.affectweave` 文件夹。
3. 点击“初始化独立 PostgreSQL 存储”，再开启“全局启用”。这是显式维护操作，只创建扩展数据库，不改 Core 对话库。
4. 在人物管理页填写名字、身份、边界和允许使用的空间，预览后批准。选择刚创建的人物，预览设为默认；也可在输入区的“本轮能力”选择人物。
5. 开始新一轮对话。普通回答仍由 FreeAgent 已配置的模型生成，没有额外分析调用。

首次人物状态在第一条已采用输入到达后生成。关闭扩展保留数据；重新启用继续使用。移除程序不等于删除独立数据库。重置人物、删除本空间状态、删除经历或删除人物都先显示具体变更。

## CLI 与 AI

```text
freeagent ext state --id cc.endview.affectweave --role maya --space local --actor owner --data-dir <Core数据目录>
freeagent control catalog
freeagent control list --data-dir <Core数据目录> --json
```

创建/修改/删除人物和状态使用 Core 的 `extensions.change_state`；设置默认人物和片段字节预算使用 manifest 白名单限制的 `extensions.set_policy`。CLI `control plan` 显示提示，GUI 使用确认卡片，AI 只能提出具体提案。来源 ID 可从管理页最近已采用输入或 `ext state` 返回值取得；昵称和模型文字不能充当身份。

## 备份与恢复

```text
freeagent ext backup --id cc.endview.affectweave --to <新的配套备份目录> --data-dir <当前数据目录>
freeagent ext verify-backup --from <配套备份目录>
freeagent ext restore --from <配套备份目录> --data-dir <新的数据目录> --package <同版本cc.endview.affectweave目录>
```

备份只暂停扩展写回，最多约 110 秒，普通 Core 任务继续。它核查原操作结果，保存两份数据库快照、控制日志和持久来源清单；等待超时、来源缺失或原写入未核清时不发布完整备份，暂停最终释放。

恢复前先停止旧 Owner 与渠道，避免同一身份运行两份 Bot。目标必须是新目录，扩展包版本必须匹配。恢复不重跑模型或工具；扩展恢复失败时保留已恢复的 Core，并关闭扩展。凭据、工作区文件仍按 Core 指南单独备份。备份含私人状态，按用户数据保管。

一次状态提交丢失回执时，使用 `freeagent control reconcile-extension --operation <原操作ID> --data-dir <数据目录>` 核对原结果。核对只读，不再次派发；找不到且不能确定结束时继续保留 UNKNOWN。

## 开发与测试

```text
go test -race ./internal/affect
go test -race ./...
python scripts/package.py --target windows-amd64 --out output/packages --go go
python scripts/package.py --target linux-amd64 --out output/packages --go go
```

PG 集成测试需要独立服务。通过 FreeAgent 的 `scripts/test-postgres.py --runtime <PG15运行时> -- go test -race ./...` 运行；没有服务会明确 skip，不能当作 PG 验收通过。生产打包要求已提交源码；本地验证才使用 `--allow-dirty`。

[架构与数据](docs/ARCHITECTURE.md) · [宿主接口](docs/FREEAGENT.md) · [规则迁写对照](docs/REFERENCE_MAPPING.md)。原 Python demo 已归档到 `reference/python`，固定提交和数值 fixture 仅供历史对照；不随产品包发行，不保留 SQLite 后端或旧协议兼容层。

MIT；发行包保留 Go 及运行依赖的许可证。
