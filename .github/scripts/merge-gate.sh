#!/usr/bin/env bash
#
# The auto-merge gate's decision logic.
#
# Lives in a script rather than inline in auto-merge.yml for two reasons. It is
# 200 lines of bash that decides whether to merge somebody's code, so it wants
# static analysis, which inline YAML does not get. And it is exercised offline
# by merge-gate.test.sh against stubbed GitHub responses, which only works if
# it can be run on its own.
#
# Inputs are environment variables, all set by the workflow:
#   GH_TOKEN, REPO, SELF_CHECK, EVENT,
#   PR_FROM_PR / PR_FROM_ISSUE / PR_FROM_RUN, WORKFLOW_RUN_HEAD_SHA
#
# Exit 0 whether it merges or holds — a hold is a normal outcome, not a
# failure. It exits non-zero only when the merge itself was attempted and
# failed.

set -euo pipefail

# ---------------------------------------------------------------
# Which pull request are we looking at?
# ---------------------------------------------------------------
PR="${PR_FROM_PR:-}"
[ -z "$PR" ] && PR="${PR_FROM_ISSUE:-}"
[ -z "$PR" ] && PR="${PR_FROM_RUN:-}"

# workflow_run does not populate pull_requests for fork branches, so
# fall back to finding the open pull request at that head sha.
if [ -z "$PR" ] && [ "$EVENT" = "workflow_run" ]; then
  head="${WORKFLOW_RUN_HEAD_SHA:-}"
  PR=$(gh api "repos/$REPO/commits/$head/pulls" \
         --jq '[.[] | select(.state == "open")] | first | .number // ""' 2>/dev/null || true)
fi

if [ -z "$PR" ]; then
  echo "no pull request associated with this event; nothing to do"
  exit 0
fi
echo "evaluating pull request #$PR"

pr_json=$(gh api "repos/$REPO/pulls/$PR")
state=$(jq -r '.state' <<<"$pr_json")
draft=$(jq -r '.draft' <<<"$pr_json")
head_sha=$(jq -r '.head.sha' <<<"$pr_json")
body=$(jq -r '.body // ""' <<<"$pr_json")
author=$(jq -r '.user.login' <<<"$pr_json")

if [ "$state" != "open" ]; then echo "not open; nothing to do"; exit 0; fi
if [ "$draft" = "true" ]; then echo "draft; nothing to do"; exit 0; fi

reasons=()

# ---------------------------------------------------------------
# Gate 1 — no maintainer-owned path touched
#
# Blast radius, not gatekeeping. A mistake in one rule is one wrong
# finding. A mistake in the engine, the parser boundary or the config
# schema is silently wrong findings across every rule and every user
# at once. rules/ and testdata/ are deliberately NOT here: a
# self-contained rule with its two fixtures is exactly the change
# this gate exists to land without ceremony.
#
# Kept in sync by hand with CONTRIBUTING.md's "Maintainer-owned
# areas" and with the pull request template's first checkbox.
# ---------------------------------------------------------------
files=$(gh api --paginate "repos/$REPO/pulls/$PR/files" --jq '.[].filename')
protected='^(internal/rule/|internal/engine/|internal/config/|internal/syntax/|action/|action\.yml$|\.github/workflows/|\.golangci\.yml$|Makefile$)'
hits=$(grep -E "$protected" <<<"$files" || true)
if [ -n "$hits" ]; then
  reasons+=("maintainer-owned paths touched: $(tr '\n' ' ' <<<"$hits")")
fi

# ---------------------------------------------------------------
# Gate 2 — no new dependency
#
# Plumbline has four dependencies and each was argued for. Adding one
# is a supply-chain decision for a tool that runs inside other
# people's CI, so it is never automatic.
# ---------------------------------------------------------------
if grep -qE '^go\.(mod|sum)$' <<<"$files"; then
  reasons+=("go.mod or go.sum changed, which needs a human to weigh the dependency")
fi

# ---------------------------------------------------------------
# Gate 3 — the contributor checklist is actually ticked
# ---------------------------------------------------------------
total_boxes=$(grep -cE '^[[:space:]]*- \[[ xX]\]' <<<"$body" || true)
unticked=$(grep -cE '^[[:space:]]*- \[ \]' <<<"$body" || true)
if [ "${total_boxes:-0}" -eq 0 ]; then
  reasons+=("the pull request body has no checklist; it was probably not written against the template")
elif [ "${unticked:-0}" -gt 0 ]; then
  reasons+=("$unticked checklist item(s) not ticked")
fi

# ---------------------------------------------------------------
# Gate 4 — every other check has finished and passed
#
# This job's own check run is excluded. It is necessarily in_progress
# while it evaluates, so counting it would make the gate wait for
# itself forever: every run would report "1 check still running" and
# exit, and nothing would ever merge.
# ---------------------------------------------------------------
all_runs=$(gh api --paginate "repos/$REPO/commits/$head_sha/check-runs" \
   --jq '.check_runs[] | "\(.name)\t\(.status)\t\(.conclusion // "-")"' \
 2>/dev/null || true)
# The exclusion is done here, in plain awk, rather than inside the jq
# filter, so that it stays visible to anyone reading the gate and
# needs no jq --arg plumbing to get a shell variable across.
runs=$(awk -F'\t' -v self="$SELF_CHECK" '$1 != self' <<<"$all_runs" || true)

if [ -z "$runs" ]; then
  reasons+=("no check runs reported yet for $head_sha")
else
  pending=$(awk -F'\t' '$2 != "completed"' <<<"$runs" || true)
  failed=$(awk -F'\t' '$2 == "completed" && $3 != "success" && $3 != "neutral" && $3 != "skipped"' <<<"$runs" || true)
  if [ -n "$pending" ]; then
    reasons+=("$(wc -l <<<"$pending" | tr -d ' ') check(s) still running: $(cut -f1 <<<"$pending" | tr '\n' ' ')")
  fi
  if [ -n "$failed" ]; then
    reasons+=("failing check(s): $(cut -f1 <<<"$failed" | tr '\n' ' ')")
  fi
fi

# ---------------------------------------------------------------
# Gate 5 — no outstanding request for changes
# ---------------------------------------------------------------
changes_requested=$(gh api --paginate "repos/$REPO/pulls/$PR/reviews" \
  --jq '[.[] | select(.state == "CHANGES_REQUESTED" or .state == "APPROVED")]
        | group_by(.user.login) | map(last)
        | map(select(.state == "CHANGES_REQUESTED")) | length' 2>/dev/null || echo 0)
if [ "${changes_requested:-0}" -gt 0 ]; then
  reasons+=("$changes_requested reviewer(s) have changes requested outstanding")
fi

# ---------------------------------------------------------------
# Gate 6 — CodeRabbit's verdict, when there is one
#
# Gate on the verdict, not on the presence of a comment. CodeRabbit
# is not installed on this repository today, so silence means "not
# applicable" and does not hold. But a review that exists and cannot
# be parsed DOES hold: an unreadable verdict is not a clean one.
# ---------------------------------------------------------------
rabbit=$(gh api --paginate "repos/$REPO/pulls/$PR/reviews" \
  --jq '[.[] | select(((.user | type) == "object")
        and ((.user.login // "") | test("coderabbit"; "i")))]
        | last | .body // ""' 2>/dev/null || true)
if [ -z "$rabbit" ]; then
  rabbit=$(gh api --paginate "repos/$REPO/issues/$PR/comments" \
    --jq '[.[] | select(((.user | type) == "object")
and ((.user.login // "") | test("coderabbit"; "i")))]
| last | .body // ""' 2>/dev/null || true)
fi

if [ -z "$rabbit" ]; then
  echo "coderabbit: no review on this pull request; not applicable"
else
  actionable=$(grep -oiE 'actionable comments posted:[[:space:]]*[0-9]+' <<<"$rabbit" \
     | grep -oE '[0-9]+' | head -1 || true)
  if [ -z "$actionable" ] && grep -qi 'no actionable comments' <<<"$rabbit"; then
    actionable=0
  fi
  if [ -z "$actionable" ]; then
    reasons+=("CodeRabbit reviewed but its verdict could not be parsed, so it is not known to be clean")
  elif [ "$actionable" -gt 0 ]; then
    reasons+=("CodeRabbit raised $actionable actionable comment(s)")
  else
    echo "coderabbit: clean (0 actionable comments)"
  fi
fi

# ---------------------------------------------------------------
# Verdict
# ---------------------------------------------------------------
if [ ${#reasons[@]} -gt 0 ]; then
  echo
  echo "HOLD — pull request #$PR is not auto-mergeable:"
  printf '  - %s\n' "${reasons[@]}"
  echo
  echo "This is a normal outcome. A maintainer reviews and merges."
  exit 0
fi

echo
echo "MERGE — pull request #$PR by @$author passed every gate."

# --squash matches this repository's settings: allow_squash_merge is
# true and delete_branch_on_merge is false, so --delete-branch is
# deliberately absent — it would ask for something the repository is
# configured not to do. Checked against the live settings rather than
# assumed.
#
# --auto rather than a direct merge so that the behaviour stays
# correct if branch protection or a merge queue is added later; with
# neither configured it merges as soon as the requirements are met.
if ! gh pr merge "$PR" --repo "$REPO" --squash --auto; then
  echo "::warning::gh pr merge failed. If this says auto-merge is not enabled," \
       "turn on 'Allow auto-merge' in the repository settings."
  exit 1
fi
