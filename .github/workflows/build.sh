#!/usr/bin/env bash

set -e

if [ "$#" -ne 1 ]; then
  echo "arg #1 must be a release tag name" && exit 1
fi

RELEASE_TAG_NAME="$1"

# download openapi.yaml
curl -L \
-o openapi.yaml \
https://github.com/gclaussn/go-bpmn/releases/download/${RELEASE_TAG_NAME}/go-bpmn-openapi.yaml

# build OpenAPI docs
npx @redocly/cli build-docs \
--title "go-bpmn HTTP API (${RELEASE_TAG_NAME})" \
--output openapi.html \
openapi.yaml

# build Github page
npm run build

# include OpenAPI docs
mv openapi.html ./dist/
