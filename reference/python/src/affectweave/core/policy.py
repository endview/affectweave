from __future__ import annotations

from dataclasses import dataclass


@dataclass(frozen=True, slots=True)
class StatePolicy:
    max_medium_delta: float = 0.05
    max_emotion_rise: float = 0.25
    max_emotion_drop: float = 0.25
    max_relationship_delta: float = 0.05
    max_thread_delta: float = 0.20
    emotion_inactive_threshold: float = 0.01
    emotion_decay_seconds: float = 3600.0
    unresolved_residual_rate: float = 0.30
    memory_salience_threshold: float = 0.20
    max_memories: int = 5


ALLOWED_GOALS = {
    "connect",
    "understand",
    "share",
    "solve",
    "protect_self",
    "repair",
    "create_distance",
    "end_interaction",
}

ALLOWED_ACTS = {
    "answer",
    "acknowledge",
    "self_disclose",
    "ask_or_clarify",
    "support",
    "repair_or_boundary",
    "assert",
    "disengage",
    "topic_shift",
    "leave_space",
}
