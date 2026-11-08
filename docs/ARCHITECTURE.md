# 架构与数据

`cmd/affectweave` 为入口；`internal/affect` 是无 I/O 的人物/情绪/表达/话题规则；`internal/storage` 为受限 PG 事务、冻结、CAS 和原操作账目；`internal/extension` 是双向 NDJSON 与宿主适配。Core 不 import 此模块，Persona/Affect/Behavior 也不拆进程或数据库。

## 状态与事实

aw.roles：人物定义、明确空间授权和当前版本。aw.states：Core/空间/人物/参与者的已确认状态。aw.inputs：可信采用身份、状态和**完整**片段/经历/设置/分析结果。aw.facts：真实生成、交付、主动机会与反馈消费引用。aw.memories：明确确认的经历。aw.operations：原控制 ID、摘要、请求指纹、结果及人物版本。没有独立任务或额度表。

先短事务推进逻辑时间并冻结输入，再释放锁调用 Host。ClaimAnalysis 和 ClaimWake 是持久一次性入口；断开后不重发，迟到分析不能替换已经采用的降级片段。冻结不是授权冻结，删除/停用/空间撤销始终重查。

状态提交与原操作记录在同一事务。重复 ID 同内容返回原结果，不同内容拒绝；预览不修改调用者数据。延迟确认只应用截至当前逻辑时间的剩余情绪刺激，不把旧事件恢复为满强度。模型调用不持 PG 行锁。

## 参数和情绪

八类中性 0.5 参数：sensitivity（可按情绪覆盖）、recovery_speed、expression、regulation、directness、disclosure、repair_preference。灵敏增益 0.5+sensitivity；恢复时间常数 3600×2^(1−2×recovery_speed)。情绪按类型/原因/对象分开，单次变化 0.25；心境单次 0.05、关系 0.05，L2 回流权重减半。未决残留 0.30。

可选交互同原因/对象内同步计算，hope 抑制 fear、gratitude 抑制 anger、fear 促进 sadness；真实结果显式转化 fear→relief、hope→disappointment，来源消费一次。不跨原因全局传染，不在状态读取时再次激活。

人格演进使用基线加累计已批准 overlay。至少两个独立确认结果；单次≤0.05、累计≤0.20；保留版本，冻结后禁止适配/回退。手动编辑人物会建立新基线。参数不改权限、事实、路由或预算，不根据 MBTI/诊断/家庭条件自动赋值。

## 反馈和表达

L0 是生成，L1 是实际 sent 指定片段，L2 是后来明确引用该片段的用户反馈；当前自动反馈闭环只接受 L2，拒绝伪造 L3。部分送达只能引用实际 sent 的部分；UNKNOWN 不作为成功。候选保存不代表采用，操作者确认不代表业务成功。

片段保留身份/边界，依次收紧经历、风格、情绪/事项；身份/边界超预算明确失败。自动分析的临时解释不进入后续已确认状态。reply_parts 必须无损拼接原文；贴纸/反应只选择 Host 提供资源。固定跟进与主动生成分开；delivery.check 不调用模型。

主动读取当前确认状态及只读衰减，选最多三个授权话题，一次模型决定 speak/skip。没有新用户输入，不写 aw.inputs、不推进持久关系。Core 保存机会/Run/Attempt/发送，扩展保存人物评估事实。模型 UNKNOWN 不重跑，发送 UNKNOWN 不重发，用户新输入撤销旧结果。总开关只保存在 Core。
