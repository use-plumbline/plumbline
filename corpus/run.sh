#!/usr/bin/env bash
#
# Run Plumbline over the pinned corpus and report per-rule finding counts.
#
# This is the command docs/corpus-run.md cites. If the numbers in that document
# and the numbers this prints ever disagree, one of them is wrong and the
# document is the one that cannot be trusted — it is prose, this is the tool.
#
# Findings are expected: the corpus contains teaching examples that genuinely
# take no authorization, and reporting them is the rule working. So a non-zero
# finding count is not a failure here. What *is* a failure is being unable to
# fetch a pinned revision or unable to lint it at all, because then the claim
# "these numbers are reproducible" has quietly stopped being true.

set -euo pipefail

root=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
manifest="$root/corpus/repos.txt"
checkouts="$root/corpus/checkouts"
binary="$root/bin/plumbline"

if [ ! -x "$binary" ]; then
  echo "corpus: $binary not found — run 'make build' first" >&2
  exit 1
fi

mkdir -p "$checkouts"

# Fetching one commit rather than cloning a branch keeps the checkout at the
# pinned revision even when the remote's default branch has moved on.
fetch() {
  local name=$1 url=$2 sha=$3 dir="$checkouts/$1"

  if [ -d "$dir/.git" ] && [ "$(git -C "$dir" rev-parse HEAD 2>/dev/null)" = "$sha" ]; then
    echo "  $name: already at $sha"
    return
  fi

  echo "  $name: fetching $sha"
  rm -rf "$dir"
  git init --quiet "$dir"
  git -C "$dir" remote add origin "$url"
  if ! git -C "$dir" fetch --quiet --depth 1 origin "$sha"; then
    echo "corpus: cannot fetch $sha from $url" >&2
    echo "corpus: the pin may have been garbage-collected, or the network is unavailable" >&2
    exit 1
  fi
  git -C "$dir" checkout --quiet FETCH_HEAD
}

# read the manifest, skipping comments and blank lines
repos=()
while read -r name url sha; do
  case "$name" in ''|'#'*) continue ;; esac
  repos+=("$name $url $sha")
done <"$manifest"

echo "Fetching the pinned corpus"
for entry in "${repos[@]}"; do
  # shellcheck disable=SC2086
  fetch $entry
done

rules=$("$binary" --list-rules | awk '{print $1}')

echo
printf '%s\n' "Plumbline $("$binary" --version | awk '{print $2}') over $(date -u +%Y-%m-%d)'s pinned corpus"
echo

total_files=0
for entry in "${repos[@]}"; do
  read -r name _ sha <<<"$entry"
  out="$checkouts/.$name.findings"

  # --fail-on never: findings are the output here, not a build verdict.
  "$binary" --fail-on never "$checkouts/$name" >"$out"

  summary=$(tail -1 "$out")
  files=$(sed -n 's/.* in \([0-9]*\) files\?\.$/\1/p' <<<"$summary")
  total_files=$((total_files + ${files:-0}))

  echo "$name @ ${sha:0:7}"
  for rid in $rules; do
    printf '  %-22s %4d\n' "$rid" "$(grep -c "\[$rid\]" "$out" || true)"
  done
  echo "  ---"
  echo "  $summary"
  echo
done

echo "$total_files files linted across ${#repos[@]} repositories."
echo "Full findings: $checkouts/.<repo>.findings"
