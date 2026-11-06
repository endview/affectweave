"""Minimal host integration example.

The example deliberately does not call any model API. Replace ``call_host_ai``
with the model invocation that already exists in your application.
"""

from __future__ import annotations

from datetime import datetime, timedelta, timezone

from affectweave import AffectWeave
from affectweave.core import (
    EvidenceLevel,
    InputEvent,
    OutcomeEvent,
    RelationshipDelta,
    make_maya_bennett_medium,
    make_maya_bennett_persona,
)


def call_host_ai(prompt_fragment: str, output_schema: dict) -> dict:
    """Stand-in for the host application's existing AI call."""

    assert "AFFECTWEAVE ENHANCEMENT PACKET" in prompt_fragment
    assert output_schema["name"] == "affectweave-character-turn/1"
    return {
        "state_proposal": {
            "intent_hypotheses": [
                {
                    "label": "friendly_greeting",
                    "confidence": 0.91,
                    "evidence": ["The user introduced themselves politely."],
                }
            ],
            "focus": ["greeting", "new acquaintance"],
            "medium_delta": {
                "valence": 0.01,
                "activation": 0.0,
                "energy": 0.0,
                "stress": 0.0,
                "energy_evidence": False,
            },
            "emotion_deltas": [
                {
                    "emotion": "interest",
                    "cause_id": "event:new-acquaintance",
                    "cause_summary": "A new person initiated a friendly conversation.",
                    "target_id": "mighon",
                    "delta": 0.08,
                    "confidence": 0.86,
                    "unresolvedness": 0.08,
                }
            ],
            "relationship_delta": {
                "familiarity": 0.01,
                "trust": 0.0,
                "warmth": 0.01,
                "reliability": 0.0,
                "conflict": 0.0,
                "stance": "friendly",
            },
            "thread_proposals": [],
            "memory_candidates": [],
            "reason_codes": ["FRIENDLY_OPENING"],
        },
        "decision": {
            "goal": "connect",
            "act": "acknowledge",
            "strength": 0.43,
            "relationship_stance": "friendly",
            "speech_budget": {
                "max_sentences": 3,
                "max_tokens": 60,
                "question_allowed": True,
                "self_disclosure": 0.20,
            },
            "disclosure_level": 0.20,
            "leave_space": True,
            "reason_codes": ["LIGHT_OPENING"],
        },
        "reply": "Hey. Nice to meet you. My day's been pretty quiet so far.",
    }


def main() -> None:
    now = datetime.now(timezone.utc)
    affectweave = AffectWeave()
    state = affectweave.create_character(
        "maya",
        persona=make_maya_bennett_persona(),
        medium=make_maya_bennett_medium(now),
        now=now,
    )

    # 1. AffectWeave prepares a provider-neutral packet.
    packet = affectweave.prepare_turn(
        "maya",
        InputEvent(
            actor_id="mighon",
            message="Hello, Maya. Nice to meet you.",
            occurred_at=now,
        ),
        idempotency_key="turn-001:prepare",
        expected_state_version=state.version,
    )

    # 2. The host uses its own AI/model integration.
    raw_output = call_host_ai(packet.to_prompt(), dict(packet.output_schema))

    # 3. AffectWeave validates the returned JSON and applies bounded state proposals.
    committed = affectweave.commit_output(
        "maya",
        packet,
        raw_output,
        idempotency_key="turn-001:commit",
        occurred_at=now,
    )

    # 4. The host actually displays/sends the reply, then records L1 delivery.
    print(committed.behavior.generated_text)
    delivered = affectweave.record_delivery(
        "maya",
        packet.behavior_id,
        committed.behavior.generated_text,
        idempotency_key="turn-001:delivery",
        occurred_at=now + timedelta(seconds=1),
    )

    # 5. A later user response can be recorded as L2/L3 outcome evidence.
    affectweave.record_outcome(
        "maya",
        packet.behavior_id,
        OutcomeEvent(
            outcome_id="turn-001:user-response",
            evidence_level=EvidenceLevel.L2,
            occurred_at=now + timedelta(seconds=5),
            summary="The user continued the conversation normally.",
            relationship_delta=RelationshipDelta(warmth=0.01),
        ),
        idempotency_key="turn-001:outcome",
        expected_state_version=delivered.state_version,
    )


if __name__ == "__main__":
    main()
