package server

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gclaussn/go-bpmn/engine"
	"github.com/gclaussn/go-bpmn/http/common"
	"github.com/stretchr/testify/assert"
)

func TestDecodeJSONRequestBody(t *testing.T) {
	assert := assert.New(t)

	validJson := `
	{
		"vgte": 1,
		"viso8601Duration": "",
		"vlte": 100,
		"vmax": [1, 2, 3],
		"vrequired": "a string",
		"vtags": [
			{"name": "a", "value": "v"},
			{"name": "z", "value": "v"},
			{"name": "A", "value": "v"},
			{"name": "Z", "value": "v"},
			{"name": "0", "value": "v"},
			{"name": "9", "value": "v"},
			{"name": "_", "value": "v"},
			{"name": "-", "value": "v"}
		],
		"vunique": [1, 2, 3],
		"vvariables": [
			{"name": "a", "data": {"encoding": "text", "value": "a text"}},
			{"name": "z", "data": {"encoding": "text", "value": "a text"}},
			{"name": "A", "data": {"encoding": "text", "value": "a text"}},
			{"name": "Z", "data": {"encoding": "text", "value": "a text"}},
			{"name": "0", "data": {"encoding": "text", "value": "a text"}},
			{"name": "9", "data": {"encoding": "text", "value": "a text"}},
			{"name": "_", "data": {"encoding": "text", "value": "a text"}},
			{"name": "-", "data": {"encoding": "text", "value": "a text"}}
		]
	}
	`

	invalidJson := `
	{
		"vgte": 0,
		"viso8601Duration": "invalid",
		"vlte": 101,
		"vmax": [1, 2, 3, 4],
		"vrequired": "",
		"vtags": [
			{"name": "", "value": "v"},
			{"name": " ", "value": "v"},
			{"name": ".", "value": "v"},
			{"name": "a", "value": ""}
		],
		"vunique": [1, 1, 2, 2, 3],
		"vvariables": [
			{"name": "", "data": null},
			{"name": " ", "data": {"encoding": "text", "value": "a text"}},
			{"name": ".", "data": {"encoding": "text", "value": "a text"}},
			{"name": "a", "data": {"value": "123"}}
		]
	}
	`

	t.Run("unsupported media type", func(t *testing.T) {
		w := httptest.NewRecorder()
		r := httptest.NewRequest("", "/", strings.NewReader(validJson))
		r.Header.Add(common.HeaderContentType, "text/plain")

		var body DecodeTest
		err := decodeJSONRequestBody(w, r, &body)
		assertProblem(t, err, common.ProblemHttpMediaType, http.StatusUnsupportedMediaType)
		assert.Contains(err.Error(), "text/plain")
	})

	t.Run("request body is empty", func(t *testing.T) {
		w := httptest.NewRecorder()
		r := httptest.NewRequest("", "/", strings.NewReader(""))

		var body DecodeTest
		err := decodeJSONRequestBody(w, r, &body)
		assertProblem(t, err, common.ProblemHttpRequestBody, http.StatusBadRequest)
		assert.Contains(err.Error(), "request body is empty")
	})

	t.Run("request body too large", func(t *testing.T) {
		var jsonBuilder strings.Builder
		jsonBuilder.WriteString(`{"vstring":"`)
		jsonBuilder.WriteString(strings.Repeat("x", 1024*1014*2))
		jsonBuilder.WriteString(`"}`)

		b := []byte(jsonBuilder.String())

		w := httptest.NewRecorder()
		r := httptest.NewRequest("", "/", bytes.NewReader(b))

		var body DecodeTest
		err := decodeJSONRequestBody(w, r, &body)
		assertProblem(t, err, common.ProblemHttpRequestBody, http.StatusBadRequest)
		assert.Contains(err.Error(), "1MB")
	})

	t.Run("malformed JSON", func(t *testing.T) {
		w := httptest.NewRecorder()
		r := httptest.NewRequest("", "/", strings.NewReader("{_}"))

		var body DecodeTest
		err := decodeJSONRequestBody(w, r, &body)
		assertProblem(t, err, common.ProblemHttpRequestBody, http.StatusBadRequest)
		assert.Contains(err.Error(), "at position 2")
	})

	t.Run("unexpected end of JSON", func(t *testing.T) {
		w := httptest.NewRecorder()
		r := httptest.NewRequest("", "/", strings.NewReader("{"))

		var body DecodeTest
		err := decodeJSONRequestBody(w, r, &body)
		assertProblem(t, err, common.ProblemHttpRequestBody, http.StatusBadRequest)
		assert.Contains(err.Error(), "unexpected end of JSON")
	})

	t.Run("invalid JSON field", func(t *testing.T) {
		w := httptest.NewRecorder()
		r := httptest.NewRequest("", "/", strings.NewReader(`{"vrequired":1}`))

		var body DecodeTest
		err := decodeJSONRequestBody(w, r, &body)
		assertProblem(t, err, common.ProblemHttpRequestBody, http.StatusBadRequest)
		assert.Contains(err.Error(), "JSON field vrequired has an invalid value at position 14")
	})

	t.Run("unknown JSON field", func(t *testing.T) {
		w := httptest.NewRecorder()
		r := httptest.NewRequest("", "/", strings.NewReader(`{"vunknown":-1}`))

		var body DecodeTest
		err := decodeJSONRequestBody(w, r, &body)
		assertProblem(t, err, common.ProblemHttpRequestBody, http.StatusBadRequest)
		assert.Contains(err.Error(), `unknown JSON field "vunknown"`)
	})

	t.Run("valid JSON", func(t *testing.T) {
		w := httptest.NewRecorder()
		r := httptest.NewRequest("", "/", strings.NewReader(validJson))

		var body DecodeTest
		err := decodeJSONRequestBody(w, r, &body)
		assert.Nil(err)

		assert.Equal(1, body.VGte)
		assert.Equal("", string(body.VISO8601Duration))
		assert.Equal(100, body.VLte)
		assert.Equal("a string", body.VRequired)
		assert.Len(body.VTags, 8)
		assert.Len(body.VVariables, 8)
	})

	t.Run("invalid JSON", func(t *testing.T) {
		w := httptest.NewRecorder()
		r := httptest.NewRequest("", "/", strings.NewReader(invalidJson))

		var body DecodeTest
		err := decodeJSONRequestBody(w, r, &body)
		assertProblem(t, err, common.ProblemHttpRequestBody, http.StatusBadRequest)

		problem := err.(common.Problem)
		assert.Len(problem.Errors, 14)

		findError := func(pointer string) common.Error {
			for i := range problem.Errors {
				if problem.Errors[i].Pointer == pointer {
					return problem.Errors[i]
				}
			}
			t.Fatalf("failed to find error for pointer %s", pointer)
			return common.Error{}
		}

		var e common.Error

		e = findError("#/vgte")
		assert.Equal("gte", e.Type)
		assert.NotEmpty(e.Detail)
		assert.Equal("0", e.Value)

		e = findError("#/viso8601Duration")
		assert.Equal("iso8601_duration", e.Type)
		assert.NotEmpty(e.Detail)
		assert.Equal("invalid", e.Value)

		e = findError("#/vlte")
		assert.Equal("lte", e.Type)
		assert.NotEmpty(e.Detail)
		assert.Equal("101", e.Value)

		e = findError("#/vmax")
		assert.Equal("max", e.Type)
		assert.NotEmpty(e.Detail)
		assert.Equal("[1 2 3 4]", e.Value)

		e = findError("#/vrequired")
		assert.Equal("required", e.Type)
		assert.NotEmpty(e.Detail)
		assert.Empty(e.Value)

		e = findError("#/vunique")
		assert.Equal("unique", e.Type)
		assert.NotEmpty(e.Detail)
		assert.Equal("[1 1 2 2 3]", e.Value)

		// tag_name
		e = findError("#/vtags/0/name")
		assert.Equal("required", e.Type)
		assert.NotEmpty(e.Detail)
		assert.Equal("", e.Value)

		e = findError("#/vtags/1/name")
		assert.Equal("tag_name", e.Type)
		assert.Equal(" ", e.Value)
		e = findError("#/vtags/2/name")
		assert.Equal("tag_name", e.Type)
		assert.Equal(".", e.Value)

		e = findError("#/vtags/3/value")
		assert.Equal("required", e.Type)
		assert.Empty(e.Value)

		// variable_name
		e = findError("#/vvariables/0/name")
		assert.Equal("required", e.Type)
		assert.NotEmpty(e.Detail)
		assert.Equal("", e.Value)

		e = findError("#/vvariables/1/name")
		assert.Equal("variable_name", e.Type)
		assert.Equal(" ", e.Value)
		e = findError("#/vvariables/2/name")
		assert.Equal("variable_name", e.Type)
		assert.Equal(".", e.Value)

		e = findError("#/vvariables/3/data/encoding")
		assert.Equal("required", e.Type)
		assert.Empty(e.Value)
	})
}

func TestDecodeJSONRequestBodyTimer(t *testing.T) {
	assert := assert.New(t)

	var body DecodeTimerTest

	t.Run("null", func(t *testing.T) {
		w := httptest.NewRecorder()
		r := httptest.NewRequest("", "/", strings.NewReader(`{"vtimer": null}`))

		err := decodeJSONRequestBody(w, r, &body)
		assertProblem(t, err, common.ProblemHttpRequestBody, http.StatusBadRequest)

		problem := err.(common.Problem)
		assert.Lenf(problem.Errors, 1, "expected one error")

		e := problem.Errors[0]
		assert.Equal("#/vtimer", e.Pointer)
		assert.Equal("timer", e.Type)
		assert.NotEmpty(e.Detail)
		assert.Empty(e.Value)
	})

	t.Run("empty", func(t *testing.T) {
		w := httptest.NewRecorder()
		r := httptest.NewRequest("", "/", strings.NewReader(`{"vtimer": {}}`))

		err := decodeJSONRequestBody(w, r, &body)
		assertProblem(t, err, common.ProblemHttpRequestBody, http.StatusBadRequest)

		problem := err.(common.Problem)
		assert.Lenf(problem.Errors, 1, "expected one error")

		e := problem.Errors[0]
		assert.Equal("#/vtimer", e.Pointer)
		assert.Equal("timer", e.Type)
		assert.NotEmpty(e.Detail)
		assert.Empty(e.Value)
	})

	t.Run("valid time", func(t *testing.T) {
		w := httptest.NewRecorder()
		r := httptest.NewRequest("", "/", strings.NewReader(`{"vtimer": {"time": "2025-06-14T06:29:10Z"}}`))

		err := decodeJSONRequestBody(w, r, &body)
		assert.Nil(err)

		assert.False(body.VTimer.Time.IsZero())
	})

	t.Run("valid time cycle", func(t *testing.T) {
		w := httptest.NewRecorder()
		r := httptest.NewRequest("", "/", strings.NewReader(`{"vtimer": {"timeCycle": "* * * * *"}}`))

		err := decodeJSONRequestBody(w, r, &body)
		assert.Nil(err)

		assert.Equal("* * * * *", body.VTimer.TimeCycle)
	})

	t.Run("valid time", func(t *testing.T) {
		w := httptest.NewRecorder()
		r := httptest.NewRequest("", "/", strings.NewReader(`{"vtimer": {"timeDuration": "PT1H"}}`))

		err := decodeJSONRequestBody(w, r, &body)
		assert.Nil(err)

		assert.Equal("PT1H", body.VTimer.TimeDuration.String())
	})

	t.Run("invalid time cycle", func(t *testing.T) {
		w := httptest.NewRecorder()
		r := httptest.NewRequest("", "/", strings.NewReader(`{"vtimer": {"timeCycle": "*"}}`))

		err := decodeJSONRequestBody(w, r, &body)

		problem := err.(common.Problem)
		assert.Lenf(problem.Errors, 1, "expected one error")

		e := problem.Errors[0]
		assert.Equal("#/vtimer/timeCycle", e.Pointer)
		assert.Equal("cron", e.Type)
		assert.NotEmpty(e.Detail)
		assert.Equal("*", e.Value)
	})
}

func TestParseId(t *testing.T) {
	assert := assert.New(t)

	t.Run("valid", func(t *testing.T) {
		r := httptest.NewRequest("", "/", nil)
		r.SetPathValue("id", "1")

		id, err := parseId(r)
		assert.Equal(int32(1), id)
		assert.Nilf(err, "expected no error")
	})

	t.Run("failed to parse value", func(t *testing.T) {
		r := httptest.NewRequest("", "/", nil)
		r.SetPathValue("id", "x")

		_, err := parseId(r)
		assert.NotNilf(err, "expected error")
	})

	t.Run("must be greater than 0", func(t *testing.T) {
		r := httptest.NewRequest("", "/", nil)
		r.SetPathValue("id", "0")

		_, err := parseId(r)
		assert.NotNilf(err, "expected error")
	})
}

func TestParsePartitionId(t *testing.T) {
	assert := assert.New(t)

	t.Run("valid", func(t *testing.T) {
		r := httptest.NewRequest("", "/", nil)
		r.SetPathValue("partition", "2024-12-24")
		r.SetPathValue("id", "123")

		partition, id, err := parsePartitionId(r)
		assert.Equal("2024-12-24", partition.String())
		assert.Equal(int32(123), id)
		assert.Nilf(err, "expected no error")
	})

	t.Run("failed to parse value", func(t *testing.T) {
		r := httptest.NewRequest("", "/", nil)
		r.SetPathValue("partition", "x")
		r.SetPathValue("id", "1")

		_, _, err := parsePartitionId(r)
		assert.NotNilf(err, "expected error")
	})
}

func TestParseQueryOptions(t *testing.T) {
	assert := assert.New(t)

	t.Run("valid", func(t *testing.T) {
		r := httptest.NewRequest("", "/?limit=50&offset=100", nil)

		queryOptions, err := parseQueryOptions(r)
		assert.Equal(50, queryOptions.Limit)
		assert.Equal(100, queryOptions.Offset)
		assert.Nilf(err, "expected no error")
	})

	t.Run("limit", func(t *testing.T) {
		t.Run("failed to parse value", func(t *testing.T) {
			r := httptest.NewRequest("", "/?limit=x", nil)

			_, err := parseQueryOptions(r)
			assert.NotNilf(err, "expected error")
		})

		t.Run("must be greater than or equal to 0", func(t *testing.T) {
			r := httptest.NewRequest("", "/?limit=-1", nil)

			_, err := parseQueryOptions(r)
			assert.NotNilf(err, "expected error")
		})
	})

	t.Run("offset", func(t *testing.T) {
		t.Run("failed to parse value", func(t *testing.T) {
			r := httptest.NewRequest("", "/?offset=x", nil)

			_, err := parseQueryOptions(r)
			assert.NotNilf(err, "expected error")
		})

		t.Run("must be greater than or equal to 0", func(t *testing.T) {
			r := httptest.NewRequest("", "/?offset=-1", nil)

			_, err := parseQueryOptions(r)
			assert.NotNilf(err, "expected error")
		})
	})
}

func assertProblem(t *testing.T, err error, expectedType common.ProblemType, expectedStatus int) {
	if err == nil {
		t.Fatal("error is nil")
	}

	problem, ok := err.(common.Problem)
	if !ok {
		t.Fatalf("error is not of type Problem: %v", err)
	}

	assert := assert.New(t)
	assert.Equal(expectedType, problem.Type)
	assert.Equal(expectedStatus, problem.Status)
	assert.NotEmpty(problem.Title)
	assert.NotEmpty(problem.Detail)
}

type DecodeTest struct {
	VGte             int                    `json:"vgte" validate:"gte=1"`
	VISO8601Duration engine.ISO8601Duration `json:"viso8601Duration" validate:"iso8601_duration"`
	VLte             int                    `json:"vlte" validate:"lte=100"`
	VMax             []int                  `json:"vmax" validate:"max=3"`
	VRequired        string                 `json:"vrequired" validate:"required"`
	VTags            []engine.Tag           `json:"vtags" validate:"dive"`
	VUnique          []int                  `json:"vunique" validate:"unique"`
	VVariables       []*engine.VariableData `json:"vvariables" validate:"dive"`
}

type DecodeTimerTest struct {
	VTimer *engine.Timer `json:"vtimer" validate:"timer"`
}
