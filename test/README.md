# Test

Docker compose based setup for exploratory testing, including a simple worker implementation.

## PG

```sh
# build
docker compose -f compose-pg.yaml build engine
docker compose -f compose-pg.yaml build worker

# create and start
docker compose -f compose-pg.yaml up -d

# stop and remove
docker compose -f compose-pg.yaml down -v
```

```sh
# print API key authorization
docker compose -f compose-pg.yaml exec engine cat /conf/authorization
```

## Mem

```sh
# build
docker compose -f compose-mem.yaml build engine
docker compose -f compose-mem.yaml build worker

# create and start
docker compose -f compose-mem.yaml up -d

# stop and remove
docker compose -f compose-mem.yaml down -v
```

## CLI

```sh
export GO_BPMN_URL="http://localhost:8080"
export GO_BPMN_AUTHORIZATION="$(docker compose -f compose-pg.yaml exec engine cat /conf/authorization)"
export GO_BPMN_DEBUG="true"

alias go-bpmn='go run cmd/go-bpmn/main.go'
```

Examples:

```sh
go-bpmn process create \
--bpmn-file test/bpmn/gateway/parallel-service-tasks.bpmn \
--bpmn-process-id parallelServiceTasksTest \
--version 1 \
--tag app=test

go-bpmn process-instance create \
--bpmn-process-id parallelServiceTasksTest \
--version 1 \
--variable x="{\"encoding\":\"text\",\"value\":\"text x\",\"encrypted\":true}"
--variable y="{\"encoding\":\"text\",\"value\":\"text y\"}"
```
