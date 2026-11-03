#!/bin/sh
# Prints the release version: info.version in config.yml beside this script.
set -eu
v=$(sed -n "s/^  version: '\([0-9]*\.[0-9]*\.[0-9]*\)'$/\1/p" "$(dirname "$0")/config.yml")
[ -n "$v" ] || { echo "config.yml has no X.Y.Z info.version" >&2; exit 1; }
echo "$v"
