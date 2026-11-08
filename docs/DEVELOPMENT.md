# AffectWeave 扩展开发指南

目标是独立可选的人物算法，遵守 FreeAgent 的轻 Core 原则。仓库当前实现纯 Go，旧 Python 只留 reference/python 的固定来源对照；发行只含 Go 程序、manifest、文档和许可证。

## 运行和测试

使用 Go 1.26.5。运行时连接必须是 Core 显式初始化的独立扩展 PG 文件，stdout 不打印调试信息。源码逻辑、存储和协议分别测试：

```text
go test ./internal/affect
# 从本仓库执行；CORE为FreeAgent源码目录，PG_RUNTIME为干净PG15运行时。
python CORE/scripts/test-postgres.py --runtime PG_RUNTIME -- go test -race ./cmd/... ./internal/...
go vet ./cmd/... ./internal/...
python scripts/package.py --target windows-amd64 --out NEW_OUTPUT --go go
python scripts/package.py --target linux-amd64 --out OTHER_OUTPUT --go go
```

没有真实 PG 时测试会明确 skip，不能称为存储通过。Core 跨进程测试须设置 FREEAGENT_TEST_AFFECTWEAVE_BINARY 指向本版二进制，并同步 extension.json 到其测试 fixture。全量测试只针对 cmd/internal，不能把私有 output 样例当发行源码。

## 增加人物规则

1. 在 internal/affect 新增纯函数，明确参数范围、时点、来源和上限；不读文件、不联网、不隐式修改长期状态。
2. 若为持久操作，在 storage 的 Preview/Apply 共用验证中加入CAS、原操作指纹和幂等；预览不能改输入对象。
3. 在 extension.json 的state_operations声明 scope/input_schema，再由Core的extensions.change_state预览与确认。业务payload不侵入Core。
4. 覆盖中性值、极值、重复来源、不同原因/对象、时间倒退、取消、来源撤销和重启；PG事务与真实独立进程另测。

## 增加表达/主动策略

表达只返回原文无损reply_parts或已授权资源选择。固定候选必须先冻结正文，用delivery.check做纯复核。动态主动开场只能在wake.evaluate：校验明确scheduled身份、空adopted_inputs、允许来源、deadline；最多一次model.generate。禁止自建计时器、Bot HTTP客户端或Key管理。长期人物变化仍是待审阅提案，不借主动开关自动写回。

模型和历史均是不可信材料，严格解析封闭结果并限长；无效结果降级/skip，不用另一个模型修JSON。原Claim/Run/Attempt代表一次真实派发，换随机key不是恢复策略。资料里的人格是AI风格角色，不推断真人心理诊断或私人事实。

## 发布与维护

两仓库配套版本、能力schema、开发指南、合成测试和NOT_RUN项一起更新。打包要求已提交源码，只有本地验证可allow-dirty；同提交重复构建应逐文件及归档一致。包不含Core数据库、私人角色、连接文件、Key或测试日志。升级不自动打开分析/交互/演进/主动总开关，卸载后Core照常运行。
