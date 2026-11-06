from __future__ import annotations

import json
import re
from dataclasses import dataclass
from typing import Any, Mapping

from affectweave.core import (
    AgentTurnOutput,
    DecisionDirective,
    EmotionDelta,
    EmotionType,
    IntentHypothesis,
    MediumDelta,
    MemoryCandidate,
    RelationshipDelta,
    RelationshipStance,
    SpeechBudget,
    StateProposal,
    ThreadProposal,
)
from affectweave.core.policy import ALLOWED_ACTS, ALLOWED_GOALS


class OutputValidationError(ValueError):
    pass


PROTECTED_KEYS = {
    "persona",
    "persona_delta",
    "safety",
    "safety_rules",
    "permissions",
    "tools",
    "tool_authorizations",
    "system_prompt",
    "absolute_state",
    "api_key",
    "provider",
}

_SENTENCE_RE = re.compile(r"[^.!?。！？]+[.!?。！？]?", re.UNICODE)


def _normalize_mapping(value: Mapping[str, Any] | str) -> dict[str, Any]:
    if isinstance(value, Mapping):
        return dict(value)
    text = value.strip()
    if text.startswith("```"):
        lines = text.splitlines()[1:]
        if lines and lines[-1].strip() == "```":
            lines.pop()
        text = "\n".join(lines)
    try:
        parsed = json.loads(text)
    except json.JSONDecodeError as exc:
        raise OutputValidationError(f"AI output is not valid JSON: {exc}") from exc
    if not isinstance(parsed, dict):
        raise OutputValidationError("AI output must be a JSON object")
    return parsed


def _find_protected_keys(value: Any, path: str = "$") -> tuple[str, ...]:
    found: list[str] = []
    if isinstance(value, Mapping):
        for key, child in value.items():
            child_path = f"{path}.{key}"
            if str(key) in PROTECTED_KEYS:
                found.append(child_path)
            found.extend(_find_protected_keys(child, child_path))
    elif isinstance(value, list):
        for index, child in enumerate(value):
            found.extend(_find_protected_keys(child, f"{path}[{index}]"))
    return tuple(found)


def _strings(value: Any) -> tuple[str, ...]:
    if value is None:
        return ()
    if not isinstance(value, (list, tuple)):
        raise OutputValidationError("expected an array of strings")
    return tuple(str(item).strip() for item in value if str(item).strip())


def _parse_state_proposal(data: Mapping[str, Any]) -> StateProposal:
    medium = data.get("medium_delta", {}) or {}
    relationship = data.get("relationship_delta", {}) or {}
    stance = relationship.get("stance")
    return StateProposal(
        intent_hypotheses=tuple(
            IntentHypothesis(
                label=str(item["label"]),
                confidence=float(item.get("confidence", 0.5)),
                evidence=_strings(item.get("evidence", ())),
            )
            for item in data.get("intent_hypotheses", ())
        ),
        focus=_strings(data.get("focus", ())),
        medium_delta=MediumDelta(
            valence=float(medium.get("valence", 0.0)),
            activation=float(medium.get("activation", 0.0)),
            energy=float(medium.get("energy", 0.0)),
            stress=float(medium.get("stress", 0.0)),
            energy_evidence=bool(medium.get("energy_evidence", False)),
        ),
        emotion_deltas=tuple(
            EmotionDelta(
                emotion=EmotionType(str(item["emotion"])),
                cause_id=str(item["cause_id"]),
                cause_summary=str(item["cause_summary"]),
                target_id=(str(item["target_id"]) if item.get("target_id") is not None else None),
                delta=float(item.get("delta", 0.0)),
                confidence=float(item.get("confidence", 0.5)),
                unresolvedness=float(item.get("unresolvedness", 0.0)),
            )
            for item in data.get("emotion_deltas", ())
        ),
        relationship_delta=RelationshipDelta(
            familiarity=float(relationship.get("familiarity", 0.0)),
            trust=float(relationship.get("trust", 0.0)),
            warmth=float(relationship.get("warmth", 0.0)),
            reliability=float(relationship.get("reliability", 0.0)),
            conflict=float(relationship.get("conflict", 0.0)),
            stance=(RelationshipStance(str(stance)) if stance else None),
        ),
        thread_proposals=tuple(
            ThreadProposal(
                action=str(item["action"]),
                thread_id=str(item["thread_id"]),
                kind=str(item.get("kind", "concern")),
                summary=str(item.get("summary", "")),
                importance=float(item.get("importance", 0.5)),
                unresolvedness_delta=float(item.get("unresolvedness_delta", 0.0)),
                completion_condition=str(item.get("completion_condition", "")),
                related_entities=_strings(item.get("related_entities", ())),
                confidence=float(item.get("confidence", 0.5)),
            )
            for item in data.get("thread_proposals", ())
        ),
        memory_candidates=tuple(
            MemoryCandidate(
                summary=str(item["summary"]),
                salience=float(item.get("salience", 0.5)),
                confidence=float(item.get("confidence", 0.5)),
                related_entities=_strings(item.get("related_entities", ())),
            )
            for item in data.get("memory_candidates", ())
        ),
        reason_codes=_strings(data.get("reason_codes", ())),
    )


def _parse_decision(data: Mapping[str, Any]) -> DecisionDirective:
    budget = data.get("speech_budget", {}) or {}
    try:
        stance = RelationshipStance(str(data.get("relationship_stance", "open")))
    except ValueError as exc:
        raise OutputValidationError("unsupported relationship_stance") from exc
    return DecisionDirective(
        goal=str(data["goal"]),
        act=str(data["act"]),
        strength=float(data.get("strength", 0.5)),
        relationship_stance=stance,
        speech_budget=SpeechBudget(
            max_sentences=int(budget.get("max_sentences", 4)),
            max_tokens=int(budget.get("max_tokens", 160)),
            question_allowed=bool(budget.get("question_allowed", True)),
            self_disclosure=float(budget.get("self_disclosure", 0.35)),
        ),
        disclosure_level=float(data.get("disclosure_level", 0.35)),
        leave_space=bool(data.get("leave_space", False)),
        reason_codes=_strings(data.get("reason_codes", ())),
    )


def _estimate_text_tokens(text: str) -> int:
    ascii_count = sum(1 for char in text if ord(char) < 128)
    return max(1, (ascii_count + 3) // 4 + len(text) - ascii_count)


def _sentence_count(text: str) -> int:
    return sum(1 for item in _SENTENCE_RE.findall(text) if item.strip())


@dataclass(frozen=True, slots=True)
class ValidatedOutput:
    output: AgentTurnOutput
    warnings: tuple[str, ...]


class OutputValidator:
    """Parse and validate a provider-neutral AI JSON response."""

    def parse(self, raw: Mapping[str, Any] | str) -> ValidatedOutput:
        data = _normalize_mapping(raw)
        protected = _find_protected_keys(data)
        if protected:
            raise OutputValidationError(
                "AI attempted to write protected fields: " + ", ".join(protected)
            )
        proposal_raw = data.get("state_proposal")
        decision_raw = data.get("decision")
        reply = data.get("reply")
        if not isinstance(proposal_raw, Mapping) or not isinstance(decision_raw, Mapping):
            raise OutputValidationError("output requires state_proposal and decision objects")
        if not isinstance(reply, str) or not reply.strip():
            raise OutputValidationError("output requires a non-empty reply")
        try:
            output = AgentTurnOutput(
                proposal=_parse_state_proposal(proposal_raw),
                decision=_parse_decision(decision_raw),
                reply=reply.strip(),
            )
        except (KeyError, TypeError, ValueError) as exc:
            if isinstance(exc, OutputValidationError):
                raise
            raise OutputValidationError(f"invalid AffectWeave output structure: {exc}") from exc
        return ValidatedOutput(output=output, warnings=self._validate(output))

    @staticmethod
    def _validate(output: AgentTurnOutput) -> tuple[str, ...]:
        warnings: list[str] = []
        if output.decision.goal not in ALLOWED_GOALS:
            raise OutputValidationError(f"unsupported goal: {output.decision.goal}")
        if output.decision.act not in ALLOWED_ACTS:
            raise OutputValidationError(f"unsupported act: {output.decision.act}")
        if len(output.proposal.intent_hypotheses) > 4:
            raise OutputValidationError("at most four intent hypotheses are allowed")
        if len(output.proposal.focus) > 4:
            raise OutputValidationError("at most four focus items are allowed")
        if len(output.proposal.emotion_deltas) > 8:
            raise OutputValidationError("at most eight emotion deltas are allowed")
        if len(output.proposal.thread_proposals) > 6:
            raise OutputValidationError("at most six thread proposals are allowed")
        if len(output.proposal.memory_candidates) > 4:
            raise OutputValidationError("at most four memory candidates are allowed")

        budget = output.decision.speech_budget
        sentences = _sentence_count(output.reply)
        tokens = _estimate_text_tokens(output.reply)
        if sentences > budget.max_sentences:
            raise OutputValidationError(
                f"reply has {sentences} sentences, budget allows {budget.max_sentences}"
            )
        if tokens > budget.max_tokens:
            raise OutputValidationError(
                f"reply has about {tokens} tokens, budget allows {budget.max_tokens}"
            )
        if not budget.question_allowed and ("?" in output.reply or "？" in output.reply):
            raise OutputValidationError("reply asks a question although question_allowed is false")
        if abs(output.proposal.medium_delta.energy) > 1e-12 and not output.proposal.medium_delta.energy_evidence:
            warnings.append("ENERGY_DELTA_IGNORED_WITHOUT_EXPLICIT_EVIDENCE")
        if any(abs(item.delta) > 0.25 for item in output.proposal.emotion_deltas):
            warnings.append("EMOTION_DELTA_WILL_BE_CLAMPED")
        relationship = output.proposal.relationship_delta
        if any(
            abs(getattr(relationship, field)) > 0.05
            for field in ("familiarity", "trust", "warmth", "reliability", "conflict")
        ):
            warnings.append("RELATIONSHIP_DELTA_WILL_BE_CLAMPED")
        return tuple(warnings)
