"""Resolve the rebase conflicts that nearly every agent branch hits.

Run from the worktree root while a rebase is stopped. Handles:
- frontend/src/locales/en.json: both sides' keys, merged by key.
- internal/core/errorkeys.go: the union of both sides' map lines.
- tools/trace/pending.txt: both sides dropped; rerun `go run ./tools/trace`
  and add back what it reports as missing.
Prints UNHANDLED <path> and exits 1 for any other conflicted file.
"""

import json
import re
import subprocess
import sys

CONFLICT = re.compile(r"<<<<<<< [^\n]*\n(.*?)=======\n(.*?)>>>>>>> [^\n]*\n", re.S)


def git(*args):
    return subprocess.run(["git", *args], capture_output=True, text=True, check=True).stdout


def merge_json(ours, theirs):
    merged = dict(ours)
    for key, value in theirs.items():
        if isinstance(merged.get(key), dict) and isinstance(value, dict):
            merged[key] = merge_json(merged[key], value)
        else:
            merged.setdefault(key, value)
    return merged


def union_lines(match):
    ours = match.group(1)
    theirs = [line for line in match.group(2).splitlines() if line not in ours.splitlines()]
    return ours + "".join(line + "\n" for line in theirs)


def rewrite(path, transform):
    with open(path) as f:
        text = f.read()
    with open(path, "w") as f:
        f.write(transform(text))


for path in git("diff", "--name-only", "--diff-filter=U").split():
    if path == "frontend/src/locales/en.json":
        merged = merge_json(json.loads(git("show", f":2:{path}")), json.loads(git("show", f":3:{path}")))
        rewrite(path, lambda _: json.dumps(merged, indent=2, ensure_ascii=False) + "\n")
    elif path == "internal/core/errorkeys.go":
        rewrite(path, lambda text: CONFLICT.sub(union_lines, text))
        subprocess.run(["gofmt", "-w", path], check=True)
    elif path == "tools/trace/pending.txt":
        rewrite(path, lambda text: CONFLICT.sub("", text))
    else:
        print("UNHANDLED", path)
        sys.exit(1)
