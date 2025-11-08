package common

import (
	"fmt"
	"strings"
)

// ProblemType determines if a problem is HTTP and engine related.
type ProblemType int

const (
	ProblemHttpMediaType ProblemType = iota + 1
	ProblemHttpRequestBody
	ProblemHttpRequestUri

	// engine error types
	ProblemConflict
	ProblemNotFound
	ProblemProcessModel
	ProblemQuery
	ProblemValidation
)

func MapProblemType(s string) ProblemType {
	switch s {
	case "HTTP_MEDIA_TYPE":
		return ProblemHttpMediaType
	case "HTTP_REQUEST_BODY":
		return ProblemHttpRequestBody
	case "HTTP_REQUEST_URI":
		return ProblemHttpRequestUri
	case "VALIDATION":
		return ProblemValidation
	case "CONFLICT":
		return ProblemConflict
	case "NOT_FOUND":
		return ProblemNotFound
	case "PROCESS_MODEL":
		return ProblemProcessModel
	case "QUERY":
		return ProblemQuery
	default:
		return 0
	}
}

func (v ProblemType) MarshalJSON() ([]byte, error) {
	return []byte(fmt.Sprintf("%q", v.String())), nil
}

func (v ProblemType) String() string {
	switch v {
	case ProblemHttpMediaType:
		return "HTTP_MEDIA_TYPE"
	case ProblemHttpRequestBody:
		return "HTTP_REQUEST_BODY"
	case ProblemHttpRequestUri:
		return "HTTP_REQUEST_URI"
	case ProblemValidation:
		return "VALIDATION"
	case ProblemConflict:
		return "CONFLICT"
	case ProblemNotFound:
		return "NOT_FOUND"
	case ProblemProcessModel:
		return "PROCESS_MODEL"
	case ProblemQuery:
		return "QUERY"
	default:
		return "UNKNOWN"
	}
}

func (v *ProblemType) UnmarshalJSON(data []byte) error {
	s := string(data)
	*v = MapProblemType(s[1 : len(s)-1])
	return nil
}

// Common format for HTTP 4xx error responses, based on https://datatracker.ietf.org/doc/html/rfc9457.
type Problem struct {
	Status int         `json:"status" validate:"required"` // HTTP status code.
	Type   ProblemType `json:"type" validate:"required"`   // Problem type.
	Title  string      `json:"title" validate:"required"`  // Human-readable problem summary.
	Detail string      `json:"detail" validate:"required"` // Human-readable, detailed information about the problem.
	Errors []Error     `json:"errors,omitempty"`           // Validation errors.
}

func (v Problem) Error() string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("HTTP %d: %s: %s: %s", v.Status, v.Type, v.Title, v.Detail))

	for i := range v.Errors {
		sb.WriteRune('\n')
		sb.WriteString(v.Errors[i].String())
	}

	return sb.String()
}

// Error represents a failed validation, pointing on a JSON property or BPMN element.
type Error struct {
	// A pointer, locating the invalid JSON property or BPMN element.
	Pointer string `json:"pointer" validate:"required"`
	// Error type.
	//
	// JSON property related values:
	//   - `cron`: value is not a valid CRON expression
	//   - `gte`: value must be greater than or equal to
	//   - `lte`: value must be less than or equal to
	//   - `max`: array exceeds a maximum of number of items
	//   - `required`: value is required
	//   - `unique`: items of an array must be unique
	//   - `iso8601_duration`: value is not a valid ISO 8601 duration
	//   - `tag_name`: key is not a valid tag name
	//   - `timer`: timer must specify a time, time cycle or time duration
	//   - `variable_name`: key is not a valid variable name
	//
	// BPMN related values:
	//   - `process` indicates an error on process level
	//   - `element` indicates an error on element level
	//   - `sequence_flow` indicates faulty sequence flow
	//
	// BPMN event related values:
	//   - `error_event`: invalid error event definition
	//   - `escalation_event`: invalid escalation event definition
	//   - `message_event`: a missing or invalid message event definition
	//   - `signal_event`: a missing or invalid signal event definition
	//   - `timer_event`: a missing or invalid timer event definition
	Type string `json:"type" validate:"required"`
	// Human-readable, detailed information about the error.
	Detail string `json:"detail" validate:"required"`
	// Value or key that caused the validation error.
	Value string `json:"value,omitempty"`
}

func (v Error) String() string {
	return fmt.Sprintf("%s: %s", v.Pointer, v.Detail)
}
