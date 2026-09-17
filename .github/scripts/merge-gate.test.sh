#!/usr/bin/env bash
#
# Offline tests for merge-gate.sh.
#
# The gate decides whether to merge somebody's code, and it does so by reading
# the GitHub API. Testing it against a real pull request means finding out
# whether it is right by watching what it does to a real branch, which is the
# wrong time to find out. So `gh` is stubbed, each scenario supplies canned API
# responses, and the assertion is simply MERGE or HOLD.
#
#   .github/scripts/merge-gate.test.sh
#
# Adding a scenario: make a directory under merge-gate-scenarios/ containing an
# `expect` file (MERGE or HOLD) and whichever of pr.json, files.json,
# check-runs.json, reviews.json and comments.json the case needs. Anything
# missing is served as an empty array.
#
# Two of these matter more than the rest and should never be deleted:
# 11-self-check-only and 12-self-check-excluded-from-pending. The gate's own
# check run is necessarily in_progress while it evaluates, so counting it would
# make the gate wait for itself forever — every run reporting "1 check still
# running" and nothing ever merging.

set -uo pipefail

HERE=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
GATE="$HERE/merge-gate.sh"
SCENARIOS="$HERE/merge-gate-scenarios"
WORK=$(mktemp -d)
trap 'rm -rf "$WORK"' EXIT

pass=0
fail=0

for dir in "$SCENARIOS"/*/; do
  name=$(basename "$dir")
  expect=$(cat "$dir/expect")

  mkdir -p "$WORK/bin"
  cat >"$WORK/bin/gh" <<STUB
#!/usr/bin/env bash
FIX="$dir"
if [ "\$1" = "pr" ] && [ "\$2" = "merge" ]; then
  echo "stub: gh pr merge \$*"
  touch "$WORK/merged"
  exit 0
fi
path=""
for a in "\$@"; do case "\$a" in repos/*) path="\$a" ;; esac; done
jqexpr=""; prev=""
for a in "\$@"; do [ "\$prev" = "--jq" ] && jqexpr="\$a"; prev="\$a"; done
file=""
case "\$path" in
  */pulls/*/files)     file="\$FIX/files.json" ;;
  */pulls/*/reviews)   file="\$FIX/reviews.json" ;;
  */issues/*/comments) file="\$FIX/comments.json" ;;
  */check-runs)        file="\$FIX/check-runs.json" ;;
  */commits/*/pulls)   file="\$FIX/commit-pulls.json" ;;
  */pulls/*)           file="\$FIX/pr.json" ;;
esac
if [ -z "\$file" ] || [ ! -f "\$file" ]; then echo "[]"; exit 0; fi
if [ -n "\$jqexpr" ]; then jq -r "\$jqexpr" <"\$file"; else cat "\$file"; fi
STUB
  chmod +x "$WORK/bin/gh"
  rm -f "$WORK/merged"

  out=$(
    export PATH="$WORK/bin:$PATH"
    export REPO="use-plumbline/plumbline"
    export SELF_CHECK="check the merge gates"
    export EVENT="pull_request_target"
    export PR_FROM_PR="42" PR_FROM_ISSUE="" PR_FROM_RUN="" WORKFLOW_RUN_HEAD_SHA=""
    export GH_TOKEN="stub"
    bash "$GATE" 2>&1
  )

  got="HOLD"
  [ -f "$WORK/merged" ] && got="MERGE"
  grep -q '^MERGE — ' <<<"$out" && got="MERGE"

  if [ "$got" = "$expect" ]; then
    printf '  ok    %-48s %s\n' "$name" "$got"
    pass=$((pass + 1))
  else
    printf '  FAIL  %-48s got %s want %s\n' "$name" "$got" "$expect"
    awk '{ print "          | " $0 }' <<<"$out"
    fail=$((fail + 1))
  fi
done

echo
echo "  $pass passed, $fail failed"
[ "$fail" -eq 0 ]
