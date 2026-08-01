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
class EnhancementPacket:
    """A passive, provider-neutral context packet produced for one turn.

    The host owns model invocation. AffectWeave only provides the prompt fragment,
    optional JSON Schema, identifiers required for the feedback loop, and a
    compact state view.
    """

    character_id: str
    turn_id: str
    behavior_id: str
    state_version: int
    prompt_fragment: str
    context: Mapping[str, Any]
    estimated_tokens: int
    token_budget: int
    overflow: bool = False
    schema_version: str = PACKET_SCHEMA_VERSION
    output_schema: Mapping[str, Any] = field(default_factory=output_schema)

    def to_dict(self) -> dict[str, Any]:
        return {
            "schema_version": self.schema_version,
            "character_id": self.character_id,
            "turn_id": self.turn_id,
            "behavior_id": self.behavior_id,
            "state_version": self.state_version,
            "prompt_fragment": self.prompt_fragment,
            "context": deepcopy(dict(self.context)),
            "output_schema": deepcopy(dict(self.output_schema)),
            "estimated_tokens": self.estimated_tokens,
            "token_budget": self.token_budget,
            "overflow": self.overflow,
        }

    def to_json(self, *, indent: int | None = 2) -> str:
        return json.dumps(self.to_dict(), ensure_ascii=False, indent=indent)

    @classmethod
    def from_dict(cls, data: Mapping[str, Any]) -> "EnhancementPacket":
        return cls(
            schema_version=str(data.get("schema_version", PACKET_SCHEMA_VERSION)),
            character_id=str(data["character_id"]),
            turn_id=str(data["turn_id"]),
            behavior_id=str(data["behavior_id"]),
            state_version=int(data["state_version"]),
            prompt_fragment=str(data["prompt_fragment"]),
            context=dict(data.get("context", {})),
            output_schema=dict(data.get("output_schema", output_schema())),
            estimated_tokens=int(data.get("estimated_tokens", 0)),
            token_budget=int(data.get("token_budget", 0)),
            overflow=bool(data.get("overflow", False)),
        )

    @classmethod
    def from_json(cls, text: str) -> "EnhancementPacket":
        data = json.loads(text)
        if not isinstance(data, dict):
            raise ValueError("packet JSON must be an object")
        return cls.from_dict(data)

    def to_prompt(self, *, include_schema: bool = False) -> str:
        if not include_schema:
            return self.prompt_fragment
        schema_text = json.dumps(self.output_schema, ensure_ascii=False, separators=(",", ":"))
        return f"{self.prompt_fragment}\n\n[AFFECTWEAVE OUTPUT JSON SCHEMA]\n{schema_text}"
