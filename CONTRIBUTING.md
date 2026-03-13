# Contribution Guidelines for Amsterdam

## Introduction

This document explains how to contribute changes to the Amsterdam project.

## Development Location

While you may have found this project on GitHub or another site, the "source of truth" for the project will always
reside on [the Erbosoft code hosting site](https://git.erbosoft.com/amy/amsterdam). Serious contributors should
contact [Amy Bowersox](https://links.inclusiveladyship.com/@amy) for access.

## AI Contribution Policy

As per our [Code of Conduct](CODE-OF-CONDUCT.md), AI contributions are acceptable, but the submitting contributor _must:_
* Fully understand the contribution.
* Be able to explain design and implementation decisions without the use of AI.
* Accept responsibility for maintenance and correctness.

Contributors should indicate AI-generated content in issue and pull request descriptions and comments, specifying which model was used.

Do _not_ use AI to reply to questions about your issue or pull request. The questions are for _you,_ the human, not an AI model.

All project contributions must be submitted by _identifiable human participants_ who accept full responibility for their content.
Automated agents, bots, or autonomous AI systems _may not_ independently submit issues, pull requests, or other contributions.

Project maintainers retain _full discretion_ to close pull requests and issues that appear to be low-quality AI-generated content.
While we welcome new contributors, we want to see those that will engage constructively with the review process, rather than deferring
to AI.

## Building Amsterdam

From the root of the source tree, just run `go build` to build the `amsterdam` executable.

To regenerate the `tailwind.css` file (located in `ui/static/css`), you will need the Tailwind CSS command-line executable.
Download it from [the Tailwind GitHub](https://github.com/tailwindlabs/tailwindcss/releases/) and install it as `tailwindcss`
in your `PATH`. Then run `go generate` to regenerate the CSS file before you run `go build` to build the executable.

## Dependencies

Dependencies are managed using [Go Modules](https://go.dev/cmd/go/#hdr-Module_maintenance).
You can find more details in the [go mod documentation](https://go.dev/ref/mod) and the [Go Modules Wiki](https://github.com/golang/go/wiki/Modules).

Pull requests should only modify `go.mod` or `go.sum` where it is related to the change at hand, whether a bug fix or a new feature.
Apart from that, these files should only be modified by pull requests whose only purpose is to update dependencies.

## Copyright

New code files should use one of the standard headers found in the `docs/templates` folder. Examples exist for Go (also works with CSS),
HTML, Jet, and YAML (also works with SQL).

The copyright range should be updated when a file is modified in a year later than the current end of the range.
