from __future__ import annotations

import tempfile
import unittest
from datetime import datetime, timedelta, timezone
from pathlib import Path

from affectweave.core import (
    AgentTurnOutput,
    BehaviorStatus,
    DecisionDirective,
    DeliveryReceipt,
    EmotionDelta,
    EmotionType,
    EvidenceLevel,
    InputEvent,
    InvalidTransition,
    MediumDelta,
    OutcomeEvent,
    AffectWeaveCore,
    RelationshipDelta,
    RelationshipStance,
    SQLiteRepository,
    SpeechBudget,
    StateProposal,
    make_maya_bennett_medium,
    make_maya_bennett_persona,
)


NOW = datetime(2026, 8, 2, 0, 30, tzinfo=timezone.utc)


def output(reply: str = "I hear you.") -> AgentTurnOutput:
    return AgentTurnOutput(
        proposal=StateProposal(
            medium_delta=MediumDelta(valence=-0.20, stress=0.20),
            emotion_deltas=(
                EmotionDelta(
                    emotion=EmotionType.ANGER,
                    cause_id="cause:test",
                    cause_summary="a boundary was crossed",
                    target_id="user",
                    delta=0.80,
                    confidence=0.90,
                    unresolvedness=0.80,
                ),
            ),
            relationship_delta=RelationshipDelta(
                trust=-0.50,
                conflict=0.70,
                stance=RelationshipStance.GUARDED,
            ),
        ),
        decision=DecisionDirective(
            goal="protect_self",
            act="assert",
            strength=0.80,
            relationship_stance=RelationshipStance.GUARDED,
            speech_budget=SpeechBudget(max_sentences=2, max_tokens=30, question_allowed=False),
            disclosure_level=0.0,
            leave_space=True,
        ),
        reply=reply,
    )


class CoreTests(unittest.TestCase):
    def create(self, core: AffectWeaveCore | None = None):
        core = core or AffectWeaveCore()
        state = core.create_character(
            "maya",
            persona=make_maya_bennett_persona(),
            medium=make_maya_bennett_medium(NOW),
            now=NOW,
        )
        return core, state

    def test_persona_is_frozen_and_state_starts_clean(self) -> None:
        core, state = self.create()
        self.assertTrue(state.persona.frozen)
        self.assertEqual({}, state.emotions)
        self.assertEqual({}, state.relationships)
        self.assertEqual(1, core.get_state("maya").version)

    def test_prepare_turn_is_idempotent(self) -> None:
        core, state = self.create()
        event = InputEvent(actor_id="user", message="hello", occurred_at=NOW, id="evt-1")
        first = core.prepare_turn(
            "maya",
            event,
            idempotency_key="prepare-1",
            expected_state_version=state.version,
        )
        second = core.prepare_turn(
            "maya",
            event,
            idempotency_key="prepare-1",
            expected_state_version=999,
        )
        self.assertEqual(first.behavior_id, second.behavior_id)
        self.assertEqual(2, core.get_state("maya").version)

    def test_commit_clamps_state_deltas(self) -> None:
        core, state = self.create()
        prep = core.prepare_turn(
            "maya",
            InputEvent(actor_id="user", message="test", occurred_at=NOW),
            idempotency_key="p",
            expected_state_version=state.version,
        )
        core.commit_agent_output(
            "maya",
            prep.behavior_id,
            output(),
            idempotency_key="c",
            expected_state_version=prep.state_version,
            occurred_at=NOW,
        )
        current = core.get_state("maya")
        self.assertAlmostEqual(0.07, current.medium.valence)
        self.assertAlmostEqual(0.37, current.medium.stress)
        anger = next(iter(current.emotions.values()))
        self.assertAlmostEqual(0.25, anger.felt_intensity)
        relation = current.relationships["user"]
        self.assertAlmostEqual(0.45, relation.trust)
        self.assertAlmostEqual(0.05, relation.conflict)
        self.assertEqual(RelationshipStance.GUARDED, relation.stance)

    def test_failed_delivery_does_not_become_observed_outcome(self) -> None:
        core, state = self.create()
        prep = core.prepare_turn(
            "maya",
            InputEvent(actor_id="user", message="test", occurred_at=NOW),
            idempotency_key="p",
            expected_state_version=state.version,
        )
        behavior = core.commit_agent_output(
            "maya",
            prep.behavior_id,
            output(),
            idempotency_key="c",
            expected_state_version=prep.state_version,
            occurred_at=NOW,
        )
        behavior = core.record_delivery(
            "maya",
            behavior.id,
            DeliveryReceipt(
                receipt_id="r1",
                delivered_text="",
                delivered=False,
                complete=False,
                occurred_at=NOW + timedelta(seconds=1),
                failure_reason="host failed",
            ),
            idempotency_key="d",
            expected_state_version=core.get_state("maya").version,
        )
        self.assertEqual(BehaviorStatus.FAILED, behavior.status)
        with self.assertRaises(InvalidTransition):
            core.record_outcome(
                "maya",
                behavior.id,
                OutcomeEvent(
                    outcome_id="o1",
                    evidence_level=EvidenceLevel.L2,
                    occurred_at=NOW + timedelta(seconds=2),
                    summary="should not apply",
                ),
                idempotency_key="o",
                expected_state_version=core.get_state("maya").version,
            )

    def test_sqlite_persists_json_state(self) -> None:
        with tempfile.TemporaryDirectory() as temp:
            path = Path(temp) / "affectweave.sqlite3"
            with SQLiteRepository(path) as repository:
                core, state = self.create(AffectWeaveCore(repository))
                core.prepare_turn(
                    "maya",
                    InputEvent(actor_id="user", message="hello", occurred_at=NOW),
                    idempotency_key="p",
                    expected_state_version=state.version,
                )
            with SQLiteRepository(path) as repository:
                restored = AffectWeaveCore(repository).get_state("maya")
                self.assertEqual(2, restored.version)
                self.assertIn("user", restored.relationships)


if __name__ == "__main__":
    unittest.main()
