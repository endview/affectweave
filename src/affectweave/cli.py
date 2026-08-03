from __future__ import annotations

import argparse
import json
import sys
from datetime import datetime, timezone
from pathlib import Path
from typing import Any

from .compiler import EnhancementPacket
from .core import (
    EvidenceLevel,
    InputEvent,
    OutcomeEvent,
    RelationshipDelta,
    SQLiteRepository,
    make_maya_bennett_medium,
    make_maya_bennett_persona,
)
from .integration import AffectWeave


def _utc_now() -> datetime:
    return datetime.now(timezone.utc)


def _read_text(path: str | None) -> str:
    if path:
        return Path(path).read_text(encoding="utf-8")
    return sys.stdin.read()


def _state_dict(state) -> dict[str, Any]:
    return {
        "character_id": state.character_id,
        "schema_version": state.schema_version,
        "state_version": state.version,
        "persona": {
            "id": state.persona.id,
            "name": state.persona.name,
            "identity_summary": state.persona.identity_summary,
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
                "id": item.id,
                "type": item.type.value,
                "cause": item.cause_summary,
                "target_id": item.target_id,
                "intensity": item.felt_intensity,
                "unresolvedness": item.unresolvedness,
            }
            for item in state.emotions.values()
            if item.active
        ],
        "relationships": {
            key: {
                "familiarity": value.familiarity,
                "trust": value.trust,
                "warmth": value.warmth,
                "reliability": value.reliability,
                "conflict": value.conflict,
                "stance": value.stance.value,
            }
            for key, value in state.relationships.items()
        },
        "active_threads": [
            {
                "id": item.id,
                "kind": item.kind,
                "summary": item.summary,
                "importance": item.importance,
                "unresolvedness": item.unresolvedness,
                "status": item.status.value,
            }
            for item in state.active_threads.values()
        ],
    }


def _print_json(value: Any) -> None:
    print(json.dumps(value, ensure_ascii=False, indent=2, default=str))


def _integration(db: str) -> tuple[SQLiteRepository, AffectWeave]:
    repository = SQLiteRepository(db)
    return repository, AffectWeave(repository)


def command_init(args: argparse.Namespace) -> int:
    repository, affectweave = _integration(args.db)
    try:
        now = _utc_now()
        state = affectweave.create_character(
            args.character,
            persona=make_maya_bennett_persona(),
            medium=make_maya_bennett_medium(now),
            now=now,
        )
        _print_json(_state_dict(state))
        return 0
    finally:
        repository.close()


def command_prepare(args: argparse.Namespace) -> int:
    repository, affectweave = _integration(args.db)
    try:
        state = affectweave.get_state(args.character)
        packet = affectweave.prepare_turn(
            args.character,
            InputEvent(
                actor_id=args.actor,
                message=args.message,
                occurred_at=_utc_now(),
            ),
            idempotency_key=args.idempotency_key,
            expected_state_version=state.version,
            token_budget=args.token_budget,
        )
        if args.packet_file:
            Path(args.packet_file).write_text(packet.to_json(), encoding="utf-8")

        if args.show_budget:
            print(
                json.dumps(packet.context_package.budget.to_dict(), ensure_ascii=False, indent=2),
                file=sys.stderr,
            )

        if args.format in {"prompt", "text"}:
            rendered = packet.to_prompt(include_schema=args.include_schema)
        elif args.format == "context":
            rendered = packet.to_context_json()
        elif args.format == "messages":
            rendered = json.dumps(packet.to_messages(), ensure_ascii=False, indent=2)
        else:
            rendered = packet.to_json()
        print(rendered)
        return 2 if packet.overflow else 0
    finally:
        repository.close()


def command_commit(args: argparse.Namespace) -> int:
    repository, affectweave = _integration(args.db)
    try:
        packet = EnhancementPacket.from_json(Path(args.packet_file).read_text(encoding="utf-8"))
        raw_output = _read_text(args.output_file)
        result = affectweave.commit_output(
            args.character,
            packet,
            raw_output,
            idempotency_key=args.idempotency_key,
            occurred_at=_utc_now(),
        )
        _print_json(
            {
                "behavior_id": result.behavior.id,
                "state_version": result.state_version,
                "generated_text": result.behavior.generated_text,
                "warnings": list(result.warnings),
                "status": result.behavior.status.value,
                "evidence_level": result.behavior.evidence_level.value,
            }
        )
        return 0
    finally:
        repository.close()


def command_delivery(args: argparse.Namespace) -> int:
    repository, affectweave = _integration(args.db)
    try:
        delivered_text = _read_text(args.text_file) if args.delivered else ""
        result = affectweave.record_delivery(
            args.character,
            args.behavior,
            delivered_text,
            idempotency_key=args.idempotency_key,
            delivered=args.delivered,
            complete=not args.partial,
            failure_reason=args.failure_reason,
            occurred_at=_utc_now(),
        )
        _print_json(
            {
                "behavior_id": result.behavior.id,
                "state_version": result.state_version,
                "status": result.behavior.status.value,
                "evidence_level": result.behavior.evidence_level.value,
                "delivered_text": result.behavior.delivered_text,
            }
        )
        return 0
    finally:
        repository.close()


def command_outcome(args: argparse.Namespace) -> int:
    repository, affectweave = _integration(args.db)
    try:
        result = affectweave.record_outcome(
            args.character,
            args.behavior,
            OutcomeEvent(
                outcome_id=args.outcome_id,
                evidence_level=EvidenceLevel(args.evidence_level),
                occurred_at=_utc_now(),
                summary=args.summary,
                relationship_delta=RelationshipDelta(
                    familiarity=args.familiarity,
                    trust=args.trust,
                    warmth=args.warmth,
                    reliability=args.reliability,
                    conflict=args.conflict,
                ),
                accept_memory_candidates=args.accept_memory_candidates,
            ),
            idempotency_key=args.idempotency_key,
        )
        _print_json(
            {
                "behavior_id": result.behavior.id,
                "state_version": result.state_version,
                "status": result.behavior.status.value,
                "evidence_level": result.behavior.evidence_level.value,
            }
        )
        return 0
    finally:
        repository.close()


def command_state(args: argparse.Namespace) -> int:
    repository, affectweave = _integration(args.db)
    try:
        _print_json(_state_dict(affectweave.get_state(args.character)))
        return 0
    finally:
        repository.close()


def build_parser() -> argparse.ArgumentParser:
    parser = argparse.ArgumentParser(
        prog="affectweave",
        description="Persistent affect and character-continuity toolkit for AI hosts.",
    )
    parser.add_argument("--db", default="runtime/affectweave.sqlite3")
    sub = parser.add_subparsers(dest="command", required=True)

    init = sub.add_parser("init", help="create a character state")
    init.add_argument("--character", default="maya")
    init.set_defaults(func=command_init)

    prepare = sub.add_parser("prepare", help="create an EnhancementPacket")
    prepare.add_argument("--character", default="maya")
    prepare.add_argument("--actor", default="user")
    prepare.add_argument("--message", required=True)
    prepare.add_argument("--idempotency-key", required=True)
    prepare.add_argument("--token-budget", type=int)
    prepare.add_argument("--packet-file")
    prepare.add_argument(
        "--format",
        choices=("json", "packet", "prompt", "text", "context", "messages"),
        default="json",
        help="render the packet, prompt fragment, context JSON, or generic message blocks",
    )
    prepare.add_argument("--include-schema", action="store_true")
    prepare.add_argument("--show-budget", action="store_true")
    prepare.set_defaults(func=command_prepare)

    commit = sub.add_parser("commit", help="validate and commit host AI JSON output")
    commit.add_argument("--character", default="maya")
    commit.add_argument("--packet-file", required=True)
    commit.add_argument("--output-file", help="read output from file; default stdin")
    commit.add_argument("--idempotency-key", required=True)
    commit.set_defaults(func=command_commit)

    delivery = sub.add_parser("delivery", help="record whether the generated reply was delivered")
    delivery.add_argument("--character", default="maya")
    delivery.add_argument("--behavior", required=True)
    delivery.add_argument("--text-file", help="read delivered text from file; default stdin")
    delivery.add_argument("--idempotency-key", required=True)
    delivery.add_argument("--delivered", action=argparse.BooleanOptionalAction, default=True)
    delivery.add_argument("--partial", action="store_true")
    delivery.add_argument("--failure-reason")
    delivery.set_defaults(func=command_delivery)

    outcome = sub.add_parser("outcome", help="record an observed L2/L3 result")
    outcome.add_argument("--character", default="maya")
    outcome.add_argument("--behavior", required=True)
    outcome.add_argument("--outcome-id", required=True)
    outcome.add_argument("--evidence-level", choices=("L2", "L3"), required=True)
    outcome.add_argument("--summary", required=True)
    outcome.add_argument("--idempotency-key", required=True)
    outcome.add_argument("--familiarity", type=float, default=0.0)
    outcome.add_argument("--trust", type=float, default=0.0)
    outcome.add_argument("--warmth", type=float, default=0.0)
    outcome.add_argument("--reliability", type=float, default=0.0)
    outcome.add_argument("--conflict", type=float, default=0.0)
    outcome.add_argument("--accept-memory-candidates", action="store_true")
    outcome.set_defaults(func=command_outcome)

    state = sub.add_parser("state", help="inspect current character state")
    state.add_argument("--character", default="maya")
    state.set_defaults(func=command_state)
    return parser


def main() -> None:
    parser = build_parser()
    args = parser.parse_args()
    raise SystemExit(args.func(args))


if __name__ == "__main__":
    main()
