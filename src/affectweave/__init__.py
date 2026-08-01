"""AffectWeave: persistent character state and prompt enhancement for AI hosts."""

from . import compiler, core
from .integration import CommitResult, DeliveryResult, AffectWeave

__all__ = [
    "CommitResult",
    "DeliveryResult",
    "AffectWeave",
    "compiler",
    "core",
]
