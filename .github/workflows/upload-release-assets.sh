#!/usr/bin/env bash

set -e

if [ "$#" -ne 1 ]; then
  echo "arg #1 must be a release ID" && exit 1
fi

RELEASE_ID="$1"

for path in ./build/*; do
  # e.g. ./build/go-bpmn-linux-amd64.tar.gz -> go-bpmn-linux-amd64.tar.gz
  file=$(basename "${path}")
  # e.g. go-bpmn-linux-amd64.tar.gz -> tar.gz
  file_ext="${file#*.}"

  if [[ "${file_ext}" == "tar.gz" ]]; then
    content_type="application/gzip"
  elif [[ "${file_ext}" == "sha256" ]]; then
    content_type="text/plain"
  elif [[ "${file_ext}" == "yaml" ]]; then
    content_type="text/yaml"
  else
    echo "unsupported file extension ${file_ext}" && exit 1
  fi

  curl -L \
    -X POST \
    -H "Accept: application/vnd.github+json" \
    -H "Authorization: Bearer ${GITHUB_TOKEN}" \
    -H "X-GitHub-Api-Version: 2022-11-28" \
    -H "Content-Type: ${content_type}" \
    "https://uploads.github.com/repos/gclaussn/go-bpmn/releases/${RELEASE_ID}/assets?name=${file}" \
    --data-binary "@${path}"
done
