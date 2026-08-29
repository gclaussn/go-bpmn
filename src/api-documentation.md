---
description: API documentation reference.
---

<script setup>
import release from './assets/release.json'

const goPkg = `github.com/gclaussn/go-bpmn@${release.version}`
const goPkgUrl = `https://pkg.go.dev/github.com/gclaussn/go-bpmn@${release.version}`
</script>

# API documentation

## HTTP API

To integrate a process engine in any language, application or system, go-bpmn provides an HTTP API.
The API is described using OpenAPI standard in version [3.1.0](https://spec.openapis.org/oas/v3.1.0.html).

::: tip Explore the API

see <a href="openapi.html" target="_blank">OpenAPI documentation</a> as HTML version

:::

The structured API description in form of a `yaml` file, can be downloaded from the release artifacts:

::: code-group

```preprocess sh [Linux]
download-openapi-linux
```

```preprocess powershell [Windows]
download-openapi-windows
```

:::

## Go API

For embedding a process engine or automating a process, in Go, visit the documentation of Go module <a :href="goPkgUrl" target="_blank">{{goPkg}}</a>
