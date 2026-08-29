---
description: A guide on how to install daemon and CLI.
---

# darwin-arm64

This guide shows how to install the go-bpmn process engine daemon and CLI.

## Download release artifact

```preprocess sh
download-darwin-arm64
```

Looking for a specific version? - [View releases on Github](https://github.com/gclaussn/go-bpmn/releases)

## Validate archive

```preprocess sh
validate-darwin-arm64
```

## Extract archive

```preprocess sh
extract-darwin-arm64
```

## Install CLI

```sh
chmod +x ./go-bpmn
```

```sh
sudo mv ./go-bpmn /usr/local/bin/go-bpmn
```

```sh
sudo chown root:root /usr/local/bin/go-bpmn
```

## Install process engine daemon

```sh
chmod +x ./go-bpmn-pgd
```

```sh
sudo mv ./go-bpmn-pgd /usr/local/bin/go-bpmn-pgd
```

```sh
sudo chown root:root /usr/local/bin/go-bpmn-pgd
```

::: tip

If you do not have root access, make the files executable and move them to a directory, which is included in `$PATH`.

```sh
chmod +x ./go-bpmn
chmod +x ./go-bpmn-pgd

mkdir -p ~/.local/bin

mv ./go-bpmn ~/.local/bin/go-bpmn
mv ./go-bpmn-pgd ~/.local/bin/go-bpmn-pgd

# add ~/.local/bin to $PATH
```

:::
