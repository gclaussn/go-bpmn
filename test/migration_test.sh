#!/usr/bin/env bash

# Tests the database schema migration from a specific release to the currently developed version.
# Requires the current version to be built.
#
# To build the current version, run:
# go run ./.github/workflows/build.go -tag-name current

set -e

if [ "$#" -ne 1 ]; then
  echo "arg #1 must be a release tag name" && exit 1
fi

release_tag_name="$1"
os_arch="linux-amd64"

export GO_BPMN_PG_DATABASE_URL="postgres://postgres:postgres@localhost:5432/test"

# download release
curl -L \
-o /tmp/go-bpmn-${os_arch}.tar.gz \
https://github.com/gclaussn/go-bpmn/releases/download/${release_tag_name}/go-bpmn-${os_arch}.tar.gz

# unarchive release
rm -rf /tmp/go-bpmn-${os_arch}-${release_tag_name} && mkdir /tmp/go-bpmn-${os_arch}-${release_tag_name}
tar -xzf /tmp/go-bpmn-${os_arch}.tar.gz -C /tmp/go-bpmn-${os_arch}-${release_tag_name}

# unarchive current
rm -rf /tmp/go-bpmn-${os_arch} && mkdir /tmp/go-bpmn-${os_arch}
tar -xzf ../build/go-bpmn-${os_arch}.tar.gz -C /tmp/go-bpmn-${os_arch}

# create database
chmod +x /tmp/go-bpmn-${os_arch}-${release_tag_name}/go-bpmn-pgd
/tmp/go-bpmn-${os_arch}-${release_tag_name}/go-bpmn-pgd -create-api-key -secret-id test-worker1

# migrate database
chmod +x /tmp/go-bpmn-${os_arch}/go-bpmn-pgd
/tmp/go-bpmn-${os_arch}/go-bpmn-pgd -create-api-key -secret-id test-worker2
