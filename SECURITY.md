# Security Policy

## Supported versions

Plumbline is pre-1.0 and has no released versions. The only supported version
is the current `main` branch, and that is what fixes land on. If you pin the
action to a tag or a commit, expect to move that pin to pick up a fix.

## Reporting a vulnerability

Report privately, through GitHub's [private vulnerability reporting][report]
form on the repository's Security tab. Please do not open a public issue for a
suspected vulnerability.

Include, as far as you can:

- what the flaw is, and which file or rule it lives in;
- a contract or repository layout that reproduces it, and the `plumbline`
  invocation you ran;
- what an attacker gets out of it.

You should get an acknowledgement within a week. If a report is confirmed,
we'll agree a disclosure timeline with you and credit you in the advisory
unless you'd rather we didn't.

[report]: https://github.com/use-plumbline/plumbline/security/advisories/new

## What counts as a vulnerability

Plumbline runs inside CI, over source it did not write, with whatever
permissions the workflow grants it. Things in scope:

- **Escaping the linter's job.** Anything that gets a contract's *contents* —
  source, file names, paths — to execute code, read files outside the scanned
  paths, or make network calls during a lint run.
- **Leaking workflow secrets.** Findings and annotations are rendered into
  logs and pull requests; anything that turns that into a channel for the
  runner's environment or token.
- **Crashing or hanging the parser** in a way a hostile input file can trigger
  deliberately — a lint run that never finishes is a denial of service on the
  repository's CI.
- **Vulnerable dependencies** that Plumbline actually reaches, in the way the
  advisory describes.

## What does not

Not security issues, though they are still worth reporting as ordinary issues:

- **A rule that misses something.** A false negative means Plumbline did not
  find a bug in *your* contract. That is a gap in the rule, and we want to know
  about it — open an issue, with the contract shape that slipped past.
- **A rule that fires wrongly.** False positives are bugs, and by
  [CONTRIBUTING.md](CONTRIBUTING.md) they are treated as serious ones, but they
  are not vulnerabilities.
- **Vulnerabilities in a contract you scanned.** Those belong to the contract's
  maintainers, not to us.

Plumbline is a linter, not an audit. A clean run means the rules it has did not
fire; it is not a statement that a contract is safe.
