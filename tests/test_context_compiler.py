from __future__ import annotations

import json
import unittest
from copy import deepcopy
from dataclasses import replace
from datetime import datetime, timezone

from affectweave.compiler import ContextCompiler, ContextConfig, ContextPackage
from affectweave.core import (
    AffectWeaveCore,
    DialogueTurn,
    InputEvent,
    make_maya_bennett_medium,
    make_maya_bennett_persona,
)


NOW = datetime(2026, 8, 3, 9, 0, tzinfo=timezone.utc)


class ContextCompilerTests(unittest.TestCase):
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
            InputEvent(
                actor_id="user",
                message="I met old friends today.",
                occurred_at=NOW,
                id="event-context",
            ),
            idempotency_key="prepare-context",
            expected_state_version=state.version,
            recent_dialogue=tuple(
                DialogueTurn(role="user" if index % 2 == 0 else "assistant", text=f"turn {index}")
                for index in range(12)
            ),
        )

    def test_compile_returns_host_neutral_context_package(self) -> None:
        package = ContextCompiler().compile(self.preparation())
        self.assertIsInstance(package, ContextPackage)
        self.assertEqual("affectweave-context/1", package.schema_version)
        self.assertEqual("event-context", package.current_event["event_id"])
        self.assertEqual("I met old friends today.", package.current_event["message"])
        self.assertLessEqual(len(package.recent_dialogue), 8)
        self.assertFalse(package.budget.overflow)

    def test_compile_is_deterministic_for_same_preparation(self) -> None:
        preparation = self.preparation()
        compiler = ContextCompiler()
        first = compiler.compile(preparation).to_dict()
        second = compiler.compile(preparation).to_dict()
        self.assertEqual(first, second)

    def test_compiler_does_not_mutate_turn_preparation(self) -> None:
        preparation = self.preparation()
        before = deepcopy(dict(preparation.state_snapshot))
        ContextCompiler(ContextConfig(token_budget=650)).compile(preparation)
        self.assertEqual(before, preparation.state_snapshot)

    def test_small_budget_records_optional_removals(self) -> None:
        preparation = self.preparation()
        package = ContextCompiler(ContextConfig(token_budget=650)).compile(preparation)
        self.assertFalse(package.budget.overflow)
        self.assertTrue(package.budget.removed_sections)
        self.assertEqual(
            len(package.budget.removed_sections),
            len(package.budget.removal_reasons),
        )
        self.assertIn("current_event", package.budget.included_sections)

    def test_active_thread_base_fields_survive_budget_trimming(self) -> None:
        preparation = self.preparation()
        snapshot = deepcopy(dict(preparation.state_snapshot))
        snapshot["active_threads"] = [
            {
                "id": "thread-boundary",
                "kind": "commitment",
                "summary": "A stated boundary must be enforced after another violation.",
                "importance": 0.95,
                "unresolvedness": 0.92,
                "completion_condition": "The user stops or the interaction ends.",
                "related_entities": ["user"],
            }
        ]
        trimmed = ContextCompiler(ContextConfig(token_budget=620)).compile(
            replace(preparation, state_snapshot=snapshot)
        )
        self.assertEqual(1, len(trimmed.active_threads))
        thread = trimmed.active_threads[0]
        self.assertEqual("thread-boundary", thread["id"])
        self.assertIn("summary", thread)
        self.assertIn("importance", thread)
        self.assertIn("unresolvedness", thread)

    def test_context_package_round_trip(self) -> None:
        package = ContextCompiler().compile(self.preparation())
        restored = ContextPackage.from_dict(json.loads(json.dumps(package.to_dict())))
        self.assertEqual(package.to_dict(), restored.to_dict())


if __name__ == "__main__":
    unittest.main()
