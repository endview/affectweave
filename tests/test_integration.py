from __future__ import annotations

import json
import subprocess
import sys
import tempfile
import unittest
from datetime import datetime, timedelta, timezone
from pathlib import Path

from affectweave import AffectWeave
from affectweave.compiler import OutputValidationError
from affectweave.core import (
    BehaviorStatus,
    EvidenceLevel,
    InputEvent,
    OutcomeEvent,
    RelationshipDelta,
    SQLiteRepository,
    make_maya_bennett_medium,
    make_maya_bennett_persona,
)


NOW = datetime(2026, 8, 2, 0, 30, tzinfo=timezone.utc)


def raw_output() -> dict:
    return {
        "state_proposal": {
            "intent_hypotheses": [{"label": "greeting", "confidence": 0.9, "evidence": ["hello"]}],
            "focus": ["greeting"],
            "medium_delta": {
                "valence": 0.01,
                "activation": 0.0,
                "energy": 0.0,
                "stress": 0.0,
                "energy_evidence": False,
            },
            "emotion_deltas": [],
            "relationship_delta": {
                "familiarity": 0.01,
                "trust": 0.0,
                "warmth": 0.01,
                "reliability": 0.0,
                "conflict": 0.0,
                "stance": "friendly",
            },
            "thread_proposals": [],
            "memory_candidates": [
                {
                    "summary": "The user opened with a friendly greeting.",
                    "salience": 0.3,
                    "confidence": 0.9,
                    "related_entities": ["user"],
                }
            ],
            "reason_codes": ["GREETING"],
        },
        "decision": {
            "goal": "connect",
            "act": "acknowledge",
            "strength": 0.4,
            "relationship_stance": "friendly",
            "speech_budget": {
                "max_sentences": 2,
                "max_tokens": 40,
                "question_allowed": True,
                "self_disclosure": 0.2,
            },
            "disclosure_level": 0.2,
            "leave_space": True,
            "reason_codes": ["LIGHT_OPENING"],
        },
        "reply": "Hey. Nice to meet you.",
    }


class IntegrationTests(unittest.TestCase):
    def create(self, repository=None):
        affectweave = AffectWeave(repository)
        state = affectweave.create_character(
            "maya",
            persona=make_maya_bennett_persona(),
            medium=make_maya_bennett_medium(NOW),
            now=NOW,
        )
        return affectweave, state

    def test_full_passive_host_flow(self) -> None:
        affectweave, state = self.create()
        packet = affectweave.prepare_turn(
            "maya",
            InputEvent(actor_id="user", message="hello", occurred_at=NOW),
            idempotency_key="prepare",
            expected_state_version=state.version,
        )
        committed = affectweave.commit_output(
            "maya",
            packet,
            raw_output(),
            idempotency_key="commit",
            occurred_at=NOW,
        )
        self.assertEqual(BehaviorStatus.PENDING, committed.behavior.status)
        delivered = affectweave.record_delivery(
            "maya",
            packet.behavior_id,
            committed.behavior.generated_text,
            idempotency_key="delivery",
            occurred_at=NOW + timedelta(seconds=1),
        )
        self.assertEqual(BehaviorStatus.DELIVERED, delivered.behavior.status)
        self.assertEqual(EvidenceLevel.L1, delivered.behavior.evidence_level)
        outcome = affectweave.record_outcome(
            "maya",
            packet.behavior_id,
            OutcomeEvent(
                outcome_id="outcome-1",
                evidence_level=EvidenceLevel.L2,
                occurred_at=NOW + timedelta(seconds=2),
                summary="The user continued normally.",
                relationship_delta=RelationshipDelta(warmth=0.01),
                accept_memory_candidates=True,
            ),
            idempotency_key="outcome",
        )
        self.assertEqual(BehaviorStatus.OUTCOME_RECORDED, outcome.behavior.status)
        self.assertEqual(1, len(affectweave.core.list_memories("maya")))

    def test_integration_never_calls_a_model(self) -> None:
        affectweave, state = self.create()
        packet = affectweave.prepare_turn(
            "maya",
            InputEvent(actor_id="user", message="hello", occurred_at=NOW),
            idempotency_key="prepare",
            expected_state_version=state.version,
        )
        self.assertTrue(packet.prompt_fragment)
        with self.assertRaises(OutputValidationError):
            affectweave.commit_output(
                "maya",
                packet,
                "not json",
                idempotency_key="commit",
            )

    def test_sqlite_flow_survives_process_boundary(self) -> None:
        with tempfile.TemporaryDirectory() as temp:
            db = Path(temp) / "affectweave.sqlite3"
            with SQLiteRepository(db) as repository:
                affectweave, state = self.create(repository)
                packet = affectweave.prepare_turn(
                    "maya",
                    InputEvent(actor_id="user", message="hello", occurred_at=NOW),
                    idempotency_key="prepare",
                    expected_state_version=state.version,
                )
            with SQLiteRepository(db) as repository:
                affectweave = AffectWeave(repository)
                committed = affectweave.commit_output(
                    "maya",
                    packet,
                    json.dumps(raw_output()),
                    idempotency_key="commit",
                )
                self.assertEqual("Hey. Nice to meet you.", committed.behavior.generated_text)

    def test_cli_init_prepare_and_state(self) -> None:
        with tempfile.TemporaryDirectory() as temp:
            db = Path(temp) / "affectweave.sqlite3"
            env = {"PYTHONPATH": str(Path(__file__).parents[1] / "src")}
            init = subprocess.run(
                [sys.executable, "-m", "affectweave.cli", "--db", str(db), "init"],
                text=True,
                capture_output=True,
                env=env,
                check=False,
            )
            self.assertEqual(0, init.returncode, init.stderr)
            prepared = subprocess.run(
                [
                    sys.executable,
                    "-m",
                    "affectweave.cli",
                    "--db",
                    str(db),
                    "prepare",
                    "--message",
                    "hello",
                    "--idempotency-key",
                    "cli-prepare",
                ],
                text=True,
                capture_output=True,
                env=env,
                check=False,
            )
            self.assertEqual(0, prepared.returncode, prepared.stderr)
            data = json.loads(prepared.stdout)
            self.assertEqual("affectweave-enhancement/1", data["schema_version"])


if __name__ == "__main__":
    unittest.main()
