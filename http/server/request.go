package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"reflect"
	"regexp"
	"strconv"
	"strings"

	"github.com/adhocore/gronx"
	"github.com/gclaussn/go-bpmn/engine"
	"github.com/gclaussn/go-bpmn/http/common"
	"github.com/go-playground/validator/v10"
)

var (
	RegexpTagName      = regexp.MustCompile("^[a-zA-Z0-9_-]+$")
	RegexpVariableName = regexp.MustCompile("^[a-zA-Z0-9_-]+$")

	validate = newValidate()
)

func newValidate() *validator.Validate {
	validate := validator.New(validator.WithRequiredStructEnabled())
	validate.RegisterTagNameFunc(func(f reflect.StructField) string {
		return strings.SplitN(f.Tag.Get("json"), ",", 2)[0] // e.g. `json:"retryTimer,omitempty"` -> retryTimer
	})

	validate.RegisterValidation("cron", func(fl validator.FieldLevel) bool {
		v := fl.Field().String()
		if v == "" {
			return true
		}
		return gronx.IsValid(fl.Field().String())
	})
	validate.RegisterValidation("iso8601_duration", func(fl validator.FieldLevel) bool {
		_, err := engine.NewISO8601Duration(fl.Field().String())
		return err == nil
	})
	validate.RegisterValidation("tag_name", func(fl validator.FieldLevel) bool {
		return RegexpTagName.MatchString(fl.Field().String())
	})
	validate.RegisterValidation("timer", func(fl validator.FieldLevel) bool {
		timer, ok := fl.Field().Interface().(engine.Timer)
		if !ok {
			return false
		}
		return !timer.Time.IsZero() || timer.TimeCycle != "" || !timer.TimeDuration.IsZero()
	}, true)
	validate.RegisterValidation("variable_name", func(fl validator.FieldLevel) bool {
		return RegexpVariableName.MatchString(fl.Field().String())
	})

	return validate
}

// decodeJSONRequestBody decodes the request body using v and validates it.
// Media type, request body or validation related errors are returned as a Problem.
//
// inspired by https://www.alexedwards.net/blog/how-to-properly-parse-a-json-request-body
func decodeJSONRequestBody(w http.ResponseWriter, r *http.Request, v any) error {
	if contentType := r.Header.Get(common.HeaderContentType); contentType != "" {
		mediaType := strings.TrimSpace(strings.Split(contentType, ";")[0])
		if mediaType != common.ContentTypeJson {
			return common.Problem{
				Status: http.StatusUnsupportedMediaType,
				Type:   common.ProblemHttpMediaType,
				Title:  "unsupported media type",
				Detail: fmt.Sprintf("media type %s is not supported", mediaType),
			}
		}
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1048576) // 1mb = 1024 * 1024

	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()

	if err := decoder.Decode(&v); err != nil {
		var syntaxError *json.SyntaxError
		var unmarshalTypeError *json.UnmarshalTypeError

		problem := common.Problem{
			Status: http.StatusBadRequest,
			Type:   common.ProblemHttpRequestBody,
			Title:  "invalid request body",
		}

		switch {
		case errors.As(err, &syntaxError):
			problem.Detail = fmt.Sprintf("malformed JSON at position %d", syntaxError.Offset)
		case errors.Is(err, io.ErrUnexpectedEOF):
			problem.Detail = "unexpected end of JSON"
		case errors.As(err, &unmarshalTypeError):
			problem.Detail = fmt.Sprintf("JSON field %s has an invalid value at position %d", unmarshalTypeError.Field, unmarshalTypeError.Offset)
		case strings.HasPrefix(err.Error(), "json: unknown field "):
			fieldName := strings.TrimPrefix(err.Error(), "json: unknown field ")
			problem.Detail = fmt.Sprintf("unknown JSON field %s", fieldName)
		case errors.Is(err, io.EOF):
			problem.Detail = "request body is empty"
		case err.Error() == "http: request body too large":
			problem.Detail = "request body size must not exceed 1MB"
		default:
			problem.Detail = fmt.Sprintf("failed to unmarshal JSON: %v", err)
		}

		return problem
	}

	if err := validate.Struct(v); err != nil {
		errors := make([]common.Error, 0)
		for _, fieldError := range err.(validator.ValidationErrors) {
			var (
				pointerBuilder strings.Builder
				next           rune
			)
			for _, r := range fieldError.Namespace() {
				if pointerBuilder.Len() == 0 {
					// skip until first dot
					if r == '.' {
						pointerBuilder.WriteString("#/")
					}
					continue
				}

				switch r {
				case '.':
					if next != '/' {
						next = '/'
					} else {
						next = '.'
					}
				case '[':
					next = '/'
				case ']':
					continue
				default:
					next = r
				}

				pointerBuilder.WriteRune(next)
			}

			var (
				detail string
				value  string
			)
			switch fieldError.Tag() {
			case "gte":
				detail = fmt.Sprintf("must be greater than or equal to %s", fieldError.Param())
				value = fmt.Sprintf("%d", fieldError.Value())
			case "lte":
				detail = fmt.Sprintf("must be less than or equal to %s", fieldError.Param())
				value = fmt.Sprintf("%d", fieldError.Value())
			case "max":
				detail = fmt.Sprintf("exceeds a maximum of %s", fieldError.Param())
				value = fmt.Sprintf("%v", fieldError.Value())
			case "required":
				detail = "is required"
			case "unique":
				detail = "must be unique"
				value = fmt.Sprintf("%v", fieldError.Value())
			// custom validation
			case "cron":
				detail = "is invalid"
				value = fmt.Sprintf("%s", fieldError.Value())
			case "iso8601_duration":
				detail = "is invalid"
				value = fmt.Sprintf("%s", fieldError.Value())
			case "tag_name":
				detail = fmt.Sprintf("must match regex %s", RegexpTagName)
				value = fmt.Sprintf("%s", fieldError.Value())
			case "timer":
				detail = "must specify a time, time cycle or time duration"
			case "variable_name":
				detail = fmt.Sprintf("must match regex %s", RegexpVariableName)
				value = fmt.Sprintf("%s", fieldError.Value())
			default:
				detail = "unknown error"
				value = fmt.Sprintf("%v", fieldError.Value())
			}

			errors = append(errors, common.Error{
				Pointer: pointerBuilder.String(),
				Type:    fieldError.Tag(),
				Detail:  detail,
				Value:   value,
			})
		}

		return common.Problem{
			Status: http.StatusBadRequest,
			Type:   common.ProblemHttpRequestBody,
			Title:  "invalid request body",
			Detail: "failed to validate request body",
			Errors: errors,
		}
	}

	return nil
}

func parseId(r *http.Request) (int32, error) {
	idValue := r.PathValue("id")
	id, err := strconv.ParseInt(idValue, 10, 32)
	if err != nil {
		return 0, common.Problem{
			Status: http.StatusBadRequest,
			Type:   common.ProblemHttpRequestUri,
			Title:  "invalid path parameter id",
			Detail: fmt.Sprintf("failed to parse value '%s'", idValue),
		}
	}
	if id < 1 {
		return 0, common.Problem{
			Status: http.StatusBadRequest,
			Type:   common.ProblemValidation,
			Title:  "invalid path parameter id",
			Detail: fmt.Sprintf("ID %s must be greater than 0", idValue),
		}
	}
	return int32(id), nil
}

func parsePartitionId(r *http.Request) (engine.Partition, int32, error) {
	partitionValue := r.PathValue("partition")
	partition, err := engine.NewPartition(partitionValue)
	if err != nil {
		return engine.Partition{}, 0, common.Problem{
			Status: http.StatusBadRequest,
			Type:   common.ProblemHttpRequestUri,
			Title:  "invalid path parameter partition",
			Detail: fmt.Sprintf("failed to parse value '%s'", partitionValue),
		}
	}

	id, err := parseId(r)
	if err != nil {
		return engine.Partition{}, 0, err
	}

	return partition, id, nil
}

func parseQueryOptions(r *http.Request) (engine.QueryOptions, error) {
	var (
		err error

		limit  int64
		offset int64
	)

	if limitValues, ok := r.URL.Query()[common.QueryLimit]; ok {
		limit, err = strconv.ParseInt(limitValues[0], 10, 32)
		if err != nil {
			return engine.QueryOptions{}, common.Problem{
				Status: http.StatusBadRequest,
				Type:   common.ProblemHttpRequestUri,
				Title:  "invalid query parameter " + common.QueryLimit,
				Detail: "failed to parse value " + limitValues[0],
			}
		}
		if limit < 0 {
			return engine.QueryOptions{}, common.Problem{
				Status: http.StatusBadRequest,
				Type:   common.ProblemValidation,
				Title:  "invalid query parameter " + common.QueryLimit,
				Detail: fmt.Sprintf("%s %d must be greater than or equal to 0", common.QueryLimit, limit),
			}
		}
	}

	if offsetValues, ok := r.URL.Query()[common.QueryOffset]; ok {
		offset, err = strconv.ParseInt(offsetValues[0], 10, 32)
		if err != nil {
			return engine.QueryOptions{}, common.Problem{
				Status: http.StatusBadRequest,
				Type:   common.ProblemHttpRequestUri,
				Title:  "invalid query parameter " + common.QueryOffset,
				Detail: "failed to parse value " + offsetValues[0],
			}
		}
		if offset < 0 {
			return engine.QueryOptions{}, common.Problem{
				Status: http.StatusBadRequest,
				Type:   common.ProblemValidation,
				Title:  "invalid query parameter " + common.QueryOffset,
				Detail: fmt.Sprintf("%s %d must be greater than or equal to 0", common.QueryOffset, offset),
			}
		}
	}

	return engine.QueryOptions{
		Limit:  int(limit),
		Offset: int(offset),
	}, nil
}
