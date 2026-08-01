from __future__ import annotations

from dataclasses import dataclass, field
from datetime import datetime, timezone
from enum import StrEnum
from hashlib import sha256
from typing import Any, Mapping
from uuid import uuid4


def utc_now() -> datetime:
    return datetime.now(timezone.utc)


def require_aware(value: datetime, name: str) -> None:
    if value.tzinfo is None or value.utcoffset() is None:
        raise ValueError(f"{name} must be timezone-aware")


def clamp(value: float, low: float = 0.0, high: float = 1.0) -> float:
    return max(low, min(high, float(value)))


def clamp_signed(value: float) -> float:
    return clamp(value, -1.0, 1.0)


def new_id(prefix: str) -> str:
    return f"{prefix}_{uuid4().hex}"


def text_hash(text: str) -> str:
    return sha256(text.encode("utf-8")).hexdigest()


class EmotionType(StrEnum):
    JOY = "joy"
    INTEREST = "interest"
    HOPE = "hope"
    RELIEF = "relief"
    GRATITUDE = "gratitude"
    ANGER = "anger"
    FEAR = "fear"
    SADNESS = "sadness"
    DISAPPOINTMENT = "disappointment"
    GUILT = "guilt"
    SHAME = "shame"
    DISGUST = "disgust"


class RelationshipStance(StrEnum):
    OPEN = "open"
    FRIENDLY = "friendly"
    CAUTIOUS = "cautious"
    GUARDED = "guarded"
    DEFENSIVE = "defensive"
    DISTANT = "distant"
    DISENGAGED = "disengaged"


class ThreadStatus(StrEnum):
    ACTIVE = "active"
    RESOLVED = "resolved"
    ABANDONED = "abandoned"


class BehaviorStatus(StrEnum):
    PENDING = "pending"
    DELIVERED = "delivered"
    FAILED = "failed"
    OUTCOME_RECORDED = "outcome_recorded"
    QUARANTINED = "quarantined"


class EvidenceLevel(StrEnum):
    L0 = "L0"
    L1 = "L1"
    L2 = "L2"
    L3 = "L3"


EVIDENCE_RANK = {
    EvidenceLevel.L0: 0,
    EvidenceLevel.L1: 1,
    EvidenceLevel.L2: 2,
    EvidenceLevel.L3: 3,
}


@dataclass(frozen=True, slots=True)
class Persona:
    id: str
    name: str
    identity_summary: str
    values: Mapping[str, float]
    stable_traits: Mapping[str, float]
    boundaries: tuple[str, ...] = ()
    preferences: tuple[str, ...] = ()
    contradictions: tuple[str, ...] = ()
    voice_notes: tuple[str, ...] = ()
    version: int = 1
    frozen: bool = True

    def __post_init__(self) -> None:
        object.__setattr__(
            self,
            "values",
            {str(key): clamp(value) for key, value in self.values.items()},
        )
        object.__setattr__(
            self,
            "stable_traits",
            {str(key): clamp(value) for key, value in self.stable_traits.items()},
        )
        if not self.id or not self.name or not self.identity_summary:
            raise ValueError("Persona requires id, name, and identity_summary")


@dataclass(slots=True)
class MediumState:
    valence: float = 0.0
    activation: float = 0.30
    energy: float = 0.60
    stress: float = 0.20
    updated_at: datetime = field(default_factory=utc_now)
    source_ids: set[str] = field(default_factory=set)

    def __post_init__(self) -> None:
        require_aware(self.updated_at, "MediumState.updated_at")
        self.valence = clamp_signed(self.valence)
        self.activation = clamp(self.activation)
        self.energy = clamp(self.energy)
        self.stress = clamp(self.stress)


@dataclass(slots=True)
class EmotionEpisode:
    type: EmotionType
    cause_id: str
    cause_summary: str
    target_id: str | None
    felt_intensity: float
    confidence: float
    unresolvedness: float
    source_event_ids: set[str]
    started_at: datetime
    updated_at: datetime
    active: bool = True
    id: str = field(default_factory=lambda: new_id("emotion"))

    def __post_init__(self) -> None:
        require_aware(self.started_at, "EmotionEpisode.started_at")
        require_aware(self.updated_at, "EmotionEpisode.updated_at")
        self.felt_intensity = clamp(self.felt_intensity)
        self.confidence = clamp(self.confidence)
        self.unresolvedness = clamp(self.unresolvedness)
        if not self.cause_id or not self.cause_summary:
            raise ValueError("EmotionEpisode requires cause identity and summary")

    @property
    def identity(self) -> tuple[str, str, str | None]:
        return (self.type.value, self.cause_id, self.target_id)


@dataclass(slots=True)
class RelationshipState:
    entity_id: str
    familiarity: float = 0.05
    trust: float = 0.50
    warmth: float = 0.50
    reliability: float = 0.50
    conflict: float = 0.0
    stance: RelationshipStance = RelationshipStance.OPEN
    updated_at: datetime = field(default_factory=utc_now)

    def __post_init__(self) -> None:
        require_aware(self.updated_at, "RelationshipState.updated_at")
        for name in ("familiarity", "trust", "warmth", "reliability", "conflict"):
            setattr(self, name, clamp(getattr(self, name)))

    @property
    def closeness(self) -> float:
        return clamp((self.familiarity + self.trust + self.warmth) / 3.0)


@dataclass(slots=True)
class ActiveThread:
    kind: str
    summary: str
    importance: float
    unresolvedness: float
    related_entities: set[str]
    source_event_ids: set[str]
    completion_condition: str = ""
    status: ThreadStatus = ThreadStatus.ACTIVE
    created_at: datetime = field(default_factory=utc_now)
    updated_at: datetime = field(default_factory=utc_now)
    id: str = field(default_factory=lambda: new_id("thread"))

    def __post_init__(self) -> None:
        require_aware(self.created_at, "ActiveThread.created_at")
        require_aware(self.updated_at, "ActiveThread.updated_at")
        self.importance = clamp(self.importance)
        self.unresolvedness = clamp(self.unresolvedness)
        if not self.kind or not self.summary:
            raise ValueError("ActiveThread requires kind and summary")


@dataclass(slots=True)
class EpisodicMemory:
    summary: str
    event_ids: tuple[str, ...]
    related_entities: tuple[str, ...]
    salience: float
    confidence: float
    evidence_level: EvidenceLevel
    created_at: datetime
    id: str = field(default_factory=lambda: new_id("memory"))

    def __post_init__(self) -> None:
        require_aware(self.created_at, "EpisodicMemory.created_at")
        self.salience = clamp(self.salience)
        self.confidence = clamp(self.confidence)
        if not self.summary or not self.event_ids:
            raise ValueError("EpisodicMemory requires summary and source events")


@dataclass(frozen=True, slots=True)
class InputEvent:
    actor_id: str
    message: str
    occurred_at: datetime
    source: str = "chat"
    id: str = field(default_factory=lambda: new_id("event"))

    def __post_init__(self) -> None:
        require_aware(self.occurred_at, "InputEvent.occurred_at")
        if not self.actor_id or not self.message.strip():
            raise ValueError("InputEvent requires actor_id and non-empty message")


@dataclass(frozen=True, slots=True)
class DialogueTurn:
    role: str
    text: str


@dataclass(frozen=True, slots=True)
class IntentHypothesis:
    label: str
    confidence: float
    evidence: tuple[str, ...] = ()

    def __post_init__(self) -> None:
        object.__setattr__(self, "confidence", clamp(self.confidence))
        if not self.label:
            raise ValueError("IntentHypothesis.label is required")


@dataclass(frozen=True, slots=True)
class MediumDelta:
    valence: float = 0.0
    activation: float = 0.0
    energy: float = 0.0
    stress: float = 0.0
    energy_evidence: bool = False


@dataclass(frozen=True, slots=True)
class EmotionDelta:
    emotion: EmotionType
    cause_id: str
    cause_summary: str
    target_id: str | None
    delta: float
    confidence: float
    unresolvedness: float

    def __post_init__(self) -> None:
        object.__setattr__(self, "confidence", clamp(self.confidence))
        object.__setattr__(self, "unresolvedness", clamp(self.unresolvedness))
        if not self.cause_id or not self.cause_summary:
            raise ValueError("EmotionDelta requires cause identity and summary")


@dataclass(frozen=True, slots=True)
class RelationshipDelta:
    familiarity: float = 0.0
    trust: float = 0.0
    warmth: float = 0.0
    reliability: float = 0.0
    conflict: float = 0.0
    stance: RelationshipStance | None = None


@dataclass(frozen=True, slots=True)
class ThreadProposal:
    action: str
    thread_id: str
    kind: str = "concern"
    summary: str = ""
    importance: float = 0.5
    unresolvedness_delta: float = 0.0
    completion_condition: str = ""
    related_entities: tuple[str, ...] = ()
    confidence: float = 0.5

    def __post_init__(self) -> None:
        object.__setattr__(self, "importance", clamp(self.importance))
        object.__setattr__(self, "confidence", clamp(self.confidence))
        if self.action not in {"upsert", "resolve", "abandon"}:
            raise ValueError("ThreadProposal.action must be upsert, resolve, or abandon")
        if not self.thread_id:
            raise ValueError("ThreadProposal.thread_id is required")


@dataclass(frozen=True, slots=True)
class MemoryCandidate:
    summary: str
    salience: float
    confidence: float
    related_entities: tuple[str, ...] = ()

    def __post_init__(self) -> None:
        object.__setattr__(self, "salience", clamp(self.salience))
        object.__setattr__(self, "confidence", clamp(self.confidence))
        if not self.summary:
            raise ValueError("MemoryCandidate.summary is required")


@dataclass(frozen=True, slots=True)
class StateProposal:
    intent_hypotheses: tuple[IntentHypothesis, ...] = ()
    focus: tuple[str, ...] = ()
    medium_delta: MediumDelta = field(default_factory=MediumDelta)
    emotion_deltas: tuple[EmotionDelta, ...] = ()
    relationship_delta: RelationshipDelta = field(default_factory=RelationshipDelta)
    thread_proposals: tuple[ThreadProposal, ...] = ()
    memory_candidates: tuple[MemoryCandidate, ...] = ()
    reason_codes: tuple[str, ...] = ()


@dataclass(frozen=True, slots=True)
class SpeechBudget:
    max_sentences: int = 4
    max_tokens: int = 160
    question_allowed: bool = True
    self_disclosure: float = 0.35

    def __post_init__(self) -> None:
        object.__setattr__(self, "max_sentences", max(1, min(int(self.max_sentences), 12)))
        object.__setattr__(self, "max_tokens", max(8, min(int(self.max_tokens), 800)))
        object.__setattr__(self, "self_disclosure", clamp(self.self_disclosure))


@dataclass(frozen=True, slots=True)
class DecisionDirective:
    goal: str
    act: str
    strength: float
    relationship_stance: RelationshipStance
    speech_budget: SpeechBudget
    disclosure_level: float
    leave_space: bool
    reason_codes: tuple[str, ...] = ()

    def __post_init__(self) -> None:
        object.__setattr__(self, "strength", clamp(self.strength))
        object.__setattr__(self, "disclosure_level", clamp(self.disclosure_level))
        if not self.goal or not self.act:
            raise ValueError("DecisionDirective requires goal and act")


@dataclass(frozen=True, slots=True)
class AgentTurnOutput:
    proposal: StateProposal
    decision: DecisionDirective
    reply: str

    def __post_init__(self) -> None:
        if not self.reply.strip():
            raise ValueError("AgentTurnOutput.reply must not be empty")


@dataclass(slots=True)
class BehaviorRecord:
    turn_id: str
    source_event_id: str
    actor_id: str
    status: BehaviorStatus = BehaviorStatus.PENDING
    evidence_level: EvidenceLevel = EvidenceLevel.L0
    decision: DecisionDirective | None = None
    generated_text: str = ""
    delivered_text: str = ""
    payload_hash: str | None = None
    pending_memory_candidates: tuple[MemoryCandidate, ...] = ()
    memory_candidates_committed: bool = False
    delivery_complete: bool = False
    outcome_ids: set[str] = field(default_factory=set)
    created_at: datetime = field(default_factory=utc_now)
    updated_at: datetime = field(default_factory=utc_now)
    id: str = field(default_factory=lambda: new_id("behavior"))

    def __post_init__(self) -> None:
        require_aware(self.created_at, "BehaviorRecord.created_at")
        require_aware(self.updated_at, "BehaviorRecord.updated_at")


@dataclass(slots=True)
class CharacterState:
    character_id: str
    persona: Persona
    medium: MediumState
    emotions: dict[str, EmotionEpisode] = field(default_factory=dict)
    relationships: dict[str, RelationshipState] = field(default_factory=dict)
    active_threads: dict[str, ActiveThread] = field(default_factory=dict)
    version: int = 1
    updated_at: datetime = field(default_factory=utc_now)
    schema_version: str = "4.0-stateful-prompt"

    def __post_init__(self) -> None:
        require_aware(self.updated_at, "CharacterState.updated_at")
        if not self.character_id:
            raise ValueError("CharacterState.character_id is required")

    def relationship_for(self, entity_id: str, now: datetime | None = None) -> RelationshipState:
        if entity_id not in self.relationships:
            self.relationships[entity_id] = RelationshipState(
                entity_id=entity_id,
                updated_at=now or self.updated_at,
            )
        return self.relationships[entity_id]


@dataclass(slots=True)
class CharacterAggregate:
    state: CharacterState
    raw_events: dict[str, InputEvent] = field(default_factory=dict)
    behaviors: dict[str, BehaviorRecord] = field(default_factory=dict)
    memories: dict[str, EpisodicMemory] = field(default_factory=dict)
    idempotency: dict[str, str] = field(default_factory=dict)


@dataclass(frozen=True, slots=True)
class TurnPreparation:
    character_id: str
    turn_id: str
    behavior_id: str
    event: InputEvent
    state_version: int
    state_snapshot: Mapping[str, Any]
    memories: tuple[EpisodicMemory, ...]
    recent_dialogue: tuple[DialogueTurn, ...]


@dataclass(frozen=True, slots=True)
class DeliveryReceipt:
    receipt_id: str
    delivered_text: str
    delivered: bool
    complete: bool
    occurred_at: datetime
    failure_reason: str | None = None

    def __post_init__(self) -> None:
        require_aware(self.occurred_at, "DeliveryReceipt.occurred_at")


@dataclass(frozen=True, slots=True)
class OutcomeEvent:
    outcome_id: str
    evidence_level: EvidenceLevel
    occurred_at: datetime
    summary: str
    relationship_delta: RelationshipDelta = field(default_factory=RelationshipDelta)
    resolved_thread_ids: tuple[str, ...] = ()
    emotion_deltas: tuple[EmotionDelta, ...] = ()
    accept_memory_candidates: bool = False

    def __post_init__(self) -> None:
        require_aware(self.occurred_at, "OutcomeEvent.occurred_at")
        if self.evidence_level not in {EvidenceLevel.L2, EvidenceLevel.L3}:
            raise ValueError("OutcomeEvent requires L2 or L3 evidence")
        if not self.outcome_id or not self.summary:
            raise ValueError("OutcomeEvent requires id and summary")
