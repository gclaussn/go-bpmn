# go-bpmn

go-bpmn is a native BPMN 2.0 process engine, built on top of PostgreSQL.

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
# pg engine daemon
go install github.com/gclaussn/go-bpmn/cmd/go-bpmn-pgd

# CLI
go install github.com/gclaussn/go-bpmn/cmd/go-bpmn
```

When used in a Go module:

```sh
go get github.com/gclaussn/go-bpmn
```

## Developmemt

See [DEVELOPMENT.md](DEVELOPMENT.md) for more information.
