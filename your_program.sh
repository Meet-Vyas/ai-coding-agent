#!/bin/sh

set -e

(
  cd "$(dirname "$0")"
  go build -o /tmp/claude-code-go app/*.go
)

exec /tmp/claude-code-go "$@"
