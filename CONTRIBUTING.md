# Contributing to Plumbline

Most contributions to Plumbline are **new rules**, and a rule is deliberately a
small, self-contained change: one file, one line in a list, two fixtures. This
guide is mostly about that path.

## Getting set up

You need Go 1.23 or newer. The Rust grammar is compiled through cgo, so you
also need a C compiler (`gcc` or `clang`) — no Rust toolchain is required to
build or test Plumbline itself.

```sh
make build   # binary into bin/
make test    # the suite, with the race detector
make lint    # gofmt, go vet, golangci-lint
make run     # lint the sample contract in testdata/
```

## Adding a rule

Rules live in [`rules/`](rules/), one per file, and no rule may import another.
Adding one means three things:

1. **`rules/your_rule.go`** — a type implementing [`rule.Rule`](internal/rule/rule.go):

   ```go
   type YourRule struct{}

   func (YourRule) Meta() rule.Meta { /* ID, Severity, Summary, Why, Fix */ }
   func (YourRule) Check(c *rule.Context) []rule.Finding { /* ... */ }
   ```

2. **One line in `all()`** in [`rules/rules.go`](rules/rules.go).

3. **Two fixtures**, at `testdata/rules/<your-rule-id>/pass.rs` and
   `fail.rs`. These are not optional — the test harness is driven off the
   registry, so a rule without both fixtures fails the build.

See [docs/adding-a-rule.md](docs/adding-a-rule.md) for the details: what
`Context` gives you, how to navigate the syntax tree, and the conventions the
fixtures follow.

### What a good rule looks like

- **It earns its noise.** A rule that fires on idiomatic contracts is worse
  than no rule, because it teaches people to ignore Plumbline. If you cannot
  keep the false-positive rate low, narrow the rule's scope until you can, and
  say what you narrowed it to.
- **It says why, and how to fix it.** `Meta.Why` explains what goes wrong on
  chain; `Meta.Fix` names the concrete remedy. Both are required.
- **Its fixtures argue the case.** The pass fixture should contain the cases a
  naive implementation would get wrong — that is what stops the next person
  from "simplifying" the rule back into a false-positive generator.

## Verify SDK behaviour, do not recall it

Rules encode claims about `soroban-sdk`. Before you encode one, check it
against [docs.rs/soroban-sdk](https://docs.rs/soroban-sdk) or the
[Stellar docs](https://developers.stellar.org/docs/build/smart-contracts), and
say in a comment what you checked and when.

What this looks like when it goes wrong is written up in
[docs/corpus-run.md](docs/corpus-run.md): four classes of false positive that
real contracts exposed, each now pinned by a fixture.

This matters more than it sounds. `missing-auth` does not treat `extend_ttl` as
a state mutation, because any account can extend any entry's TTL with
`ExtendFootprintTTLOp` without the contract's involvement. That is the kind of
detail that separates a useful rule from an annoying one, and you only get it
from the source.

## Maintainer-owned areas

Some paths need a maintainer's review before they merge. This is about **blast
radius, not gatekeeping**.

Plumbline's output is an assertion about someone else's contract. A mistake in
one rule is one wrong finding, in one place, caught by that rule's own fixtures.
A mistake in the engine, the parser boundary or the config schema is silently
wrong findings across *every* rule and every user at once — and the failure mode
that matters most is the quiet one, because a clean Plumbline run reads as
reassurance. A wrong rule published under Plumbline's name is the thing to avoid.

Maintainer-owned:

| Path | Why |
| --- | --- |
| `internal/rule/` | The `Rule` interface, the registry, and the AST helpers every rule is built on |
| `internal/engine/` | File discovery, rule execution, and where severity is stamped onto findings |
| `internal/config/` | The `.plumbline.toml` schema, which is a compatibility promise ([RELEASING.md](RELEASING.md)) |
| `internal/syntax/` | The parser boundary — a subtly wrong `Node` corrupts every rule's view of the tree |
| `action.yml`, `action/` | The Action's contract with every workflow that uses it |
| `.github/workflows/` | What CI checks, and the merge gate itself |
| `.golangci.yml`, `Makefile` | What "passing" means for everyone else |

**Everything else is the contributor surface**, and that is deliberate:
`rules/`, `testdata/`, `cmd/plumbline/`, `internal/report/` and all
documentation. A new rule is one file, one line in `all()`, and two fixtures —
touching none of the above. That is the shape the project is built around, and
the gate is set up so those contributions are not held up.

Touching a maintainer-owned path is not discouraged, and plenty of good changes
have to. It just means a person reads it before it lands.

### The auto-merge gate

[`.github/workflows/auto-merge.yml`](.github/workflows/auto-merge.yml) merges a
pull request only when all six hold: no maintainer-owned path touched, no
`go.mod`/`go.sum` change, every box in the pull request template ticked, every
other check green, no outstanding request for changes, and — where a review
exists — a clean CodeRabbit verdict.

Anything else is **held, not rejected**. A hold prints the specific reasons in
the workflow log, and a maintainer takes it from there. The path list above is
the same one the gate matches on and the same one the template's first checkbox
names; if one changes, all three change together.

## Working on the parser boundary

Rules never import tree-sitter. [`internal/syntax`](internal/syntax/) wraps it
and exposes `Node`; `rule.Node` aliases that type. If you find yourself wanting
a tree-sitter API inside a rule, add the accessor you need to `internal/syntax`
instead — a test asserts the parser stays confined there.

## Commits and pull requests

- Commit messages explain **why**, not just what. If a decision has a
  non-obvious reason, that reason belongs in the message.
- Keep commits incremental and honest. One logical change per commit.
- Put the pull request description in the pull request, not in a committed
  file.
- `make lint && make test` must pass before you open a PR. CI runs the same
  things, plus `make corpus-check`.
- **If your change moves the corpus numbers, say so.** The corpus is pinned by
  commit, so [`corpus/baseline.txt`](corpus/baseline.txt) only changes when
  Plumbline changes. Update it with
  `make corpus && cp corpus/checkouts/.summary corpus/baseline.txt`, say which
  rule moved and why, and update the tables in
  [docs/corpus-run.md](docs/corpus-run.md) in the same pull request. A new rule
  that adds findings across hundreds of real contracts is exactly what that
  check exists to put in front of a person.

## Code of conduct

Participation is governed by the [Code of Conduct](CODE_OF_CONDUCT.md).

## Reporting a vulnerability

Not through an issue or a PR — see the [security policy](SECURITY.md). A rule
that misses a bug is not a vulnerability; that one is an ordinary issue, and a
welcome one.

## License

Contributions are accepted under the [Apache-2.0](LICENSE) license.
