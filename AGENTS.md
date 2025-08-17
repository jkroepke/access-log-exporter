# Instructions for AI Agents

The following guidelines apply to all files in this repository.

Before you start contributing, read [`DEVELOPER.md`](DEVELOPER.md) for a basic
understanding of how the project is structured and works.

Ensure that that the local go version matches the one specified in
[`go.mod`](go.mod).
Never update the Go version in `go.mod`.

## Programmatic checks

Before committing any changes, always run:

1. `make fmt` – formats all Go code.
2. `make lint` – runs the linter.
3. `make test` – executes the test suite.

If a command fails because of missing dependencies or network restrictions, note this in the PR's Testing section using the provided disclaimer.

## Pull requests

Summarise your changes and cite relevant lines in the repository. Mention the output of the programmatic checks.

## Program overview

`access-log-export` is written in Go and acts as a prometheus exporter for
access logs.
It collects and exports metrics from access logs collected through syslog protocol.

Configuration is usually done through a YAML file or environment variables. The
project's `docs/` directory contains detailed guides such as
[`docs/Configuration.md`](docs/Configuration.md) and
[`docs/Home.md`](docs/Home.md).
