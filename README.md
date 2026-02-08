<p align="center">
  <a href="https://gclaussn.github.io/go-bpmn/">go-bpmn</a> is a native BPMN 2.0 process engine, built on top of PostgreSQL.
</p>

## Features

- Easy setup that requires only a database URL
- Embeddable in any Go application
- Process automation, using workers
- Unit testing with an in-memory engine
- Integration via HTTP API
- OpenAPI documentation for client generation
- CLI for process operation

## Installation

requires Go 1.24+

```sh
# process engine daemon
go install github.com/gclaussn/go-bpmn/cmd/go-bpmn-pgd@latest

# CLI
go install github.com/gclaussn/go-bpmn/cmd/go-bpmn@latest
```

When used in a Go module:

```sh
go get github.com/gclaussn/go-bpmn
```

## Development

See [DEVELOPMENT.md](DEVELOPMENT.md) for more information.
