# Plumbline

A linter for [Soroban](https://developers.stellar.org/docs/build/smart-contracts)
smart contracts, shipped as a GitHub Action.

Plumbline reads Soroban contract source and reports issues that are easy to miss
in review — a state-mutating function with no authorization check, a `panic!`
where a contract `Error` belongs, arithmetic that can silently overflow. Findings
land as inline annotations on the pull request that introduced them.

Rules are **AST checks**, not regex. Contract source is parsed with the
tree-sitter Rust grammar, so a rule can ask structural questions — "does any path
through this function reach a `require_auth` call?" — instead of matching text.

> **Status: early.** Four rules, released as `v0.1.0`. They are tested, and
> checked against 319 files of real third-party contracts so they do not fire
> on idiomatic Soroban — the run is published in
> [docs/corpus-run.md](docs/corpus-run.md), including what it got wrong. But
> four rules is a floor under review, not coverage. See
> [what it does not catch](#what-it-does-not-catch).

## About

Soroban contracts fail in ways that look fine in a diff. A function that writes
storage but never calls `require_auth` reads like every other function. An
`.unwrap()` is one character wider than a `?`. A `+` on a balance is invisible
until it wraps. Reviewers catch these when they are looking for them, and miss
them when they are reading for something else — so Plumbline looks for them
every time, on every push, and points at the line.

That only works if the tool is trusted, which sets the project's priorities:

- **A rule earns its noise, or it doesn't ship.** A check that fires on
  idiomatic contracts teaches people to ignore the whole tool. Rules are
  narrowed until their false-positive rate is low, and each one ships with the
  pass fixture that proves it.
- **Structure over text.** Rules ask questions about the syntax tree, so they
  can tell a storage write from a same-named method call, and can follow every
  path through a function rather than the ones a regex happens to match.
- **Claims about the SDK get checked, not remembered.** Rules encode real
  `soroban-sdk` behaviour, verified against the docs and annotated with what
  was checked.

Plumbline is a linter, not an audit, and it is deliberately syntactic — it sees
names and shapes, not resolved types. A clean run means the rules it has did not
fire. It is a floor under review, not a substitute for one.

It's built for contract authors and their reviewers: drop the action into a
workflow and findings arrive as annotations on the pull request, with no Rust
toolchain and no service to sign up for. It is early — see
[Contributing](CONTRIBUTING.md) if you want to add a rule.

## Use it in CI

Drop this in as `.github/workflows/plumbline.yml`. It is the whole file — no
Rust toolchain, no service to sign up for, no token beyond the default one.

```yaml
name: Plumbline

on: [pull_request]

permissions:
  contents: read

jobs:
  lint:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v7
      - uses: use-plumbline/plumbline@v0.1.0
        with:
          paths: contracts/
```

Findings arrive as inline annotations on the changed files. `missing-auth` is
an error and fails the build; the other two are warnings and do not.

Pin the version. `@main` moves.

Inputs, outputs, exit codes and more recipes:
[docs/github-action.md](docs/github-action.md).

## Use it locally

```sh
make build
./bin/plumbline contracts/                # lint a directory
./bin/plumbline --list-rules              # what it checks
./bin/plumbline --explain missing-auth    # why, and how to fix it
./bin/plumbline --format json contracts/  # for tooling
```

## The rules

| Rule | Severity | Reports |
| --- | --- | --- |
| `missing-auth` | error | A contract function writes storage with no `require_auth` on any path through it |
| `panic-in-contract` | warning | `panic!`, `.unwrap()` or `.expect()` where a contract `Error` belongs |
| `unchecked-arithmetic` | warning | Arithmetic on a token-sized integer without `checked_*` or `saturating_*` |
| `contractmeta-missing` | note | A contract declaration with no authored `contractmeta!` entry |

Each rule carries its own "why it matters" and "how to fix it" — run
`plumbline --explain <rule>`.

## Configuration

Optional. Plumbline needs no configuration to be useful, and
`.plumbline.toml` exists so that one rule you disagree with means turning that
rule down rather than deleting the action:

```toml
exclude = ["contracts/vendor/**"]

[rules]
missing-auth = "warning"   # "error", "warning", "note", or "off"
```

Full reference: [docs/configuration.md](docs/configuration.md).

## What it does not catch

Plumbline is a linter, not an audit, and it is syntactic — it sees names and
shapes, not resolved types. Three rules is three rules. A clean run means the
rules it has did not fire.

Nothing here checks reentrancy, token transfer accounting, TTL and archival
correctness, oracle or price manipulation, signature and replay handling,
upgrade safety, or cross-contract call ordering. There is no dataflow analysis:
a rule reads one file at a time and does not know what a helper in another
module does.

The rules it does have carry known blind spots, listed in the
[v0.1.0 release notes](https://github.com/use-plumbline/plumbline/releases/tag/v0.1.0).

## Documentation

- [The corpus run](docs/corpus-run.md) — what the rules were checked against, and what they got wrong
- [Architecture](docs/architecture.md) — how the engine, rules and parser fit together
- [Configuration](docs/configuration.md) — `.plumbline.toml`, severities and excludes
- [The GitHub Action](docs/github-action.md) — inputs, outputs and exit codes
- [JSON output](docs/json-output.md) — the schema, for tooling
- [Adding a rule](docs/adding-a-rule.md) — the `Rule` interface, the fixture convention, and how to see the AST
- [Releasing](RELEASING.md) — how versions are cut and what one promises
- [Contributing](CONTRIBUTING.md)
- [Code of Conduct](CODE_OF_CONDUCT.md)
- [Security policy](SECURITY.md) — how to report a vulnerability, and what counts as one

## License

[Apache-2.0](LICENSE)
