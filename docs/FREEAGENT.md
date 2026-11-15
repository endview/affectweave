# FreeAgent 接入契约

本次配套 FreeAgent v8.8.0-dev.7 / AffectWeave 1.6.0-dev.3，继续使用 v8.7 公共协议。公共协议以 FreeAgent Extension Kit 为准；本文件描述扩展适配。

协议 freeagent-ext/1、state_version=2、受限独立 PG schema aw/storage_version=1。一行一个 JSON-RPC 2.0，stdout 仅协议，1 MiB 帧、最多四个并发请求。读循环持续分发 Host 响应；等待 model.generate 时不会堵住取消。普通请求最多20秒；分析2–30秒+清理余量；wake.evaluate遵守Host的5–120秒绝对截止时间，不能统一延长状态读取。

initialize 必须协商 context_analysis 和 scheduled_generation；缺能力拒绝启用。扩展通过 FREEAGENT_EXTENSION_PG_FILE 取得私有连接，不能读取 Core PG、Provider Key 或其他空间。

| 回调 | 行为 |
| --- | --- |
| context.prepare | 可信 adopted_inputs；全片段冻结；显式分析开关开启时一次无工具 model.generate |
| turn.finished | L0生成事实、无损reply_parts、已授权delivery.reply、固定delivery.propose或有界wake_hint |
| delivery.check | 只读当前事项，撤销已解决的固定跟进；无模型请求 |
| wake.evaluate | scheduled触发、adopted_inputs为空；授权话题，一次说/不说及最终文本；不发消息、不改人格 |
| state.read | 人物、状态、近期输入/分析/经历/事实、有效参数、证据与版本 |
| state.preview/apply/result | 具体修改CAS、原操作幂等/核对，长期变化必须确认 |
| facts.sync / state.sources | 补原事实和配套来源核查，绝不补执行 |
| state.pause/pause_status/resume | 备份的有界暂停；取消和到期释放 |

状态操作包括 create_role/edit_role/delete_role、apply_proposal、record_outcome、reset_state/delete_state/delete_memory、set_evolution/adapt_persona/freeze_persona/rollback_persona。Model payload 不能提供PG身份、来源权限或虚构结果证据；Core以scope.resource/payload绑定原控制。

普通设置语义和默认值由 extension.json 声明：role_id、context_bytes、reply_mode、max_text_parts、analysis_enabled/deadline、expression_enabled、fixed_followup_enabled。主动总开关**不在此设置中复制**，由delivery.generation_enabled唯一授权。仅安装不激活任何自动功能。持久变更走现有控制确认，GUI卡片/CLI提示/AI提案一致。

context_analysis：每输入一次、每Run两次；每次≤4096估算输入tokens/1024输出tokens，默认8秒。辅助失败降级，不影响普通回答。wake.evaluate：每机会一次，默认30秒；原机会、模型、结果、发送分别记录。reply-to反馈来源来自Core同Bot/目标/会话的真实sent片段，扩展只消费一次。L2不是业务成功证明。

Core负责授权/预算/取消/执行/有序投递；渠道负责平台格式/HTTP/typing/资源；扩展负责人物和表达动机。不新增通用插件UI运行时、独立调度数据库或兼容旧宿主分支。

回复表达实现见 [AffectWeave 自然分段实施方案](AFFECTWEAVE_REPLY_SEGMENTATION_PLAN_2026-11-16.md) 和[本次验证](VALIDATION_REPLY_SEGMENTATION_2026-11-16.md)。分条提示、人物模式、安全空行边界与均衡合并均在本仓库实现；FreeAgent 只登记通用选择、界面和渠道配套，不接管表达策略。

人物模式在 context.prepare 和 turn.finished 共用相同函数：冻结人物的有效直接度 < 0.7 且表达参数 ≥ 0.4 时采用自然分段，否则一条；无人物回退一条。分隔空行归前一段，CRLF 原样保存，拼接必须等于原输出。超出上限按源码长度选择相邻安全边界均衡合并；不猜测单换行的短句含义。引用式链接、跨段强调、围栏/缩进代码、列表、引用与公式保守保留结构。提示 ≤512 UTF-8 字节，属于扩展上下文额度；预算不足省去提示，不挤掉人物边界。

旧 Core v8.7/dev.5 已有所需 context/reply_parts/outbox 契约，可使用新算法；通用字段文案、首条回复引用和投递计数需 dev.7。新 Core 搭配旧 dev.2 扩展仍使用旧表达算法，宿主不替它实现人物策略。未安装或未选中时无本策略。
