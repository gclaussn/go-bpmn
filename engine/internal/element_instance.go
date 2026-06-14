package internal

import (
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/gclaussn/go-bpmn/engine"
	"github.com/gclaussn/go-bpmn/model"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

type ElementInstanceEntity struct {
	Partition time.Time
	Id        int32

	ParentId      pgtype.Int4
	PrevElementId pgtype.Int4 // internal field
	PrevId        pgtype.Int4 // internal field

	ElementId         int32
	ProcessId         int32
	ProcessInstanceId int32

	BpmnElementId   string
	BpmnElementType model.ElementType
	Context         pgtype.Text // internal field, used to persist error code, escalation code etc.
	CreatedAt       time.Time
	CreatedBy       string
	EndedAt         pgtype.Timestamp
	ExecutionCount  int // internal field
	IsMultiInstance bool
	StartedAt       pgtype.Timestamp
	State           engine.InstanceState

	parent *ElementInstanceEntity
	prev   *ElementInstanceEntity
}

func (e ElementInstanceEntity) ElementInstance() engine.ElementInstance {
	return engine.ElementInstance{
		Partition: engine.Partition(e.Partition),
		Id:        e.Id,

		ParentId: e.ParentId.Int32,

		ElementId:         e.ElementId,
		ProcessId:         e.ProcessId,
		ProcessInstanceId: e.ProcessInstanceId,

		BpmnElementId:   e.BpmnElementId,
		BpmnElementType: e.BpmnElementType,
		CreatedAt:       e.CreatedAt,
		CreatedBy:       e.CreatedBy,
		EndedAt:         timeOrNil(e.EndedAt),
		IsMultiInstance: e.IsMultiInstance,
		StartedAt:       timeOrNil(e.StartedAt),
		State:           e.State,
	}
}

type ElementInstanceRepository interface {
	Insert(*ElementInstanceEntity) error
	Select(partition time.Time, id int32) (*ElementInstanceEntity, error)

	// SelectActive selects element instances of the given process instance
	// that are not in state COMPLETED, TERMINATED and CANCELED.
	SelectActive(*ProcessInstanceEntity) ([]*ElementInstanceEntity, error)

	// SelectActiveChildren selects children of the given parent element instance
	// that are not in state COMPLETED, TERMINATED and CANCELED.
	SelectActiveChildren(parent *ElementInstanceEntity) ([]*ElementInstanceEntity, error)

	// SelectByProcessInstanceAndState selects element instances related to a process instance.
	// Only element instances with the same state as the process instance are selected.
	SelectByProcessInstanceAndState(*ProcessInstanceEntity) ([]*ElementInstanceEntity, error)

	// SelectBoundaryEvents selects element instances, which are attached to the given element instance.
	// Only element instances with state CREATED are selected.
	SelectBoundaryEvents(time.Time, int32) ([]*ElementInstanceEntity, error)

	// SelectParallelGateways selects element instances, which have the same
	//  - parent element instance
	//  - element
	//  - state
	SelectParallelGateways(*ElementInstanceEntity) ([]*ElementInstanceEntity, error)

	Update(*ElementInstanceEntity) error
	UpdateBatch([]*ElementInstanceEntity) error

	Query(engine.ElementInstanceCriteria, engine.QueryOptions) ([]engine.ElementInstance, error)
}

type JoinParallelGatewayTask struct {
}

func (t JoinParallelGatewayTask) Execute(ctx Context, task *TaskEntity) error {
	processInstance, err := ctx.ProcessInstances().Select(task.Partition, task.ProcessInstanceId.Int32)
	if err == pgx.ErrNoRows {
		return engine.Error{
			Type:   engine.ErrorNotFound,
			Title:  "failed to find process instance",
			Detail: fmt.Sprintf("process instance %s/%d could not be found", task.Partition.Format(time.DateOnly), task.ProcessInstanceId.Int32),
		}
	}
	if err != nil {
		return err
	}

	if processInstance.EndedAt.Valid {
		task.State = engine.WorkCanceled
		return nil
	}

	process, err := ctx.ProcessCache().GetOrCacheById(ctx, processInstance.ProcessId)
	if err != nil {
		return err
	}

	execution, err := ctx.ElementInstances().Select(task.Partition, task.ElementInstanceId.Int32)
	if err != nil {
		return err
	}
	if execution.EndedAt.Valid {
		task.State = engine.WorkCanceled
		return nil // already completed by another task instance, terminated or canceled
	}

	waiting, err := ctx.ElementInstances().SelectParallelGateways(execution)
	if err != nil {
		return err
	}

	// sort waiting executions by ID, but ensure that the current execution is first
	// this guarantees that the current execution is always part of a possible join
	// otherwise it would be possible that not all joins are executed
	// since such a join is only performed when an execution is in state STARTED
	slices.SortFunc(waiting, func(a *ElementInstanceEntity, b *ElementInstanceEntity) int {
		if a.Id == execution.Id {
			return -1
		} else if b.Id == execution.Id {
			return 1
		} else {
			return int(a.Id - b.Id)
		}
	})

	joined, err := process.graph.joinParallelGateway(waiting)
	if err != nil {
		return engine.Error{
			Type:   engine.ErrorProcessModel,
			Title:  "failed to join parallel gateway",
			Detail: err.Error(),
		}
	}
	if len(joined) == 0 {
		return nil
	}

	scope, err := ctx.ElementInstances().Select(execution.Partition, execution.ParentId.Int32)
	if err != nil {
		return err
	}

	ec := executionContext{
		engineOrWorkerId: ctx.Options().EngineId,
		executions:       []*ElementInstanceEntity{scope},
		process:          process,
		processInstance:  processInstance,
	}

	for _, joinedExecution := range joined {
		joinedExecution.parent = scope
		ec.addExecution(joinedExecution)
	}

	if err := ec.continueExecutions(ctx); err != nil {
		if _, ok := err.(engine.Error); ok {
			return err
		} else {
			return fmt.Errorf("failed to continue executions %+v: %v", ec.executions, err)
		}
	}

	return nil
}

func (ec *executionContext) callProcess(ctx Context, job *JobEntity, jobCompletion *engine.JobCompletion) error {
	execution := ec.executions[1]

	node, err := ec.process.graph.node(execution.BpmnElementId)
	if err != nil {
		job.Error = pgtype.Text{String: err.Error(), Valid: true}
		return nil
	}

	var process *ProcessEntity
	if calledElement := node.bpmnElement.Model.(model.CallActivity).CalledElement; calledElement != "" {
		s := strings.SplitN(calledElement, ":", 2)
		if len(s) == 2 {
			process = &ProcessEntity{
				BpmnProcessId: s[0],
				Version:       s[1],
			}
		} else {
			latestProcess, err := ctx.Processes().SelectLatest(s[0])
			if err == pgx.ErrNoRows {
				job.Error = pgtype.Text{String: fmt.Sprintf("process %s could not be found", s[0]), Valid: true}
				return nil
			}
			if err != nil {
				return err
			}

			process = latestProcess
		}
	}

	var calledProcess *engine.CalledProcess
	if jobCompletion != nil && jobCompletion.CalledProcess != nil {
		calledProcess = jobCompletion.CalledProcess
	}

	if process != nil {
		if calledProcess == nil {
			calledProcess = &engine.CalledProcess{
				BpmnProcessId: process.BpmnProcessId,
				Version:       process.Version,
			}
		} else {
			if calledProcess.BpmnProcessId == "" {
				calledProcess.BpmnProcessId = process.BpmnProcessId
			}
			if calledProcess.Version == "" {
				calledProcess.Version = process.Version
			}
		}
	}

	if calledProcess == nil {
		job.Error = pgtype.Text{String: "expected a called process", Valid: true}
		return nil
	}
	if calledProcess.BpmnProcessId == "" {
		job.Error = pgtype.Text{String: "expected the BPMN process ID of a called process", Valid: true}
		return nil
	}
	if calledProcess.Version == "" {
		job.Error = pgtype.Text{String: "expected the version of a called process", Valid: true}
		return nil
	}

	if _, err := CreateProcessInstance(ctx, engine.CreateProcessInstanceCmd{
		BpmnProcessId:  calledProcess.BpmnProcessId,
		CorrelationKey: calledProcess.CorrelationKey,
		Tags:           calledProcess.Tags,
		Variables:      calledProcess.Variables,
		Version:        calledProcess.Version,
		WorkerId:       ec.engineOrWorkerId,
	}, ec.processInstance, execution); err != nil {
		if _, ok := err.(engine.Error); ok {
			job.Error = pgtype.Text{String: err.Error(), Valid: true}
		} else {
			return fmt.Errorf("failed to create process instance: %v", err)
		}
	}

	return nil
}

func (ec *executionContext) evaluateExclusiveGateway(job *JobEntity, jobCompletion *engine.JobCompletion) error {
	execution := ec.executions[1]

	if execution.BpmnElementType != model.ElementExclusiveGateway {
		job.Error = pgtype.Text{String: fmt.Sprintf("expected BPMN element %s to be an exclusive gateway", execution.BpmnElementId), Valid: true}
		return nil
	}

	graph := ec.process.graph

	node, err := graph.node(execution.BpmnElementId)
	if err != nil {
		job.Error = pgtype.Text{String: err.Error(), Valid: true}
		return nil
	}

	exclusiveGateway := node.bpmnElement.Model.(model.ExclusiveGateway)

	if (jobCompletion == nil || jobCompletion.ExclusiveGatewayDecision == "") && exclusiveGateway.Default == "" {
		job.Error = pgtype.Text{String: "expected an exclusive gateway decision", Valid: true}
		return nil
	}

	var targetId string
	if jobCompletion != nil {
		targetId = jobCompletion.ExclusiveGatewayDecision

		if err := graph.ensureSequenceFlow(job.BpmnElementId, targetId); err != nil {
			job.Error = pgtype.Text{String: err.Error(), Valid: true}
			return nil
		}
	} else {
		outgoing := node.bpmnElement.OutgoingById(exclusiveGateway.Default)
		targetId = outgoing.Target.Id
	}

	execution.State = engine.InstanceCompleted

	scope := ec.executions[0]

	next, err := graph.createExecutionAt(scope, targetId)
	if err != nil {
		job.Error = pgtype.Text{String: fmt.Sprintf("failed to create execution at %s: %v", targetId, err), Valid: true}
		return nil
	}

	ec.addExecution(&next)
	return nil
}

func (ec *executionContext) evaluateInclusiveGateway(job *JobEntity, jobCompletion *engine.JobCompletion) error {
	execution := ec.executions[1]

	if execution.BpmnElementType != model.ElementInclusiveGateway {
		job.Error = pgtype.Text{String: fmt.Sprintf("expected BPMN element %s to be an inclusive gateway", execution.BpmnElementId), Valid: true}
		return nil
	}

	graph := ec.process.graph

	node, err := graph.node(execution.BpmnElementId)
	if err != nil {
		job.Error = pgtype.Text{String: err.Error(), Valid: true}
		return nil
	}

	inclusiveGateway := node.bpmnElement.Model.(model.InclusiveGateway)

	if (jobCompletion == nil || len(jobCompletion.InclusiveGatewayDecision) == 0) && inclusiveGateway.Default == "" {
		job.Error = pgtype.Text{String: "expected an inclusive gateway decision", Valid: true}
		return nil
	}

	var targetIds []string
	if jobCompletion != nil {
		targetIds = jobCompletion.InclusiveGatewayDecision
		for i := range targetIds {
			for j := i + 1; j < len(targetIds); j++ {
				if targetIds[i] == targetIds[j] {
					job.Error = pgtype.Text{String: fmt.Sprintf("decision contains duplicate BPMN element ID %s", targetIds[i]), Valid: true}
					return nil
				}
			}
		}
		for i := range targetIds {
			if err := graph.ensureSequenceFlow(job.BpmnElementId, targetIds[i]); err != nil {
				job.Error = pgtype.Text{String: err.Error(), Valid: true}
				return nil
			}
		}
	}

	if inclusiveGateway.Default != "" {
		outgoing := node.bpmnElement.OutgoingById(inclusiveGateway.Default)
		if !slices.Contains(targetIds, outgoing.Target.Id) {
			targetIds = append(targetIds, outgoing.Target.Id)
		}
	}

	execution.State = engine.InstanceCompleted

	scope := ec.executions[0]

	for _, targetId := range targetIds {
		next, err := graph.createExecutionAt(scope, targetId)
		if err != nil {
			job.Error = pgtype.Text{String: fmt.Sprintf("failed to create execution at %s: %v", targetId, err), Valid: true}
			return nil
		}

		ec.addExecution(&next)
	}

	return nil
}

func (ec *executionContext) execute(ctx Context, job *JobEntity, jobCompletion *engine.JobCompletion, retryTimer engine.ISO8601Duration) (bool, error) {
	if jobCompletion == nil {
		return true, nil
	}

	var isErrorCodeSet bool
	if jobCompletion.ErrorCode != "" {
		isErrorCodeSet = true
	}

	var isEscalationCodeSet bool
	if jobCompletion.EscalationCode != "" {
		isEscalationCodeSet = true
	}

	if isErrorCodeSet && isEscalationCodeSet {
		job.Error = pgtype.Text{String: "expected error code or escalation code to be set", Valid: true}
		return false, nil
	}

	switch {
	case isErrorCodeSet:
		return ec.throwError(ctx, jobCompletion.ErrorCode)
	case isEscalationCodeSet:
		return ec.escalateJob(ctx, job, jobCompletion.EscalationCode, retryTimer)
	default:
		return true, nil
	}
}
