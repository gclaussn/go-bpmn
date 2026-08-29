---
description: A guide on how to install daemon and CLI.
---

# windows-amd64

This guide shows how to install the go-bpmn process engine daemon and CLI.

## Download release artifact

```preprocess powershell
download-windows-amd64
```

Looking for a specific version? - [View releases on Github](https://github.com/gclaussn/go-bpmn/releases)

## Validate archive

```preprocess powershell
validate-windows-amd64
```

## Extract archive

```preprocess powershell
extract-windows-amd64
```

## Install executables

Move the executables into a directory, which is included in the `Path` environment variable,
or add the directory, containing the executables, to the `Path` environment variable via **Control panel** -> **Edit environment variables for your account**.
