#!/usr/bin/env bash
# Build through Go's cache, then run the selected proposal program.
set -euo pipefail
root=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)
program=${1:?usage: proposal-tool.sh workflow|decision|plan|diff [arguments...]}
shift
case "$program" in
  workflow|decision|plan|diff) ;;
  *) echo "unknown proposal program: $program" >&2; exit 2 ;;
esac
module="$root/go-src/llm-agent/workflows/proposal-$program"
command="proposal-$program"
if [[ "$program" == plan ]]; then
  module="$root/go-src/llm-agent/workflows/proposal-workflow"
  command=plan-workflow
fi
if [[ "$program" == diff ]]; then
  module="$root/go-src/llm-agent/workflows/proposal-decision"
  command=proposal-diff
fi
mkdir -p "$root/.local/bin"
if [[ "$module" == "$root/go-src/llm-agent/workflows/proposal-decision" ]]; then
  # The grammar C sources are gitignored; go generate restores them from their pinned releases.
  (cd "$module" && go generate ./internal/highlight/grammars/...)
fi
(cd "$module" && go build -o "$root/.local/bin/$command" "./cmd/$command")
exec "$root/.local/bin/$command" "$@"
