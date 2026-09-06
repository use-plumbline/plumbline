# The corpus run

Plumbline's rules are only worth running if they stay quiet on correct code. The
only way to know that is to point them at contracts written by people who have
never heard of Plumbline, read every finding, and treat anything the rules got
wrong as a defect in the rules.

This document records what that produced: what was scanned, what fired, what was
wrong, and what is still unknown. It is written so that someone who does not
trust it can re-run it and check.

**Everything here is reproducible except where it says it is not.** Where a
number could not be verified, it says so rather than being filled in.

```sh
make corpus
```

---

## What was scanned

### This run — 2026-09-06

Plumbline `v0.1.0-18-g98f2d98`, five rules. Corpus pinned by commit in
[`corpus/repos.txt`](../corpus/repos.txt).

| Repository | Commit | Files linted | Files declaring `#[contract]` |
| --- | --- | --- | --- |
| [stellar/soroban-examples](https://github.com/stellar/soroban-examples) | `a1bf52cde6c11dffd870053bccf3d47ee0206e06` | 79 | 42 |
| [OpenZeppelin/stellar-contracts](https://github.com/OpenZeppelin/stellar-contracts) | `6ea3075d44de51ea2a0d26fa45cbee6d385731b5` | 240 | 53 |
| **Total** | | **319** | **95** |

"Files linted" is after Plumbline's own exclusions: build output, `tests/`,
`test/`, `test.rs` and `tests.rs` are skipped, because the mock contracts in
them are never compiled into a deployed wasm. That exclusion is itself one of
the four fixes described below.

### The v0.1.0 run — 2026-08-15

Recorded in commit [`b904e11`][b904e11] and
[`docs/sessions/2026-08-15-session-3.md`](sessions/2026-08-15-session-3.md).
**Quoted here, not independently re-verified** — see [what is
uncertain](#what-remains-uncertain).

| Repository | Commit | Files | Errors | Warnings |
| --- | --- | --- | --- | --- |
| stellar/soroban-examples | **not recorded** | 78 | 9 | 85 |
| OpenZeppelin/stellar-contracts | **not recorded** | 240 | 4 | 19 |
| Four private working workspaces | **unrecoverable** | 66 | 0 | 7 |

No commit was pinned at the time, which is exactly why `corpus/repos.txt` now
exists. The four workspaces are unnamed in git history and cannot be identified,
so that row cannot be reproduced by anyone, including us.

### Does the old claim hold up?

The two public repositories were re-linted at current revisions:

| | 2026-08-15 | 2026-09-05 | |
| --- | --- | --- | --- |
| soroban-examples | 78 files, 9 errors, 85 warnings | 79 files, 9 errors, 86 warnings | one file added upstream, carrying one warning |
| stellar-contracts | 240 files, 4 errors, 19 warnings | 240 files, 4 errors, 19 warnings | identical |
| `missing-auth` total | 13 | 13 | identical |

The v0.1.0 claim — "the corpus now reports 13 missing-auth errors, all of them
entry points that genuinely take no authorization" — reproduces exactly, three
weeks and one upstream commit later.

---

## What fired, and what was wrong

### This run

| Rule | Severity | soroban-examples | stellar-contracts | Total |
| --- | --- | --- | --- | --- |
| `contractmeta-missing` | note | 41 | 50 | **91** |
| `hardcoded-address-literal` | warning | 0 | 0 | **0** |
| `missing-auth` | error | 9 | 4 | **13** |
| `panic-in-contract` | warning | 57 | 17 | **74** |
| `unchecked-arithmetic` | warning | 29 | 2 | **31** |

### `missing-auth` — 13 findings, all 13 read

| | |
| --- | --- |
| True positive | 12 |
| Rule cannot decide | 1 |
| False positive | 0 |

Every one is a contract entry point that writes storage and calls no
`require_auth` on any path. Nine are teaching examples in `soroban-examples`
(`increment`, `errors`, `events`, `custom_types`, `other_custom_types`,
`increment_with_fuzz`, `increment_with_pause`, `pause`, `ttl`) where
authorization is deliberately outside what the example teaches. The rule is
right that no authorization exists; whether that matters in a file whose job is
to demonstrate TTL is a judgement it does not make.

Three would matter in production, and are worth naming:

- `stellar-contracts/examples/pausable/src/contract.rs` — `emergency_reset`
  carries `#[when_paused]` and nothing else. Anyone can reset the counter while
  the contract is paused. `#[when_paused]` is a state check, not an
  authorization check — a distinction pinned by fixture, below.
- `stellar-contracts/examples/upgradeable/lazy-v1/src/contract.rs` —
  `set_score(user, points)` writes `Score(user)` for an arbitrary address with
  no authorization, in a contract that has an admin.
- `soroban-examples/pause/src/lib.rs` — `set(paused)` lets any caller pause the
  contract.

**The one the rule cannot decide:**
`stellar-contracts/examples/merkle-voting/src/contract.rs` — `vote` is gated by
`Distributor::verify_and_set_claimed(e, vote_data, proof)`, a Merkle-proof check
that Plumbline cannot see through. The proof establishes that the voter is in
the allowlist and has not already voted; it does not establish that the caller
controls that identity, so whether it is sufficient authorization is a question
about the contract's threat model, not one a syntactic linter can answer. It is
reported, and this is the documented limit rather than a bug to be silenced.

### `unchecked-arithmetic` — 31 findings, all 31 read

| | |
| --- | --- |
| True positive | 30 |
| False positive | 1 |

Twenty-five of the thirty are in `soroban-examples/liquidity_pool` — reserve and
share arithmetic on `i128`, which is exactly the arithmetic the rule exists for.
The rest are token amounts in `fuzzing`, `mint-lock` and `merkle-voting`.
`eth_abi` is a true positive of a different shape: `input.b + input.c` on
Solidity `uint256`, decoded through `alloy`, which overflows the same way.

**The false positive**, stated plainly:

`soroban-examples/alloc/src/lib.rs:18` — `sum += i` inside
`pub fn sum(_env: Env, count: u32) -> u32`. `sum` is `u32`: Rust infers it from
the return type and from the element type of the vector being iterated.
Plumbline resolves `let mut sum = 0` to an unsuffixed literal (no type
information) and the loop binding `i` to unknown, and the rule reports what it
cannot type. It is not token arithmetic and cannot overflow into money.

This is the documented consequence of a deliberate trade-off, not a regression —
session 3 chose to keep reporting unresolvable operands because narrowing to
*known* 128-bit ones drops most of the true positives in `liquidity_pool`. It is
still a false positive, it is counted as one here, and closing it needs
return-type inference the rule does not have. Tracked rather than hidden.

### `panic-in-contract` — 74 findings, sampled

Every finding is a literal `panic!`, `.unwrap()` or `.expect()` inside a contract
entry point, which is what the rule says it reports. **These were reviewed by
file distribution and by reading a sample, not classified one by one**, so no
true/false split is claimed for this rule. What was checked exhaustively is that
none of the 74 comes from a path Plumbline is supposed to skip: no finding in any
`tests/`, `test/`, `test.rs` or `tests.rs`, confirming the test-scaffolding fix
still holds on a corpus three weeks newer than the one it was written against.

### `contractmeta-missing` — 91 findings, and a problem

This rule was added by [PR #27][pr27] after the v0.1.0 corpus run, so this is the
first time it has met third-party code. The result:

| | soroban-examples | stellar-contracts | Total |
| --- | --- | --- | --- |
| Files declaring `#[contract]` | 42 | 53 | 95 |
| Findings | 41 | 50 | 91 |
| **Hit rate** | **97.6%** | **94.3%** | **95.8%** |

Exactly one file in 319 uses `contractmeta!` —
`soroban-examples/liquidity_pool/src/lib.rs`. Every other contract in both of
the ecosystem's reference repositories would be annotated.

Each finding is factually correct: the metadata really is absent. So this is not
a false positive in the sense the four below are — the rule is not wrong about
the code. The problem is the rate. Plumbline's own stated standard is that
[a rule earns its noise or it doesn't ship](../README.md#about), because a check
that fires on idiomatic contracts teaches people to ignore the whole tool, and
96% of idiomatic contracts is not a rule finding defects — it is a rule
disagreeing with the ecosystem's convention.

Two things limit the damage today: the severity is `note`, the lowest, so it
does not fail a build at the default `--fail-on error`; and it can be switched
off in `.plumbline.toml`. Neither changes the annotation count on a pull request
using the default `github` format.

**No decision is taken here.** This document reports the measurement; the
decision — default-off, narrower scope, or accepted as-is — is
[issue #31](https://github.com/use-plumbline/plumbline/issues/31), with these
numbers attached. What this does establish is that the corpus run pays for
itself: the rule passed CI, its own fixtures, and code review, and the first
thing that caught the rate was pointing it at real contracts.

---

## The four false positives v0.1.0 found and fixed

These are the most valuable paragraphs in this document. They are what it looks
like when the tool meets real code and loses the argument. Reproduced from
[`b904e11`][b904e11]; each is now pinned by a fixture, so `make test` fails if it
comes back.

### 1. Test scaffolding — 44 findings

> Mock contracts in `src/test.rs`, `tests/` and `test/`, and in `#[cfg(test)] mod`
> blocks, are written to exercise one path and are never compiled into a deployed
> wasm. They accounted for 44 of the findings on their own. Walking now skips
> those files and directories, and a contract declared inside `#[cfg(test)]` is
> not an entry point. Naming a file on the command line still lints it, so
> `plumbline src/test.rs` does what it says.

The single largest source of noise in a real workspace: a mock with no
`require_auth` and an `.unwrap()` in it is *correct code* that every rule would
otherwise report.

**Pinned by** [`internal/engine/engine_test.go`](../internal/engine/engine_test.go)
— `TestDiscoverRust` asserts `src/test.rs`, `src/tests.rs`, `tests/integration.rs`
and `src/test/helpers.rs` are all skipped; `TestDiscoverRustTakesNamedFilesAsGiven`
asserts naming one explicitly still lints it. Also
[`testdata/rules/missing-auth/pass.rs`](../testdata/rules/missing-auth/pass.rs),
whose `#[cfg(test)] mod tests` block contains a deliberately unauthorized mock.

### 2. Authorizing attribute macros — 6 findings

> OpenZeppelin's `#[only_owner]`, `#[only_admin]`, `#[only_role]` and
> `#[only_any_role]` expand into a require_auth before the body runs, so the
> authorization is real even though no require_auth is visible. `has_role` and
> `has_any_role` are deliberately not in the set: they check that an address
> holds a role without requiring it to have authorized the call. `when_paused`
> and `when_not_paused` are not authorization at all, and a fail fixture pins
> that.

The distinction is the whole fix. Treating every `stellar-macros` attribute as
authorization would have silenced real findings — and this run confirms it, since
`pausable/emergency_reset` is reported *because* `#[when_paused]` is correctly
not counted.

**Pinned by** [`testdata/rules/missing-auth/pass.rs`](../testdata/rules/missing-auth/pass.rs)
— `set_owner_fee` (`#[only_owner]`) and `set_operator_fee` (`#[only_role]`
stacked above `#[when_not_paused]`, so the authorizing attribute is found when it
is not the one nearest the function) — and by
[`fail.rs`](../testdata/rules/missing-auth/fail.rs), where `set_fee` carries
`#[when_not_paused]` alone and **must** still be reported.

### 3. One-shot initializers — 2 findings

> `if env.storage()...has(&k) { return Err(..) }` makes a function callable
> exactly once, which is the same argument that already exempts `__constructor` —
> contracts predating constructor support use it for exactly that job. The match
> requires the condition to be the `has` call itself, so the opposite guard,
> `if !has(&k) { panic!() }`, is not mistaken for it; that one asserts the
> contract is initialized and authorizes nothing.

**Pinned by** [`pass.rs`](../testdata/rules/missing-auth/pass.rs) — `init`
(returns `Err`) and `init_panicking` (panics) — and by
[`fail.rs`](../testdata/rules/missing-auth/fail.rs), where `reconfigure` uses the
negated guard and must still be reported.

This exemption is deliberately over-broad and is a known false *negative*: it
matches the guard anywhere in the body, so a function that guards a one-shot key
and separately needs authorization is not reported. Recorded in the v0.1.0
release notes rather than discovered later.

### 4. Ledger arithmetic read as token arithmetic — 5 findings

> `env.ledger().sequence() + window` is TTL and expiry work on a u32, not a
> balance, but every operand resolved to "unknown" and the rule reports unknown.
> `Ledger::sequence`, `Ledger::timestamp`, `Ledger::protocol_version` and
> `Vec::len` have known narrow return types, verified on docs.rs, and one
> resolved operand resolves the expression.

One resolved operand is enough because Rust requires both sides of an arithmetic
operator to share a type — the surgical fix that avoided making the rule quieter
about money.

**Pinned by** [`testdata/rules/unchecked-arithmetic/pass.rs`](../testdata/rules/unchecked-arithmetic/pass.rs)
— `expiry` (`sequence() + window`, `timestamp() + 3600`), `epoch_end`
(`sequence() / config.epoch_length`, where the *other* operand is unresolvable),
and `next_index` (`entries.len() + 1`).

---

## What remains uncertain

**The v0.1.0 numbers are quoted, not re-measured.** No commit was pinned when
that run happened, so the 174-findings figure and the four per-class counts
(44 / 6 / 2 / 5) cannot be independently reproduced. They are cited from
[`b904e11`][b904e11] as a record of what was observed, and this document does not
re-assert them as current fact. The two figures that *were* re-checked —
per-repository totals and the 13 `missing-auth` errors — reproduce.

**One third of the original corpus is gone.** The four private working
workspaces (66 files, 0 errors, 7 warnings) are unnamed in git history. Nobody
can reproduce that row. Whatever those contracts exercised that the two public
repositories do not is no longer covered.

**The corpus is small and narrow.** 319 files, 95 contracts, two repositories,
both of them reference material written to be exemplary. Reference contracts are
not production contracts: they are shorter, better commented, and written by
people who know the SDK well. A rule that is quiet here is not proven quiet on a
real DeFi codebase — it is proven quiet on the friendliest sample available.
Expanding the corpus is [issue #32](https://github.com/use-plumbline/plumbline/issues/32).

**`panic-in-contract`'s 74 findings were not individually classified.** Only the
sample and the path-exclusion check were done, so no accuracy claim is made for
that rule beyond "it reported what it says it reports."

**The analysis is syntactic and single-file.** Plumbline sees names and shapes,
not resolved types, and reads one file at a time. `alloc/sum` above is a false
positive purely because of that. A rule cannot follow a helper into another
module, and `missing-auth` cannot see authorization performed by a mechanism it
does not have a name for — the Merkle-proof case is the example in this corpus.

**Known blind spots per rule** are in the
[v0.1.0 release notes](https://github.com/use-plumbline/plumbline/releases/tag/v0.1.0),
in both directions — where a rule is known to fire wrongly, and where it is known
to stay quiet when it should not.

---

## Reproducing this

```sh
git clone https://github.com/use-plumbline/plumbline
cd plumbline
make corpus
```

`make corpus` fetches each repository in [`corpus/repos.txt`](../corpus/repos.txt)
at its pinned commit and prints the per-rule table above. It needs network access
on first run; afterwards the checkouts are reused. Cold run: about 15 seconds.

Full findings are written to `corpus/checkouts/.<repo>.findings`. To read them
the way this document did:

```sh
# every error, with the function and the line it writes on
grep 'error:' corpus/checkouts/.*.findings

# one rule at a time
grep '\[unchecked-arithmetic\]' corpus/checkouts/.soroban-examples.findings

# confirm no finding comes from a path that should have been skipped
grep -E '/(tests?)/|/tests?\.rs:' corpus/checkouts/.*.findings   # expect no output
```

### CI holds these numbers to account

The corpus is pinned by commit, so the counts are deterministic: they only move
when Plumbline moves. [`corpus/baseline.txt`](../corpus/baseline.txt) records
them, and CI runs `make corpus-check`, which fails on any difference.

That means a change altering what Plumbline says about 300-odd files of real
contracts cannot land without someone noticing. Had it existed in August,
`contractmeta-missing`'s 91 findings would have arrived as a red check on
[PR #27][pr27] rather than as a discovery three weeks later.

Updating the baseline is a normal part of a change that intends to move it:

```sh
make corpus && cp corpus/checkouts/.summary corpus/baseline.txt
```

The pull request should then say which rule moved and why, and this document's
tables updated in the same change.

To advance a pin, change the commit in `corpus/repos.txt`, re-run, read every new
finding, and update both the baseline and the numbers here. They must move
together: this document is only worth something if it matches what the tool
actually prints.

[b904e11]: https://github.com/use-plumbline/plumbline/commit/b904e112057d0bd7cffa4323a9c93cd7d6c0ea10
[pr27]: https://github.com/use-plumbline/plumbline/pull/27
