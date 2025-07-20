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
	InstanceCompleted
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
//   - [JobSetTimer]: timer catch event
type JobType int

const (
	JobEvaluateExclusiveGateway JobType = iota + 1
	JobEvaluateInclusiveGateway
	JobExecute
	JobSetTimer
)

func MapJobType(s string) JobType {
	switch s {
	case "EVALUATE_EXCLUSIVE_GATEWAY":
		return JobEvaluateExclusiveGateway
	case "EVALUATE_INCLUSIVE_GATEWAY":
		return JobEvaluateInclusiveGateway
	case "EXECUTE":
		return JobExecute
	case "SET_TIMER":
		return JobSetTimer
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
	case JobSetTimer:
		return "SET_TIMER"
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
//   - [TaskTriggerTimerEvent] triggers a timer catch event
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
	TaskTriggerTimerEvent

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
	case "TRIGGER_TIMER_EVENT":
		return TaskTriggerTimerEvent
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
	case TaskTriggerTimerEvent:
		return "TRIGGER_TIMER_EVENT"
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
	Id int32 `json:"id" validate:"required"` // Element ID.

	ProcessId int32 `json:"processId" validate:"required"` // ID of the enclosing process.

	BpmnElementId   string            `json:"bpmnElementId" validate:"required"`   // Element ID within the BPMN XML.
	BpmnElementName string            `json:"bpmnElementName,omitempty"`           // Element name within the BPMN XML.
	BpmnElementType model.ElementType `json:"bpmnElementType" validate:"required"` // BPMN element type.
	IsMultiInstance bool              `json:"multiInstance,omitempty"`             // Determines if the element is a multi instance.
}

func (v Element) String() string {
	return fmt.Sprintf("%d:%s:%s", v.Id, v.BpmnElementId, v.BpmnElementType)
}

// ElementCriteria specifies the results, returned by an element instance query.
type ElementCriteria struct {
	ProcessId int32 `json:"processId,omitempty"` // Process filter.
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
	StateChangedBy  string            `json:"stateChangedBy" validate:"required"`  // ID of the worker or engine that changed the state.
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

	BpmnElementId      string     `json:"bpmnElementId" validate:"required"`    // Element ID within the BPMN XML.
	BpmnErrorCode      string     `json:"bpmnErrorCode,omitempty"`              // Code, indicating a BPMN error.
	BpmnEscalationCode string     `json:"bpmnEscalationCode,omitempty"`         // Code, indicating a BPMN escalation.
	CompletedAt        *time.Time `json:"completedAt,omitempty"`                // Completion time.
	CorrelationKey     string     `json:"correlationKey,omitempty"`             // Correlation key of the process instance.
	CreatedAt          time.Time  `json:"createdAt" validate:"required"`        // Creation time.
	CreatedBy          string     `json:"createdBy" validate:"required"`        // ID of the worker or engine that created the job.
	DueAt              time.Time  `json:"dueAt" validate:"required"`            // Due date.
	Error              string     `json:"error,omitempty"`                      // Error, indicating a technical problem.
	LockedAt           *time.Time `json:"lockedAt,omitempty"`                   // Lock time.
	LockedBy           string     `json:"lockedBy,omitempty"`                   // ID of the worker that locked the job.
	RetryCount         int        `json:"retryCount" validate:"required,gte=0"` // Number of retries left. If `0` an incident is created. Otherwise, a retry job is created.
	// Duration until a retry job becomes due.
	// At this point in time a retry job can be locked by a worker.
	RetryTimer ISO8601Duration `json:"retryTimer,omitempty"`
	Type       JobType         `json:"type" validate:"required"` // Job type.
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

	ElementInstanceId int32 `json:"elementInstanceId,omitempty"` // Element instance filter.
	ProcessId         int32 `json:"processId,omitempty"`         // Process filter.
	ProcessInstanceId int32 `json:"processInstanceId,omitempty"` // Process instance filter.
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

	BpmnProcessId  string            `json:"bpmnProcessId" validate:"required"`  // ID of the process element within the BPMN XML.
	CorrelationKey string            `json:"correlationKey,omitempty"`           // Key, used to correlate a process instance with a business entity.
	CreatedAt      time.Time         `json:"createdAt" validate:"required"`      // Creation time.
	CreatedBy      string            `json:"createdBy" validate:"required"`      // ID of the worker or engine that created the process instance.
	EndedAt        *time.Time        `json:"endedAt,omitempty"`                  // End time.
	StartedAt      *time.Time        `json:"startedAt,omitempty"`                // Start time.
	State          InstanceState     `json:"state" validate:"required"`          // Current state.
	StateChangedBy string            `json:"stateChangedBy" validate:"required"` // ID of the worker or engine that changed the state.
	Tags           map[string]string `json:"tags,omitempty"`                     // Tags, consisting of name and value pairs.
	Version        string            `json:"version" validate:"required"`        // Process version.
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

// Task is a unit of work, which must be locked, executed and completed by an engine.
type Task struct {
	Partition Partition `json:"partition" validate:"required"` // Task partition.
	Id        int32     `json:"id" validate:"required"`        // Task ID.

	ElementId         int32 `json:"elementId,omitempty"`         // ID of the related element.
	ElementInstanceId int32 `json:"elementInstanceId,omitempty"` // ID of the related element instance.
	ProcessId         int32 `json:"processId,omitempty"`         // ID of the related process.
	ProcessInstanceId int32 `json:"processInstanceId,omitempty"` // ID of the enclosing process instance.

	CompletedAt *time.Time `json:"completedAt,omitempty"`                // Completion time.
	CreatedAt   time.Time  `json:"createdAt" validate:"required"`        // Creation time.
	CreatedBy   string     `json:"createdBy" validate:"required"`        // ID of the worker or engine that created the task.
	DueAt       time.Time  `json:"dueAt" validate:"required"`            // Due date.
	Error       string     `json:"error,omitempty"`                      // Error, indicating a technical problem.
	LockedAt    *time.Time `json:"lockedAt,omitempty"`                   // Lock time.
	LockedBy    string     `json:"lockedBy,omitempty"`                   // ID of the engine that locked the task.
	RetryCount  int        `json:"retryCount" validate:"required,gte=0"` // Number of retries left. If `0` an incident is created. Otherwise, a retry task is created.
	// Duration until a retry task becomes due.
	// At this point in time a retry task can be locked by an engine.
	RetryTimer     ISO8601Duration `json:"retryTimer,omitempty"`
	SerializedTask string          `json:"serializedTask,omitempty"` // JSON serialized task.
	Type           TaskType        `json:"type" validate:"required"` // Task type.
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
