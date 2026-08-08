#!/usr/bin/env bash
#
# Thin wrapper for the Plumbline GitHub Action.
#
# Builds the linter from the action's own checkout, runs it against the
# caller's workspace, and reports the result as step outputs. All the linting
# decisions live in the binary; this script only moves data between the runner
# and the CLI.

set -euo pipefail

action_path="${PLUMBLINE_ACTION_PATH:?the action must set PLUMBLINE_ACTION_PATH}"
fail_on="${PLUMBLINE_FAIL_ON:-error}"
format="${PLUMBLINE_FORMAT:-github}"

# The paths input is whitespace or newline separated, so word splitting is the
# behaviour we want here rather than a bug.
# shellcheck disable=SC2206
paths=(${PLUMBLINE_PATHS:-.})
if [ ${#paths[@]} -eq 0 ]; then
  paths=(".")
fi

workdir="${RUNNER_TEMP:-/tmp}"
binary="${workdir}/plumbline"
output="${workdir}/plumbline-output.txt"

echo "::group::Building Plumbline"
( cd "$action_path" && go build -o "$binary" ./cmd/plumbline )
"$binary" --version
echo "::endgroup::"

# Annotations are workflow commands on stdout, so the output has to reach the
# runner's log to become annotations at all. tee keeps a copy for counting.
# PIPESTATUS is read before anything else can overwrite it.
set +e
"$binary" --format "$format" --fail-on "$fail_on" "${paths[@]}" | tee "$output"
status=${PIPESTATUS[0]}
set -e

if [ "$format" = "github" ]; then
  findings=$(grep -c '^::\(error\|warning\|notice\) ' "$output" || true)
else
  findings=""
fi

if [ -n "${GITHUB_OUTPUT:-}" ]; then
  {
    echo "findings=${findings}"
    echo "exit-code=${status}"
  } >>"$GITHUB_OUTPUT"
fi

if [ -n "${GITHUB_STEP_SUMMARY:-}" ]; then
  if [ "$status" -eq 0 ] && [ "${findings:-0}" = "0" ]; then
    echo "**Plumbline**: no findings in \`${paths[*]}\`." >>"$GITHUB_STEP_SUMMARY"
  else
    echo "**Plumbline**: ${findings:-some} finding(s) in \`${paths[*]}\` — see the annotations on the changed files." >>"$GITHUB_STEP_SUMMARY"
  fi
fi

# Exit code 2 means the linter could not run at all, which is a workflow
# problem rather than a finding; it is passed through unchanged.
exit "$status"
