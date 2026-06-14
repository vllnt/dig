# Issue intake policy (BLOCKING)

Every issue enters through a **structured template**. Free-form / "blank" issues
are off. This is what keeps the tracker usable once the repo is public.

## The mechanism

- `.github/ISSUE_TEMPLATE/config.yml` sets **`blank_issues_enabled: false`** — the
  new-issue chooser offers only the templates below, no "open a blank issue" link.
- `.github/ISSUE_TEMPLATE/bug_report.yml` — bug: what happened, repro, expected vs
  actual, `dig --version`, OS/arch (most fields required).
- `.github/ISSUE_TEMPLATE/feature_request.yml` — proposal: problem, solution,
  alternatives, area.
- Security is **not** an issue: `config.yml` `contact_links` routes it to GitHub
  private advisories (see `SECURITY.md`). Never tell anyone to file a security
  report as a normal issue.

## Keep in sync

| Change | Also update |
|--------|-------------|
| Add/rename a template | This rule + `CONTRIBUTING.md` (it points at `.github/ISSUE_TEMPLATE/`) |
| Change where security reports go | `config.yml` contact link **and** `SECURITY.md` |
| Make the repo public | Confirm `blank_issues_enabled: false` is live before announcing |

## Never

- Re-enable blank issues (`blank_issues_enabled: true`) or delete `config.yml` —
  that silently reopens the free-form path.
- Add a template without required-field validation — the point is structured intake.
- Route security reports into the public issue tracker.
