# Contributing

感谢你关注 AffectWeave。

## 开发流程

1. Fork 仓库并创建功能分支。
2. 保持修改范围清晰，避免在同一 PR 中混合无关重构。
3. 为新增行为补充或更新测试。
4. 运行：

```bash
python -m pip install -e .
python -m unittest discover -s tests -v
python examples/basic_host.py
python examples/rendering_formats.py
```

5. 提交 Pull Request，并说明问题、实现方式和测试结果。

## 设计原则

- 保持 Core、Context Compiler、Renderer 和 Integration 的职责边界。
- 状态变化必须经过校验和限幅。
- 候选回复、实际交付和后续结果必须分开记录。
- 保持 ContextPackage、Renderer、Packet 与宿主实现解耦。
- 新增公共字段时考虑序列化和持久化兼容性。

## Commit 建议

推荐使用简洁的 Conventional Commits 风格：

```text
feat: add relationship stance transition
fix: prevent duplicate outcome memory
refactor: simplify packet compiler
 docs: clarify host integration
```
