# dig-client

Python client for a local [dig](https://github.com/vllnt/dig) daemon — search,
organize, reconcile, and export a knowledge base over HTTP. Dependency-free
(standard library only), local-first.

## Install

```sh
pip install dig-client
```

Start a daemon next to your KB (dig binary from https://dig.vllnt.com):

```sh
dig serve            # binds 127.0.0.1:3978 (loopback only)
```

## Use

```python
from dig_client import DigClient

dig = DigClient()  # http://127.0.0.1:3978

# search — fts (default), vector, or hybrid (semantic)
hits = dig.find("invoice acme 2024", mode="hybrid", limit=5)

# agent memory — capture, then recall a token-budgeted pack
dig.retain(session_markdown, as_="memory/sessions/today.md")
pack = dig.recall("billing ledger decision", budget=1000)

# reorganize by policy — preview, then apply (reversible)
dig.org(apply=False)   # preview the plan
dig.org(apply=True)    # commit it
dig.undo()             # step back

# reproducible dataset export (JSONL text)
jsonl = dig.export(filter="label:finance")

# read-only inspection
dig.drift()
dig.log()
```

Target a specific KB with `kb="/path/or/name"` on any call; omit it to use the
KB at the daemon's working directory. Errors raise `DigError` carrying the HTTP
status.

The client speaks the same contract as `dig serve`, a thin adapter over the dig
CLI — so it never drifts from the tool.
