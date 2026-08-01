from __future__ import annotations

from dataclasses import replace
from datetime import datetime
from math import exp
from typing import Iterable, Sequence

from .models import (
    ActiveThread,
    AgentTurnOutput,
    BehaviorRecord,
    BehaviorStatus,
    CharacterAggregate,
    CharacterState,
    DeliveryReceipt,
    EmotionDelta,
    EmotionEpisode,
    EpisodicMemory,
    EVIDENCE_RANK,
    EvidenceLevel,
    InputEvent,
    MediumState,
    OutcomeEvent,
    RelationshipDelta,
    RelationshipStance,
    StateProposal,
    ThreadStatus,
    TurnPreparation,
    clamp,
    clamp_signed,
    new_id,
    require_aware,
    text_hash,
    utc_now,
)
from .policy import ALLOWED_ACTS, ALLOWED_GOALS, StatePolicy
from .repository import InMemoryRepository, StateRepository


class CoreError(RuntimeError):
    pass


class InvalidTransition(CoreError):
    pass


class BehaviorNotFound(CoreError):
    pass


class EventAlreadyProcessed(CoreError):
    pass


class StateVersionMismatch(CoreError):
    pass


class ProposalRejected(CoreError):
    pass


class AffectWeaveCore:
    """Deterministic state engine for persistent affect and character continuity."""

    def __init__(
        self,
        repository: StateRepository | None = None,
        *,
        policy: StatePolicy | None = None,
    ) -> None:
        self.repository = repository or InMemoryRepository()
        self.policy = policy or StatePolicy()

    def create_character(
        self,
        character_id: str,
        *,
        persona,
        medium: MediumState | None = None,
        now: datetime | None = None,
    ) -> CharacterState:
        timestamp = now or utc_now()
        require_aware(timestamp, "create_character.now")
        state = CharacterState(
            character_id=character_id,
            persona=persona,
            medium=medium or MediumState(updated_at=timestamp),
            updated_at=timestamp,
        )
        return self.repository.create(CharacterAggregate(state=state)).state

    def get_state(self, character_id: str) -> CharacterState:
        return self.repository.load(character_id).state

    def get_behavior(self, character_id: str, behavior_id: str) -> BehaviorRecord:
        aggregate = self.repository.load(character_id)
        try:
            return aggregate.behaviors[behavior_id]
        except KeyError as exc:
            raise BehaviorNotFound(behavior_id) from exc

    def list_memories(self, character_id: str) -> tuple[EpisodicMemory, ...]:
        aggregate = self.repository.load(character_id)
        return tuple(
            sorted(
                aggregate.memories.values(),
                key=lambda item: (item.created_at, item.id),
            )
        )

    def prepare_turn(
        self,
        character_id: str,
        event: InputEvent,
        *,
        idempotency_key: str,
        expected_state_version: int,
        recent_dialogue: Sequence = (),
    ) -> TurnPreparation:
        aggregate = self.repository.load(character_id)
        idem_key = f"prepare:{idempotency_key}"
        if idem_key in aggregate.idempotency:
            behavior = self._behavior(aggregate, aggregate.idempotency[idem_key])
            return self._preparation_from_aggregate(
                aggregate,
                behavior,
                event=aggregate.raw_events[behavior.source_event_id],
                recent_dialogue=tuple(recent_dialogue),
            )
        self._require_version(aggregate.state, expected_state_version)
        if event.id in aggregate.raw_events:
            raise EventAlreadyProcessed(event.id)

        before = aggregate.state.version
        aggregate.raw_events[event.id] = event
        relationship = aggregate.state.relationship_for(event.actor_id, event.occurred_at)
        relationship.updated_at = max(relationship.updated_at, event.occurred_at)
        behavior = BehaviorRecord(
            turn_id=new_id("turn"),
            source_event_id=event.id,
            actor_id=event.actor_id,
            created_at=event.occurred_at,
            updated_at=event.occurred_at,
        )
        aggregate.behaviors[behavior.id] = behavior
        aggregate.idempotency[idem_key] = behavior.id
        aggregate.state.version += 1
        aggregate.state.updated_at = max(aggregate.state.updated_at, event.occurred_at)
        saved = self.repository.save(aggregate, expected_version=before)
        return self._preparation_from_aggregate(
            saved,
            saved.behaviors[behavior.id],
            event=event,
            recent_dialogue=tuple(recent_dialogue),
        )

    def commit_agent_output(
        self,
        character_id: str,
        behavior_id: str,
        output: AgentTurnOutput,
        *,
        idempotency_key: str,
        expected_state_version: int,
        occurred_at: datetime | None = None,
    ) -> BehaviorRecord:
        timestamp = occurred_at or utc_now()
        require_aware(timestamp, "commit_agent_output.occurred_at")
        aggregate = self.repository.load(character_id)
        idem_key = f"agent-output:{idempotency_key}"
        if idem_key in aggregate.idempotency:
            return self._behavior(aggregate, aggregate.idempotency[idem_key])
        self._require_version(aggregate.state, expected_state_version)
        behavior = self._behavior(aggregate, behavior_id)
        if behavior.status != BehaviorStatus.PENDING or behavior.generated_text:
            raise InvalidTransition("agent output can only be committed once to a pending behavior")
        if output.decision.goal not in ALLOWED_GOALS:
            raise ProposalRejected(f"unsupported goal: {output.decision.goal}")
        if output.decision.act not in ALLOWED_ACTS:
            raise ProposalRejected(f"unsupported act: {output.decision.act}")

        before = aggregate.state.version
        source_event = aggregate.raw_events[behavior.source_event_id]
        self._apply_state_proposal(
            aggregate.state,
            output.proposal,
            actor_id=behavior.actor_id,
            event_id=source_event.id,
            now=timestamp,
        )
        relationship = aggregate.state.relationship_for(behavior.actor_id, timestamp)
        relationship.stance = output.decision.relationship_stance
        relationship.updated_at = timestamp
        behavior.decision = output.decision
        behavior.generated_text = output.reply.strip()
        behavior.payload_hash = text_hash(behavior.generated_text)
        behavior.pending_memory_candidates = output.proposal.memory_candidates
        behavior.updated_at = timestamp
        aggregate.idempotency[idem_key] = behavior.id
        aggregate.state.version += 1
        aggregate.state.updated_at = max(aggregate.state.updated_at, timestamp)
        saved = self.repository.save(aggregate, expected_version=before)
        return saved.behaviors[behavior.id]

    def record_delivery(
        self,
        character_id: str,
        behavior_id: str,
        receipt: DeliveryReceipt,
        *,
        idempotency_key: str,
        expected_state_version: int,
    ) -> BehaviorRecord:
        aggregate = self.repository.load(character_id)
        idem_key = f"delivery:{idempotency_key}"
        if idem_key in aggregate.idempotency:
            return self._behavior(aggregate, aggregate.idempotency[idem_key])
        self._require_version(aggregate.state, expected_state_version)
        behavior = self._behavior(aggregate, behavior_id)
        if behavior.status != BehaviorStatus.PENDING:
            raise InvalidTransition(f"delivery cannot be recorded from {behavior.status}")
        if not behavior.generated_text:
            raise InvalidTransition("delivery cannot precede generation")

        before = aggregate.state.version
        if receipt.delivered and receipt.delivered_text.strip():
            behavior.status = BehaviorStatus.DELIVERED
            behavior.evidence_level = EvidenceLevel.L1
            behavior.delivered_text = receipt.delivered_text.strip()
            behavior.delivery_complete = receipt.complete
        else:
            behavior.status = BehaviorStatus.FAILED
            behavior.evidence_level = EvidenceLevel.L0
            behavior.delivered_text = ""
            behavior.delivery_complete = False
        behavior.updated_at = receipt.occurred_at
        aggregate.idempotency[idem_key] = behavior.id
        aggregate.state.version += 1
        aggregate.state.updated_at = max(aggregate.state.updated_at, receipt.occurred_at)
        saved = self.repository.save(aggregate, expected_version=before)
        return saved.behaviors[behavior.id]

    def record_outcome(
        self,
        character_id: str,
        behavior_id: str,
        outcome: OutcomeEvent,
        *,
        idempotency_key: str,
        expected_state_version: int,
    ) -> BehaviorRecord:
        aggregate = self.repository.load(character_id)
        idem_key = f"outcome:{idempotency_key}"
        if idem_key in aggregate.idempotency:
            return self._behavior(aggregate, aggregate.idempotency[idem_key])
        self._require_version(aggregate.state, expected_state_version)
        behavior = self._behavior(aggregate, behavior_id)
        if behavior.status not in {BehaviorStatus.DELIVERED, BehaviorStatus.OUTCOME_RECORDED}:
            raise InvalidTransition("only delivered behavior can receive an outcome")
        if outcome.outcome_id in behavior.outcome_ids:
            return behavior
        if EVIDENCE_RANK[outcome.evidence_level] <= EVIDENCE_RANK[behavior.evidence_level]:
            raise InvalidTransition("outcome evidence must upgrade the behavior")

        before = aggregate.state.version
        multiplier = 0.5 if outcome.evidence_level == EvidenceLevel.L2 else 1.0
        relationship = aggregate.state.relationship_for(behavior.actor_id, outcome.occurred_at)
        self._apply_relationship_delta(
            relationship,
            outcome.relationship_delta,
            now=outcome.occurred_at,
            multiplier=multiplier,
        )
        for delta in outcome.emotion_deltas:
            self._apply_emotion_delta(
                aggregate.state,
                delta,
                event_id=outcome.outcome_id,
                now=outcome.occurred_at,
                multiplier=multiplier,
            )
        for thread_id in outcome.resolved_thread_ids:
            thread = aggregate.state.active_threads.get(thread_id)
            if thread is None:
                continue
            if outcome.evidence_level == EvidenceLevel.L3:
                thread.status = ThreadStatus.RESOLVED
                thread.unresolvedness = 0.0
            else:
                thread.unresolvedness = clamp(thread.unresolvedness - 0.35)
                if thread.unresolvedness <= 0.05:
                    thread.status = ThreadStatus.RESOLVED
            thread.source_event_ids.add(outcome.outcome_id)
            thread.updated_at = outcome.occurred_at

        if outcome.accept_memory_candidates and not behavior.memory_candidates_committed:
            for candidate in behavior.pending_memory_candidates:
                if candidate.salience < self.policy.memory_salience_threshold:
                    continue
                memory = EpisodicMemory(
                    summary=candidate.summary,
                    event_ids=(behavior.source_event_id, outcome.outcome_id),
                    related_entities=tuple(
                        dict.fromkeys((behavior.actor_id, *candidate.related_entities))
                    ),
                    salience=candidate.salience,
                    confidence=candidate.confidence,
                    evidence_level=outcome.evidence_level,
                    created_at=outcome.occurred_at,
                )
                aggregate.memories[memory.id] = memory
            behavior.memory_candidates_committed = True

        behavior.status = BehaviorStatus.OUTCOME_RECORDED
        behavior.evidence_level = outcome.evidence_level
        behavior.outcome_ids.add(outcome.outcome_id)
        behavior.updated_at = outcome.occurred_at
        aggregate.idempotency[idem_key] = behavior.id
        aggregate.state.version += 1
        aggregate.state.updated_at = max(aggregate.state.updated_at, outcome.occurred_at)
        saved = self.repository.save(aggregate, expected_version=before)
        return saved.behaviors[behavior.id]

    def advance_time(
        self,
        character_id: str,
        now: datetime,
        *,
        idempotency_key: str,
        expected_state_version: int,
    ) -> CharacterState:
        require_aware(now, "advance_time.now")
        aggregate = self.repository.load(character_id)
        idem_key = f"time:{idempotency_key}"
        if idem_key in aggregate.idempotency:
            return aggregate.state
        self._require_version(aggregate.state, expected_state_version)
        before = aggregate.state.version
        for episode in aggregate.state.emotions.values():
            elapsed = max(0.0, (now - episode.updated_at).total_seconds())
            thread = aggregate.state.active_threads.get(episode.cause_id)
            residual = 0.0
            if thread is not None and thread.status == ThreadStatus.ACTIVE:
                residual = clamp(
                    thread.importance
                    * thread.unresolvedness
                    * self.policy.unresolved_residual_rate
                )
            decayed = residual + (episode.felt_intensity - residual) * exp(
                -elapsed / max(self.policy.emotion_decay_seconds, 0.001)
            )
            episode.felt_intensity = clamp(decayed)
            episode.unresolvedness = (
                thread.unresolvedness
                if thread is not None and thread.status == ThreadStatus.ACTIVE
                else 0.0
            )
            episode.active = episode.felt_intensity >= self.policy.emotion_inactive_threshold
            episode.updated_at = now
        aggregate.idempotency[idem_key] = character_id
        aggregate.state.version += 1
        aggregate.state.updated_at = max(aggregate.state.updated_at, now)
        return self.repository.save(aggregate, expected_version=before).state

    def _apply_state_proposal(
        self,
        state: CharacterState,
        proposal: StateProposal,
        *,
        actor_id: str,
        event_id: str,
        now: datetime,
    ) -> None:
        medium = proposal.medium_delta
        state.medium.valence = clamp_signed(
            state.medium.valence
            + clamp(
                medium.valence,
                -self.policy.max_medium_delta,
                self.policy.max_medium_delta,
            )
        )
        state.medium.activation = clamp(
            state.medium.activation
            + clamp(
                medium.activation,
                -self.policy.max_medium_delta,
                self.policy.max_medium_delta,
            )
        )
        state.medium.stress = clamp(
            state.medium.stress
            + clamp(
                medium.stress,
                -self.policy.max_medium_delta,
                self.policy.max_medium_delta,
            )
        )
        if medium.energy_evidence:
            state.medium.energy = clamp(
                state.medium.energy
                + clamp(
                    medium.energy,
                    -self.policy.max_medium_delta,
                    self.policy.max_medium_delta,
                )
            )
        state.medium.source_ids.add(event_id)
        state.medium.updated_at = now

        for delta in proposal.emotion_deltas:
            self._apply_emotion_delta(state, delta, event_id=event_id, now=now)

        relationship = state.relationship_for(actor_id, now)
        self._apply_relationship_delta(
            relationship,
            proposal.relationship_delta,
            now=now,
        )

        for candidate in proposal.thread_proposals:
            thread = state.active_threads.get(candidate.thread_id)
            if candidate.action == "upsert":
                if thread is None:
                    if candidate.confidence < 0.55 or not candidate.summary:
                        continue
                    thread = ActiveThread(
                        id=candidate.thread_id,
                        kind=candidate.kind,
                        summary=candidate.summary,
                        importance=candidate.importance,
                        unresolvedness=clamp(
                            max(0.10, candidate.unresolvedness_delta),
                            0.0,
                            1.0,
                        ),
                        related_entities=set(candidate.related_entities) | {actor_id},
                        source_event_ids={event_id},
                        completion_condition=candidate.completion_condition,
                        created_at=now,
                        updated_at=now,
                    )
                    state.active_threads[thread.id] = thread
                else:
                    thread.unresolvedness = clamp(
                        thread.unresolvedness
                        + clamp(
                            candidate.unresolvedness_delta,
                            -self.policy.max_thread_delta,
                            self.policy.max_thread_delta,
                        )
                    )
                    if candidate.summary:
                        thread.summary = candidate.summary
                    thread.importance = max(thread.importance, candidate.importance)
                    thread.related_entities.update(candidate.related_entities)
                    thread.related_entities.add(actor_id)
                    thread.source_event_ids.add(event_id)
                    thread.updated_at = now
            elif candidate.action == "resolve" and thread is not None:
                if candidate.confidence >= 0.85:
                    thread.status = ThreadStatus.RESOLVED
                    thread.unresolvedness = 0.0
                else:
                    thread.unresolvedness = clamp(
                        thread.unresolvedness - self.policy.max_thread_delta
                    )
                thread.source_event_ids.add(event_id)
                thread.updated_at = now
            elif candidate.action == "abandon" and thread is not None:
                if candidate.confidence >= 0.85:
                    thread.status = ThreadStatus.ABANDONED
                    thread.unresolvedness = 0.0
                thread.source_event_ids.add(event_id)
                thread.updated_at = now

    def _apply_emotion_delta(
        self,
        state: CharacterState,
        delta: EmotionDelta,
        *,
        event_id: str,
        now: datetime,
        multiplier: float = 1.0,
    ) -> None:
        identity = (delta.emotion.value, delta.cause_id, delta.target_id)
        existing = next(
            (item for item in state.emotions.values() if item.identity == identity),
            None,
        )
        bounded = clamp(
            delta.delta * multiplier,
            -self.policy.max_emotion_drop,
            self.policy.max_emotion_rise,
        )
        if existing is None:
            if bounded <= 0.0:
                return
            existing = EmotionEpisode(
                type=delta.emotion,
                cause_id=delta.cause_id,
                cause_summary=delta.cause_summary,
                target_id=delta.target_id,
                felt_intensity=bounded,
                confidence=delta.confidence,
                unresolvedness=delta.unresolvedness,
                source_event_ids={event_id},
                started_at=now,
                updated_at=now,
            )
            state.emotions[existing.id] = existing
            return
        existing.felt_intensity = clamp(existing.felt_intensity + bounded)
        existing.confidence = max(existing.confidence, delta.confidence)
        existing.unresolvedness = delta.unresolvedness
        existing.cause_summary = delta.cause_summary
        existing.source_event_ids.add(event_id)
        existing.updated_at = now
        existing.active = existing.felt_intensity >= self.policy.emotion_inactive_threshold

    def _apply_relationship_delta(
        self,
        relationship,
        delta: RelationshipDelta,
        *,
        now: datetime,
        multiplier: float = 1.0,
    ) -> None:
        maximum = self.policy.max_relationship_delta * multiplier
        for name in ("familiarity", "trust", "warmth", "reliability", "conflict"):
            incoming = getattr(delta, name) * multiplier
            setattr(
                relationship,
                name,
                clamp(getattr(relationship, name) + clamp(incoming, -maximum, maximum)),
            )
        if delta.stance is not None:
            relationship.stance = delta.stance
        relationship.updated_at = now

    def _retrieve_memories(
        self,
        aggregate: CharacterAggregate,
        *,
        actor_id: str,
        now: datetime,
    ) -> tuple[EpisodicMemory, ...]:
        scored: list[tuple[float, EpisodicMemory]] = []
        for memory in aggregate.memories.values():
            age_days = max(0.0, (now - memory.created_at).total_seconds()) / 86_400.0
            recency = exp(-age_days / 14.0)
            entity_match = actor_id in memory.related_entities
            score = (
                1.5 * float(entity_match)
                + 0.6 * memory.salience
                + 0.4 * memory.confidence
                + 0.3 * recency
            )
            scored.append((score, memory))
        scored.sort(key=lambda item: (-item[0], -item[1].created_at.timestamp(), item[1].id))
        return tuple(item for _, item in scored[: self.policy.max_memories])

    def _preparation_from_aggregate(
        self,
        aggregate: CharacterAggregate,
        behavior: BehaviorRecord,
        *,
        event: InputEvent,
        recent_dialogue: tuple,
    ) -> TurnPreparation:
        memories = self._retrieve_memories(
            aggregate,
            actor_id=event.actor_id,
            now=event.occurred_at,
        )
        state = aggregate.state
        relationship = state.relationship_for(event.actor_id, event.occurred_at)
        active_emotions = sorted(
            (item for item in state.emotions.values() if item.active),
            key=lambda item: (-item.felt_intensity, item.id),
        )
        active_threads = sorted(
            (item for item in state.active_threads.values() if item.status == ThreadStatus.ACTIVE),
            key=lambda item: (event.actor_id not in item.related_entities, -(item.importance * item.unresolvedness), item.id),
        )
        snapshot = {
            "character_id": state.character_id,
            "state_version": state.version,
            "persona": {
                "id": state.persona.id,
                "name": state.persona.name,
                "identity_summary": state.persona.identity_summary,
                "values": dict(state.persona.values),
                "stable_traits": dict(state.persona.stable_traits),
                "boundaries": list(state.persona.boundaries),
                "preferences": list(state.persona.preferences),
                "contradictions": list(state.persona.contradictions),
                "voice_notes": list(state.persona.voice_notes),
                "frozen": state.persona.frozen,
            },
            "medium": {
                "valence": state.medium.valence,
                "activation": state.medium.activation,
                "energy": state.medium.energy,
                "stress": state.medium.stress,
            },
            "emotions": [
                {
                    "type": item.type.value,
                    "cause_id": item.cause_id,
                    "cause": item.cause_summary,
                    "target_id": item.target_id,
                    "intensity": item.felt_intensity,
                    "confidence": item.confidence,
                    "unresolvedness": item.unresolvedness,
                }
                for item in active_emotions[:6]
            ],
            "relationship": {
                "entity_id": relationship.entity_id,
                "familiarity": relationship.familiarity,
                "trust": relationship.trust,
                "warmth": relationship.warmth,
                "reliability": relationship.reliability,
                "conflict": relationship.conflict,
                "stance": relationship.stance.value,
            },
            "active_threads": [
                {
                    "id": item.id,
                    "kind": item.kind,
                    "summary": item.summary,
                    "importance": item.importance,
                    "unresolvedness": item.unresolvedness,
                    "completion_condition": item.completion_condition,
                    "related_entities": sorted(item.related_entities),
                }
                for item in active_threads[:6]
            ],
        }
        return TurnPreparation(
            character_id=state.character_id,
            turn_id=behavior.turn_id,
            behavior_id=behavior.id,
            event=event,
            state_version=state.version,
            state_snapshot=snapshot,
            memories=memories,
            recent_dialogue=recent_dialogue,
        )

    @staticmethod
    def _require_version(state: CharacterState, expected: int) -> None:
        if state.version != expected:
            raise StateVersionMismatch(
                f"expected state version {expected}, found {state.version}"
            )

    @staticmethod
    def _behavior(aggregate: CharacterAggregate, behavior_id: str) -> BehaviorRecord:
        try:
            return aggregate.behaviors[behavior_id]
        except KeyError as exc:
            raise BehaviorNotFound(behavior_id) from exc
