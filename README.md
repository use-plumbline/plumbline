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

> **Status: early.** The engine and the first three rules work and are tested.
> Nothing is stable yet, and `.plumbline.toml` configuration is not implemented.

## Use it in CI

```yaml
- uses: use-plumbline/plumbline@main
  with:
    paths: contracts/
```

Options and exit codes: [docs/github-action.md](docs/github-action.md).

## Use it locally

```sh
make build
./bin/plumbline contracts/          # lint a directory
./bin/plumbline --list-rules        # what it checks
./bin/plumbline --explain missing-auth
```

## The rules

| Rule | Severity | Reports |
| --- | --- | --- |
| `missing-auth` | error | A contract function writes storage with no `require_auth` on any path through it |
| `panic-in-contract` | warning | `panic!`, `.unwrap()` or `.expect()` where a contract `Error` belongs |
| `unchecked-arithmetic` | warning | Arithmetic on a token-sized integer without `checked_*` or `saturating_*` |

Each rule carries its own "why it matters" and "how to fix it" — run
`plumbline --explain <rule>`.

## Documentation

- [Architecture](docs/architecture.md) — how the engine, rules and parser fit together
- [Adding a rule](docs/adding-a-rule.md) — the `Rule` interface, the fixture convention, and how to see the AST
- [The GitHub Action](docs/github-action.md) — inputs, outputs and exit codes
- [Contributing](CONTRIBUTING.md)

## License

[Apache-2.0](LICENSE)
