"""Integration tests: the SDK drives a REAL `dig serve` against a real temp KB —
no mocks. The dig binary comes from $DIG_BIN (CI builds it) or `dig` on PATH.
"""

from __future__ import annotations

import os
import subprocess
import sys
import tempfile
import time
import unittest
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parents[1] / "src"))

from dig_client import DigClient, DigError  # noqa: E402

DIG = os.environ.get("DIG_BIN", "dig")
PORT = 3986
POLICY = """
[[rule]]
name  = "invoices"
match = { ext = ["pdf"], content_matches = "invoice" }
into  = "finance/invoices"
label = ["finance"]
"""


def _dig(*args: str) -> None:
    result = subprocess.run([DIG, *args], capture_output=True, text=True, check=False)
    if result.returncode != 0:
        raise RuntimeError(f"dig {' '.join(args)} failed: {result.stderr or result.stdout}")


class TestDigClient(unittest.TestCase):
    daemon: subprocess.Popen[bytes]
    kb: str

    @classmethod
    def setUpClass(cls) -> None:
        cls.kb = tempfile.mkdtemp(prefix="dig-py-")
        inbox = Path(cls.kb) / "inbox"
        inbox.mkdir(parents=True, exist_ok=True)
        (inbox / "acme.pdf").write_text("ACME invoice #1007")
        (inbox / "todo.md").write_text("- [ ] things")
        _dig("init", cls.kb)
        (Path(cls.kb) / ".dig" / "policy.toml").write_text(POLICY)
        _dig("--kb", cls.kb, "scan")

        cls.daemon = subprocess.Popen(
            [DIG, "serve", "--addr", f"127.0.0.1:{PORT}"],
            stdout=subprocess.DEVNULL,
            stderr=subprocess.DEVNULL,
        )
        client = DigClient(base_url=f"http://127.0.0.1:{PORT}")
        for _ in range(50):
            try:
                client.health()
                break
            except Exception:  # noqa: BLE001
                time.sleep(0.1)
        else:
            raise RuntimeError("daemon never became healthy")

    @classmethod
    def tearDownClass(cls) -> None:
        cls.daemon.terminate()
        cls.daemon.wait(timeout=5)

    def client(self) -> DigClient:
        return DigClient(base_url=f"http://127.0.0.1:{PORT}")

    def test_health(self) -> None:
        h = self.client().health()
        self.assertEqual(h["status"], "ok")
        self.assertTrue(h["version"])

    def test_find(self) -> None:
        hits = self.client().find("invoice", kb=self.kb)
        self.assertTrue(any(h["Path"].endswith("acme.pdf") for h in hits))

    def test_org_preview_apply_undo(self) -> None:
        moved = lambda: (Path(self.kb) / "finance" / "invoices" / "acme.pdf").exists()  # noqa: E731

        self.client().org(kb=self.kb)  # preview
        self.assertFalse(moved(), "preview must not move files")

        self.client().org(kb=self.kb, apply=True)
        self.assertTrue(moved(), "apply must move the file")

        self.client().undo(kb=self.kb)
        self.assertFalse(moved(), "undo must revert")

    def test_log_and_drift(self) -> None:
        self.assertIsNotNone(self.client().log(kb=self.kb))
        self.assertIsNotNone(self.client().drift(kb=self.kb))

    def test_retain_then_recall(self) -> None:
        fact = "Decision: adopt the new ledger in Q3; Dana owns the migration."
        retained = self.client().retain(fact, kb=self.kb, as_="memory/py.md")
        self.assertIn("Retained memory/py.md", retained["output"])

        pack = self.client().recall("ledger migration Dana", kb=self.kb, budget=400)
        self.assertEqual(pack["budgetTokens"], 400)
        self.assertTrue(pack["manifest"])
        self.assertTrue(
            any("new ledger in Q3" in item["content"] for item in pack["items"]),
            "recall should surface the retained fact",
        )

    def test_error_path(self) -> None:
        with self.assertRaises(DigError) as ctx:
            self.client().find("anything", kb="/no/such/kb")
        self.assertGreaterEqual(ctx.exception.status, 400)


if __name__ == "__main__":
    unittest.main()
