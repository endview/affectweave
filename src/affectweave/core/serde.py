from __future__ import annotations

from datetime import datetime
from typing import Any, Mapping

from .models import (
    ActiveThread,
    AgentTurnOutput,
    BehaviorRecord,
    BehaviorStatus,
    CharacterAggregate,
    CharacterState,
    DecisionDirective,
    EmotionDelta,
    EmotionEpisode,
    EmotionType,
    EpisodicMemory,
    EvidenceLevel,
    InputEvent,
    IntentHypothesis,
    MediumDelta,
    MediumState,
    MemoryCandidate,
    Persona,
    RelationshipDelta,
    RelationshipStance,
    RelationshipState,
    SpeechBudget,
    StateProposal,
    ThreadProposal,
    ThreadStatus,
)


def _dt(value: str) -> datetime:
    return datetime.fromisoformat(value)


def persona_to_dict(value: Persona) -> dict[str, Any]:
    return {
        "id": value.id,
        "name": value.name,
        "identity_summary": value.identity_summary,
        "values": dict(value.values),
        "stable_traits": dict(value.stable_traits),
        "boundaries": list(value.boundaries),
        "preferences": list(value.preferences),
        "contradictions": list(value.contradictions),
        "voice_notes": list(value.voice_notes),
        "version": value.version,
        "frozen": value.frozen,
    }


def persona_from_dict(data: Mapping[str, Any]) -> Persona:
    return Persona(
        id=str(data["id"]),
        name=str(data["name"]),
        identity_summary=str(data["identity_summary"]),
        values=dict(data.get("values", {})),
        stable_traits=dict(data.get("stable_traits", {})),
        boundaries=tuple(data.get("boundaries", ())),
        preferences=tuple(data.get("preferences", ())),
        contradictions=tuple(data.get("contradictions", ())),
        voice_notes=tuple(data.get("voice_notes", ())),
        version=int(data.get("version", 1)),
        frozen=bool(data.get("frozen", True)),
    )


def decision_to_dict(value: DecisionDirective) -> dict[str, Any]:
    return {
        "goal": value.goal,
        "act": value.act,
        "strength": value.strength,
        "relationship_stance": value.relationship_stance.value,
        "speech_budget": {
            "max_sentences": value.speech_budget.max_sentences,
            "max_tokens": value.speech_budget.max_tokens,
            "question_allowed": value.speech_budget.question_allowed,
            "self_disclosure": value.speech_budget.self_disclosure,
        },
        "disclosure_level": value.disclosure_level,
        "leave_space": value.leave_space,
        "reason_codes": list(value.reason_codes),
    }


def decision_from_dict(data: Mapping[str, Any]) -> DecisionDirective:
    budget = data.get("speech_budget", {})
    return DecisionDirective(
        goal=str(data["goal"]),
        act=str(data["act"]),
        strength=float(data.get("strength", 0.5)),
        relationship_stance=RelationshipStance(data.get("relationship_stance", "open")),
        speech_budget=SpeechBudget(
            max_sentences=int(budget.get("max_sentences", 4)),
            max_tokens=int(budget.get("max_tokens", 160)),
            question_allowed=bool(budget.get("question_allowed", True)),
            self_disclosure=float(budget.get("self_disclosure", 0.35)),
        ),
        disclosure_level=float(data.get("disclosure_level", 0.35)),
        leave_space=bool(data.get("leave_space", False)),
        reason_codes=tuple(data.get("reason_codes", ())),
    )


def memory_candidate_to_dict(value: MemoryCandidate) -> dict[str, Any]:
    return {
        "summary": value.summary,
        "salience": value.salience,
        "confidence": value.confidence,
        "related_entities": list(value.related_entities),
    }


def memory_candidate_from_dict(data: Mapping[str, Any]) -> MemoryCandidate:
    return MemoryCandidate(
        summary=str(data["summary"]),
        salience=float(data.get("salience", 0.5)),
        confidence=float(data.get("confidence", 0.5)),
        related_entities=tuple(data.get("related_entities", ())),
    )


def aggregate_to_dict(value: CharacterAggregate) -> dict[str, Any]:
    state = value.state
    return {
        "state": {
            "character_id": state.character_id,
            "persona": persona_to_dict(state.persona),
            "medium": {
                "valence": state.medium.valence,
                "activation": state.medium.activation,
                "energy": state.medium.energy,
                "stress": state.medium.stress,
                "updated_at": state.medium.updated_at.isoformat(),
                "source_ids": sorted(state.medium.source_ids),
            },
            "emotions": {
                key: {
                    "id": item.id,
                    "type": item.type.value,
                    "cause_id": item.cause_id,
                    "cause_summary": item.cause_summary,
                    "target_id": item.target_id,
                    "felt_intensity": item.felt_intensity,
                    "confidence": item.confidence,
                    "unresolvedness": item.unresolvedness,
                    "source_event_ids": sorted(item.source_event_ids),
                    "started_at": item.started_at.isoformat(),
                    "updated_at": item.updated_at.isoformat(),
                    "active": item.active,
                }
                for key, item in state.emotions.items()
            },
            "relationships": {
                key: {
                    "entity_id": item.entity_id,
                    "familiarity": item.familiarity,
                    "trust": item.trust,
                    "warmth": item.warmth,
                    "reliability": item.reliability,
                    "conflict": item.conflict,
                    "stance": item.stance.value,
                    "updated_at": item.updated_at.isoformat(),
                }
                for key, item in state.relationships.items()
            },
            "active_threads": {
                key: {
                    "id": item.id,
                    "kind": item.kind,
                    "summary": item.summary,
                    "importance": item.importance,
                    "unresolvedness": item.unresolvedness,
                    "related_entities": sorted(item.related_entities),
                    "source_event_ids": sorted(item.source_event_ids),
                    "completion_condition": item.completion_condition,
                    "status": item.status.value,
                    "created_at": item.created_at.isoformat(),
                    "updated_at": item.updated_at.isoformat(),
                }
                for key, item in state.active_threads.items()
            },
            "version": state.version,
            "updated_at": state.updated_at.isoformat(),
            "schema_version": state.schema_version,
        },
        "raw_events": {
            key: {
                "id": item.id,
                "actor_id": item.actor_id,
                "message": item.message,
                "occurred_at": item.occurred_at.isoformat(),
                "source": item.source,
            }
            for key, item in value.raw_events.items()
        },
        "behaviors": {
            key: {
                "id": item.id,
                "turn_id": item.turn_id,
                "source_event_id": item.source_event_id,
                "actor_id": item.actor_id,
                "status": item.status.value,
                "evidence_level": item.evidence_level.value,
                "decision": decision_to_dict(item.decision) if item.decision else None,
                "generated_text": item.generated_text,
                "delivered_text": item.delivered_text,
                "payload_hash": item.payload_hash,
                "pending_memory_candidates": [
                    memory_candidate_to_dict(candidate)
                    for candidate in item.pending_memory_candidates
                ],
                "memory_candidates_committed": item.memory_candidates_committed,
                "delivery_complete": item.delivery_complete,
                "outcome_ids": sorted(item.outcome_ids),
                "created_at": item.created_at.isoformat(),
                "updated_at": item.updated_at.isoformat(),
            }
            for key, item in value.behaviors.items()
        },
        "memories": {
            key: {
                "id": item.id,
                "summary": item.summary,
                "event_ids": list(item.event_ids),
                "related_entities": list(item.related_entities),
                "salience": item.salience,
                "confidence": item.confidence,
                "evidence_level": item.evidence_level.value,
                "created_at": item.created_at.isoformat(),
            }
            for key, item in value.memories.items()
        },
        "idempotency": dict(value.idempotency),
    }


def aggregate_from_dict(data: Mapping[str, Any]) -> CharacterAggregate:
    raw_state = data["state"]
    raw_medium = raw_state["medium"]
    medium = MediumState(
        valence=float(raw_medium.get("valence", 0.0)),
        activation=float(raw_medium.get("activation", 0.3)),
        energy=float(raw_medium.get("energy", 0.6)),
        stress=float(raw_medium.get("stress", 0.2)),
        updated_at=_dt(raw_medium["updated_at"]),
        source_ids=set(raw_medium.get("source_ids", ())),
    )
    emotions = {
        key: EmotionEpisode(
            id=str(item.get("id", key)),
            type=EmotionType(item["type"]),
            cause_id=str(item["cause_id"]),
            cause_summary=str(item["cause_summary"]),
            target_id=item.get("target_id"),
            felt_intensity=float(item.get("felt_intensity", 0.0)),
            confidence=float(item.get("confidence", 0.5)),
            unresolvedness=float(item.get("unresolvedness", 0.0)),
            source_event_ids=set(item.get("source_event_ids", ())),
            started_at=_dt(item["started_at"]),
            updated_at=_dt(item["updated_at"]),
            active=bool(item.get("active", True)),
        )
        for key, item in raw_state.get("emotions", {}).items()
    }
    relationships = {
        key: RelationshipState(
            entity_id=str(item["entity_id"]),
            familiarity=float(item.get("familiarity", 0.05)),
            trust=float(item.get("trust", 0.5)),
            warmth=float(item.get("warmth", 0.5)),
            reliability=float(item.get("reliability", 0.5)),
            conflict=float(item.get("conflict", 0.0)),
            stance=RelationshipStance(item.get("stance", "open")),
            updated_at=_dt(item["updated_at"]),
        )
        for key, item in raw_state.get("relationships", {}).items()
    }
    threads = {
        key: ActiveThread(
            id=str(item.get("id", key)),
            kind=str(item["kind"]),
            summary=str(item["summary"]),
            importance=float(item.get("importance", 0.5)),
            unresolvedness=float(item.get("unresolvedness", 0.5)),
            related_entities=set(item.get("related_entities", ())),
            source_event_ids=set(item.get("source_event_ids", ())),
            completion_condition=str(item.get("completion_condition", "")),
            status=ThreadStatus(item.get("status", "active")),
            created_at=_dt(item["created_at"]),
            updated_at=_dt(item["updated_at"]),
        )
        for key, item in raw_state.get("active_threads", {}).items()
    }
    state = CharacterState(
        character_id=str(raw_state["character_id"]),
        persona=persona_from_dict(raw_state["persona"]),
        medium=medium,
        emotions=emotions,
        relationships=relationships,
        active_threads=threads,
        version=int(raw_state.get("version", 1)),
        updated_at=_dt(raw_state["updated_at"]),
        schema_version=str(raw_state.get("schema_version", "4.0-stateful-prompt")),
    )
    raw_events = {
        key: InputEvent(
            id=str(item.get("id", key)),
            actor_id=str(item["actor_id"]),
            message=str(item["message"]),
            occurred_at=_dt(item["occurred_at"]),
            source=str(item.get("source", "chat")),
        )
        for key, item in data.get("raw_events", {}).items()
    }
    behaviors = {
        key: BehaviorRecord(
            id=str(item.get("id", key)),
            turn_id=str(item["turn_id"]),
            source_event_id=str(item["source_event_id"]),
            actor_id=str(item["actor_id"]),
            status=BehaviorStatus(item.get("status", "pending")),
            evidence_level=EvidenceLevel(item.get("evidence_level", "L0")),
            decision=(
                decision_from_dict(item["decision"])
                if item.get("decision") is not None
                else None
            ),
            generated_text=str(item.get("generated_text", "")),
            delivered_text=str(item.get("delivered_text", "")),
            payload_hash=item.get("payload_hash"),
            pending_memory_candidates=tuple(
                memory_candidate_from_dict(value)
                for value in item.get("pending_memory_candidates", ())
            ),
            memory_candidates_committed=bool(item.get("memory_candidates_committed", False)),
            delivery_complete=bool(item.get("delivery_complete", False)),
            outcome_ids=set(item.get("outcome_ids", ())),
            created_at=_dt(item["created_at"]),
            updated_at=_dt(item["updated_at"]),
        )
        for key, item in data.get("behaviors", {}).items()
    }
    memories = {
        key: EpisodicMemory(
            id=str(item.get("id", key)),
            summary=str(item["summary"]),
            event_ids=tuple(item.get("event_ids", ())),
            related_entities=tuple(item.get("related_entities", ())),
            salience=float(item.get("salience", 0.5)),
            confidence=float(item.get("confidence", 0.5)),
            evidence_level=EvidenceLevel(item.get("evidence_level", "L2")),
            created_at=_dt(item["created_at"]),
        )
        for key, item in data.get("memories", {}).items()
    }
    return CharacterAggregate(
        state=state,
        raw_events=raw_events,
        behaviors=behaviors,
        memories=memories,
        idempotency=dict(data.get("idempotency", {})),
    )
