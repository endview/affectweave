from __future__ import annotations

from dataclasses import dataclass
from datetime import datetime
from typing import Any, Mapping, Sequence

from .compiler import (
    ContextCompiler,
    ContextRenderer,
    EnhancementPacket,
    OutputValidator,
    PromptCompiler,
    TextPromptRenderer,
)
from .core import (
    AffectWeaveCore,
    BehaviorRecord,
    CharacterState,
    DeliveryReceipt,
    DialogueTurn,
    InputEvent,
    OutcomeEvent,
    StateRepository,
    text_hash,
    utc_now,
)


@dataclass(frozen=True, slots=True)
class CommitResult:
    behavior: BehaviorRecord
    state_version: int
    warnings: tuple[str, ...]


@dataclass(frozen=True, slots=True)
class DeliveryResult:
    behavior: BehaviorRecord
    state_version: int


class AffectWeave:
    """Passive integration facade for any AI host.

    ``prepare_turn()`` compiles a host-neutral context package and selects a
    default rendering. The host invokes its own model, submits the structured
    result through ``commit_output()``, and reports delivery/outcome separately.
    """

    def __init__(
        self,
        repository: StateRepository | None = None,
        *,
        core: AffectWeaveCore | None = None,
        context_compiler: ContextCompiler | None = None,
        renderer: ContextRenderer | None = None,
        validator: OutputValidator | None = None,
        compiler: PromptCompiler | ContextCompiler | None = None,
    ) -> None:
        if core is not None and repository is not None:
            raise ValueError("pass either core or repository, not both")
        if context_compiler is not None and compiler is not None:
            raise ValueError("pass either context_compiler or compiler, not both")

        selected_compiler = context_compiler
        if selected_compiler is None and compiler is not None:
            selected_compiler = (
                compiler.context_compiler
                if isinstance(compiler, PromptCompiler)
                else compiler
            )

        self.core = core or AffectWeaveCore(repository)
        self.context_compiler = selected_compiler or ContextCompiler()
        self.renderer = renderer or TextPromptRenderer()
        self.validator = validator or OutputValidator()

    @property
    def compiler(self) -> ContextCompiler:
        """Compatibility access to the active context compiler."""

        return self.context_compiler

    def create_character(self, character_id: str, *, persona, medium=None, now=None) -> CharacterState:
        return self.core.create_character(
            character_id,
            persona=persona,
            medium=medium,
            now=now,
        )

    def get_state(self, character_id: str) -> CharacterState:
        return self.core.get_state(character_id)

    def prepare_turn(
        self,
        character_id: str,
        event: InputEvent,
        *,
        idempotency_key: str,
        expected_state_version: int | None = None,
        recent_dialogue: Sequence[DialogueTurn] = (),
        token_budget: int | None = None,
        render_format: str | None = None,
    ) -> EnhancementPacket:
        expected = (
            self.core.get_state(character_id).version
            if expected_state_version is None
            else expected_state_version
        )
        preparation = self.core.prepare_turn(
            character_id,
            event,
            idempotency_key=idempotency_key,
            expected_state_version=expected,
            recent_dialogue=recent_dialogue,
        )
        context = self.context_compiler.compile(
            preparation,
            token_budget=token_budget,
        )
        if render_format is None:
            default_rendering = self.renderer.render(context)
            return EnhancementPacket(
                context_package=context,
                default_rendering=default_rendering,
            )
        return EnhancementPacket.from_context(context, render_format=render_format)

    def commit_output(
        self,
        character_id: str,
        packet: EnhancementPacket,
        raw_output: Mapping[str, Any] | str,
        *,
        idempotency_key: str,
        expected_state_version: int | None = None,
        occurred_at: datetime | None = None,
    ) -> CommitResult:
        if packet.character_id != character_id:
            raise ValueError("packet character_id does not match target character")
        if packet.overflow:
            raise ValueError("cannot commit output from an overflowed packet")
        validated = self.validator.parse(raw_output)
        expected = packet.state_version if expected_state_version is None else expected_state_version
        behavior = self.core.commit_agent_output(
            character_id,
            packet.behavior_id,
            validated.output,
            idempotency_key=idempotency_key,
            expected_state_version=expected,
            occurred_at=occurred_at,
        )
        return CommitResult(
            behavior=behavior,
            state_version=self.core.get_state(character_id).version,
            warnings=validated.warnings,
        )

    def record_delivery(
        self,
        character_id: str,
        behavior_id: str,
        delivered_text: str,
        *,
        idempotency_key: str,
        receipt_id: str | None = None,
        delivered: bool = True,
        complete: bool = True,
        failure_reason: str | None = None,
        occurred_at: datetime | None = None,
        expected_state_version: int | None = None,
    ) -> DeliveryResult:
        timestamp = occurred_at or utc_now()
        expected = (
            self.core.get_state(character_id).version
            if expected_state_version is None
            else expected_state_version
        )
        receipt = DeliveryReceipt(
            receipt_id=receipt_id or f"delivery:{behavior_id}:{text_hash(delivered_text)}",
            delivered_text=delivered_text,
            delivered=delivered,
            complete=complete,
            occurred_at=timestamp,
            failure_reason=failure_reason,
        )
        behavior = self.core.record_delivery(
            character_id,
            behavior_id,
            receipt,
            idempotency_key=idempotency_key,
            expected_state_version=expected,
        )
        return DeliveryResult(
            behavior=behavior,
            state_version=self.core.get_state(character_id).version,
        )

    def record_outcome(
        self,
        character_id: str,
        behavior_id: str,
        outcome: OutcomeEvent,
        *,
        idempotency_key: str,
        expected_state_version: int | None = None,
    ) -> DeliveryResult:
        expected = (
            self.core.get_state(character_id).version
            if expected_state_version is None
            else expected_state_version
        )
        behavior = self.core.record_outcome(
            character_id,
            behavior_id,
            outcome,
            idempotency_key=idempotency_key,
            expected_state_version=expected,
        )
        return DeliveryResult(
            behavior=behavior,
            state_version=self.core.get_state(character_id).version,
        )
