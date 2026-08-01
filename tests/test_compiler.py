from __future__ import annotations

import unittest
from datetime import datetime, timezone

from affectweave.compiler import (
    EnhancementPacket,
    OutputValidationError,
    OutputValidator,
    PromptCompiler,
)
from affectweave.core import (
    DialogueTurn,
    InputEvent,
    AffectWeaveCore,
    make_maya_bennett_medium,
    make_maya_bennett_persona,
)


NOW = datetime(2026, 8, 2, 0, 30, tzinfo=timezone.utc)


def valid_raw(reply: str = "Yeah. I get that.") -> dict:
    return {
        "state_proposal": {
            "intent_hypotheses": [
                {"label": "casual_sharing", "confidence": 0.8, "evidence": ["user shared an event"]}
            ],
            "focus": ["shared experience"],
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
                    "cause_id": "event:sharing",
                    "cause_summary": "the user shared a personal experience",
                    "target_id": "user",
                    "delta": 0.08,
                    "confidence": 0.8,
                    "unresolvedness": 0.1,
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
            "reason_codes": ["CASUAL_SHARING"],
        },
        "decision": {
            "goal": "connect",
            "act": "acknowledge",
            "strength": 0.45,
            "relationship_stance": "friendly",
            "speech_budget": {
                "max_sentences": 3,
                "max_tokens": 60,
                "question_allowed": False,
                "self_disclosure": 0.25,
            },
            "disclosure_level": 0.25,
            "leave_space": True,
            "reason_codes": ["DO_NOT_INTERVIEW"],
        },
        "reply": reply,
    }


class CompilerTests(unittest.TestCase):
    def preparation(self):
        core = AffectWeaveCore()
        state = core.create_character(
            "maya",
            persona=make_maya_bennett_persona(),
            medium=make_maya_bennett_medium(NOW),
            now=NOW,
        )
        return core.prepare_turn(
            "maya",
            InputEvent(actor_id="user", message="I met old friends today.", occurred_at=NOW),
            idempotency_key="p",
            expected_state_version=state.version,
            recent_dialogue=(
                DialogueTurn(role="user", text="hello"),
                DialogueTurn(role="assistant", text="Hey."),
            ),
        )

    def test_packet_is_provider_neutral_and_round_trips(self) -> None:
        packet = PromptCompiler().compile(self.preparation())
        self.assertIn("AFFECTWEAVE ENHANCEMENT PACKET", packet.prompt_fragment)
        self.assertNotIn("OpenAI", packet.prompt_fragment)
        self.assertNotIn("Anthropic", packet.prompt_fragment)
        self.assertFalse(packet.overflow)
        restored = EnhancementPacket.from_json(packet.to_json())
        self.assertEqual(packet.behavior_id, restored.behavior_id)
        self.assertEqual(packet.prompt_fragment, restored.prompt_fragment)
        self.assertEqual("affectweave-character-turn/1", restored.output_schema["name"])

    def test_small_budget_trims_optional_context(self) -> None:
        packet = PromptCompiler().compile(self.preparation(), token_budget=700)
        self.assertLessEqual(packet.estimated_tokens, 700)
        self.assertFalse(packet.overflow)

    def test_validator_accepts_valid_output(self) -> None:
        result = OutputValidator().parse(valid_raw())
        self.assertEqual("connect", result.output.decision.goal)
        self.assertEqual("Yeah. I get that.", result.output.reply)

    def test_validator_rejects_protected_fields_recursively(self) -> None:
        raw = valid_raw()
        raw["state_proposal"]["nested"] = {"persona": {"name": "changed"}}
        with self.assertRaises(OutputValidationError):
            OutputValidator().parse(raw)

    def test_validator_enforces_speech_budget(self) -> None:
        raw = valid_raw("One. Two. Three. Four.")
        with self.assertRaises(OutputValidationError):
            OutputValidator().parse(raw)

    def test_validator_rejects_question_when_disallowed(self) -> None:
        raw = valid_raw("Really?")
        with self.assertRaises(OutputValidationError):
            OutputValidator().parse(raw)


if __name__ == "__main__":
    unittest.main()
