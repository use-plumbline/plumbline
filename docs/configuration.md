# Configuration

Plumbline works with no configuration at all. `.plumbline.toml` exists for the
day one rule disagrees with your codebase — so that the answer is turning that
rule down, not deleting the action and losing the other two.

Put the file at the root of the repository. Plumbline reads `.plumbline.toml`
from the directory it is run in, which on a GitHub runner is the workspace
root. Nothing is required; every setting has a default.

```toml
# Files Plumbline should not read at all.
exclude = [
  "contracts/vendor/**",
  "**/generated.rs",
]

# Severity per rule: "error", "warning", "note", or "off".
[rules]
missing-auth         = "error"
panic-in-contract    = "warning"
unchecked-arithmetic = "off"
```

## `[rules]`

Each key is a rule ID — the name in `plumbline --list-rules` and in every
finding — and each value is one of:

| Value | Effect |
| --- | --- |
| `"error"` | Report at error severity. |
| `"warning"` | Report at warning severity. |
| `"note"` | Report at note severity. |
| `"off"` | Do not run the rule. |

Severity is what decides whether a finding fails the build, through the
`--fail-on` threshold (`fail-on` in the action). Lowering a rule to `"warning"`
under the default threshold keeps its findings on the pull request as
annotations without turning the build red.

A rule you do not mention keeps the severity it declares. Today's defaults:

| Rule | Default |
| --- | --- |
| `missing-auth` | `error` |
| `panic-in-contract` | `warning` |
| `unchecked-arithmetic` | `warning` |

**A rule ID Plumbline does not recognise is an error**, not a setting that
quietly does nothing. A misspelt key would otherwise look exactly like a rule
you had switched off successfully, and you would find out on the day it
mattered. The same goes for a value that is not one of the four above, and for
any key outside `exclude` and `[rules]`.

That does mean a config naming a rule from a newer Plumbline fails on an older
one. The error names the rule and points at `--list-rules`.

## `exclude`

A list of glob patterns. A file matching any of them is not read, not parsed,
not linted, and not counted in the file total.

Patterns are matched against the path as Plumbline discovered it — relative to
the working directory when you passed `contracts/`, absolute when you passed
`/srv/contracts` — with forward slashes on every platform.

| | |
| --- | --- |
| `*` | Any run of characters **within** one path segment. |
| `**` | Any number of segments, including none. |
| A pattern naming a directory | Excludes everything beneath it. |

```toml
exclude = [
  "vendor",                # everything under any top-level vendor/
  "contracts/*/generated", # one segment deep, then that directory
  "**/legacy/**",          # a legacy/ directory anywhere
  "**/*_generated.rs",     # by filename, anywhere
]
```

You rarely need `exclude` for the obvious cases. Plumbline already skips
`target/`, `node_modules/`, `.git/`, Rust's `tests/` and `test/` directories,
and files named `test.rs` or `tests.rs` — see
[what Plumbline reads](#what-plumbline-reads-without-being-told).

## Pointing at a different file

```sh
plumbline --config config/plumbline.toml contracts/
```

```yaml
- uses: use-plumbline/plumbline@v0.1.0
  with:
    paths: contracts/
    config: config/plumbline.toml
```

`--config` is strict: if the file is not there, the run fails with exit code 2
rather than silently falling back to the defaults. A `.plumbline.toml` that
Plumbline merely looked for and did not find is optional, as it has to be —
otherwise the action would not work until you wrote one.

## What Plumbline reads without being told

Directories never descended into:

- `target/` — Cargo's build output: vendored and generated Rust, not the
  contract under review.
- `node_modules/`, `.git/`
- `tests/` and `test/` — Rust's integration-test directory and the usual name
  for a `#[cfg(test)] mod test;` submodule.

Files never picked up by walking a directory:

- `test.rs`, `tests.rs`

Contracts declared inside a `#[cfg(test)]` module are not treated as entry
points, wherever the module lives.

All of this is about mocks. A contract written to be called by a test is built
to exercise one path, not to be safe on chain, so a mock with no `require_auth`
and an `.unwrap()` in it is correct code that every rule would report.

Naming a file on the command line overrides all of it, so `plumbline
src/test.rs` lints exactly that file.

## Recipes

**Adopting Plumbline on a contract that has findings.** Annotate everything,
fail on nothing, and tighten later:

```yaml
- uses: use-plumbline/plumbline@v0.1.0
  with:
    paths: contracts/
    fail-on: never
```

**One rule is wrong about your contract.** Keep it visible, stop it failing
the build:

```toml
[rules]
missing-auth = "warning"
```

**One rule does not apply at all.** Say so, and say why — the next person to
read the file will want to know:

```toml
[rules]
# Entry points here are authorized by a Merkle proof rather than
# require_auth, which missing-auth cannot see.
missing-auth = "off"
```

**A vendored dependency you did not write:**

```toml
exclude = ["contracts/vendor/**"]
```
