from __future__ import annotations

import json
import unittest
from copy import deepcopy
from datetime import datetime, timezone

from affectweave.compiler import (
    ContextCompiler,
    EnhancementPacket,
    JSONRenderer,
    MessageBlockRenderer,
    PromptCompiler,
    TextPromptRenderer,
)
from affectweave.core import (
    AffectWeaveCore,
    InputEvent,
    make_maya_bennett_medium,
    make_maya_bennett_persona,
)


NOW = datetime(2026, 8, 3, 9, 10, tzinfo=timezone.utc)


class RendererTests(unittest.TestCase):
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
                message="hello",
                occurred_at=NOW,
                id="event-render",
            ),
            idempotency_key="prepare-render",
            expected_state_version=state.version,
        )

    def context(self):
        return ContextCompiler().compile(self.preparation())

    def test_text_json_and_messages_share_the_same_context(self) -> None:
        context = self.context()
        text = TextPromptRenderer().render(context)
        structured = JSONRenderer().render(context)
        messages = MessageBlockRenderer().render(context)

        self.assertIn("event-render", text.content)
        self.assertEqual("event-render", structured.content["current_event"]["event_id"])
        self.assertEqual("developer", messages.content[0]["role"])
        self.assertEqual(text.content, messages.content[0]["content"])

    def test_renderers_do_not_mutate_context(self) -> None:
        context = self.context()
        before = deepcopy(context.to_dict())
        TextPromptRenderer().render(context)
        JSONRenderer().render(context)
        MessageBlockRenderer().render(context)
        self.assertEqual(before, context.to_dict())

    def test_packet_supports_all_rendering_formats(self) -> None:
        packet = EnhancementPacket.from_context(self.context())
        self.assertIn("AFFECTWEAVE ENHANCEMENT PACKET", packet.to_prompt())
        context_json = json.loads(packet.to_context_json())
        self.assertEqual("affectweave-context/1", context_json["schema_version"])
        self.assertEqual("developer", packet.to_messages()[0]["role"])

    def test_packet_round_trip_preserves_context_and_default_rendering(self) -> None:
        packet = EnhancementPacket.from_context(self.context())
        restored = EnhancementPacket.from_json(packet.to_json())
        self.assertEqual(packet.context_package.to_dict(), restored.context_package.to_dict())
        self.assertEqual(packet.default_rendering.to_dict(), restored.default_rendering.to_dict())
        self.assertEqual(packet.to_prompt(), restored.to_prompt())

    def test_legacy_packet_can_be_loaded(self) -> None:
        legacy = {
            "schema_version": "affectweave-enhancement/1",
            "character_id": "maya",
            "turn_id": "turn-old",
            "behavior_id": "behavior-old",
            "state_version": 3,
            "prompt_fragment": "[AFFECTWEAVE ENHANCEMENT PACKET]\nlegacy",
            "context": {
                "current_message": {
                    "event_id": "event-old",
                    "actor_id": "user",
                    "message": "hello",
                },
                "character_state": {
                    "persona": {"name": "Maya"},
                    "medium": {},
                    "emotions": [],
                    "relationship": {},
                    "active_threads": [],
                },
                "hard_rules": [],
                "task": [],
                "state_change_limits": {},
            },
            "estimated_tokens": 20,
            "token_budget": 1600,
            "overflow": False,
        }
        packet = EnhancementPacket.from_dict(legacy)
        self.assertEqual("event-old", packet.context_package.current_event["event_id"])
        self.assertEqual("legacy", packet.to_prompt().splitlines()[-1])

    def test_prompt_compiler_compatibility_wrapper(self) -> None:
        with self.assertWarns(DeprecationWarning):
            packet = PromptCompiler().compile(self.preparation())
        self.assertEqual("event-render", packet.context_package.current_event["event_id"])


if __name__ == "__main__":
    unittest.main()
