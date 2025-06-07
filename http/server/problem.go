package server

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"

	"github.com/gclaussn/go-bpmn/engine"
)

type ProblemType int

const (
	ProblemTypeHttpMediaType ProblemType = iota + 1
	ProblemTypeHttpRequestBody
	ProblemTypeHttpRequestUri
	ProblemTypeValidation

	// engine error types
	ProblemTypeConflict
	ProblemTypeNotFound
	ProblemTypeProcessModel
	ProblemTypeQuery
)

func MapProblemType(s string) ProblemType {
	switch s {
	case "HTTP_MEDIA_TYPE":
		return ProblemTypeHttpMediaType
	case "HTTP_REQUEST_BODY":
		return ProblemTypeHttpRequestBody
	case "HTTP_REQUEST_URI":
		return ProblemTypeHttpRequestUri
	case "VALIDATION":
		return ProblemTypeValidation
	case "CONFLICT":
		return ProblemTypeConflict
	case "NOT_FOUND":
		return ProblemTypeNotFound
	case "PROCESS_MODEL":
		return ProblemTypeProcessModel
	case "QUERY":
		return ProblemTypeQuery
	default:
		return 0
	}
}

func (v ProblemType) MarshalJSON() ([]byte, error) {
	return []byte(fmt.Sprintf("%q", v.String())), nil
}

func (v ProblemType) String() string {
	switch v {
	case ProblemTypeHttpMediaType:
		return "HTTP_MEDIA_TYPE"
	case ProblemTypeHttpRequestBody:
		return "HTTP_REQUEST_BODY"
	case ProblemTypeHttpRequestUri:
		return "HTTP_REQUEST_URI"
	case ProblemTypeValidation:
		return "VALIDATION"
	case ProblemTypeConflict:
		return "CONFLICT"
	case ProblemTypeNotFound:
		return "NOT_FOUND"
	case ProblemTypeProcessModel:
		return "PROCESS_MODEL"
	case ProblemTypeQuery:
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

	Errors []Error `json:"errors,omitempty"` // Validation errors - only set if problem type is `VALIDATION`.
}

func (v Problem) Error() string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("HTTP %d: %s: %s: %s", v.Status, v.Type, v.Title, v.Detail))

	for i := 0; i < len(v.Errors); i++ {
		sb.WriteRune('\n')
		sb.WriteString(v.Errors[i].String())
	}

	return sb.String()
}

// Error represents a validation error, pointing on a JSON property.
type Error struct {
	// A JSON pointer, locating the invalid property.
	Pointer string `json:"pointer" validate:"required"`
	// Error type.
	//
	// Possible values:
	//   - `gte`: value must be greater than or equal to
	//   - `lte`: value must be less than or equal to
	//   - `max`: array exceeds a maximum of number of items
	//   - `required`: value is required
	//   - `unique`: items of an array must be unique
	//   - `iso8601_duration`: value is not a valid ISO 8601 duration
	//   - `tag_name`: key is not a valid tag name
	//   - `variable_name`: key is not a valid variable name
	Type string `json:"type" validate:"required"`
	// Human-readable, detailed information about the error.
	Detail string `json:"detail" validate:"required"`
	// Value or key that caused the validation error.
	Value string `json:"value,omitempty"`
}

func (v Error) String() string {
	return fmt.Sprintf("%s: %s", v.Pointer, v.Detail)
}

func encodeJSONProblemResponseBody(w http.ResponseWriter, r *http.Request, err error) {
	problem, ok := err.(Problem)
	if !ok {
		engineErr, ok := err.(engine.Error)
		if !ok || engineErr.Type == 0 {
			log.Printf("%s %s: unexpected error occurred: %v", r.Method, r.RequestURI, err)

			problem = Problem{
				Status: http.StatusInternalServerError,
				Title:  "unexpected error occurred",
				Detail: "see server logs",
			}
		} else {
			var (
				status      int
				problemType ProblemType
			)

			switch engineErr.Type {
			case engine.ErrorConflict:
				status = http.StatusConflict
				problemType = ProblemTypeConflict
			case engine.ErrorNotFound:
				status = http.StatusNotFound
				problemType = ProblemTypeNotFound
			case engine.ErrorProcessModel:
				status = http.StatusUnprocessableEntity
				problemType = ProblemTypeProcessModel
			case engine.ErrorQuery:
				status = http.StatusBadRequest
				problemType = ProblemTypeQuery
			default:
				status = http.StatusInternalServerError
			}

			problem = Problem{
				Status: status,
				Type:   problemType,
				Title:  engineErr.Title,
				Detail: engineErr.Detail,
			}
		}
	}

	w.Header().Set(HeaderContentType, ContentTypeProblemJson)
	w.WriteHeader(problem.Status)

	if err := json.NewEncoder(w).Encode(problem); err != nil {
		log.Printf("%s %s: failed to create JSON problem response body: %v", r.Method, r.RequestURI, err)
		http.Error(w, "unexpected error occurred - see server logs", http.StatusInternalServerError)
	}
}
