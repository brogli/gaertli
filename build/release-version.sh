#!/bin/sh
# Prints the release version X.Y.<run>: X.Y is info.version's first two parts
# in config.yml beside this script, <run> the first argument.
set -eu
xy=$(sed -n "s/^  version: '\([0-9]*\.[0-9]*\)\.[0-9]*'$/\1/p" "$(dirname "$0")/config.yml")
[ -n "$xy" ] || { echo "config.yml has no X.Y.Z info.version" >&2; exit 1; }
echo "$xy.$1"
