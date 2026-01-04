package common

import "github.com/gclaussn/go-bpmn/engine"

// Response of a batch operation like the unlocking of jobs or tasks.
type CountRes struct {
	Count int `json:"count" validate:"required,gte=0"` // The number of affected entities.
}

// Response of a task execution.
type ExecuteTasksRes struct {
	Locked    int `json:"locked" validate:"required,gte=0"`    // Number of locked tasks.
	Completed int `json:"completed" validate:"required,gte=0"` // Number of completed tasks.
	Failed    int `json:"failed" validate:"required,gte=0"`    // Number of failed tasks.

	CompletedTasks []engine.Task `json:"completedTasks" validate:"required"` // Completed tasks.
	FailedTasks    []engine.Task `json:"failedTasks" validate:"required"`    // Failed tasks.
}

// Process instance or element instance variable response.
type GetVariablesRes struct {
	Count     int                   `json:"count" validate:"required,gte=0"` // Number of variables.
	Variables []engine.VariableData `json:"variables" validate:"required"`   // Variables, including data.
}

// Response of a job locking.
type LockJobsRes struct {
	Count int          `json:"count" validate:"required,gte=0"` // Number of locked jobs.
	Jobs  []engine.Job `json:"jobs" validate:"required"`        // Locked jobs.
}

// query responses

// Response of an element query.
type ElementRes struct {
	Count   int              `json:"count" validate:"required,gte=0"` // Number of results.
	Results []engine.Element `json:"results" validate:"required"`     // Query results.
}

// Response of an element instance query.
type ElementInstanceRes struct {
	Count   int                      `json:"count" validate:"required,gte=0"` // Number of results.
	Results []engine.ElementInstance `json:"results" validate:"required"`     // Query results.
}

// Response of an incident query.
type IncidentRes struct {
	Count   int               `json:"count" validate:"required,gte=0"` // Number of results.
	Results []engine.Incident `json:"results" validate:"required"`     // Query results.
}

// Response of a job query.
type JobRes struct {
	Count   int          `json:"count" validate:"required,gte=0"` // Number of results.
	Results []engine.Job `json:"results" validate:"required"`     // Query results.
}

// Response of a message query.
type MessageRes struct {
	Count   int              `json:"count" validate:"required,gte=0"` // Number of results.
	Results []engine.Message `json:"results" validate:"required"`     // Query results.
}

// Response of a process query.
type ProcessRes struct {
	Count   int              `json:"count" validate:"required,gte=0"` // Number of results.
	Results []engine.Process `json:"results" validate:"required"`     // Query results.
}

// Response of a process instance query.
type ProcessInstanceRes struct {
	Count   int                      `json:"count" validate:"required,gte=0"` // Number of results.
	Results []engine.ProcessInstance `json:"results" validate:"required"`     // Query results.
}

// Response of a task query.
type TaskRes struct {
	Count   int           `json:"count" validate:"required,gte=0"` // Number of results.
	Results []engine.Task `json:"results" validate:"required"`     // Query results.
}

// Response of variable query.
type VariableRes struct {
	Count   int               `json:"count" validate:"required,gte=0"` // Number of results.
	Results []engine.Variable `json:"results" validate:"required"`     // Query results.
}
