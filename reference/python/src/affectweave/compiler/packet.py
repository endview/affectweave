from __future__ import annotations

import json
from copy import deepcopy
from dataclasses import dataclass, field
from typing import Any, Mapping


PACKET_SCHEMA_VERSION = "affectweave-enhancement/1"
OUTPUT_SCHEMA_NAME = "affectweave-character-turn/1"


_EMOTIONS = [
    "joy",
    "interest",
    "hope",
    "relief",
    "gratitude",
    "anger",
    "fear",
    "sadness",
    "disappointment",
    "guilt",
    "shame",
    "disgust",
]
_STANCES = [
    "open",
    "friendly",
    "cautious",
    "guarded",
    "defensive",
    "distant",
    "disengaged",
]
_GOALS = [
    "connect",
    "understand",
    "share",
    "solve",
    "protect_self",
    "repair",
    "create_distance",
    "end_interaction",
]
_ACTS = [
    "answer",
    "acknowledge",
    "self_disclose",
    "ask_or_clarify",
    "support",
    "repair_or_boundary",
    "assert",
    "disengage",
    "topic_shift",
    "leave_space",
]


def _string_array() -> dict[str, Any]:
    return {"type": "array", "items": {"type": "string"}}


def _state_proposal_schema() -> dict[str, Any]:
    return {
        "type": "object",
        "additionalProperties": False,
        "required": [
            "intent_hypotheses",
            "focus",
            "medium_delta",
            "emotion_deltas",
            "relationship_delta",
            "thread_proposals",
            "memory_candidates",
            "reason_codes",
        ],
        "properties": {
            "intent_hypotheses": {
                "type": "array",
                "items": {
                    "type": "object",
                    "additionalProperties": False,
                    "required": ["label", "confidence", "evidence"],
                    "properties": {
                        "label": {"type": "string"},
                        "confidence": {"type": "number"},
                        "evidence": _string_array(),
                    },
                },
            },
            "focus": _string_array(),
            "medium_delta": {
                "type": "object",
                "additionalProperties": False,
                "required": [
                    "valence",
                    "activation",
                    "energy",
                    "stress",
                    "energy_evidence",
                ],
                "properties": {
                    "valence": {"type": "number"},
                    "activation": {"type": "number"},
                    "energy": {"type": "number"},
                    "stress": {"type": "number"},
                    "energy_evidence": {"type": "boolean"},
                },
            },
            "emotion_deltas": {
                "type": "array",
                "items": {
                    "type": "object",
                    "additionalProperties": False,
                    "required": [
                        "emotion",
                        "cause_id",
                        "cause_summary",
                        "target_id",
                        "delta",
                        "confidence",
                        "unresolvedness",
                    ],
                    "properties": {
                        "emotion": {"type": "string", "enum": _EMOTIONS},
                        "cause_id": {"type": "string"},
                        "cause_summary": {"type": "string"},
                        "target_id": {"type": ["string", "null"]},
                        "delta": {"type": "number"},
                        "confidence": {"type": "number"},
                        "unresolvedness": {"type": "number"},
                    },
                },
            },
            "relationship_delta": {
                "type": "object",
                "additionalProperties": False,
                "required": [
                    "familiarity",
                    "trust",
                    "warmth",
                    "reliability",
                    "conflict",
                    "stance",
                ],
                "properties": {
                    "familiarity": {"type": "number"},
                    "trust": {"type": "number"},
                    "warmth": {"type": "number"},
                    "reliability": {"type": "number"},
                    "conflict": {"type": "number"},
                    "stance": {
                        "type": ["string", "null"],
                        "enum": [*_STANCES, None],
                    },
                },
            },
            "thread_proposals": {
                "type": "array",
                "items": {
                    "type": "object",
                    "additionalProperties": False,
                    "required": [
                        "action",
                        "thread_id",
                        "kind",
                        "summary",
                        "importance",
                        "unresolvedness_delta",
                        "completion_condition",
                        "related_entities",
                        "confidence",
                    ],
                    "properties": {
                        "action": {
                            "type": "string",
                            "enum": ["upsert", "resolve", "abandon"],
                        },
                        "thread_id": {"type": "string"},
                        "kind": {
                            "type": "string",
                            "enum": [
                                "goal",
                                "concern",
                                "commitment",
                                "pending_outcome",
                            ],
                        },
                        "summary": {"type": "string"},
                        "importance": {"type": "number"},
                        "unresolvedness_delta": {"type": "number"},
                        "completion_condition": {"type": "string"},
                        "related_entities": _string_array(),
                        "confidence": {"type": "number"},
                    },
                },
            },
            "memory_candidates": {
                "type": "array",
                "items": {
                    "type": "object",
                    "additionalProperties": False,
                    "required": [
                        "summary",
                        "salience",
                        "confidence",
                        "related_entities",
                    ],
                    "properties": {
                        "summary": {"type": "string"},
                        "salience": {"type": "number"},
                        "confidence": {"type": "number"},
                        "related_entities": _string_array(),
                    },
                },
            },
            "reason_codes": _string_array(),
        },
    }


def _decision_schema() -> dict[str, Any]:
    return {
        "type": "object",
        "additionalProperties": False,
        "required": [
            "goal",
            "act",
            "strength",
            "relationship_stance",
            "speech_budget",
            "disclosure_level",
            "leave_space",
            "reason_codes",
        ],
        "properties": {
            "goal": {"type": "string", "enum": _GOALS},
            "act": {"type": "string", "enum": _ACTS},
            "strength": {"type": "number"},
            "relationship_stance": {"type": "string", "enum": _STANCES},
            "speech_budget": {
                "type": "object",
                "additionalProperties": False,
                "required": [
                    "max_sentences",
                    "max_tokens",
                    "question_allowed",
                    "self_disclosure",
                ],
                "properties": {
                    "max_sentences": {"type": "integer"},
                    "max_tokens": {"type": "integer"},
                    "question_allowed": {"type": "boolean"},
                    "self_disclosure": {"type": "number"},
                },
            },
            "disclosure_level": {"type": "number"},
            "leave_space": {"type": "boolean"},
            "reason_codes": _string_array(),
        },
    }


def output_schema() -> dict[str, Any]:
    """Return a provider-neutral JSON Schema for the host model output."""

    return {
        "name": OUTPUT_SCHEMA_NAME,
        "schema": {
            "type": "object",
            "additionalProperties": False,
            "required": ["state_proposal", "decision", "reply"],
            "properties": {
                "state_proposal": _state_proposal_schema(),
                "decision": _decision_schema(),
                "reply": {"type": "string"},
            },
        },
    }


@dataclass(frozen=True, slots=True)
class ContextBudgetReport:
    maximum_tokens: int
    estimated_tokens: int
    overflow: bool
    included_sections: tuple[str, ...] = ()
    removed_sections: tuple[str, ...] = ()
    removal_reasons: tuple[str, ...] = ()

    def __post_init__(self) -> None:
        if self.maximum_tokens < 0 or self.estimated_tokens < 0:
            raise ValueError("token counts must be non-negative")
        if len(self.removal_reasons) not in {0, len(self.removed_sections)}:
            raise ValueError("removal_reasons must be empty or align with removed_sections")

    def to_dict(self) -> dict[str, Any]:
        return {
            "maximum_tokens": self.maximum_tokens,
            "estimated_tokens": self.estimated_tokens,
            "overflow": self.overflow,
            "included_sections": list(self.included_sections),
            "removed_sections": list(self.removed_sections),
            "removal_reasons": list(self.removal_reasons),
        }

    @classmethod
    def from_dict(cls, data: Mapping[str, Any]) -> "ContextBudgetReport":
        return cls(
            maximum_tokens=int(data.get("maximum_tokens", 0)),
            estimated_tokens=int(data.get("estimated_tokens", 0)),
            overflow=bool(data.get("overflow", False)),
            included_sections=tuple(str(item) for item in data.get("included_sections", ())),
            removed_sections=tuple(str(item) for item in data.get("removed_sections", ())),
            removal_reasons=tuple(str(item) for item in data.get("removal_reasons", ())),
        )


@dataclass(frozen=True, slots=True)
class ContextTrace:
    compiler_version: str
    source_event_ids: tuple[str, ...]
    source_state_version: int

    def to_dict(self) -> dict[str, Any]:
        return {
            "compiler_version": self.compiler_version,
            "source_event_ids": list(self.source_event_ids),
            "source_state_version": self.source_state_version,
        }

    @classmethod
    def from_dict(cls, data: Mapping[str, Any]) -> "ContextTrace":
        return cls(
            compiler_version=str(data.get("compiler_version", "unknown")),
            source_event_ids=tuple(str(item) for item in data.get("source_event_ids", ())),
            source_state_version=int(data.get("source_state_version", 0)),
        )


CONTEXT_SCHEMA_VERSION = "affectweave-context/1"


@dataclass(frozen=True, slots=True)
class ContextPackage:
    """Host-neutral, bounded character context for one turn."""

    character_id: str
    turn_id: str
    behavior_id: str
    state_version: int
    immutable_rules: tuple[str, ...]
    current_event: Mapping[str, Any]
    character: Mapping[str, Any]
    current_state: Mapping[str, Any]
    relationship: Mapping[str, Any]
    active_threads: tuple[Mapping[str, Any], ...]
    relevant_memories: tuple[Mapping[str, Any], ...]
    recent_dialogue: tuple[Mapping[str, Any], ...]
    response_policy: Mapping[str, Any]
    output_contract: Mapping[str, Any]
    budget: ContextBudgetReport
    trace: ContextTrace
    schema_version: str = CONTEXT_SCHEMA_VERSION
    context_version: str = "1"

    def to_context_dict(
        self,
        *,
        include_budget: bool = True,
        include_trace: bool = True,
    ) -> dict[str, Any]:
        data: dict[str, Any] = {
            "immutable_rules": list(self.immutable_rules),
            "current_event": deepcopy(dict(self.current_event)),
            "character": deepcopy(dict(self.character)),
            "current_state": deepcopy(dict(self.current_state)),
            "relationship": deepcopy(dict(self.relationship)),
            "active_threads": [deepcopy(dict(item)) for item in self.active_threads],
            "relevant_memories": [deepcopy(dict(item)) for item in self.relevant_memories],
            "recent_dialogue": [deepcopy(dict(item)) for item in self.recent_dialogue],
            "response_policy": deepcopy(dict(self.response_policy)),
            "output_contract": deepcopy(dict(self.output_contract)),
        }
        if include_budget:
            data["budget"] = self.budget.to_dict()
        if include_trace:
            data["trace"] = self.trace.to_dict()
        return data

    def to_dict(self) -> dict[str, Any]:
        return {
            "schema_version": self.schema_version,
            "context_version": self.context_version,
            "character_id": self.character_id,
            "turn_id": self.turn_id,
            "behavior_id": self.behavior_id,
            "state_version": self.state_version,
            **self.to_context_dict(),
        }

    @classmethod
    def from_dict(cls, data: Mapping[str, Any]) -> "ContextPackage":
        return cls(
            schema_version=str(data.get("schema_version", CONTEXT_SCHEMA_VERSION)),
            context_version=str(data.get("context_version", "1")),
            character_id=str(data["character_id"]),
            turn_id=str(data["turn_id"]),
            behavior_id=str(data["behavior_id"]),
            state_version=int(data["state_version"]),
            immutable_rules=tuple(str(item) for item in data.get("immutable_rules", ())),
            current_event=dict(data.get("current_event", {})),
            character=dict(data.get("character", {})),
            current_state=dict(data.get("current_state", {})),
            relationship=dict(data.get("relationship", {})),
            active_threads=tuple(dict(item) for item in data.get("active_threads", ())),
            relevant_memories=tuple(dict(item) for item in data.get("relevant_memories", ())),
            recent_dialogue=tuple(dict(item) for item in data.get("recent_dialogue", ())),
            response_policy=dict(data.get("response_policy", {})),
            output_contract=dict(data.get("output_contract", {})),
            budget=ContextBudgetReport.from_dict(dict(data.get("budget", {}))),
            trace=ContextTrace.from_dict(dict(data.get("trace", {}))),
        )

    @classmethod
    def from_legacy_packet(cls, data: Mapping[str, Any]) -> "ContextPackage":
        context = dict(data.get("context", {}))
        state = dict(context.get("character_state", {}))
        current = dict(context.get("current_message", {}))
        return cls(
            character_id=str(data["character_id"]),
            turn_id=str(data["turn_id"]),
            behavior_id=str(data["behavior_id"]),
            state_version=int(data["state_version"]),
            immutable_rules=tuple(str(item) for item in context.get("hard_rules", ())),
            current_event=current,
            character=dict(state.get("persona", {})),
            current_state={
                "medium": dict(state.get("medium", {})),
                "emotions": list(state.get("emotions", ())),
            },
            relationship=dict(state.get("relationship", {})),
            active_threads=tuple(dict(item) for item in state.get("active_threads", ())),
            relevant_memories=tuple(dict(item) for item in context.get("relevant_memories", ())),
            recent_dialogue=tuple(dict(item) for item in context.get("recent_dialogue", ())),
            response_policy={
                "task": list(context.get("task", ())),
                "state_change_limits": dict(context.get("state_change_limits", {})),
            },
            output_contract={
                "schema_name": str(dict(data.get("output_schema", {})).get("name", OUTPUT_SCHEMA_NAME)),
                "format": "json_object",
                "required_top_level_fields": ["state_proposal", "decision", "reply"],
            },
            budget=ContextBudgetReport(
                maximum_tokens=int(data.get("token_budget", 0)),
                estimated_tokens=int(data.get("estimated_tokens", 0)),
                overflow=bool(data.get("overflow", False)),
            ),
            trace=ContextTrace(
                compiler_version="affectweave-prompt-1-legacy",
                source_event_ids=(str(current.get("event_id", "")),),
                source_state_version=int(data.get("state_version", 0)),
            ),
        )


@dataclass(frozen=True, slots=True)
class RenderedContext:
    format: str
    content: Any
    estimated_tokens: int
    renderer_version: str

    def to_dict(self) -> dict[str, Any]:
        return {
            "format": self.format,
            "content": deepcopy(self.content),
            "estimated_tokens": self.estimated_tokens,
            "renderer_version": self.renderer_version,
        }

    @classmethod
    def from_dict(cls, data: Mapping[str, Any]) -> "RenderedContext":
        return cls(
            format=str(data.get("format", "text")),
            content=deepcopy(data.get("content", "")),
            estimated_tokens=int(data.get("estimated_tokens", 0)),
            renderer_version=str(data.get("renderer_version", "unknown")),
        )


@dataclass(frozen=True, slots=True)
class EnhancementPacket:
    """Context package plus the output contract and default host rendering."""

    context_package: ContextPackage
    default_rendering: RenderedContext
    schema_version: str = PACKET_SCHEMA_VERSION
    output_schema: Mapping[str, Any] = field(default_factory=output_schema)

    @classmethod
    def from_context(
        cls,
        context: ContextPackage,
        *,
        render_format: str = "text",
        schema: Mapping[str, Any] | None = None,
    ) -> "EnhancementPacket":
        from .renderers import render_context

        return cls(
            context_package=context,
            default_rendering=render_context(context, render_format),
            output_schema=dict(schema or output_schema()),
        )

    @property
    def character_id(self) -> str:
        return self.context_package.character_id

    @property
    def turn_id(self) -> str:
        return self.context_package.turn_id

    @property
    def behavior_id(self) -> str:
        return self.context_package.behavior_id

    @property
    def state_version(self) -> int:
        return self.context_package.state_version

    @property
    def prompt_fragment(self) -> str:
        return self.to_prompt()

    @property
    def context(self) -> Mapping[str, Any]:
        return self.context_package.to_dict()

    @property
    def estimated_tokens(self) -> int:
        return self.default_rendering.estimated_tokens

    @property
    def token_budget(self) -> int:
        return self.context_package.budget.maximum_tokens

    @property
    def overflow(self) -> bool:
        return self.context_package.budget.overflow

    def render(self, format_name: str = "text") -> RenderedContext:
        from .renderers import render_context

        if self.default_rendering.format == format_name:
            return self.default_rendering
        return render_context(self.context_package, format_name)

    def to_prompt(self, *, include_schema: bool = False) -> str:
        rendered = self.render("text")
        prompt = str(rendered.content)
        if not include_schema:
            return prompt
        schema_text = json.dumps(self.output_schema, ensure_ascii=False, separators=(",", ":"))
        return f"{prompt}\n\n[AFFECTWEAVE OUTPUT JSON SCHEMA]\n{schema_text}"

    def to_context_json(self, *, indent: int | None = 2) -> str:
        rendered = self.render("json")
        return json.dumps(rendered.content, ensure_ascii=False, indent=indent)

    def to_messages(self) -> list[dict[str, Any]]:
        rendered = self.render("messages")
        return [dict(item) for item in rendered.content]

    def to_dict(self) -> dict[str, Any]:
        return {
            "schema_version": self.schema_version,
            "character_id": self.character_id,
            "turn_id": self.turn_id,
            "behavior_id": self.behavior_id,
            "state_version": self.state_version,
            "context_package": self.context_package.to_dict(),
            "default_rendering": self.default_rendering.to_dict(),
            "output_schema": deepcopy(dict(self.output_schema)),
            # Compatibility fields retained for v0.1 hosts.
            "prompt_fragment": self.to_prompt(),
            "context": self.context_package.to_dict(),
            "estimated_tokens": self.estimated_tokens,
            "token_budget": self.token_budget,
            "overflow": self.overflow,
        }

    def to_json(self, *, indent: int | None = 2) -> str:
        """Serialize the full packet for persistence or cross-process handoff."""

        return json.dumps(self.to_dict(), ensure_ascii=False, indent=indent)

    @classmethod
    def from_dict(cls, data: Mapping[str, Any]) -> "EnhancementPacket":
        if "context_package" in data:
            context = ContextPackage.from_dict(dict(data["context_package"]))
        else:
            context = ContextPackage.from_legacy_packet(data)

        if "default_rendering" in data:
            rendering = RenderedContext.from_dict(dict(data["default_rendering"]))
        else:
            rendering = RenderedContext(
                format="text",
                content=str(data.get("prompt_fragment", "")),
                estimated_tokens=int(data.get("estimated_tokens", 0)),
                renderer_version="affectweave-text-legacy",
            )
        return cls(
            schema_version=str(data.get("schema_version", PACKET_SCHEMA_VERSION)),
            context_package=context,
            default_rendering=rendering,
            output_schema=dict(data.get("output_schema", output_schema())),
        )

    @classmethod
    def from_json(cls, text: str) -> "EnhancementPacket":
        data = json.loads(text)
        if not isinstance(data, dict):
            raise ValueError("packet JSON must be an object")
        return cls.from_dict(data)
