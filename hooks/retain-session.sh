#!/usr/bin/env sh
# Capture a finished Claude Code session into a dig knowledge base as memory.
#
# Registered as a SessionEnd hook by this plugin. It is deliberately double
# opt-in and fail-open: it does nothing unless DIG_RETAIN_SESSIONS=1 is set AND
# the session's working directory is inside a dig KB, and it always exits 0 so
# it can never block or break the end of a session.
#
# stdin: the SessionEnd hook payload (JSON) — transcript_path, session_id, cwd.
# effect: dig retain --transcript <path> into memory/sessions/<date>/<id>.md.
set -u

# 1. Opt in explicitly, and only if dig is installed.
[ "${DIG_RETAIN_SESSIONS:-}" = "1" ] || exit 0
command -v dig >/dev/null 2>&1 || exit 0

payload=$(cat)

# Pull a string field out of the (compact) JSON payload. Best-effort; a miss
# yields an empty string and the guards below turn the hook into a no-op.
field() {
	printf '%s' "$payload" | sed -n "s/.*\"$1\"[[:space:]]*:[[:space:]]*\"\([^\"]*\)\".*/\1/p" | head -1
}

transcript=$(field transcript_path)
session=$(field session_id)
cwd=$(field cwd)

[ -n "$transcript" ] && [ -f "$transcript" ] || exit 0

# 2. Only capture when the session ran inside a dig KB (a .dig at or above cwd).
dir="${cwd:-$PWD}"
kb=""
while [ -n "$dir" ]; do
	if [ -d "$dir/.dig" ]; then
		kb="$dir"
		break
	fi
	[ "$dir" = "/" ] && break
	dir=$(dirname "$dir")
done
[ -n "$kb" ] || exit 0

# 3. Retain the rendered transcript at a dated, session-keyed path. A resumed
# session re-retains the fuller transcript to the same path (latest wins).
date_path=$(date -u +%Y/%m/%d 2>/dev/null || echo unknown)
as="memory/sessions/${date_path}/${session:-session}.md"

dig --kb "$kb" retain --transcript "$transcript" --as "$as" >/dev/null 2>&1 || exit 0
exit 0
