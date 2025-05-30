package engine

import (
	"fmt"
	"time"

	"github.com/gclaussn/go-bpmn/model"
)

// InstanceState describes possible process instance and element instance states.
type InstanceState int

const (
	InstanceCanceled InstanceState = iota + 1
	InstanceCreated
	InstanceEnded
	InstanceQueued
	InstanceStarted
	InstanceSuspended
	InstanceTerminated
)

func MapInstanceState(s string) InstanceState {
	switch s {
	case "CANCELED":
		return InstanceCanceled
	case "CREATED":
		return InstanceCreated
	case "ENDED":
		return InstanceEnded
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
	case InstanceCreated:
		return "CREATED"
	case InstanceEnded:
		return "ENDED"
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
//   - [JobEvaluateExclusiveGateway] must result in a job completion with exactly one BPMN element ID to continue with
//   - [JobEvaluateInclusiveGateway] must result in a job completion with one or multiple BPMN element IDs to continue with
//   - [JobExecute] execution of a BPMN element with type business rule, script, send or service task
type JobType int

const (
	JobEvaluateExclusiveGateway JobType = iota + 1
	JobEvaluateInclusiveGateway
	JobExecute
)

func MapJobType(s string) JobType {
	switch s {
	case "EVALUATE_EXCLUSIVE_GATEWAY":
		return JobEvaluateExclusiveGateway
	case "EVALUATE_INCLUSIVE_GATEWAY":
		return JobEvaluateInclusiveGateway
	case "EXECUTE":
		return JobExecute
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
//
// Management related types, that are only relevant for a pg engine:
//
//   - [TaskCreatePartition] creates the table partitions for a specific date
//   - [TaskDetachPartition] detaches completed table partitions
//   - [TaskDropPartition] drops a detached table partitions
type TaskType int

const (
	TaskDequeueProcessInstance TaskType = iota + 1
	TaskJoinParallelGateway
	TaskStartProcessInstance

	// management
	TaskCreatePartition
	TaskDetachPartition
	TaskDropPartition
)

func MapTaskType(s string) TaskType {
	switch s {
	case "DEQUEUE_PROCESS_INSTANCE":
		return TaskDequeueProcessInstance
	case "JOIN_PARALLEL_GATEWAY":
		return TaskJoinParallelGateway
	case "START_PROCESS_INSTANCE":
		return TaskStartProcessInstance
	// management
	case "CREATE_PARTITION":
		return TaskCreatePartition
	case "DETACH_PARTITION":
		return TaskDetachPartition
	case "DROP_PARTITION":
		return TaskDropPartition
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
	// management
	case TaskCreatePartition:
		return "CREATE_PARTITION"
	case TaskDetachPartition:
		return "DETACH_PARTITION"
	case TaskDropPartition:
		return "DROP_PARTITION"
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
	Id int32 `json:"id"` // Element ID.

	ProcessId int32 `json:"processId"` // ID of the enclosing process.

	BpmnElementId   string            `json:"bpmnElementId"`             // Element ID within the BPMN XML.
	BpmnElementName string            `json:"bpmnElementName,omitempty"` // Element name within the BPMN XML.
	BpmnElementType model.ElementType `json:"bpmnElementType"`           // BPMN element type.
	IsMultiInstance bool              `json:"multiInstance,omitempty"`   // Determines if the element is a multi instance.
}

func (v Element) String() string {
	return fmt.Sprintf("%d:%s:%s", v.Id, v.BpmnElementId, v.BpmnElementType)
}

// ElementCriteria specifies the results, returned by an element instance query.
type ElementCriteria struct {
	ProcessId int32 `json:"processId,omitempty"` // Process filter.
}

// ElementInstance represents an instance of a BPMN element in the scope of an process instance.
type ElementInstance struct {
	Partition Partition `json:"partition"` // Element instance partition.
	Id        int32     `json:"id"`        // Element instance ID.

	ParentId int32 `json:"parentId,omitempty"` // ID of the parent element instance, the enclosing scope.

	ElementId         int32 `json:"elementId"`         // ID of the related element.
	ProcessId         int32 `json:"processId"`         // ID of the related process.
	ProcessInstanceId int32 `json:"processInstanceId"` // ID of the enclosing process instance.

	BpmnElementId   string            `json:"bpmnElementId"`           // Element ID within the BPMN XML.
	BpmnElementType model.ElementType `json:"bpmnElementType"`         // BPMN element type.
	CreatedAt       time.Time         `json:"createdAt"`               // Creation time.
	CreatedBy       string            `json:"createdBy"`               // ID of the worker or engine that created the element instance.
	EndedAt         *time.Time        `json:"endedAt,omitempty"`       // End time.
	IsMultiInstance bool              `json:"multiInstance,omitempty"` // Determines if the element instance is a multi instance.
	StartedAt       *time.Time        `json:"startedAt,omitempty"`     // Start time.
	State           InstanceState     `json:"state"`                   // Current state.
	StateChangedBy  string            `json:"stateChangedBy"`          // ID of the worker or engine that changed the state.
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

	BpmnElementId string          `json:"bpmnElementId,omitempty"`
	States        []InstanceState `json:"states,omitempty" validate:"max=7,unique"` // States to include.
}

// Incident represents a failed job or task, which has no more retries left.
// An incident can only be related to either a job or a task.
type Incident struct {
	Partition Partition `json:"partition"` // Incident partition.
	Id        int32     `json:"id"`        // Incident ID.

	ElementId         int32 `json:"elementId,omitempty"`         // ID of the related element.
	ElementInstanceId int32 `json:"elementInstanceId,omitempty"` // ID of the related element instance.
	JobId             int32 `json:"jobId,omitempty"`             // ID of the related job.
	ProcessId         int32 `json:"processId,omitempty"`         // ID of the related process.
	ProcessInstanceId int32 `json:"processInstanceId,omitempty"` // ID of the enclosing process instance.
	TaskId            int32 `json:"taskId,omitempty"`            // ID of the related task.

	CreatedAt  time.Time  `json:"createdAt"`            // Creation time
	CreatedBy  string     `json:"createdBy"`            // ID of the engine that created the incident.
	ResolvedAt *time.Time `json:"resolvedAt,omitempty"` // Resolution time.
	ResolvedBy string     `json:"resolvedBy,omitempty"` // ID of the worker that resolved the incident.
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

// Job represents a unit of work in the scope of an element instance, which must be locked, executed and completed by a worker.
type Job struct {
	Partition Partition `json:"partition"` // Job partition.
	Id        int32     `json:"id"`        // Job ID.

	ElementId         int32 `json:"elementId"`         // ID of the related element.
	ElementInstanceId int32 `json:"elementInstanceId"` // ID of the related element instance.
	ProcessId         int32 `json:"processId"`         // ID of the related process.
	ProcessInstanceId int32 `json:"processInstanceId"` // ID of the enclosing process instance.

	BpmnElementId      string     `json:"bpmnElementId"`                // Element ID within the BPMN XML.
	BpmnErrorCode      string     `json:"bpmnErrorCode,omitempty"`      // Code, indicating a BPMN error.
	BpmnEscalationCode string     `json:"bpmnEscalationCode,omitempty"` // Code, indicating a BPMN escalation.
	CompletedAt        *time.Time `json:"completedAt,omitempty"`        // Completion time.
	CorrelationKey     string     `json:"correlationKey,omitempty"`     // Correlation key of the process instance.
	CreatedAt          time.Time  `json:"createdAt"`                    // Creation time.
	CreatedBy          string     `json:"createdBy"`                    // ID of the worker or engine that created the job.
	DueAt              time.Time  `json:"dueAt"`                        // Due date.
	Error              string     `json:"error,omitempty"`              // Error, indicating a technical problem.
	LockedAt           *time.Time `json:"lockedAt,omitempty"`           // Lock time.
	LockedBy           string     `json:"lockedBy,omitempty"`           // ID of the worker that locked the job.
	RetryCount         int        `json:"retryCount"`                   // Number of retries left. If `0` an incident is created. Otherwise, a retry job is created.
	// Duration until a retry job becomes due.
	// At this point in time a retry job can be locked by a worker.
	RetryTimer ISO8601Duration `json:"retryTimer,omitempty"`
	Type       JobType         `json:"type"` // Job type.
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

// JobCompletion is used to complete jobs of various types.
type JobCompletion struct {
	// Code of a BPMN error to trigger.
	BpmnErrorCode string `json:"bpmnErrorCode,omitempty"`
	// Code of a BPMN escalation to trigger.
	BpmnEscalationCode string `json:"bpmnEscalationCode,omitempty"`

	// Evaluated BPMN element ID to continue with after the exclusive gateway.
	// Applicable when job type is `EVALUATE_EXCLUSIVE_GATEWAY`.
	ExclusiveGatewayDecision string `json:"exclusiveGatewayDecision,omitempty"`
	// Evaluated BPMN element IDs to continue with after the inclusive gateway.
	// Applicable when job type is `EVALUATE_INCLUSIVE_GATEWAY`.
	InclusiveGatewayDecision []string `json:"inclusiveGatewayDecision,omitempty"`
}

// JobCriteria specifies the results, returned by a job query.
type JobCriteria struct {
	Partition Partition `json:"partition"`    // Partition filter.
	Id        int32     `json:"id,omitempty"` // Job filter.

	ElementInstanceId int32 `json:"elementInstanceId,omitempty"` // Element instance filter.
	ProcessId         int32 `json:"processId,omitempty"`         // Process filter.
	ProcessInstanceId int32 `json:"processInstanceId,omitempty"` // Process instance filter.
}

type Process struct {
	Id int32 `json:"id"` // Process ID.

	BpmnProcessId string            `json:"bpmnProcessId"`         // ID of the process element within the BPMN XML.
	CreatedAt     time.Time         `json:"createdAt"`             // Creation time.
	CreatedBy     string            `json:"createdBy"`             // ID of the worker that created the process.
	Parallelism   int               `json:"parallelism,omitempty"` // Maximum number of parallel process instances being executed.
	Tags          map[string]string `json:"tags,omitempty"`        // Tags, consisting of name and value pairs.
	Version       string            `json:"version"`               // Process version.
}

func (v Process) String() string {
	return fmt.Sprintf("%d:%s:%s", v.Id, v.BpmnProcessId, v.Version)
}

// ProcessCriteria specifies the results, returned by a process query.
type ProcessCriteria struct {
	Id int32 `json:"id,omitempty"` // Process filter.

	Tags map[string]string `json:"tags,omitempty"` // Tags, a process must have, to be included.
}

type ProcessInstance struct {
	Partition Partition `json:"partition"` // Process instance partition
	Id        int32     `json:"id"`        // Process instance ID

	ParentId int32 `json:"parentId,omitempty"` // ID of the parent process instance.
	RootId   int32 `json:"rootId,omitempty"`   // ID of the root process instance.

	ProcessId int32 `json:"processId"` // ID of the related process.

	BpmnProcessId  string            `json:"bpmnProcessId"`            // ID of the process element within the BPMN XML.
	CorrelationKey string            `json:"correlationKey,omitempty"` // Key, used to correlate a process instance with a business entity.
	CreatedAt      time.Time         `json:"createdAt"`                // Creation time.
	CreatedBy      string            `json:"createdBy"`                // ID of the worker or engine that created the process instance.
	EndedAt        *time.Time        `json:"endedAt,omitempty"`        // End time.
	StartedAt      *time.Time        `json:"startedAt,omitempty"`      // Start time.
	State          InstanceState     `json:"state"`                    // Current state.
	StateChangedBy string            `json:"stateChangedBy"`           // ID of the worker or engine that changed the state.
	Tags           map[string]string `json:"tags,omitempty"`           // Tags, consisting of name and value pairs.
	Version        string            `json:"version"`                  // Process version.
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

// Task represents a unit of work, which must be locked and executed by an engine.
type Task struct {
	Partition Partition `json:"partition"` // Task partition.
	Id        int32     `json:"id"`        // Task ID.

	ElementId         int32 `json:"elementId,omitempty"`         // ID of the related element.
	ElementInstanceId int32 `json:"elementInstanceId,omitempty"` // ID of the related element instance.
	ProcessId         int32 `json:"processId,omitempty"`         // ID of the related process.
	ProcessInstanceId int32 `json:"processInstanceId,omitempty"` // ID of the enclosing process instance.

	CompletedAt *time.Time `json:"completedAt,omitempty"` // Completion time.
	CreatedAt   time.Time  `json:"createdAt"`             // Creation time.
	CreatedBy   string     `json:"createdBy"`             // ID of the worker or engine that created the task.
	DueAt       time.Time  `json:"dueAt"`                 // Due date.
	Error       string     `json:"error,omitempty"`       // Error, indicating a technical problem.
	LockedAt    *time.Time `json:"lockedAt,omitempty"`    // Lock time.
	LockedBy    string     `json:"lockedBy,omitempty"`    // ID of the engine that locked the task.
	RetryCount  int        `json:"retryCount"`            // Number of retries left. If `0` an incident is created. Otherwise, a retry task is created.
	// Duration until a retry task becomes due.
	// At this point in time a retry task can be locked by an engine.
	RetryTimer     ISO8601Duration `json:"retryTimer,omitempty"`
	SerializedTask string          `json:"serializedTask,omitempty"` // JSON serialized task.
	Type           TaskType        `json:"type"`                     // Task type.
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

	ElementInstanceId int32 `json:"elementInstanceId,omitempty"` // Element instance filter.
	ProcessId         int32 `json:"processId,omitempty"`         // Process filter.
	ProcessInstanceId int32 `json:"processInstanceId,omitempty"` // Process instance filter.

	Type TaskType `json:"type,omitempty"` // Task type.
}

type Variable struct {
	Partition Partition `json:"partition"` // Variable partition.
	Id        int32     `json:"id"`        // Variable ID.

	ElementId         int32 `json:"elementId,omitempty"`         // ID of the related element - set if the variable exists at element instance scope.
	ElementInstanceId int32 `json:"elementInstanceId,omitempty"` // ID of the related element instance - set if the variable exists at element instance scope.
	ProcessId         int32 `json:"processId"`                   // ID of the related process.
	ProcessInstanceId int32 `json:"processInstanceId"`           // ID of the enclosing process instance.

	CreatedAt   time.Time `json:"createdAt"` // Creation time.
	CreatedBy   string    `json:"createdBy"` // ID of the worker that created the variable.
	Encoding    string    `json:"encoding"`  // Encoding of the variable value - e.g. `json`.
	IsEncrypted bool      `json:"encrypted"` // Determines if the variable value is encrypted.
	Name        string    `json:"name"`      // Variable name.
	UpdatedAt   time.Time `json:"updatedAt"` // Last modification time.
	UpdatedBy   string    `json:"updatedBy"` // ID of the worker that updated the variable.
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
