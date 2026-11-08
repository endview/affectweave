# Python 参考与 Go 迁写

参考提交：`d8be9d3255006c51a96c114e36949771ae26517c`。迁写前 28 项 Python 测试实际通过。归档只是原证据，生产测试不依赖 Python。

| 原有范围 | Go 实现/检查 | 变化 |
| --- | --- | --- |
| Persona、心境、情绪原因共存、关系、未决事件 | affect/types.go、rules.go；规则测试和 Python fixture | 保留数值规则；拒绝未知字段和非有限数字 |
| prepare_turn、版本、去重、重开持久化 | storage.Adopt / Apply / Operation；真实 PG/CAS/丢回执检查 | 已采用输入冻结；会话序号不冲突；SQLite 改为唯一 PG |
| commit_agent_output 与反馈 | 生成事实 + Core 控制提案 + record_outcome | 取消即时长期学习；未确认不生效；自评不提升证据 |
| 编译、确定性、裁剪、不修改源、预算报告 | affect.Compile / Context；预算/原文/源状态检查 | 单个人物片段，不重复历史；身份/边界溢出时报错 |
| Text/JSON/Messages、packet round-trip | 片段内结构化 JSON + NDJSON；宿主契约 | 消息与 Markdown 由 Core 负责，不保留旧 packet/PromptCompiler 兼容包装 |
| 输出校验、保护字段、speech budget | Output.Validate、strictJSON；规则与协议检查 | 风格字段有界；Core 普通回答不受 Demo 强制 JSON 或字数拦截 |
| 被动宿主、CLI、不私自调用模型 | extension.Server；真实宿主/恢复/GUI 检查 | Core 正常模型链路与 CLI，不另建回答引擎 |
| 交付/反馈、跨进程连续性 | aw.facts、冻结输入、操作 ledger、配套恢复 | UNKNOWN/partial 独立，不补跑工具或模型 |

`internal/affect/testdata/python-reference.json` 是从固定参考实际导出的 5 类场景：限幅、共存、残留衰减、能量证据、低置信度解决；数值容差 1e-12，离散决定严格比较。新的来源、授权、确认和 UNKNOWN 行为按新契约验收，不假称与旧 demo 行为完全相同。

整体 Go 迁写消除了生产 Python/SQLite 包装；业务仍独立于 FreeAgent Core，没有新增通用 Go SDK 或插件平台。

以上映射说明规则、持久化与宿主接口的迁写，不代表普通聊天已自动产生状态提案或完成结果回流。2026-11-07 源码核查确认：手动人物编辑与新版本采用已实现；自动分析、人格参数驱动、情绪交互和经审查的人格演进尚待实施。完整冻结、反馈关联及后续交付范围见[后续实施方案](AFFECTWEAVE_NEXT_IMPLEMENTATION_PLAN_2026-11-07.md)。

## 1.6 Go 实施增量

冻结：storage/frozen.go、analysis.go；动态分析：extension/analysis.go；可信反馈：storage/feedback.go、feedback_context.go；参数与交互：affect/parameters.go、interactions.go；演进与版本：storage/evolution.go；主动：affect/proactive.go、extension/proactive.go、storage/wake.go。运行权威仍在Core。自动L3升级未实现，明确拒绝；真人场景验收不能由Python旧功能表替代。
