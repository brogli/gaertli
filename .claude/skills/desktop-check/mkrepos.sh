#!/usr/bin/env bash
set -euo pipefail
T=$1; rm -rf "$T"; mkdir -p "$T"
export GIT_AUTHOR_NAME=Ann GIT_AUTHOR_EMAIL=ann@example.com GIT_COMMITTER_NAME=Ann GIT_COMMITTER_EMAIL=ann@example.com
g() { git -C "$T/work" "$@"; }
git init -q --bare -b main "$T/remote.git"
git init -q -b main "$T/work"
g remote add origin "$T/remote.git"; g config core.autocrlf false
seq 1 30 | sed 's/^/line /' > "$T/work/hunks.txt"
printf 'shared\n' > "$T/work/conflict.txt"
printf 'a\r\nb\r\nc\r\n' > "$T/work/crlf.txt"
printf 'no newline at end' > "$T/work/nonl.txt"
mkdir -p "$T/work/docs" "$T/work/src"; echo "# Docs" > "$T/work/docs/readme.md"; echo 'package main' > "$T/work/src/main.go"
g add -A; g commit -q -m "initial: add base files"
printf '\x89PNG\r\n\x1a\n\x00\x00binary' > "$T/work/logo.png"; echo "func a() {}" >> "$T/work/src/main.go"
g add -A; g commit -q -m "add logo and func a"
echo "more docs" >> "$T/work/docs/readme.md"; echo "func b() {}" >> "$T/work/src/main.go"
g add -A; g commit -q -m "docs and func b" -m "Body line one.
Body line two."
g tag v1
g branch B; g branch topic/x
g checkout -q -b feature
echo "feature side" > "$T/work/conflict.txt"; g commit -qam "feature: change conflict.txt"
echo "feature only" > "$T/work/feature.txt"; g add -A; g commit -qm "feature: add feature.txt"
g checkout -q main
echo "main side" > "$T/work/conflict.txt"; g commit -qam "main: change conflict.txt"
g checkout -q -b side
echo "side file" > "$T/work/side.txt"; g add -A; g commit -qm "side: add side.txt"
g checkout -q main
GIT_AUTHOR_NAME=Bob GIT_AUTHOR_EMAIL=bob@example.com g merge -q --no-ff side -m "Merge branch 'side'"
g push -q -u origin main B topic/x feature 2>/dev/null
g branch -q --set-upstream-to=origin/B B
git clone -q "$T/remote.git" "$T/other"
git -C "$T/other" checkout -q B
echo "ok: $(g log --oneline --all | wc -l) commits"
