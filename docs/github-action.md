# The GitHub Action

Plumbline runs as a composite action. It builds the linter from its own checkout
and runs it against your workspace, so there is nothing to install and no
container to pull.

```yaml
name: Lint contracts

on: [pull_request]

permissions:
  contents: read

jobs:
  plumbline:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v7
      - uses: use-plumbline/plumbline@v0.1.0
        with:
          paths: contracts/
```

Findings appear as inline annotations on the changed files in the pull request.

## Inputs

| Input | Default | Description |
| --- | --- | --- |
| `paths` | `.` | Files or directories to lint, whitespace or newline separated. Directories are walked for `.rs` sources; build output and Rust's test sources are skipped — see [configuration](configuration.md#what-plumbline-reads-without-being-told). |
| `fail-on` | `error` | Lowest severity that fails the run: `error`, `warning`, `note`, or `never`. Findings below the threshold are still annotated. |
| `format` | `github` | `github` emits the workflow commands that become annotations. `text` is easier to read when debugging a workflow. `json` is for a later step that consumes the findings. |
| `config` | — | Path to a [configuration file](configuration.md). Defaults to `.plumbline.toml` in the repository root when it exists, and to Plumbline's defaults when it does not. A path given here must exist. |

## Outputs

| Output | Description |
| --- | --- |
| `findings` | Number of findings reported. Only populated when `format` is `github`; other formats leave it empty. |
| `exit-code` | The linter's exit code. |

## Exit codes

| Code | Meaning |
| --- | --- |
| `0` | Nothing at or above the `fail-on` threshold |
| `1` | Findings at or above the threshold |
| `2` | Plumbline could not run — a bad flag, or a path that does not exist |

Exit `2` is a workflow problem rather than a finding, and is passed through
unchanged so a mistyped path fails loudly instead of looking like a clean run.

## Recipes

**Annotate everything, fail on nothing.** Useful when adopting Plumbline on an
existing contract: you get the full picture without a red build on day one.

```yaml
- uses: use-plumbline/plumbline@v0.1.0
  with:
    paths: contracts/
    fail-on: never
```

**Hold a new contract to a higher bar** by failing on warnings too:

```yaml
- uses: use-plumbline/plumbline@v0.1.0
  with:
    paths: contracts/vault
    fail-on: warning
```

**Several directories:**

```yaml
- uses: use-plumbline/plumbline@v0.1.0
  with:
    paths: |
      contracts/vault
      contracts/router
```

**Tune a rule instead of dropping the action.** Commit a `.plumbline.toml` at
the repository root and the action picks it up with no change to the workflow:

```toml
exclude = ["contracts/vendor/**"]

[rules]
missing-auth = "warning"
```

See [configuration.md](configuration.md) for the full reference.

**Act on the result in a later step:**

```yaml
- uses: use-plumbline/plumbline@v0.1.0
  id: lint
  with:
    paths: contracts/
    fail-on: never

- run: echo "Plumbline reported ${{ steps.lint.outputs.findings }} finding(s)."
```

## Notes

- **Pin the version.** Use a tag — `@v0.1.0` — or a commit SHA. `@main` moves,
  and no floating `v0` tag is published. What a version promises is in
  [RELEASING.md](../RELEASING.md).
- **Annotations need the workflow's own log.** The action writes workflow
  commands to stdout, which the runner turns into annotations. It does not call
  the GitHub API and needs no token beyond the default `contents: read`.
- **Annotations only render inline on lines the pull request touched.** Findings
  elsewhere in the file still appear in the run's annotation list and in the
  step log.
- **No Rust toolchain is required.** Plumbline parses contract source rather
  than compiling it.
