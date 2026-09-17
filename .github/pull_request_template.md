<!--
Thanks for contributing. The boxes below are read by the auto-merge gate, not
just by a human — see the note at the bottom for what that means and why.
Tick only what is actually true; an unticked box holds the pull request for a
maintainer, which is a fine outcome and not a failure.
-->

## What this changes

<!-- One or two sentences. If it adds a rule, say what the rule reports. -->

Closes #

## Why

<!-- What goes wrong on chain without it, or what was wrong with the old
behaviour. For a rule, this is the same argument as its Meta.Why. -->

## Checklist

- [ ] **No maintainer-owned path touched.** Nothing under `internal/rule/`,
      `internal/engine/`, `internal/config/`, `internal/syntax/`, `action/`,
      `.github/workflows/`, and no change to `action.yml`, `.golangci.yml` or
      `Makefile`. (Rules in `rules/`, fixtures in `testdata/`, docs and the CLI
      are *not* maintainer-owned — those are the contributor surface.)
- [ ] **No new dependency.** `go.mod` and `go.sum` are unchanged.
- [ ] **A new rule ships with both fixtures**, at
      `testdata/rules/<rule-id>/pass.rs` and `fail.rs`, and the pass fixture
      contains the cases a naive implementation would get wrong.
- [ ] **SDK claims are verified, not recalled.** Anything asserted about
      `soroban-sdk` was checked against docs.rs, with a comment saying what was
      checked and when.
- [ ] **`make lint && make test` pass locally.**
- [ ] **The corpus baseline is honest.** Either `make corpus-check` passes
      unchanged, or `corpus/baseline.txt` was updated *and this pull request
      says which rule moved and why*, with the tables in
      [docs/corpus-run.md](../blob/main/docs/corpus-run.md) updated to match.

## How a reviewer can check this

<!-- The commands you actually ran. For a rule:
     go test ./rules/ -run 'TestRuleFixtures/<rule-id>' -v
     make corpus
-->

---

<details>
<summary>Why some changes merge automatically and others wait for a person</summary>

Plumbline's output is an assertion about someone else's contract. A rule that
fires wrongly does not just annoy — it teaches people to ignore the tool, and a
tool people ignore protects nothing. A rule that stays quiet when it should not
is worse, because a clean run reads as reassurance.

So the gate is deliberately narrow. It merges changes it can verify
mechanically — a self-contained rule with its fixtures, a doc fix, a test —
and routes everything else to a maintainer. "Maintainer-owned" is about blast
radius, not gatekeeping: a mistake in one rule is one wrong finding, while a
mistake in the engine, the parser boundary or the config schema is silently
wrong findings across every rule and every user at once.

The corpus check exists for the same reason. It runs every rule against 319
files of real third-party contracts pinned by commit, so a change that alters
what Plumbline says about real code shows up as a diff rather than as a
surprise after release.

The full reasoning is in [CONTRIBUTING.md](../blob/main/CONTRIBUTING.md) and
[docs/corpus-run.md](../blob/main/docs/corpus-run.md).

</details>
