package client

import (
	"testing"

	"github.com/gclaussn/go-bpmn/engine"
	"github.com/stretchr/testify/assert"
)

func TestEncodeQueryOptions(t *testing.T) {
	assert := assert.New(t)

	assert.Equal("", encodeQueryOptions(engine.QueryOptions{}))
	assert.Equal("", encodeQueryOptions(engine.QueryOptions{Limit: -1, Offset: -1}))
	assert.Equal("", encodeQueryOptions(engine.QueryOptions{Limit: 0, Offset: 0}))
	assert.Equal("?limit=1&offset=1", encodeQueryOptions(engine.QueryOptions{Limit: 1, Offset: 1}))
	assert.Equal("?limit=2", encodeQueryOptions(engine.QueryOptions{Limit: 2}))
	assert.Equal("?offset=3", encodeQueryOptions(engine.QueryOptions{Offset: 3}))
}
