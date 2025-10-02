package client

import (
	"net/url"
	"strconv"
	"strings"

	"github.com/gclaussn/go-bpmn/engine"
	"github.com/gclaussn/go-bpmn/http/common"
)

func encodeQueryOptions(options engine.QueryOptions) string {
	values := make(url.Values)

	if options.Offset > 0 {
		values.Add(common.QueryOffset, strconv.Itoa(options.Offset))
	}
	if options.Limit > 0 {
		values.Add(common.QueryLimit, strconv.Itoa(options.Limit))
	}

	if len(values) == 0 {
		return ""
	}

	return "?" + values.Encode()
}

func resolve(path string, partition engine.Partition, id int32) string {
	path = strings.Replace(path, "{partition}", partition.String(), 1)
	return strings.Replace(path, "{id}", strconv.Itoa(int(id)), 1)
}
