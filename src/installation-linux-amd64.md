---
description: A guide on how to install daemon and CLI.
---

# linux-amd64

This guide shows how to install the go-bpmn process engine daemon and CLI.

## Download release artifact

```preprocess sh
download-linux-amd64
```

Looking for a specific version? - [View releases on Github](https://github.com/gclaussn/go-bpmn/releases)

## Validate archive

```preprocess sh
validate-linux-amd64
```

## Extract archive

```preprocess sh
extract-linux-amd64
```

## Install CLI

```sh
sudo install -o root -g root -m 0755 ./go-bpmn /usr/local/bin/go-bpmn
```

## Install process engine daemon

```sh
sudo install -o root -g root -m 0755 ./go-bpmn-pgd /usr/local/bin/go-bpmn-pgd
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
