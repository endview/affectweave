from __future__ import annotations

import json
import sqlite3
from copy import deepcopy
from pathlib import Path
from threading import RLock
from typing import Protocol

from .models import CharacterAggregate
from .serde import aggregate_from_dict, aggregate_to_dict


class RepositoryError(RuntimeError):
    pass


class CharacterAlreadyExists(RepositoryError):
    pass


class CharacterNotFound(RepositoryError):
    pass


class ConcurrentStateUpdate(RepositoryError):
    pass


class StateRepository(Protocol):
    def create(self, aggregate: CharacterAggregate) -> CharacterAggregate: ...

    def load(self, character_id: str) -> CharacterAggregate: ...

    def save(
        self,
        aggregate: CharacterAggregate,
        *,
        expected_version: int,
    ) -> CharacterAggregate: ...


class InMemoryRepository:
    def __init__(self) -> None:
        self._items: dict[str, CharacterAggregate] = {}
        self._lock = RLock()

    def create(self, aggregate: CharacterAggregate) -> CharacterAggregate:
        with self._lock:
            key = aggregate.state.character_id
            if key in self._items:
                raise CharacterAlreadyExists(key)
            self._items[key] = deepcopy(aggregate)
            return deepcopy(aggregate)

    def load(self, character_id: str) -> CharacterAggregate:
        with self._lock:
            try:
                return deepcopy(self._items[character_id])
            except KeyError as exc:
                raise CharacterNotFound(character_id) from exc

    def save(
        self,
        aggregate: CharacterAggregate,
        *,
        expected_version: int,
    ) -> CharacterAggregate:
        with self._lock:
            key = aggregate.state.character_id
            existing = self._items.get(key)
            if existing is None:
                raise CharacterNotFound(key)
            if existing.state.version != expected_version:
                raise ConcurrentStateUpdate(
                    f"expected version {expected_version}, found {existing.state.version}"
                )
            self._items[key] = deepcopy(aggregate)
            return deepcopy(aggregate)


class SQLiteRepository:
    def __init__(self, path: str | Path) -> None:
        self.path = Path(path)
        self.path.parent.mkdir(parents=True, exist_ok=True)
        self._connection = sqlite3.connect(self.path)
        self._connection.row_factory = sqlite3.Row
        self._lock = RLock()
        self._connection.execute("PRAGMA journal_mode=WAL")
        self._connection.execute(
            """
            CREATE TABLE IF NOT EXISTS affectweave_aggregates (
                character_id TEXT PRIMARY KEY,
                state_version INTEGER NOT NULL,
                schema_version TEXT NOT NULL,
                payload_json TEXT NOT NULL,
                updated_at TEXT NOT NULL
            )
            """
        )
        self._connection.commit()

    def __enter__(self) -> "SQLiteRepository":
        return self

    def __exit__(self, exc_type, exc, tb) -> None:
        self.close()

    def close(self) -> None:
        self._connection.close()

    def create(self, aggregate: CharacterAggregate) -> CharacterAggregate:
        payload = json.dumps(
            aggregate_to_dict(aggregate),
            ensure_ascii=False,
            separators=(",", ":"),
            sort_keys=True,
        )
        with self._lock, self._connection:
            try:
                self._connection.execute(
                    """
                    INSERT INTO affectweave_aggregates
                    (character_id, state_version, schema_version, payload_json, updated_at)
                    VALUES (?, ?, ?, ?, ?)
                    """,
                    (
                        aggregate.state.character_id,
                        aggregate.state.version,
                        aggregate.state.schema_version,
                        payload,
                        aggregate.state.updated_at.isoformat(),
                    ),
                )
            except sqlite3.IntegrityError as exc:
                raise CharacterAlreadyExists(aggregate.state.character_id) from exc
        return self.load(aggregate.state.character_id)

    def load(self, character_id: str) -> CharacterAggregate:
        with self._lock:
            row = self._connection.execute(
                "SELECT payload_json FROM affectweave_aggregates WHERE character_id = ?",
                (character_id,),
            ).fetchone()
        if row is None:
            raise CharacterNotFound(character_id)
        return aggregate_from_dict(json.loads(row["payload_json"]))

    def save(
        self,
        aggregate: CharacterAggregate,
        *,
        expected_version: int,
    ) -> CharacterAggregate:
        payload = json.dumps(
            aggregate_to_dict(aggregate),
            ensure_ascii=False,
            separators=(",", ":"),
            sort_keys=True,
        )
        with self._lock, self._connection:
            cursor = self._connection.execute(
                """
                UPDATE affectweave_aggregates
                SET state_version = ?, schema_version = ?, payload_json = ?, updated_at = ?
                WHERE character_id = ? AND state_version = ?
                """,
                (
                    aggregate.state.version,
                    aggregate.state.schema_version,
                    payload,
                    aggregate.state.updated_at.isoformat(),
                    aggregate.state.character_id,
                    expected_version,
                ),
            )
            if cursor.rowcount != 1:
                exists = self._connection.execute(
                    "SELECT state_version FROM affectweave_aggregates WHERE character_id = ?",
                    (aggregate.state.character_id,),
                ).fetchone()
                if exists is None:
                    raise CharacterNotFound(aggregate.state.character_id)
                raise ConcurrentStateUpdate(
                    f"expected version {expected_version}, found {exists['state_version']}"
                )
        return self.load(aggregate.state.character_id)
