# AffectWeave v0.2-demo — Context Pipeline

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
