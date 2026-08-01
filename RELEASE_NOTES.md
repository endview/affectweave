# AffectWeave v0.1-demo

首个公开演示版本。

## Highlights

- 持续维护人物人格、中期状态、情绪、关系与未完成事件；
- 每轮生成紧凑的 `EnhancementPacket`；
- 保存与具体原因和对象绑定的情绪实例；
- 跟踪人物与不同交流对象的关系状态；
- 区分候选回复、实际交付和后续结果；
- 支持内存与 SQLite 持久化；
- 提供 Python API、CLI 和完整宿主示例。

## Quick verification

```bash
python -m pip install -e .
python -m unittest discover -s tests -v
python examples/basic_host.py
```

## Release asset

```text
affectweave-0.1.0.dev0-py3-none-any.whl
```
