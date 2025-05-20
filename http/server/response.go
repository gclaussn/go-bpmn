package server

import (
	"encoding/json"
	"log"
	"net/http"

	"github.com/gclaussn/go-bpmn/engine"
)

// Response of a batch operation like the unlocking of jobs or tasks.
type CountRes struct {
	Count int `json:"count"` // The number of affected entities.
}

// Response of a task execution.
type ExecuteTasksRes struct {
	Locked    int `json:"locked"`    // Number of locked tasks.
	Completed int `json:"completed"` // Number of completed tasks.
	Failed    int `json:"failed"`    // Number of failed tasks.

	CompletedTasks []engine.Task `json:"completedTasks,omitempty"` // Completed tasks.
	FailedTasks    []engine.Task `json:"failedTasks,omitempty"`    // Failed tasks.
}

type GetVariablesRes struct {
	Count     int                    `json:"count"`               // Number of variables.
	Variables map[string]engine.Data `json:"variables,omitempty"` // Variable map, using variable names as keys.
}

// Response of a job locking.
type LockJobsRes struct {
	Count int          `json:"count"`          // Number of locked jobs.
	Jobs  []engine.Job `json:"jobs,omitempty"` // Locked jobs.
}

// query responses

// Response of an element query.
type ElementRes struct {
	Count   int              `json:"count"`             // Number of results.
	Results []engine.Element `json:"results,omitempty"` // Query results.
}

// Response of an element instance query.
type ElementInstanceRes struct {
	Count   int                      `json:"count"`             // Number of results.
	Results []engine.ElementInstance `json:"results,omitempty"` // Query results.
}

// Response of an incident query.
type IncidentRes struct {
	Count   int               `json:"count"`             // Number of results.
	Results []engine.Incident `json:"results,omitempty"` // Query results.
}

// Response of a job query.
type JobRes struct {
	Count   int          `json:"count"`             // Number of results.
	Results []engine.Job `json:"results,omitempty"` // Query results.
}

// Response of a process query.
type ProcessRes struct {
	Count   int              `json:"count"`             // Number of results.
	Results []engine.Process `json:"results,omitempty"` // Query results.
}

// Response of a process instance query.
type ProcessInstanceRes struct {
	Count   int                      `json:"count"`             // Number of results.
	Results []engine.ProcessInstance `json:"results,omitempty"` // Query results.
}

// Response of a task query.
type TaskRes struct {
	Count   int           `json:"count"`             // Number of results.
	Results []engine.Task `json:"results,omitempty"` // Query results.
}

// Response of variable query.
type VariableRes struct {
	Count   int               `json:"count"`             // Number of results.
	Results []engine.Variable `json:"results,omitempty"` // Query results.
}

func encodeJSONResponseBody(w http.ResponseWriter, r *http.Request, v any, statusCode int) {
	w.Header().Set(HeaderContentType, ContentTypeJson)
	w.WriteHeader(statusCode)

	if err := json.NewEncoder(w).Encode(v); err != nil {
		log.Printf("%s %s: failed to create JSON response body: %v", r.Method, r.RequestURI, err)
		http.Error(w, "unexpected error occurred - see logs", http.StatusInternalServerError)
	}
}
