package engine

import (
	"fmt"
	"strconv"
	"time"

	"github.com/gclaussn/go-bpmn/model"
)

// InstanceState describes possible process instance and element instance states.
type InstanceState int

const (
	InstanceCanceled InstanceState = iota + 1
	InstanceCompleted
	InstanceCreated
	InstanceQueued
	InstanceStarted
	InstanceSuspended
	InstanceTerminated
)

func MapInstanceState(s string) InstanceState {
	switch s {
	case "CANCELED":
		return InstanceCanceled
	case "COMPLETED":
		return InstanceCompleted
	case "CREATED":
		return InstanceCreated
	case "QUEUED":
		return InstanceQueued
	case "STARTED":
		return InstanceStarted
	case "SUSPENDED":
		return InstanceSuspended
	case "TERMINATED":
		return InstanceTerminated
	default:
		return 0
	}
}

func (v InstanceState) MarshalJSON() ([]byte, error) {
	s := v.String()
	if s == "" {
		return []byte("null"), nil
	}
	return []byte(fmt.Sprintf("%q", s)), nil
}

func (v InstanceState) String() string {
	switch v {
	case InstanceCanceled:
		return "CANCELED"
	case InstanceCompleted:
		return "COMPLETED"
	case InstanceCreated:
		return "CREATED"
	case InstanceQueued:
		return "QUEUED"
	case InstanceStarted:
		return "STARTED"
	case InstanceSuspended:
		return "SUSPENDED"
	case InstanceTerminated:
		return "TERMINATED"
	default:
		return ""
	}
}

func (v *InstanceState) UnmarshalJSON(data []byte) error {
	s := string(data)
	if s == "null" {
		return nil
	}
	if len(s) > 2 {
		s = s[1 : len(s)-1]
		*v = MapInstanceState(s)
	}
	if *v == 0 {
		return fmt.Errorf("invalid instance state data %s", s)
	}
	return nil
}

// JobType describes the different types of jobs, a worker needs to execute.
//
// Each type is used for a specific set of BPMN element types:
//
//   - [JobEvaluateExclusiveGateway]: forking exclusive gateway
//   - [JobEvaluateInclusiveGateway]: forking inclusive gateway
//   - [JobExecute]: business rule, script, send and service task
//   - [JobSetErrorCode]: error boundary event
//   - [JobSetEscalationCode]: escalation boundary event
//   - [JobSetTimer]: timer boundary and catch event
//   - [JobSubscribeMessage]: message catch event
//   - [JobSubscribeSignal]: signal boundary and catch event
type JobType int

const (
	JobEvaluateExclusiveGateway JobType = iota + 1
	JobEvaluateInclusiveGateway
	JobExecute
	JobSetErrorCode
	JobSetEscalationCode
	JobSetTimer
	JobSubscribeMessage
	JobSubscribeSignal
)

func MapJobType(s string) JobType {
	switch s {
	case "EVALUATE_EXCLUSIVE_GATEWAY":
		return JobEvaluateExclusiveGateway
	case "EVALUATE_INCLUSIVE_GATEWAY":
		return JobEvaluateInclusiveGateway
	case "EXECUTE":
		return JobExecute
	case "SET_ERROR_CODE":
		return JobSetErrorCode
	case "SET_ESCALATION_CODE":
		return JobSetEscalationCode
	case "SET_TIMER":
		return JobSetTimer
	case "SUBSCRIBE_MESSAGE":
		return JobSubscribeMessage
	case "SUBSCRIBE_SIGNAL":
		return JobSubscribeSignal
	default:
		return 0
	}
}

func (v JobType) MarshalJSON() ([]byte, error) {
	s := v.String()
	if s == "" {
		return []byte("null"), nil
	}
	return []byte(fmt.Sprintf("%q", s)), nil
}

func (v JobType) String() string {
	switch v {
	case JobEvaluateExclusiveGateway:
		return "EVALUATE_EXCLUSIVE_GATEWAY"
	case JobEvaluateInclusiveGateway:
		return "EVALUATE_INCLUSIVE_GATEWAY"
	case JobExecute:
		return "EXECUTE"
	case JobSetErrorCode:
		return "SET_ERROR_CODE"
	case JobSetEscalationCode:
		return "SET_ESCALATION_CODE"
	case JobSetTimer:
		return "SET_TIMER"
	case JobSubscribeMessage:
		return "SUBSCRIBE_MESSAGE"
	case JobSubscribeSignal:
		return "SUBSCRIBE_SIGNAL"
	default:
		return ""
	}
}

func (v *JobType) UnmarshalJSON(data []byte) error {
	s := string(data)
	if s == "null" {
		return nil
	}
	if len(s) > 2 {
		s = s[1 : len(s)-1]
		*v = MapJobType(s)
	}
	if *v == 0 {
		return fmt.Errorf("invalid job type data %s", s)
	}
	return nil
}

// TaskType describes the different types of tasks, an engine needs to execute.
//
//   - [TaskDequeueProcessInstance] dequeues a queued process instance
//   - [TaskJoinParallelGateway] continues a parallel gateway by joining executions
//   - [TaskStartProcessInstance] starts a queued process instance
//   - [TaskTriggerEvent] triggers a message, signal or timer event
//
// Management related types, that are only relevant for a pg engine:
//
//   - [TaskCreatePartition] creates a table partition for a specific date
//   - [TaskDetachPartition] detaches a completed table partition
//   - [TaskDropPartition] drops a detached table partition
//   - [TaskPurgeMessages] purges messages that are expired
//   - [TaskPurgeSignals] purges signals that have no active subscribers anymore
type TaskType int

const (
	TaskDequeueProcessInstance TaskType = iota + 1
	TaskJoinParallelGateway
	TaskStartProcessInstance
	TaskTriggerEvent

	// management
	TaskCreatePartition
	TaskDetachPartition
	TaskDropPartition
	TaskPurgeMessages
	TaskPurgeSignals
)

func MapTaskType(s string) TaskType {
	switch s {
	case "DEQUEUE_PROCESS_INSTANCE":
		return TaskDequeueProcessInstance
	case "JOIN_PARALLEL_GATEWAY":
		return TaskJoinParallelGateway
	case "START_PROCESS_INSTANCE":
		return TaskStartProcessInstance
	case "TRIGGER_EVENT":
		return TaskTriggerEvent
	// management
	case "CREATE_PARTITION":
		return TaskCreatePartition
	case "DETACH_PARTITION":
		return TaskDetachPartition
	case "DROP_PARTITION":
		return TaskDropPartition
	case "PURGE_MESSAGES":
		return TaskPurgeMessages
	case "PURGE_SIGNALS":
		return TaskPurgeSignals
	default:
		return 0
	}
}

func (v TaskType) MarshalJSON() ([]byte, error) {
	s := v.String()
	if s == "" {
		return []byte("null"), nil
	}
	return []byte(fmt.Sprintf("%q", s)), nil
}

func (v TaskType) String() string {
	switch v {
	case TaskDequeueProcessInstance:
		return "DEQUEUE_PROCESS_INSTANCE"
	case TaskJoinParallelGateway:
		return "JOIN_PARALLEL_GATEWAY"
	case TaskStartProcessInstance:
		return "START_PROCESS_INSTANCE"
	case TaskTriggerEvent:
		return "TRIGGER_EVENT"
	// management
	case TaskCreatePartition:
		return "CREATE_PARTITION"
	case TaskDetachPartition:
		return "DETACH_PARTITION"
	case TaskDropPartition:
		return "DROP_PARTITION"
	case TaskPurgeMessages:
		return "PURGE_MESSAGES"
	case TaskPurgeSignals:
		return "PURGE_SIGNALS"
	default:
		return ""
	}
}

func (v *TaskType) UnmarshalJSON(data []byte) error {
	s := string(data)
	if s == "null" {
		return nil
	}
	if len(s) > 2 {
		s = s[1 : len(s)-1]
		*v = MapTaskType(s)
	}
	if *v == 0 {
		return fmt.Errorf("invalid task type data %s", s)
	}
	return nil
}

// Data is used to store any data within an engine.
type Data struct {
	Encoding    string `json:"encoding" validate:"required"` // Encoding of the value - e.g. `json`.
	IsEncrypted bool   `json:"encrypted,omitempty"`          // Determines if a value is encrypted before it is stored.
	Value       string `json:"value" validate:"required"`    // Data value, encoded as a string.
}

// Element represents a BPMN element of a process.
type Element struct {
	Id int32 `json:"id" validate:"required"` // Element ID.

	ProcessId int32 `json:"processId" validate:"required"` // ID of the enclosing process.

	BpmnElementId   string            `json:"bpmnElementId" validate:"required"`   // Element ID within the BPMN XML.
	BpmnElementName string            `json:"bpmnElementName,omitempty"`           // Element name within the BPMN XML.
	BpmnElementType model.ElementType `json:"bpmnElementType" validate:"required"` // BPMN element type.
	IsMultiInstance bool              `json:"multiInstance,omitempty"`             // Determines if the element is a multi instance.

	EventDefinition *EventDefinition `json:"eventDefinition,omitempty"` // Definition, in case of a BPMN event.
}

func (v Element) String() string {
	return fmt.Sprintf("%d:%s:%s", v.Id, v.BpmnElementId, v.BpmnElementType)
}

// ElementCriteria specifies the results, returned by an element instance query.
type ElementCriteria struct {
	ProcessId int32 `json:"processId,omitempty"` // Process filter.

	BpmnElementId string `json:"bpmnElementId,omitempty"` // BPMN element ID filter.
}

// ElementInstance is an instance of a BPMN element in the scope of an process instance.
type ElementInstance struct {
	Partition Partition `json:"partition" validate:"required"` // Element instance partition.
	Id        int32     `json:"id" validate:"required"`        // Element instance ID.

	ParentId int32 `json:"parentId,omitempty"` // ID of the parent element instance, the enclosing scope.

	ElementId         int32 `json:"elementId" validate:"required"`         // ID of the related element.
	ProcessId         int32 `json:"processId" validate:"required"`         // ID of the related process.
	ProcessInstanceId int32 `json:"processInstanceId" validate:"required"` // ID of the enclosing process instance.

	BpmnElementId   string            `json:"bpmnElementId" validate:"required"`   // Element ID within the BPMN XML.
	BpmnElementType model.ElementType `json:"bpmnElementType" validate:"required"` // BPMN element type.
	CreatedAt       time.Time         `json:"createdAt" validate:"required"`       // Creation time.
	CreatedBy       string            `json:"createdBy" validate:"required"`       // ID of the worker or engine that created the element instance.
	EndedAt         *time.Time        `json:"endedAt,omitempty"`                   // End time.
	IsMultiInstance bool              `json:"multiInstance,omitempty"`             // Determines if the element instance is a multi instance.
	StartedAt       *time.Time        `json:"startedAt,omitempty"`                 // Start time.
	State           InstanceState     `json:"state" validate:"required"`           // Current state.
}

func (v ElementInstance) HasParent() bool {
	return v.ParentId != 0
}

func (v ElementInstance) IsEnded() bool {
	return v.EndedAt != nil
}

func (v ElementInstance) String() string {
	return fmt.Sprintf("%s/%d", v.Partition, v.Id)
}

// ElementInstanceCriteria specifies the results, returned by an element instance query.
type ElementInstanceCriteria struct {
	Partition Partition `json:"partition"`    // Partition filter.
	Id        int32     `json:"id,omitempty"` // Element instance filter.

	ProcessId         int32 `json:"processId,omitempty"`         // Process filter.
	ProcessInstanceId int32 `json:"processInstanceId,omitempty"` // Process instance filter.

	BpmnElementId string          `json:"bpmnElementId,omitempty"`                  // BPMN element ID filter.
	States        []InstanceState `json:"states,omitempty" validate:"max=7,unique"` // States to include.
}

// EventDefinition is a generic definition of a BPMN event, while a BPMN element has exactly one type.
type EventDefinition struct {
	IsSuspended bool `json:"suspended"` // Determines if an event definition is suspended.

	ErrorCode      string `json:"errorCode,omitempty"`      // Code of a BPMN error - set in case of an error event.
	EscalationCode string `json:"escalationCode,omitempty"` // Code of a BPMN escalation - set in case of an escalation event.
	MessageName    string `json:"messageName,omitempty"`    // Name of the message - set in case of a message event.
	SignalName     string `json:"signalName,omitempty"`     // Name of the signal - set in case of a signal event.
	Timer          *Timer `json:"timer,omitempty"`          // A timer definition - set in case of a timer event.
}

// Incident represents a failed job or task, which has no more retries left.
// An incident is related to either a job or a task.
type Incident struct {
	Partition Partition `json:"partition" validate:"required"` // Incident partition.
	Id        int32     `json:"id" validate:"required"`        // Incident ID.

	ElementId         int32 `json:"elementId,omitempty"`         // ID of the related element.
	ElementInstanceId int32 `json:"elementInstanceId,omitempty"` // ID of the related element instance.
	JobId             int32 `json:"jobId,omitempty"`             // ID of the related job.
	ProcessId         int32 `json:"processId,omitempty"`         // ID of the related process.
	ProcessInstanceId int32 `json:"processInstanceId,omitempty"` // ID of the enclosing process instance.
	TaskId            int32 `json:"taskId,omitempty"`            // ID of the related task.

	CreatedAt  time.Time  `json:"createdAt" validate:"required"` // Creation time
	CreatedBy  string     `json:"createdBy" validate:"required"` // ID of the engine that created the incident.
	ResolvedAt *time.Time `json:"resolvedAt,omitempty"`          // Resolution time.
	ResolvedBy string     `json:"resolvedBy,omitempty"`          // ID of the worker that resolved the incident.
}

func (v Incident) IsResolved() bool {
	return v.ResolvedAt != nil
}

func (v Incident) String() string {
	return fmt.Sprintf("%s/%d", v.Partition, v.Id)
}

// IncidentCriteria specifies the results, returned by an element instance query.
type IncidentCriteria struct {
	Partition Partition `json:"partition"`    // Partition filter.
	Id        int32     `json:"id,omitempty"` // Incident filter.

	JobId             int32 `json:"jobId,omitempty"`             // Job filter.
	ProcessInstanceId int32 `json:"processInstanceId,omitempty"` // Process instance filter.
	TaskId            int32 `json:"taskId,omitempty"`            // Process filter.
}

// Job is a unit of work related to an element instance, which must be locked, executed and completed by a worker.
type Job struct {
	Partition Partition `json:"partition" validate:"required"` // Job partition.
	Id        int32     `json:"id" validate:"required"`        // Job ID.

	ElementId         int32 `json:"elementId" validate:"required"`         // ID of the related element.
	ElementInstanceId int32 `json:"elementInstanceId" validate:"required"` // ID of the related element instance.
	ProcessId         int32 `json:"processId" validate:"required"`         // ID of the related process.
	ProcessInstanceId int32 `json:"processInstanceId" validate:"required"` // ID of the enclosing process instance.

	BpmnElementId  string     `json:"bpmnElementId" validate:"required"`    // Element ID within the BPMN XML.
	CompletedAt    *time.Time `json:"completedAt,omitempty"`                // Completion time.
	CorrelationKey string     `json:"correlationKey,omitempty"`             // Correlation key of the process instance.
	CreatedAt      time.Time  `json:"createdAt" validate:"required"`        // Creation time.
	CreatedBy      string     `json:"createdBy" validate:"required"`        // ID of the worker or engine that created the job.
	DueAt          time.Time  `json:"dueAt" validate:"required"`            // Point in time when a job can be locked by a worker.
	Error          string     `json:"error,omitempty"`                      // Error, indicating a technical problem.
	LockedAt       *time.Time `json:"lockedAt,omitempty"`                   // Lock time.
	LockedBy       string     `json:"lockedBy,omitempty"`                   // ID of the worker that locked the job.
	RetryCount     int        `json:"retryCount" validate:"required,gte=0"` // Retry indicator, which is increased for each failed attempt.
	Type           JobType    `json:"type" validate:"required"`             // Job type.
}

func (v Job) HasError() bool {
	return v.Error != ""
}

func (v Job) IsCompleted() bool {
	return v.CompletedAt != nil
}

func (v Job) IsLocked() bool {
	return v.LockedAt != nil
}

func (v Job) String() string {
	return fmt.Sprintf("%s/%d", v.Partition, v.Id)
}

// JobCriteria specifies the results, returned by a job query.
type JobCriteria struct {
	Partition Partition `json:"partition"`    // Partition filter.
	Id        int32     `json:"id,omitempty"` // Job filter.

	ElementId         int32 `json:"elementId,omitempty"`         // Element filter.
	ElementInstanceId int32 `json:"elementInstanceId,omitempty"` // Element instance filter.
	ProcessId         int32 `json:"processId,omitempty"`         // Process filter.
	ProcessInstanceId int32 `json:"processInstanceId,omitempty"` // Process instance filter.
}

// Message represents a sent message.
//
// If a message is correlated, a message subscriber (message start or catch event) has been notified.
// Otherwise a message is buffered (waiting to be correlated) until it expires.
type Message struct {
	Id int64 `json:"id" validate:"required"` // Message ID.

	CorrelationKey string     `json:"correlationKey" validate:"required"` // Key, used to correlate a message subscription with the message.
	CreatedAt      time.Time  `json:"createdAt" validate:"required"`      // Message sent time.
	CreatedBy      string     `json:"createdBy" validate:"required"`      // ID of the worker that sent the message.
	ExpiresAt      *time.Time `json:"expiresAt,omitempty"`                // Point in time, when the message expires.
	IsCorrelated   bool       `json:"correlated" validate:"required"`     // Indicates if the message is correlated or not.
	Name           string     `json:"name" validate:"required"`           // Message name.
	UniqueKey      string     `json:"uniqueKey,omitempty"`                // Key that uniquely identifies the message.
}

// MessageCriteria specifies the results, returned by a message query.
type MessageCriteria struct {
	Id int64 `json:"id,omitempty"` // Message filter.

	ExcludeExpired bool   `json:"excludeExpired"` // Determines if expired messages are returned.
	Name           string `json:"name,omitempty"` // Message name filter.
}

// Process represents a BPMN process that consists of a set of BPMN elements.
type Process struct {
	Id int32 `json:"id" validate:"required"` // Process ID.

	BpmnProcessId string            `json:"bpmnProcessId" validate:"required"`      // ID of the process element within the BPMN XML.
	CreatedAt     time.Time         `json:"createdAt" validate:"required"`          // Creation time.
	CreatedBy     string            `json:"createdBy" validate:"required"`          // ID of the worker that created the process.
	Parallelism   int               `json:"parallelism,omitempty" validate:"gte=0"` // Maximum number of parallel process instances being executed.
	Tags          map[string]string `json:"tags,omitempty"`                         // Tags, consisting of name and value pairs.
	Version       string            `json:"version" validate:"required"`            // Process version.
}

func (v Process) String() string {
	return fmt.Sprintf("%d:%s:%s", v.Id, v.BpmnProcessId, v.Version)
}

// ProcessCriteria specifies the results, returned by a process query.
type ProcessCriteria struct {
	Id int32 `json:"id,omitempty"` // Process filter.

	Tags map[string]string `json:"tags,omitempty"` // Tags, a process must have, to be included.
}

// Process instance is an instance of a BPMN process.
type ProcessInstance struct {
	Partition Partition `json:"partition" validate:"required"` // Process instance partition
	Id        int32     `json:"id" validate:"required"`        // Process instance ID

	ParentId int32 `json:"parentId,omitempty"` // ID of the parent process instance.
	RootId   int32 `json:"rootId,omitempty"`   // ID of the root process instance.

	ProcessId int32 `json:"processId"` // ID of the related process.

	BpmnProcessId  string            `json:"bpmnProcessId" validate:"required"` // ID of the process element within the BPMN XML.
	CorrelationKey string            `json:"correlationKey,omitempty"`          // Key, used to correlate a process instance with a business entity.
	CreatedAt      time.Time         `json:"createdAt" validate:"required"`     // Creation time.
	CreatedBy      string            `json:"createdBy" validate:"required"`     // ID of the worker or engine that created the process instance.
	EndedAt        *time.Time        `json:"endedAt,omitempty"`                 // End time.
	StartedAt      *time.Time        `json:"startedAt,omitempty"`               // Start time.
	State          InstanceState     `json:"state" validate:"required"`         // Current state.
	Tags           map[string]string `json:"tags,omitempty"`                    // Tags, consisting of name and value pairs.
	Version        string            `json:"version" validate:"required"`       // Process version.
}

func (v ProcessInstance) HasParent() bool {
	return v.ParentId != 0
}

func (v ProcessInstance) IsEnded() bool {
	return v.EndedAt != nil
}

func (v ProcessInstance) IsRoot() bool {
	return v.RootId == 0
}

func (v ProcessInstance) String() string {
	return fmt.Sprintf("%s/%d", v.Partition, v.Id)
}

// ProcessInstanceCriteria specifies the results, returned by a process instance query.
type ProcessInstanceCriteria struct {
	Partition Partition `json:"partition"`    // Partition filter.
	Id        int32     `json:"id,omitempty"` // Process instance filter.

	ProcessId int32 `json:"processId,omitempty"` // Process filter.

	Tags map[string]string `json:"tags,omitempty"` // Tags, a process instance must have, to be included.
}

// Signal represents a notification of signal subscribers (signal start or catch events).
type Signal struct {
	Id int64 `json:"id" validate:"required"` // Signal ID.

	CreatedAt       time.Time `json:"createdAt" validate:"required"`             // Signal sent time.
	CreatedBy       string    `json:"createdBy" validate:"required"`             // ID of the worker or engine that sent the signal.
	Name            string    `json:"name" validate:"required"`                  // Name of the signal.
	SubscriberCount int       `json:"subscriberCount" validate:"required,gte=0"` // Number of notified signal subscribers.
}

func (v Signal) String() string {
	return strconv.FormatInt(v.Id, 10)
}

// Task is a unit of work, which must be locked, executed and completed by an engine.
type Task struct {
	Partition Partition `json:"partition" validate:"required"` // Task partition.
	Id        int32     `json:"id" validate:"required"`        // Task ID.

	ElementId         int32 `json:"elementId,omitempty"`         // ID of the related element.
	ElementInstanceId int32 `json:"elementInstanceId,omitempty"` // ID of the related element instance.
	ProcessId         int32 `json:"processId,omitempty"`         // ID of the related process.
	ProcessInstanceId int32 `json:"processInstanceId,omitempty"` // ID of the enclosing process instance.

	CompletedAt    *time.Time `json:"completedAt,omitempty"`                // Completion time.
	CreatedAt      time.Time  `json:"createdAt" validate:"required"`        // Creation time.
	CreatedBy      string     `json:"createdBy" validate:"required"`        // ID of the worker or engine that created the task.
	DueAt          time.Time  `json:"dueAt" validate:"required"`            // Point in time when a task can be locked by an engine.
	Error          string     `json:"error,omitempty"`                      // Error, indicating a technical problem.
	LockedAt       *time.Time `json:"lockedAt,omitempty"`                   // Lock time.
	LockedBy       string     `json:"lockedBy,omitempty"`                   // ID of the engine that locked the task.
	RetryCount     int        `json:"retryCount" validate:"required,gte=0"` // Retry indicator, which is increased for each failed attempt.
	SerializedTask string     `json:"serializedTask,omitempty"`             // JSON serialized task.
	Type           TaskType   `json:"type" validate:"required"`             // Task type.
}

func (v Task) HasError() bool {
	return v.Error != ""
}

func (v Task) IsCompleted() bool {
	return v.CompletedAt != nil
}

func (v Task) IsLocked() bool {
	return v.LockedAt != nil
}

func (v Task) String() string {
	return fmt.Sprintf("%s/%d", v.Partition, v.Id)
}

// TaskCriteria specifies the results, returned by a task query.
type TaskCriteria struct {
	Partition Partition `json:"partition"`    // Partition filter.
	Id        int32     `json:"id,omitempty"` // Task filter.

	ElementId         int32 `json:"elementId,omitempty"`         // Element filter.
	ElementInstanceId int32 `json:"elementInstanceId,omitempty"` // Element instance filter.
	ProcessId         int32 `json:"processId,omitempty"`         // Process filter.
	ProcessInstanceId int32 `json:"processInstanceId,omitempty"` // Process instance filter.

	Type TaskType `json:"type,omitempty"` // Task type.
}

// A timer defines a point in time using a time value, a CRON expression or a duration.
type Timer struct {
	// A point in time.
	Time time.Time `json:"time"`
	// CRON expression that specifies a cyclic timer.
	TimeCycle string `json:"timeCycle,omitempty" validate:"cron"`
	// Duration based timer that uses the engine's time to calculate a point in time.
	TimeDuration ISO8601Duration `json:"timeDuration" validate:"iso8601_duration"`
}

func (t Timer) String() string {
	if !t.Time.IsZero() {
		return t.Time.UTC().Truncate(time.Millisecond).Format(time.RFC3339Nano)
	} else if t.TimeCycle != "" {
		return t.TimeCycle
	} else if !t.TimeDuration.IsZero() {
		return t.TimeDuration.String()
	} else {
		return ""
	}
}

// Variable is data, identified by a name, that exists in the scope of a process instance or element instance.
type Variable struct {
	Partition Partition `json:"partition" validate:"required"` // Variable partition.
	Id        int32     `json:"id" validate:"required"`        // Variable ID.

	ElementId         int32 `json:"elementId,omitempty"`                   // ID of the related element - set if the variable exists at element instance scope.
	ElementInstanceId int32 `json:"elementInstanceId,omitempty"`           // ID of the related element instance - set if the variable exists at element instance scope.
	ProcessId         int32 `json:"processId" validate:"required"`         // ID of the related process.
	ProcessInstanceId int32 `json:"processInstanceId" validate:"required"` // ID of the enclosing process instance.

	CreatedAt   time.Time `json:"createdAt" validate:"required"` // Creation time.
	CreatedBy   string    `json:"createdBy" validate:"required"` // ID of the worker that created the variable.
	Encoding    string    `json:"encoding" validate:"required"`  // Encoding of the variable value - e.g. `json`.
	IsEncrypted bool      `json:"encrypted"`                     // Determines if the variable value is encrypted.
	Name        string    `json:"name" validate:"required"`      // Variable name.
	UpdatedAt   time.Time `json:"updatedAt" validate:"required"` // Last modification time.
	UpdatedBy   string    `json:"updatedBy" validate:"required"` // ID of the worker that updated the variable.
}

func (v Variable) String() string {
	return fmt.Sprintf("%s/%d", v.Partition, v.Id)
}

// VariableCriteria specifies the results, returned by a variable query.
type VariableCriteria struct {
	Partition Partition `json:"partition"` // Partition filter.

	ElementInstanceId int32 `json:"elementInstanceId,omitempty"` // Element instance filter.
	ProcessInstanceId int32 `json:"processInstanceId,omitempty"` // Process instance filter.

	Names []string `json:"names,omitempty"` // Names of variables to include.
}
