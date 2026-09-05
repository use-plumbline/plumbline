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
# finding count is not a failure. What *is* a failure is being unable to fetch a
# pinned revision or unable to lint it, because then "these numbers are
# reproducible" has quietly stopped being true.
#
#   ./corpus/run.sh            print the per-rule table
#   ./corpus/run.sh --check    also diff it against corpus/baseline.txt
#
# --check is what CI runs. The corpus is pinned by commit, so the counts only
# move when Plumbline moves: a diff means this change altered what Plumbline
# says about 300-odd files of real contracts, and that is worth a human
# deciding on rather than discovering after release.

set -euo pipefail

root=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
manifest="$root/corpus/repos.txt"
baseline="$root/corpus/baseline.txt"
checkouts="$root/corpus/checkouts"
summary="$checkouts/.summary"
binary="$root/bin/plumbline"

check=false
[ "${1:-}" = "--check" ] && check=true

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

  # One retry: a transient network failure here would otherwise read as a
  # corpus regression, which is the one thing this must never be confused with.
  for attempt in 1 2; do
    if git -C "$dir" fetch --quiet --depth 1 origin "$sha"; then
      git -C "$dir" checkout --quiet FETCH_HEAD
      return
    fi
    echo "  $name: fetch attempt $attempt failed" >&2
    sleep 3
  done

  echo "corpus: cannot fetch $sha from $url" >&2
  echo "corpus: the pin may have been garbage-collected, or the network is unavailable" >&2
  exit 1
}

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

: >"$summary"
for entry in "${repos[@]}"; do
  read -r name _ _ <<<"$entry"
  out="$checkouts/.$name.findings"

  # --fail-on never: findings are the output here, not a build verdict.
  "$binary" --fail-on never "$checkouts/$name" >"$out"

  files=$(sed -n 's/.* in \([0-9]*\) files\?\.$/\1/p' "$out" | tail -1)
  printf '%s\tfiles\t%s\n' "$name" "${files:-0}" >>"$summary"
  for rid in $rules; do
    printf '%s\t%s\t%s\n' "$name" "$rid" "$(grep -c "\[$rid\]" "$out" || true)" >>"$summary"
  done
done

echo
echo "Plumbline $("$binary" --version | awk '{print $2}') over the pinned corpus"
echo
for entry in "${repos[@]}"; do
  read -r name _ sha <<<"$entry"
  echo "$name @ ${sha:0:7}"
  awk -F'\t' -v r="$name" '$1 == r { printf "  %-22s %5s\n", $2, $3 }' "$summary"
  echo
done

echo "Full findings: $checkouts/.<repo>.findings"

$check || exit 0

echo
if [ ! -f "$baseline" ]; then
  echo "corpus: no baseline at $baseline; write one with:" >&2
  echo "  cp $summary $baseline" >&2
  exit 1
fi

if diff -u "$baseline" "$summary" >/tmp/corpus-diff 2>&1; then
  echo "corpus: matches corpus/baseline.txt"
  exit 0
fi

cat >&2 <<MSG

corpus: the pinned corpus reports different numbers than corpus/baseline.txt.

$(cat /tmp/corpus-diff)

The corpus is pinned by commit, so this did not change on its own — something
in this branch changed what Plumbline says about real contracts.

If that was the point of the change, that is fine, and updating the baseline is
part of it:

    make corpus && cp corpus/checkouts/.summary corpus/baseline.txt

Then say in the pull request which rule moved and why, and update the per-rule
table in docs/corpus-run.md so the published numbers still match the tool. A
new rule that adds hundreds of findings across the corpus is exactly the thing
this check exists to put in front of a human.
MSG
exit 1
