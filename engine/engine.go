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
	CompleteJob(context.Context, CompleteJobCmd) (Job, error)

	// CreateProcess creates a process, using a BPMN definition that is provided as XML.
	//
	// If a process with the same BPMN process ID and version exists, the BPMN XML is compared.
	// When the BPMN XML equals, the existing process is returned.
	// When the BPMN XML differs, an error of type [ErrorConflict] is returned.
	CreateProcess(context.Context, CreateProcessCmd) (Process, error)

	// CreateProcessInstance creates an instance of an existing BPMN process in a specific version.
	CreateProcessInstance(context.Context, CreateProcessInstanceCmd) (ProcessInstance, error)

	// CreateQuery creates a query with default options.
	CreateQuery() Query

	// ExecuteTasks locks and executes due tasks, which match the specified conditions.
	//
	// Due tasks are normally handled by a task executor, running inside the engine.
	// When waiting for a due task to be completed during testing, this method must be called!
	ExecuteTasks(context.Context, ExecuteTasksCmd) ([]Task, []Task, error)

	// GetBpmnXml the BPMN XML of an existing BPMN process.
	GetBpmnXml(context.Context, GetBpmnXmlCmd) (string, error)

	// GetElementVariables gets variables of an active or ended element instance.
	GetElementVariables(context.Context, GetElementVariablesCmd) (map[string]Data, error)

	// GetProcessVariables gets variables of an active or ended process instance.
	GetProcessVariables(context.Context, GetProcessVariablesCmd) (map[string]Data, error)

	// LockJobs locks due jobs, which match the specified conditions.
	LockJobs(context.Context, LockJobsCmd) ([]Job, error)

	// ResolveIncident resolves a job or task related incident.
	//
	// When an incident is resolved, a retry job or task is created. The retry count is set to 0.
	ResolveIncident(context.Context, ResolveIncidentCmd) error

	// ResumeProcessInstance resumes a suspended process instance.
	ResumeProcessInstance(context.Context, ResumeProcessInstanceCmd) error

	// SendMessage sends a message to notify a message subscriber or buffer it.
	//
	// A subscriber can be a message start or catch event.
	// In case of a message start event, a new process instance is created.
	// In case of a message catch event, an existing process instance is continued.
	SendMessage(context.Context, SendMessageCmd) (Message, error)

	// SendSignal sends a signal to notify signal subscribers.
	//
	// A subscriber can be a signal start or catch event.
	// In case of a signal start event, a new process instance is created.
	// In case of a signal catch event, an existing process instance is continued.
	SendSignal(context.Context, SendSignalCmd) (Signal, error)

	// SetElementVariables sets or deletes variables of an active element instance.
	SetElementVariables(context.Context, SetElementVariablesCmd) error

	// SetProcessVariables sets or deletes variables of an active process instance.
	SetProcessVariables(context.Context, SetProcessVariablesCmd) error

	// SetTime increases the engine's time for testing purposes.
	SetTime(context.Context, SetTimeCmd) error

	// SuspendProcessInstance suspends a started process instance.
	SuspendProcessInstance(context.Context, SuspendProcessInstanceCmd) error

	// UnlockJobs locked, but uncompleted, jobs that are currently locked by a specific worker.
	UnlockJobs(context.Context, UnlockJobsCmd) (int, error)

	// UnlockTasks locked, but uncompleted, tasks that are currently locked by a specific engine.
	UnlockTasks(context.Context, UnlockTasksCmd) (int, error)

	// Shutdown shuts the engine down.
	Shutdown()
}

// A Query allows to query entities, using query options.
type Query interface {
	QueryElements(context.Context, ElementCriteria) ([]Element, error)
	QueryElementInstances(context.Context, ElementInstanceCriteria) ([]ElementInstance, error)
	QueryIncidents(context.Context, IncidentCriteria) ([]Incident, error)
	QueryJobs(context.Context, JobCriteria) ([]Job, error)
	QueryMessages(context.Context, MessageCriteria) ([]Message, error)
	QueryProcesses(context.Context, ProcessCriteria) ([]Process, error)
	QueryProcessInstances(context.Context, ProcessInstanceCriteria) ([]ProcessInstance, error)
	QueryTasks(context.Context, TaskCriteria) ([]Task, error)
	QueryVariables(context.Context, VariableCriteria) ([]Variable, error)

	// SetOptions sets options that are used when performing a query.
	SetOptions(QueryOptions)
}

// Options are common configuration options that are shared between engine implementations.
type Options struct {
	DefaultQueryLimit    int           // Default limit for queries, executed without an explicit limit.
	Encryption           Encryption    // Encryption is needed for the encryption and decryption of variable data.
	EngineId             string        // ID of the engine.
	TaskExecutorEnabled  bool          // Enables or disables the engine's task executor.
	TaskExecutorInterval time.Duration // Interval between execution of due tasks.
	TaskExecutorLimit    int           // Maximum number of due tasks to lock and execute at once.
	TaskRetryLimit       int           // Maximum number of task retries.

	OnTaskExecutionFailure func(Task, error) // Called when the engine failed to execute a locked task.
}

func (o Options) Validate() error {
	if strings.TrimSpace(o.EngineId) == "" {
		return errors.New("engine ID must not be empty or blank")
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
	if o.TaskRetryLimit < 0 {
		return errors.New("task retry limit must be greater than or equal to 0")
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
