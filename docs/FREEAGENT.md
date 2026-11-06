# FreeAgent 接口

协议 `freeagent-ext/1`，一行一个 JSON-RPC 2.0，stdout 仅协议，最多 4 个并发请求、1 MiB 帧、20 秒请求期限。`$/cancel` 按 invocation 取消，协议不自动重试写入。

manifest 声明 `state`、`passive_context`、`storage: postgresql` 和 `turn.finished`。被动上下文不允许工具、工作流、额外模型预算、工作区/历史 needs 或操作拦截；Host API 仅可报告活动。普通 Core 不依赖该能力。

| 方法 | 行为 |
| --- | --- |
| initialize / shutdown | 身份/版本注册与退出 |
| context.prepare | 采用 Core 验证的 identity/adopted_inputs、角色 variant 和冻结设置，返回有界片段 |
| turn.finished | 记录 L0 生成，不声称发送/显示/已读 |
| state.read | 授权空间的人物、当前状态、最近输入引用、经历和事实 |
| state.preview | 以 expected_revision 预览具体 before/after，不写状态 |
| state.apply | 原 operation_id/digest/change；事务内更新和去重 |
| state.result | 只读原结果，未找到不等于证明未执行 |
| state.sources | 只读分页来源，配套快照核对 |
| state.pause / pause_status / resume | 唯一令牌和绝对期限的有限暂停，取消/到期释放 |
| facts.sync | 补登记 Core 原记录，不派发模型/工具；上下文自动补读有界近期同角色记录 |

动作包括 create_role、edit_role、delete_role、apply_proposal、record_outcome、reset_state、delete_state、delete_memory。GUI/CLI/AI 经 Core 同一控制面确认。人物的空间必须明确授权。删除当前状态不承诺清除保留的审计记录。

Core 只交付 `FREEAGENT_EXTENSION_PG_FILE`，其中是独立库连接。未初始化就报告存储错误，不建立临时 SQLite 或内存假持久化。包不附真实连接、Key、人物或对话。

普通设置白名单在 manifest.model_settings：role_id、context_bytes。环境映射仍 user-only，不开放任意 settings/env_from。本轮选人用 Core `CapabilitySelection.extension_variants`，快照冻结，后续修改只影响新轮。
