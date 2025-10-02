package server

import (
	"encoding/json"
	"log"
	"net/http"

	"github.com/gclaussn/go-bpmn/engine"
	"github.com/gclaussn/go-bpmn/http/common"
)

func encodeJSONProblemResponseBody(w http.ResponseWriter, r *http.Request, err error) {
	problem, ok := err.(common.Problem)
	if !ok {
		engineErr, ok := err.(engine.Error)
		if !ok || engineErr.Type == 0 {
			log.Printf("%s %s: unexpected error occurred: %v", r.Method, r.RequestURI, err)

			problem = common.Problem{
				Status: http.StatusInternalServerError,
				Title:  "unexpected error occurred",
				Detail: "see server logs",
			}
		} else {
			var (
				status      int
				problemType common.ProblemType
			)

			switch engineErr.Type {
			case engine.ErrorConflict:
				status = http.StatusConflict
				problemType = common.ProblemConflict
			case engine.ErrorNotFound:
				status = http.StatusNotFound
				problemType = common.ProblemNotFound
			case engine.ErrorProcessModel:
				status = http.StatusUnprocessableEntity
				problemType = common.ProblemProcessModel
			case engine.ErrorQuery:
				status = http.StatusBadRequest
				problemType = common.ProblemQuery
			case engine.ErrorValidation:
				status = http.StatusBadRequest
				problemType = common.ProblemValidation
			default:
				status = http.StatusInternalServerError
			}

			errors := make([]common.Error, len(engineErr.Causes))
			for i, cause := range engineErr.Causes {
				errors[i] = common.Error{
					Pointer: cause.Pointer,
					Type:    cause.Type,
					Detail:  cause.Detail,
				}
			}

			problem = common.Problem{
				Status: status,
				Type:   problemType,
				Title:  engineErr.Title,
				Detail: engineErr.Detail,
				Errors: errors,
			}
		}
	}

	w.Header().Set(common.HeaderContentType, common.ContentTypeProblemJson)
	w.WriteHeader(problem.Status)

	if err := json.NewEncoder(w).Encode(problem); err != nil {
		log.Printf("%s %s: failed to create JSON problem response body: %v", r.Method, r.RequestURI, err)
		http.Error(w, "unexpected error occurred - see server logs", http.StatusInternalServerError)
	}
}

func encodeJSONResponseBody(w http.ResponseWriter, r *http.Request, v any, statusCode int) {
	w.Header().Set(common.HeaderContentType, common.ContentTypeJson)
	w.WriteHeader(statusCode)

	if err := json.NewEncoder(w).Encode(v); err != nil {
		log.Printf("%s %s: failed to create JSON response body: %v", r.Method, r.RequestURI, err)
		http.Error(w, "unexpected error occurred - see logs", http.StatusInternalServerError)
	}
}
