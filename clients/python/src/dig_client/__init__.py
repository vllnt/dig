"""vllnt-dig — a dependency-free Python client for a local dig daemon (`dig serve`).

Drives the same surface as the CLI over HTTP: search, drift, history, export
(read) and organize / reconcile / undo (mutations, preview-by-default).
Local-first: it talks only to the loopback daemon you run.

Example:
    from dig_client import DigClient

    dig = DigClient()  # http://127.0.0.1:3978
    hits = dig.find("invoice acme", mode="hybrid", limit=5)
    dig.org(apply=True)   # commit a reorg (reversible)
    dig.undo()
"""

from __future__ import annotations

import json
import urllib.error
import urllib.parse
import urllib.request
from typing import Any

__all__ = ["DigClient", "DigError"]

DEFAULT_BASE_URL = "http://127.0.0.1:3978"


class DigError(Exception):
    """Raised when the daemon returns a non-2xx response."""

    def __init__(self, message: str, status: int) -> None:
        super().__init__(message)
        self.status = status


class DigClient:
    """Client for a local dig daemon."""

    def __init__(self, base_url: str = DEFAULT_BASE_URL, timeout: float = 120.0) -> None:
        self.base_url = base_url.rstrip("/")
        self.timeout = timeout

    def health(self) -> dict[str, Any]:
        """Liveness + daemon version."""
        return self._request("GET", "/health", {})

    def find(
        self,
        query: str,
        kb: str | None = None,
        mode: str | None = None,
        limit: int | None = None,
    ) -> list[dict[str, Any]]:
        """Search the KB, ranked. ``mode`` is fts (default), vector, or hybrid."""
        return self._request(
            "GET", "/find", {"kb": kb, "query": query, "mode": mode, "limit": limit}
        )

    def drift(self, kb: str | None = None) -> Any:
        """Report how the KB diverges from its policy. Read-only."""
        return self._request("GET", "/drift", {"kb": kb})

    def log(self, kb: str | None = None) -> Any:
        """Browse change history, newest first. Read-only."""
        return self._request("GET", "/log", {"kb": kb})

    def export(
        self, kb: str | None = None, filter: str | None = None, at: str | None = None
    ) -> str:
        """Export a reproducible, provenance-tagged dataset (JSONL text). Read-only."""
        body = self._request("GET", "/export", {"kb": kb, "filter": filter, "at": at})
        if isinstance(body, dict):
            return str(body.get("output", ""))
        return ""

    def org(self, kb: str | None = None, apply: bool = False) -> Any:
        """Apply organization policy. Previews unless ``apply`` is True (reversible)."""
        return self._request("POST", "/org", {"kb": kb, "apply": apply})

    def reconcile(self, kb: str | None = None, apply: bool = False) -> Any:
        """Converge the KB to policy. Previews unless ``apply`` is True (reversible)."""
        return self._request("POST", "/reconcile", {"kb": kb, "apply": apply})

    def undo(self, kb: str | None = None) -> Any:
        """Revert the last changeset."""
        return self._request("POST", "/undo", {"kb": kb})

    def _request(self, method: str, path: str, params: dict[str, Any]) -> Any:
        query = {k: _str(v) for k, v in params.items() if v is not None}
        url = self.base_url + path
        if query:
            url += "?" + urllib.parse.urlencode(query)
        req = urllib.request.Request(url, method=method)  # noqa: S310 (loopback only)
        try:
            with urllib.request.urlopen(req, timeout=self.timeout) as resp:  # noqa: S310
                return _parse(resp.read())
        except urllib.error.HTTPError as exc:
            body = exc.read()
            raise DigError(_error(body) or exc.reason, exc.code) from None


def _str(value: Any) -> str:
    if isinstance(value, bool):
        return "true" if value else "false"
    return str(value)


def _parse(raw: bytes) -> Any:
    if not raw:
        return None
    return json.loads(raw.decode("utf-8"))


def _error(raw: bytes) -> str | None:
    try:
        body = json.loads(raw.decode("utf-8"))
    except (ValueError, UnicodeDecodeError):
        return None
    if isinstance(body, dict) and "error" in body:
        return str(body["error"])
    return None
