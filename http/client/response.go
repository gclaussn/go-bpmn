package client

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"

	"github.com/gclaussn/go-bpmn/engine"
	"github.com/gclaussn/go-bpmn/http/common"
)

func decodeJSONResponseBody(res *http.Response, v any) error {
	defer res.Body.Close()

	decoder := json.NewDecoder(res.Body)

	contentType := res.Header.Get(common.HeaderContentType)
	if contentType == common.ContentTypeProblemJson {
		var problem common.Problem
		if err := decoder.Decode(&problem); err != nil {
			return fmt.Errorf("failed to decode JSON problem response body: %v", err)
		}

		var errorType engine.ErrorType
		switch problem.Type {
		case common.ProblemConflict:
			errorType = engine.ErrorConflict
		case common.ProblemNotFound:
			errorType = engine.ErrorNotFound
		case common.ProblemProcessModel:
			errorType = engine.ErrorProcessModel
		case common.ProblemQuery:
			errorType = engine.ErrorQuery
		case common.ProblemValidation:
			errorType = engine.ErrorValidation
		default:
			return problem
		}

		causes := make([]engine.ErrorCause, len(problem.Errors))
		for i, e := range problem.Errors {
			causes[i] = engine.ErrorCause{
				Pointer: e.Pointer,
				Type:    e.Type,
				Detail:  e.Detail,
			}
		}

		return engine.Error{
			Type:   errorType,
			Title:  problem.Title,
			Detail: problem.Detail,
			Causes: causes,
		}
	}

	if res.StatusCode >= 300 {
		text := fmt.Sprintf(
			"%s %s: HTTP %d",
			http.MethodPost,
			res.Request.URL.Path,
			res.StatusCode,
		)

		b, err := io.ReadAll(res.Body)
		if err != nil {
			return fmt.Errorf("%s: %v", text, err)
		} else if len(b) != 0 {
			return fmt.Errorf("%s: %s", text, string(b))
		} else {
			return errors.New(text)
		}
	}

	if v == nil {
		return nil
	}
	if err := decoder.Decode(&v); err != nil {
		return fmt.Errorf("failed to decode JSON response body: %v", err)
	}

	return nil
}
