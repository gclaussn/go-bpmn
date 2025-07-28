# Development

## Testing

```sh
# start test database container
docker run --rm -d -p 5432:5432 -e POSTGRES_USER=postgres -e POSTGRES_PASSWORD=postgres -e POSTGRES_DB=test postgres:15.2-alpine

# set database URL for testing
export GO_BPMN_TEST_DATABASE_URL="postgres://postgres:postgres@localhost:5432/test"
```

```sh
# run tests, which do not rely on a database
go test -v ./... -short

# run all tests
go test -v ./...

# create coverage report
go test -v -coverpkg=./... -coverprofile coverage.out ./...
go tool cover -html coverage.out -o coverage.html
```

## Building

```sh
# CLI
CGO_ENABLED=0 go build -o go-bpmn ./cmd/go-bpmn

# mem engine daemon
CGO_ENABLED=0 go build -o go-bpmn-memd ./cmd/go-bpmn-memd

# pg engine daemon
CGO_ENABLED=0 go build -o go-bpmn-pgd ./cmd/go-bpmn-pgd
```

## OpenAPI

```sh
# generate YAML
go run cmd/openapi/main.go -output-path openapi.yaml
```
