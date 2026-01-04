package engine

import (
	"time"
)

// CompleteJobCmd provides data for the completion of a locked job.
type CompleteJobCmd struct {
	// Job partition.
	Partition Partition `json:"-"`
	// Job ID.
	Id int32 `json:"-"`

	// Optional completion, used to succeed a job.
	Completion *JobCompletion `json:"completion,omitempty"`
	// Variables to set or delete at element instance scope. For a variable deletion, no data must be provided.
	ElementVariables []VariableData `json:"elementVariables,omitempty" validate:"max=100,dive"`
	// Optional error, used to fail a job due to a technical problem.
	Error string `json:"error,omitempty"`
	// Variables to set or delete at process instance scope. For a variable deletion, no data must be provided.
	ProcessVariables []VariableData `json:"processVariables,omitempty" validate:"max=100,dive"`
	// Maximum number of retries. If the retry count is less than the retry limit, a retry job is created. Otherwise, an incident is created.
	RetryLimit int `json:"retryLimit,omitempty" validate:"gte=0"`
	// Duration until a retry job becomes due. At this point in time a retry job can be locked by a worker.
	RetryTimer ISO8601Duration `json:"retryTimer" validate:"iso8601_duration"`
	// ID of the worker that locked and completed the job.
	WorkerId string `json:"workerId" validate:"required"`
}

// CreateProcessCmd provides data for the creation of a process.
//
// Parallelism is applied for all process versions with the same BPMN process ID.
// The latest process version determines the maximum number of parallel process instances being executed.
type CreateProcessCmd struct {
	// ID of the process element within the BPMN XML.
	BpmnProcessId string `json:"bpmnProcessId" validate:"required"`
	// Model of the BPMN process as XML.
	BpmnXml string `json:"bpmnXml" validate:"required"`
	// Error event definitions.
	Errors []ErrorDefinition `json:"errors,omitempty" validate:"max=100,dive"`
	// Escalation event definitions.
	Escalations []EscalationDefinition `json:"escalations,omitempty" validate:"max=100,dive"`
	// Message event definitions.
	Messages []MessageDefinition `json:"messages,omitempty" validate:"max=100,dive"`
	// Maximum number of parallel process instances being executed. If `0`, the number of parallel process instances is unlimited.
	Parallelism int `json:"parallelism,omitempty" validate:"gte=0"`
	// Signal event definitions.
	Signals []SignalDefinition `json:"signals,omitempty" validate:"max=100,dive"`
	// Tags.
	Tags []Tag `json:"tags,omitempty" validate:"max=100,dive"`
	// Timer event definitions.
	Timers []TimerDefinition `json:"timers,omitempty" validate:"max=100,dive"`
	// Any process version.
	Version string `json:"version" validate:"required"`
	// ID of the worker that created the process.
	WorkerId string `json:"workerId" validate:"required"`
}

// CreateProcessInstanceCmd provides data for the creation of a process instance.
type CreateProcessInstanceCmd struct {
	// BPMN ID of an existing process.
	BpmnProcessId string `json:"bpmnProcessId" validate:"required"`
	// Optional key, used to correlate a process instance with a business entity.
	CorrelationKey string `json:"correlationKey,omitempty"`
	// Tags.
	Tags []Tag `json:"tags,omitempty" validate:"max=100,dive"`
	// Variables to set at process instance scope.
	Variables []VariableData `json:"variables,omitempty" validate:"max=100,dive"`
	// Version of an existing process.
	Version string `json:"version" validate:"required"`
	// ID of the worker that created the process instance.
	WorkerId string `json:"workerId" validate:"required"`
}

// ExecuteTasksCmd specifies which due tasks are locked and executed by an engine.
type ExecuteTasksCmd struct {
	// Partition condition.
	Partition Partition `json:"partition"`
	// Task condition - must be used in combination with a partition.
	Id int32 `json:"id,omitempty"`

	// Process condition.
	ProcessId int32 `json:"processId,omitempty"`
	// Process instance condition - must be used in combination with a partition.
	ProcessInstanceId int32 `json:"processInstanceId,omitempty"`
	// Task type condition.
	Type TaskType `json:"type,omitempty"`

	// Maximum number of tasks to lock and execute.
	Limit int `json:"limit,omitempty" validate:"gte=1,lte=100"`
}

// GetBpmnXmlCmd is a command for fetching the BPMN XML of an existing process.
type GetBpmnXmlCmd struct {
	// Process ID.
	ProcessId int32 `json:"-"`
}

// GetElementVariablesCmd is used to get the variables of a specific element instance.
type GetElementVariablesCmd struct {
	// Element instance partition.
	Partition Partition `json:"-"`
	// Element instance ID.
	ElementInstanceId int32 `json:"-"`

	// Names of element variables to get.
	// If empty, all variables are included.
	Names []string `json:"-"`
}

// GetProcessVariablesCmd is used to get the variables of a specific process instance.
type GetProcessVariablesCmd struct {
	// Process instance partition.
	Partition Partition `json:"-"`
	// Process instance ID.
	ProcessInstanceId int32 `json:"-"`

	// Names of process variables to get.
	// If empty, all variables are included.
	Names []string `json:"-"`
}

// LockJobsCmd specifies which due jobs are locked by a worker.
type LockJobsCmd struct {
	// Partition condition.
	Partition Partition `json:"partition"`
	// Job condition - must be used in combination with a partition.
	Id int32 `json:"id,omitempty"`

	// IDs of processes to include.
	ProcessIds []int32 `json:"processIds,omitempty" validate:"max=100,unique"`
	// Process instance condition - must be used in combination with a partition.
	ProcessInstanceId int32 `json:"processInstanceId,omitempty"`

	// Maximum number of jobs to lock.
	Limit int `json:"limit,omitempty" validate:"gte=1,lte=1000"`
	// ID of the worker that locks the jobs.
	WorkerId string `json:"workerId" validate:"required"`
}

// ResolveIncidentCmd is a command for resolving a job or a task related incident.
type ResolveIncidentCmd struct {
	// Incident partition.
	Partition Partition `json:"-"`
	// Incident ID.
	Id int32 `json:"-"`

	// Duration until the retry job or task becomes due.
	RetryTimer ISO8601Duration `json:"retryTimer" validate:"iso8601_duration"`

	// ID of the worker that resolved the incident
	WorkerId string `json:"workerId" validate:"required"`
}

// ResumeProcessInstanceCmd is a command for resuming a suspended process instance.
type ResumeProcessInstanceCmd struct {
	// Process instance partition.
	Partition Partition `json:"-"`
	// Process instance ID.
	Id int32 `json:"-"`

	// ID of the worker that resumed the process instance.
	WorkerId string `json:"workerId" validate:"required"`
}

// SendMessageCmd is used to notify a message subscriber or buffer a message.
type SendMessageCmd struct {
	// Key, used to correlate a message subscription with the message.
	CorrelationKey string `json:"correlationKey" validate:"required"`
	// A timer that defines when the message expires.
	ExpirationTimer *Timer `json:"expirationTimer"`
	// Message name.
	Name string `json:"name" validate:"required"`
	// Optional key that uniquely identifies the message.
	// If a message with the same name, correlation key and unique key already exists, the message is discarded.
	UniqueKey string `json:"uniqueKey,omitempty"`
	// Variables to set or delete at process instance scope. For a variable deletion, no data must be provided.
	Variables []VariableData `json:"variables,omitempty" validate:"max=100,dive"`
	// ID of the worker that sent the message.
	WorkerId string `json:"workerId" validate:"required"`
}

// SendSignalCmd is used to notify all subscribers.
type SendSignalCmd struct {
	// Signal name.
	Name string `json:"name" validate:"required"`
	// Variables to set or delete at process instance scope. For a variable deletion, no data must be provided.
	Variables []VariableData `json:"variables,omitempty" validate:"max=100,dive"`
	// ID of the worker that sent the signal.
	WorkerId string `json:"workerId" validate:"required"`
}

// SetElementVariablesCmd is used to set or delete variables at element instance scope.
type SetElementVariablesCmd struct {
	// Element instance partition.
	Partition Partition `json:"-"`
	// Element instance ID.
	ElementInstanceId int32 `json:"-"`

	// Variables to set or delete. For a variable deletion, no data must be provided.
	Variables []VariableData `json:"variables,omitempty" validate:"max=100,dive"`
	// ID of the worker that set the variables.
	WorkerId string `json:"workerId" validate:"required"`
}

// SetProcessVariablesCmd is used to set or delete variables at process instance scope.
type SetProcessVariablesCmd struct {
	// Process instance partition.
	Partition Partition `json:"-"`
	// Process instance ID.
	ProcessInstanceId int32 `json:"-"`

	// Variables to set or delete. For a variable deletion, no data must be provided.
	Variables []VariableData `json:"variables,omitempty" validate:"max=100,dive"`
	// ID of the worker that set the variables.
	WorkerId string `json:"workerId" validate:"required"`
}

// SetTimeCmd is a command for increasing the engine's time for testing purposes.
type SetTimeCmd struct {
	// A future point in time.
	Time time.Time `json:"time" validate:"required"`
}

// SuspendProcessInstanceCmd is a command for suspending an active process instance.
type SuspendProcessInstanceCmd struct {
	// Process instance partition.
	Partition Partition `json:"-"`
	// Process instance ID.
	Id int32 `json:"-"`

	// ID of the worker that suspended the process instance.
	WorkerId string `json:"workerId" validate:"required"`
}

// UnlockJobsCmd specifies which locked, but uncompleted, jobs are unlocked.
type UnlockJobsCmd struct {
	// Partition condition.
	Partition Partition `json:"partition"`
	// Job condition - must be used in combination with a partition.
	Id int32 `json:"id,omitempty"`

	// Condition that restricts the jobs, to be locked by a specific worker.
	WorkerId string `json:"workerId" validate:"required"`
}

// UnlockTasksCmd specifies which locked, but uncompleted, tasks are unlocked.
type UnlockTasksCmd struct {
	// Partition condition.
	Partition Partition `json:"partition"`
	// Task condition - must be used in combination with a partition.
	Id int32 `json:"id,omitempty"`

	// Condition that restricts the tasks, to be locked by a specific engine.
	EngineId string `json:"engineId" validate:"required"`
}

// command related types

// A job completion is used to complete jobs of various types.
type JobCompletion struct {
	// Code of a BPMN error, used to specify or trigger a BPMN error.
	// Applicable when job type is `SET_ERROR_CODE` or `EXECUTE`.
	ErrorCode string `json:"errorCode,omitempty"`
	// Code of a BPMN escalation, used to specify or trigger a BPMN escalation.
	// Applicable when job type is `SET_ESCALATION_CODE` or `EXECUTE`.
	EscalationCode string `json:"escalationCode,omitempty"`
	// Evaluated BPMN element ID to continue with after the exclusive gateway.
	// Applicable when job type is `EVALUATE_EXCLUSIVE_GATEWAY`.
	ExclusiveGatewayDecision string `json:"exclusiveGatewayDecision,omitempty"`
	// Evaluated BPMN element IDs to continue with after the inclusive gateway.
	// Applicable when job type is `EVALUATE_INCLUSIVE_GATEWAY`.
	InclusiveGatewayDecision []string `json:"inclusiveGatewayDecision,omitempty"`
	// Key, used to correlate a message subscription with a message.
	// Applicable when job type is `SUBSCRIBE_MESSAGE`.
	MessageCorrelationKey string `json:"messageCorrelationKey,omitempty"`
	// Name of the message to subscribe to.
	// Applicable when job type is `SUBSCRIBE_MESSAGE`.
	MessageName string `json:"messageName,omitempty"`
	// Name of the signal to subscribe to.
	// Applicable when job type is `SUBSCRIBE_SIGNAL`.
	SignalName string `json:"signalName,omitempty"`
	// A timer definition.
	// Applicable when job type is `SET_TIMER`.
	Timer *Timer `json:"timer,omitempty"`
}

// ErrorDefinition is used to define an error event.
type ErrorDefinition struct {
	BpmnElementId string `json:"bpmnElementId" validate:"required"` // Element ID of the error event within the BPMN XML.
	ErrorCode     string `json:"errorCode,omitempty"`               // Code of a BPMN error.
}

// EscalationDefinition is used to define an escalation event.
type EscalationDefinition struct {
	BpmnElementId  string `json:"bpmnElementId" validate:"required"` // Element ID of the escalation event within the BPMN XML.
	EscalationCode string `json:"escalationCode,omitempty"`          // Code of a BPMN escalation.
}

// MessageDefinition is used to define a message event.
type MessageDefinition struct {
	BpmnElementId string `json:"bpmnElementId" validate:"required"` // Element ID of the message event within the BPMN XML.
	MessageName   string `json:"messageName" validate:"required"`   // Name of a BPMN message.
}

// SignalDefinition is used to define a signal event.
type SignalDefinition struct {
	BpmnElementId string `json:"bpmnElementId" validate:"required"` // Element ID of the signal event within the BPMN XML.
	SignalName    string `json:"signalName" validate:"required"`    // Name of a BPMN signal.
}

// TimerDefinition is used to define a timer event.
type TimerDefinition struct {
	BpmnElementId string `json:"bpmnElementId" validate:"required"` // Element ID of the timer event within the BPMN XML.
	Timer         *Timer `json:"timer" validate:"timer"`            // Timer that defines a point in time.
}
