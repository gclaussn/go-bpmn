package engine

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
)

const (
	DefaultEngineId = "default-engine" // Default ID of an engine, used when no specific ID is provided via [Options].
)

// An Engine creates, executes and manages process instances, based on the BPMN 2.0 specification.
type Engine interface {
	// CompleteJob completes a locked job.
	CompleteJob(CompleteJobCmd) (Job, error)

	// CreateProcess creates a process, using a BPMN definition that is provided as XML.
	//
	// If a process with the same BPMN process ID and version exists, the BPMN XML is compared.
	// When the BPMN XML equals, the existing process is returned.
	// When the BPMN XML differs, an error of type [ErrorConflict] is returned.
	CreateProcess(CreateProcessCmd) (Process, error)

	// CreateProcessInstance creates an instance of an existing BPMN process in a specific version.
	CreateProcessInstance(CreateProcessInstanceCmd) (ProcessInstance, error)

	// ExecuteTasks locks and executes due tasks, which match the specified conditions.
	//
	// Due tasks are normally handled by a task executor, running inside the engine.
	// When waiting for a due task to be completed during testing, this method must be called!
	ExecuteTasks(ExecuteTasksCmd) ([]Task, []Task, error)

	// GetBpmnXml the BPMN XML of an existing BPMN process.
	GetBpmnXml(GetBpmnXmlCmd) (string, error)

	// GetElementVariables gets variables of an active or ended element instance.
	GetElementVariables(GetElementVariablesCmd) (map[string]Data, error)

	// GetProcessVariables gets variables of an active or ended process instance.
	GetProcessVariables(GetProcessVariablesCmd) (map[string]Data, error)

	// LockJobs locks due jobs, which match the specified conditions.
	LockJobs(LockJobsCmd) ([]Job, error)

	// Query executes a query, which results in zero, one or multiple entities.
	//
	// Specific criteria defines, which type of entities are returned.
	// The following criteria types are supported:
	//   - [ElementCriteria]
	//   - [ElementInstanceCriteria]
	//   - [IncidentCriteria]
	//   - [JobCriteria]
	//   - [ProcessCriteria]
	//   - [ProcessInstanceCriteria]
	//   - [TaskCriteria]
	//   - [VariableCriteria]
	Query(criteria any) ([]any, error)

	// QueryWithOptions executes a query with common options like limit.
	QueryWithOptions(criteria any, options QueryOptions) ([]any, error)

	// ResolveIncident resolves a job or task related incident.
	ResolveIncident(ResolveIncidentCmd) error

	// ResumeProcessInstance resumes a suspended process instance.
	ResumeProcessInstance(ResumeProcessInstanceCmd) error

	// SetElementVariables sets or deletes variables of an active element instance.
	SetElementVariables(SetElementVariablesCmd) error

	// SetProcessVariables sets or deletes variables of an active process instance.
	SetProcessVariables(SetProcessVariablesCmd) error

	// SetTime increases the engine's time for testing purposes.
	SetTime(SetTimeCmd) error

	// SuspendProcessInstance suspends a started process instance.
	SuspendProcessInstance(SuspendProcessInstanceCmd) error

	// UnlockJobs locked, but uncompleted, jobs that are currently locked by a specific worker.
	UnlockJobs(UnlockJobsCmd) (int, error)

	// UnlockTasks locked, but uncompleted, tasks that are currently locked by a specific engine.
	UnlockTasks(UnlockTasksCmd) (int, error)

	// WithContext returns a context-aware engine.
	WithContext(ctx context.Context) Engine

	// Shutdown shuts the engine down.
	Shutdown()
}

// Options are common configuration options that are shared between engine implementations.
type Options struct {
	DefaultQueryLimit    int             // Default limit for queries, executed without an explicit limit.
	Encryption           Encryption      // Encryption is needed for the encryption and decryption of variable data.
	EngineId             string          // ID of the engine.
	JobRetryCount        int             // Optional retry count, used for job creation.
	JobRetryTimer        ISO8601Duration // Optional retry timer, used for job creation.
	TaskExecutorEnabled  bool            // Enables or disables the engine's task executor.
	TaskExecutorInterval time.Duration   // Interval between execution of due tasks.
	TaskExecutorLimit    int             // Maximum number of due tasks to lock and execute at once.

	OnTaskExecutionFailure func(Task, error) // Called when the engine failed to execute a locked task.
}

func (o Options) Validate() error {
	if strings.TrimSpace(o.EngineId) == "" {
		return errors.New("engine ID must not be empty or blank")
	}
	if o.JobRetryCount < 1 {
		return errors.New("job retry count must be greater than or equal to 1")
	}
	if o.TaskExecutorInterval.Milliseconds() < 1000 {
		return errors.New("task executor interval must be greater than or equal to 1000 ms")
	}
	if o.TaskExecutorLimit < 1 {
		return errors.New("task executor limit must be greater than or equal to 1")
	}
	if o.TaskExecutorLimit > 1000 {
		return errors.New("task executor limit must be must be less than or equal to 1000")
	}

	return nil
}

// QueryOptions are used to limit or offset query results.
// The zero value does not affect a query.
type QueryOptions struct {
	// Limit specifies the maximum number of results to return.
	// If Limit <= 0, the option's DefaultQueryLimit is applied.
	Limit int
	// Offset specifies the number of results to skip, before returning any result.
	// If Offset <= 0, no results are skipped.
	Offset int
}

func NewPartition(v string) (Partition, error) {
	t, err := time.Parse(time.DateOnly, v)
	return Partition(t), err
}

type Partition time.Time

func (p Partition) IsDate(u time.Time) bool {
	t := time.Time(p)
	if t.IsZero() {
		return false
	}

	ya, ma, da := t.Date()
	yb, mb, db := u.Date()
	return ya == yb && ma == mb && da == db
}

func (p Partition) IsZero() bool {
	return time.Time(p).IsZero()
}

func (p Partition) MarshalJSON() ([]byte, error) {
	if p.IsZero() {
		return []byte("null"), nil
	}
	return []byte(fmt.Sprintf("%q", p.String())), nil
}

func (p Partition) String() string {
	return time.Time(p).Format(time.DateOnly)
}

func (p *Partition) UnmarshalJSON(data []byte) error {
	s := string(data)
	if s == "null" {
		return nil
	}
	if len(s) != 12 {
		return fmt.Errorf("invalid partition data %s", s)
	}

	s = s[1 : len(s)-1]
	t, err := time.Parse(time.DateOnly, s)
	if err != nil {
		return fmt.Errorf("failed to parse partition %s", s)
	}

	*p = Partition(t)
	return nil
}

type Error struct {
	Type   ErrorType
	Title  string
	Detail string
	Causes []ErrorCause
}

func (e Error) Error() string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("%s: %s: %s", e.Type, e.Title, e.Detail))

	for _, cause := range e.Causes {
		sb.WriteRune('\n')
		sb.WriteString(cause.String())
	}

	return sb.String()
}

type ErrorType int

const (
	ErrorBug ErrorType = iota + 1
	ErrorConflict
	ErrorNotFound
	ErrorProcessModel
	ErrorQuery
	ErrorValidation
)

func MapErrorType(s string) ErrorType {
	switch s {
	case "BUG":
		return ErrorBug
	case "CONFLICT":
		return ErrorConflict
	case "NOT_FOUND":
		return ErrorNotFound
	case "PROCESS_MODEL":
		return ErrorProcessModel
	case "QUERY":
		return ErrorQuery
	case "VALIDATION":
		return ErrorValidation
	default:
		return 0
	}
}

func (v ErrorType) String() string {
	switch v {
	case ErrorBug:
		return "BUG"
	case ErrorConflict:
		return "CONFLICT"
	case ErrorNotFound:
		return "NOT_FOUND"
	case ErrorProcessModel:
		return "PROCESS_MODEL"
	case ErrorQuery:
		return "QUERY"
	case ErrorValidation:
		return "VALIDATION"
	default:
		return "UNKNOWN"
	}
}

// A cause of a process model or validation [Error] like an unsupported BPMN element or an invalid event definition.
type ErrorCause struct {
	Pointer string // A pointer, locating the invalid BPMN element or sequence flow.
	Type    string // Type indicator.
	Detail  string // Human-readable, detailed information about the cause.
}

func (e ErrorCause) String() string {
	return fmt.Sprintf("%s: %s: %s", e.Type, e.Pointer, e.Detail)
}
