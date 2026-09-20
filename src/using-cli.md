---
description: How to connect and use the CLI?
---

# Using CLI

To use the `go-bpmn` command-line interface, the URL of a process engine HTTP API as well as an API key are required.

The configuration (environment variable `GO_BPMN_HTTP_BIND_ADDRESS`) of a process engine daemon defines the URL's host and port.
The default bind address is `127.0.0.1:8080`.

An API key must be created, using the daemon's CLI:

```sh
go-bpmn-pgd api-key create --secret-id example > example-authorization
```

::: danger Please note

The result of the command is an authorization string - a **secret** that is printed only **once**.

:::

## Configuration

The URL can be provided as environment or as global CLI option `--url`.
Whereas the authorization string must be configured per an environment variable.

```sh
export GO_BPMN_URL="http://127.0.0.1:8080"
export GO_BPMN_AUTHORIZATION="$(cat example-authorization)"
```

After URL and authorization string are set, the CLI can be used:

```sh
go-bpmn [command] [flags]
```

## Debugging

The `--debug` option allows to debug the HTTP communication. To enable debugging for every command, set environment variable `GO_BPMN_DEBUG` to `true`.

```sh
go-bpmn --debug process-instance create --bpmn-process-id example --version 1
```

The debug output, includes HTTP method, URL, request body, status code, response headers and response body: 

```
2025/05/24 13:15:15 POST http://localhost:8080/process-instances
2025/05/24 13:15:15 request body:
{
  "bpmnProcessId": "example",
  "version": "1",
  "workerId": "go-bpmn"
}
2025/05/24 13:15:15 status code: 404
2025/05/24 13:15:15 response headers:
2025/05/24 13:15:15 Content-Length: 123
2025/05/24 13:15:15 Content-Type: application/problem+json
2025/05/24 13:15:15 Date: Sat, 24 May 2025 11:15:15 GMT
2025/05/24 13:15:15 response body:
{
  "status": 404,
  "type": "NOT_FOUND",
  "title": "failed to create process instance",
  "detail": "process example:1 could not be found"
}
Error: failed to create process instance: process example:1 could not be found
```
