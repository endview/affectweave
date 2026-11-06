# AffectWeave 1.0.0-dev.1 — FreeAgent Go 扩展

整体 Go 迁写、独立 PostgreSQL、人物/关系/情绪/事件与经历、有界人物片段、已采用输入冻结、具体持久提案确认、真实交付事实和配套备份恢复。默认 Core 不加载业务或扩展进程。

实际验收以 FreeAgent v8.3 报告为准；开发候选不等于全部原生设备和真实群场景稳定验收。原 Python 版本的发布记录保留在下面，属于历史。

# 历史：v0.2-demo — Context Pipeline

本版本将状态选择与提示词渲染正式拆分。

## Highlights

- 新增标准化 `ContextPackage`；
- 新增 `ContextCompiler` 与 `ContextConfig`；
- 支持文本提示词、Context JSON 和通用消息块；
- 新增上下文预算、裁剪记录和编译追踪；
- 保留 `PromptCompiler`、`packet.prompt_fragment`、旧 Packet 读取与原有行为反馈流程；
- Core、SQLite 数据结构与状态更新行为保持不变；
- 自动测试扩展至 28 项。

## Quick verification

```bash
python -m pip install -e .
python -m unittest discover -s tests -v
python examples/basic_host.py
python examples/rendering_formats.py
```

## Release asset

```text
affectweave-0.2.0.dev0-py3-none-any.whl
```
