# Contribution Guidelines for Amsterdam

## Introduction

This document explains how to contribute changes to the Amsterdam project.

## Development Location

While you may have found this project on GitHub or another site, the "source of truth" for the project will always
reside on [the Erbosoft code hosting site](https://git.erbosoft.com/amy/amsterdam). This is to ensure long-term
independence from platforms controlled by third parties; the mirrors to GitHub and/or other sites are for visibility.

Serious contributors should contact Amy Bowersox (amy@erbosoft.com or via a contact method listed in
[her LinkStack](https://links.inclusiveladyship.com/@amy)) for access.

## AI Contribution Policy

AI contributions are allowed, but _must_ follow the policy set out in our [Code of Conduct](CODE-OF-CONDUCT.md). Failure to do so
will result in summary rejection of contributions and possible restriction of participation in the project.

## Contribution Workflow

1. [Open an issue](https://git.erbosoft.com/amy/amsterdam/issues/new) describing the problem or proposed feature.
2. Discuss the design with [maintainers](MAINTAINERS).
3. Submit a pull request referencing the issue.
4. Participate in code review.

## Versioning

Amsterdam employs [semantic versioning](https://semver.org/), with version numbers in the "major.minor.bugfix" format. Please see
the linked specification for further details.

## Git Branching

Amsterdam employs the [GitHub Flow](https://docs.github.com/en/get-started/using-github/github-flow) style of branch management.

One thing to remember: when working on a feature branch, and seeking to merge new changes from `main` into your branch, _never_ use `git merge`,
but _always_ use `git rebase` to rebase your feature branch off the tip of `main`.  The rule to remember is, changes flowing "inward"
_from_ feature branches _to_ `main` use `git merge`, but changes flowing "outward" _from_ `main` _to_ feature branches use `git rebase`.

## Dependencies

Dependencies are managed using [Go Modules](https://go.dev/cmd/go/#hdr-Module_maintenance).
You can find more details in the [go mod documentation](https://go.dev/ref/mod) and the [Go Modules Wiki](https://github.com/golang/go/wiki/Modules).

Pull requests should only modify `go.mod` or `go.sum` where it is related to the change at hand, whether a bug fix or a new feature.
Apart from that, these files should only be modified by pull requests whose only purpose is to update dependencies.

## Copyright

New code files should use one of the standard headers found in the `docs/templates` folder. Examples exist for Go (also works with CSS),
HTML, Jet, and YAML (also works with SQL).

The copyright range should be updated when a file is modified in a year later than the current end of the range.
