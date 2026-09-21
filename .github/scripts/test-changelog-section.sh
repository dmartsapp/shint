#!/usr/bin/env bash
# Tests for changelog-section.sh:  bash .github/scripts/test-changelog-section.sh
set -uo pipefail
here="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
tmp="$(mktemp -d)"; trap 'rm -rf "$tmp"' EXIT
failures=0
cat > "$tmp/CHANGELOG.md" <<'CL'
# Changelog

Intro.

## v4.2.0 - 2026-09-21

New commands.

- **One.** Text with `code` and "quotes" and a $dollar.
- Two.

## v4.1.0 - 2026-09-21

Folded in.

## v4.0.6 - 2026-09-20

- Old.
CL
check() { if eval "$2"; then echo "PASS  $1"; else echo "FAIL  $1"; sed 's/^/      /' <<<"$out"; failures=$((failures+1)); fi; }

out="$(bash "$here/changelog-section.sh" v4.2.0 "$tmp/CHANGELOG.md")"
check "prints the section without its heading or the next section" '[ "$(head -n1 <<<"$out")" = "New commands." ] && grep -q "^- Two\.$" <<<"$out" && ! grep -q "^## " <<<"$out" && ! grep -q "Folded in" <<<"$out"'
check "keeps backticks, quotes and dollars as they are" 'grep -qF "\`code\` and \"quotes\" and a \$dollar" <<<"$out"'
check "has no blank line at either end" '[ -n "$(head -n1 <<<"$out")" ] && [ -n "$(tail -n1 <<<"$out")" ]'
out="$(bash "$here/changelog-section.sh" v4.0.6 "$tmp/CHANGELOG.md")"
check "the last section works" '[ "$out" = "- Old." ]'
out="$(bash "$here/changelog-section.sh" v9.9.9 "$tmp/CHANGELOG.md")"; rc=$?
check "a missing section prints nothing and succeeds, for the caller to fall back" '[ -z "$out" ] && [ $rc = 0 ]'
out="$(bash "$here/changelog-section.sh" v4.2 "$tmp/CHANGELOG.md" 2>&1)"; rc=$?
check "a bad tag is refused" '[ $rc = 1 ] && grep -q "not vX.Y.Z" <<<"$out"'
out="$(bash "$here/changelog-section.sh" v4.2.0 "$tmp/nope.md" 2>&1)"; rc=$?
check "a missing file is refused" '[ $rc = 1 ] && grep -q "no " <<<"$out"'
out="$(cd "$here/../.." && bash .github/scripts/changelog-section.sh v4.2.0 | head -n1)"
check "works on this repository's own changelog" '[ -n "$out" ]'

echo; if [ "$failures" = 0 ]; then echo "all passed"; else echo "$failures FAILED"; exit 1; fi
