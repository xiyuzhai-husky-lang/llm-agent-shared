#!/usr/bin/env bash
set -euo pipefail

if [[ $# -ne 1 ]]; then
  printf '用法：build.sh <example-dir>\n' >&2
  exit 2
fi

repo_root=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/../../../../../.." && pwd -P)
example_dir=$(cd -- "$1" && pwd -P)

if python3 "$repo_root/llm-agent/gadgets/code-pieces/build.py" \
  "$example_dir/pieces.json" -o "$example_dir"; then
  printf 'code-pieces 退出码：0\n' >&2
else
  step_exit_code=$?
  printf 'code-pieces 失败，退出码：%s\n' "$step_exit_code" >&2
  exit "$step_exit_code"
fi
