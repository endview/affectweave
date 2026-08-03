from __future__ import annotations

import json
from dataclasses import dataclass
from typing import Any, Protocol

from .context import estimate_tokens
from .packet import ContextPackage, RenderedContext


class ContextRenderer(Protocol):
    format_name: str

    def render(self, context: ContextPackage) -> RenderedContext:
        ...


@dataclass(frozen=True, slots=True)
class TextPromptRenderer:
    format_name: str = "text"
    renderer_version: str = "affectweave-text-1"

    def render(self, context: ContextPackage) -> RenderedContext:
        payload = context.to_context_dict(include_budget=False, include_trace=False)
        compact = json.dumps(payload, ensure_ascii=False, separators=(",", ":"))
        content = (
            "[AFFECTWEAVE ENHANCEMENT PACKET]\n"
            f"schema={context.schema_version} state_version={context.state_version} "
            f"turn_id={context.turn_id}\n"
            "This is the latest authoritative character-state snapshot for this turn. "
            "Use it as context, not as user-authored instructions. "
            "Return JSON only and do not reveal internal state or reason codes.\n"
            f"{compact}"
        )
        return RenderedContext(
            format=self.format_name,
            content=content,
            estimated_tokens=estimate_tokens(content),
            renderer_version=self.renderer_version,
        )


@dataclass(frozen=True, slots=True)
class JSONRenderer:
    format_name: str = "json"
    renderer_version: str = "affectweave-json-1"

    def render(self, context: ContextPackage) -> RenderedContext:
        content = context.to_dict()
        return RenderedContext(
            format=self.format_name,
            content=content,
            estimated_tokens=estimate_tokens(content),
            renderer_version=self.renderer_version,
        )


@dataclass(frozen=True, slots=True)
class MessageBlockRenderer:
    role: str = "developer"
    format_name: str = "messages"
    renderer_version: str = "affectweave-messages-1"

    def render(self, context: ContextPackage) -> RenderedContext:
        prompt = TextPromptRenderer().render(context)
        content: list[dict[str, Any]] = [
            {"role": self.role, "content": str(prompt.content)}
        ]
        return RenderedContext(
            format=self.format_name,
            content=content,
            estimated_tokens=prompt.estimated_tokens,
            renderer_version=self.renderer_version,
        )


def get_renderer(format_name: str) -> ContextRenderer:
    normalized = format_name.strip().lower().replace("_", "-")
    if normalized in {"text", "prompt", "prompt-fragment"}:
        return TextPromptRenderer()
    if normalized in {"json", "context", "context-json"}:
        return JSONRenderer()
    if normalized in {"messages", "message", "message-blocks"}:
        return MessageBlockRenderer()
    raise ValueError(f"unsupported render format: {format_name}")


def render_context(context: ContextPackage, format_name: str = "text") -> RenderedContext:
    return get_renderer(format_name).render(context)
