#!/usr/bin/env bash
# Fetches every grammar `manifest.json` pins into the gitignored `.local/diff-advanced/grammars/<name>/` at the
# repository root, at its pinned commit, and runs the pinned `tree-sitter generate` for a grammar whose repository
# ships no parser. The Go packages beside this script compile against what it leaves there. A grammar already fetched at
# the pinned commit and CLI version is left as it is.
set -euo pipefail
grammars_root=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd -P)
repo_root=$(cd -- "$grammars_root/../../../../.." && pwd -P)
target_root=$repo_root/.local/diff-advanced/grammars
mkdir -p "$target_root"
python3 - "$grammars_root/manifest.json" <<'EOF' |
import json, sys
manifest = json.load(open(sys.argv[1]))
for grammar in manifest["grammars"]:
    print(grammar["name"], grammar["repository"], grammar["commit"], grammar["directory"],
          "generate" if grammar["generate"] else "fetch", manifest["tree_sitter_cli"])
EOF
while read -r name repository commit directory mode cli; do
  checkout=$target_root/$name
  stamp="$repository $commit $directory $mode $cli"
  if [[ -f $checkout.stamp && $(cat "$checkout.stamp") == "$stamp" ]]; then
    continue
  fi
  rm -rf "$checkout" "$checkout.stamp"
  git init -q "$checkout"
  git -C "$checkout" fetch -q --depth 1 "$repository" "$commit"
  git -C "$checkout" checkout -q FETCH_HEAD
  if [[ $(git -C "$checkout" rev-parse HEAD) != "$commit" ]]; then
    echo "$name: fetched $(git -C "$checkout" rev-parse HEAD), expected $commit" >&2
    exit 1
  fi
  if [[ $mode == generate ]]; then
    (cd -- "$checkout/$directory" && npx -y "tree-sitter-cli@$cli" generate src/grammar.json)
  fi
  for file in src/parser.c src/tree_sitter/parser.h; do
    if [[ ! -f $checkout/$directory/$file ]]; then
      echo "$name: $directory/$file is missing after the fetch" >&2
      exit 1
    fi
  done
  echo "$stamp" > "$checkout.stamp"
done
