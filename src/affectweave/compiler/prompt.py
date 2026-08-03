from __future__ import annotations

import warnings

from affectweave.core import TurnPreparation

from .context import ContextCompiler, ContextConfig, estimate_tokens
from .packet import EnhancementPacket


PromptConfig = ContextConfig


class PromptCompiler:
    """Compatibility wrapper around :class:`ContextCompiler`.

    New code should compile a :class:`ContextPackage` with ``ContextCompiler``
    and render it explicitly. This wrapper preserves the v0.1 API by returning
    an :class:`EnhancementPacket` with text as its default rendering.
    """

    def __init__(self, config: PromptConfig | None = None) -> None:
        warnings.warn(
            "PromptCompiler is renamed to ContextCompiler; the compatibility "
            "wrapper will remain available through v0.2-demo.",
            DeprecationWarning,
            stacklevel=2,
        )
        self.context_compiler = ContextCompiler(config)
        self.config = self.context_compiler.config

    def compile(
        self,
        preparation: TurnPreparation,
        *,
        token_budget: int | None = None,
    ) -> EnhancementPacket:
        context = self.context_compiler.compile(preparation, token_budget=token_budget)
        return EnhancementPacket.from_context(context, render_format="text")


__all__ = [
    "PromptCompiler",
    "PromptConfig",
    "estimate_tokens",
]
