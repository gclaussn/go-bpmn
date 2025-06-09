package client

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"

	"github.com/gclaussn/go-bpmn/engine"
	"github.com/gclaussn/go-bpmn/http/server"
)

func decodeJSONResponseBody(res *http.Response, v any) error {
	defer res.Body.Close()

	decoder := json.NewDecoder(res.Body)

	contentType := res.Header.Get(server.HeaderContentType)
	if contentType == server.ContentTypeProblemJson {
		var problem server.Problem
		if err := decoder.Decode(&problem); err != nil {
			return fmt.Errorf("failed to decode JSON problem response body: %v", err)
		}

		var errorType engine.ErrorType
		switch problem.Type {
		case server.ProblemTypeConflict:
			errorType = engine.ErrorConflict
		case server.ProblemTypeNotFound:
			errorType = engine.ErrorNotFound
		case server.ProblemTypeProcessModel:
			errorType = engine.ErrorProcessModel
		case server.ProblemTypeQuery:
			errorType = engine.ErrorQuery
		case server.ProblemTypeValidation:
			if len(problem.Errors) > 0 {
				return problem
			}
			errorType = engine.ErrorValidation
		default:
			return problem
		}

		return engine.Error{
			Type:   errorType,
			Title:  problem.Title,
			Detail: problem.Detail,
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
