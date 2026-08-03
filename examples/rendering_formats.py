"""Show the three ContextPackage rendering forms."""

from __future__ import annotations

from datetime import datetime, timezone

from affectweave import AffectWeave
from affectweave.core import (
    InputEvent,
    make_maya_bennett_medium,
    make_maya_bennett_persona,
)


def main() -> None:
    now = datetime.now(timezone.utc)
    affectweave = AffectWeave()
    state = affectweave.create_character(
        "maya",
        persona=make_maya_bennett_persona(),
        medium=make_maya_bennett_medium(now),
        now=now,
    )
    packet = affectweave.prepare_turn(
        "maya",
        InputEvent(
            actor_id="mighon",
            message="I spent the evening with old friends.",
            occurred_at=now,
        ),
        idempotency_key="rendering-example",
        expected_state_version=state.version,
    )

    print("=== Text prompt ===")
    print(packet.to_prompt())
    print("\n=== Context JSON ===")
    print(packet.to_context_json())
    print("\n=== Message blocks ===")
    print(packet.to_messages())
    print("\n=== Budget ===")
    print(packet.context_package.budget.to_dict())


if __name__ == "__main__":
    main()
