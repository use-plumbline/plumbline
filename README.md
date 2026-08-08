# Plumbline

A linter for [Soroban](https://developers.stellar.org/docs/build/smart-contracts) smart
contracts, shipped as a GitHub Action.

Plumbline reads Soroban contract source (Rust) and reports issues that are easy to miss
in review — a state-mutating function with no authorization check, a `panic!` where a
contract `Error` belongs, arithmetic that can silently overflow.

## Status

Early. The engine, the rule interface, and the first rules are being built now. Nothing
here is stable yet.

## Design

- **Plumbline is written in Go.** It *reads* Rust; it is not written in Rust. There is no
  Rust in this repo outside of test fixtures and sample contracts.
- **Rules are AST checks**, not regex. Contract source is parsed with the tree-sitter
  Rust grammar so a rule can ask real structural questions ("does this function body
  reach a `require_auth` call?") instead of pattern-matching text.
- **One rule per file, no rule depends on another.** Every rule is independently
  testable and has a pass fixture and a fail fixture proving it fires when it should and
  stays quiet when it shouldn't.

## Layout

| Path | What lives there |
| --- | --- |
| `cmd/plumbline/` | CLI entrypoint |
| `internal/engine/` | Source loading, parsing, rule execution, finding collection |
| `internal/rule/` | The `Rule` interface and the rule registry |
| `rules/` | One file per rule |
| `testdata/` | Sample contract and per-rule pass/fail fixtures |

## License

[Apache-2.0](LICENSE)
