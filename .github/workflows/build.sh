#!/usr/bin/env bash

set -e

if [ "$#" -ne 1 ]; then
  echo "arg #1 must be a tag name" && exit 1
fi

TAG_NAME="$1"

rm -rf ./build && mkdir ./build

export CGO_ENABLED=0

# generate OpenAPI YAML
go run cmd/openapi/main.go . ./build/go-bpmn-openapi.yaml

# build executables
goos=("linux" "windows")
goarch=("amd64" "amd64")

for i in ${!goos[@]}; do
  export GOOS="${goos[$i]}"
  export GOARCH="${goarch[$i]}"

  go build \
  -ldflags "-X main.version=${TAG_NAME}" \
  -o ./go-bpmn \
  ./cmd/go-bpmn

  go build \
  -ldflags "-X github.com/gclaussn/go-bpmn/daemon.version=${TAG_NAME}" \
  -o ./go-bpmn-memd \
  ./cmd/go-bpmn-memd

  go build \
  -ldflags "-X github.com/gclaussn/go-bpmn/daemon.version=${TAG_NAME}" \
  -o ./go-bpmn-pgd \
  ./cmd/go-bpmn-pgd

  tar cfz \
  ./build/go-bpmn-${GOOS}-${GOARCH}.tar.gz \
  go-bpmn go-bpmn-memd go-bpmn-pgd

  (cd ./build && sha256sum go-bpmn-${GOOS}-${GOARCH}.tar.gz > go-bpmn-${GOOS}-${GOARCH}.sha256)
done

ls -la ./build
