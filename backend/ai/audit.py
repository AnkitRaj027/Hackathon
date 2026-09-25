import logging
from datetime import datetime, timezone
from threading import Lock
from typing import Any


logger = logging.getLogger("vault.ai.audit")


class AuditLog:
    def __init__(self) -> None:
        self._events: list[dict[str, Any]] = []
        self._lock = Lock()

    def record(self, **event: Any) -> None:
        entry = {"timestamp": datetime.now(timezone.utc).isoformat(), **event}
        with self._lock:
            self._events.append(entry)
        logger.info("vault_ai_audit %s", entry)

    def list_events(self) -> list[dict[str, Any]]:
        with self._lock:
            return [dict(event) for event in self._events]