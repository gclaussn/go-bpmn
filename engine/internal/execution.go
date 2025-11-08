package internal

import (
	"fmt"
	"slices"

	"github.com/gclaussn/go-bpmn/engine"
	"github.com/gclaussn/go-bpmn/model"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

type executionContext struct {
	engineOrWorkerId string
	process          *ProcessEntity
	processInstance  *ProcessInstanceEntity
}

// continueExecutions continues each execution until a wait state is reached or no more outgoing sequence flows exist.
func (ec executionContext) continueExecutions(ctx Context, executions []*ElementInstanceEntity) error {
	i := 0
	for i < len(executions) {
		execution := executions[i]

		i++

		if execution.ExecutionCount > 0 {
			continue // skip scope
		}

		// find parent within executions
		if execution.parent == nil {
			parentId := execution.ParentId.Int32
			for j := range executions {
				if executions[j].Id == parentId {
					execution.parent = executions[j]
					break
				}
			}
		}

		// find parent within repository
		if execution.parent == nil {
			parent, err := ctx.ElementInstances().Select(execution.Partition, execution.ParentId.Int32)
			if err != nil {
				return err
			}

			execution.parent = parent
			executions = append(executions, parent)
			i++
		}

		// continue execution
		var err error
		if executions, err = ec.process.graph.continueExecution(executions, execution); err != nil {
			return engine.Error{
				Type:   engine.ErrorProcessModel,
				Title:  "failed to continue execution",
				Detail: err.Error(),
			}
		}

		switch execution.State {
		case engine.InstanceStarted:
			execution.StartedAt = pgtype.Timestamp{Time: ctx.Time(), Valid: true}
		case engine.InstanceCompleted:
			execution.EndedAt = pgtype.Timestamp{Time: ctx.Time(), Valid: true}

			if !execution.StartedAt.Valid {
				execution.StartedAt = pgtype.Timestamp{Time: ctx.Time(), Valid: true}
			}
		case engine.InstanceTerminated:
			execution.EndedAt = pgtype.Timestamp{Time: ctx.Time(), Valid: true}
		}

		scope := execution.parent
		if scope.State == engine.InstanceCompleted && !scope.ParentId.Valid {
			scope.EndedAt = pgtype.Timestamp{Time: ctx.Time(), Valid: true}

			// complete process instance
			ec.processInstance.EndedAt = pgtype.Timestamp{Time: ctx.Time(), Valid: true}
			ec.processInstance.State = engine.InstanceCompleted
		}
	}

	var (
		jobs     []*JobEntity  // jobs to insert
		jobsIdx  []int         // indices of job related executions
		tasks    []*TaskEntity // tasks to insert
		tasksIdx []int         // indices of task related executions
	)

	// create jobs and tasks
	for i, execution := range executions {
		switch execution.State {
		case engine.InstanceQueued, engine.InstanceSuspended, engine.InstanceTerminated:
			continue
		}

		if execution.ExecutionCount != 0 {
			continue
		}

		var (
			jobType      engine.JobType
			taskType     engine.TaskType
			taskInstance Task
		)

		if execution.State == engine.InstanceCreated {
			if execution.Context.Valid {
				continue // skip defined boundary event
			}

			switch execution.BpmnElementType {
			case model.ElementMessageCatchEvent:
				jobType = engine.JobSubscribeMessage
			case model.ElementSignalCatchEvent:
				jobType = engine.JobSubscribeSignal
			case model.ElementTimerCatchEvent:
				jobType = engine.JobSetTimer
			case model.ElementErrorBoundaryEvent:
				jobType = engine.JobSetErrorCode
			default:
				return engine.Error{
					Type:   engine.ErrorBug,
					Title:  "failed to define job or task for created execution",
					Detail: fmt.Sprintf("BPMN element type %s is not supported", execution.BpmnElementType),
				}
			}
		} else if execution.State == engine.InstanceStarted {
			switch execution.BpmnElementType {
			case
				model.ElementBusinessRuleTask,
				model.ElementScriptTask,
				model.ElementSendTask,
				model.ElementServiceTask:
				jobType = engine.JobExecute
			case model.ElementExclusiveGateway:
				jobType = engine.JobEvaluateExclusiveGateway
			case model.ElementInclusiveGateway:
				jobType = engine.JobEvaluateInclusiveGateway
			case model.ElementParallelGateway:
				taskType = engine.TaskJoinParallelGateway
				taskInstance = JoinParallelGatewayTask{}
			case
				model.ElementMessageCatchEvent,
				model.ElementSignalCatchEvent,
				model.ElementTimerCatchEvent:
				// no job or task to create
			default:
				return engine.Error{
					Type:   engine.ErrorBug,
					Title:  "failed to define job or task for started execution",
					Detail: fmt.Sprintf("BPMN element type %s is not supported", execution.BpmnElementType),
				}
			}
		}

		if jobType != 0 {
			job := JobEntity{
				Partition: execution.Partition,

				ElementId:         execution.ElementId,
				ProcessId:         execution.ProcessId,
				ProcessInstanceId: execution.ProcessInstanceId,

				BpmnElementId:  execution.BpmnElementId,
				CorrelationKey: ec.processInstance.CorrelationKey,
				CreatedAt:      ctx.Time(),
				CreatedBy:      ec.engineOrWorkerId,
				DueAt:          ctx.Time(),
				Type:           jobType,
			}

			jobs = append(jobs, &job)
			jobsIdx = append(jobsIdx, i)
		} else if taskType != 0 {
			task := TaskEntity{
				Partition: execution.Partition,

				ElementId:         pgtype.Int4{Int32: execution.ElementId, Valid: true},
				ProcessId:         pgtype.Int4{Int32: execution.ProcessId, Valid: true},
				ProcessInstanceId: pgtype.Int4{Int32: execution.ProcessInstanceId, Valid: true},

				CreatedAt: ctx.Time(),
				CreatedBy: ec.engineOrWorkerId,
				DueAt:     ctx.Time(),
				Type:      taskType,

				Instance: taskInstance,
			}

			tasks = append(tasks, &task)
			tasksIdx = append(tasksIdx, i)
		}
	}

	// insert or update executions
	for _, execution := range executions {
		if execution.Id == 0 {
			execution.CreatedAt = ctx.Time()
			execution.CreatedBy = ec.engineOrWorkerId

			parent := execution.parent
			if parent != nil {
				execution.ParentId = pgtype.Int4{Int32: parent.Id, Valid: true}
				execution.parent = nil
			}

			prev := execution.prev
			if prev != nil {
				execution.PrevElementId = pgtype.Int4{Int32: prev.ElementId, Valid: true}
				execution.PrevId = pgtype.Int4{Int32: prev.Id, Valid: true}
				execution.prev = nil
			}

			if err := ctx.ElementInstances().Insert(execution); err != nil {
				return err
			}
		} else {
			execution.parent = nil

			if err := ctx.ElementInstances().Update(execution); err != nil {
				return err
			}
		}
	}

	// insert jobs
	for i, job := range jobs {
		idx := jobsIdx[i]
		job.ElementInstanceId = executions[idx].Id

		if err := ctx.Jobs().Insert(job); err != nil {
			return err
		}
	}

	// insert tasks
	for i, task := range tasks {
		idx := tasksIdx[i]
		task.ElementInstanceId = pgtype.Int4{Int32: executions[idx].Id, Valid: true}

		if err := ctx.Tasks().Insert(task); err != nil {
			return err
		}
	}

	if !ec.processInstance.EndedAt.Valid {
		return nil
	}

	// end process instance
	if err := ctx.ProcessInstances().Update(ec.processInstance); err != nil {
		return err
	}

	if err := dequeueProcessInstance(ctx, ec.processInstance, ec.engineOrWorkerId); err != nil {
		return fmt.Errorf("failed to dequeue process instance: %v", err)
	}

	if !ec.processInstance.MessageId.Valid {
		return nil
	}

	message, err := ctx.Messages().Select(ec.processInstance.MessageId.Int64)
	if err != nil {
		return err
	}

	message.ExpiresAt = pgtype.Timestamp{Time: ctx.Time(), Valid: true}
	return ctx.Messages().Update(message)
}

func (ec executionContext) handleJob(ctx Context, job *JobEntity, jobCompletion *engine.JobCompletion) error {
	execution, err := ctx.ElementInstances().Select(job.Partition, job.ElementInstanceId)
	if err != nil {
		return err
	}

	scope, err := ctx.ElementInstances().Select(job.Partition, execution.ParentId.Int32)
	if err != nil {
		return err
	}

	executions := []*ElementInstanceEntity{scope, execution}

	graph := ec.process.graph

	node, ok := graph.nodes[execution.BpmnElementId]
	if !ok {
		job.Error = pgtype.Text{String: fmt.Sprintf("BPMN process %s has no element %s", graph.processElement.Id, execution.BpmnElementId), Valid: true}
		return nil
	}

	switch job.Type {
	case engine.JobEvaluateExclusiveGateway:
		bpmnElement := node.bpmnElement
		if bpmnElement.Type != model.ElementExclusiveGateway {
			job.Error = pgtype.Text{String: fmt.Sprintf("expected BPMN element %s to be an exclusive gateway", execution.BpmnElementId), Valid: true}
			return nil
		}

		exclusiveGateway := bpmnElement.Model.(model.ExclusiveGateway)

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
			outgoing := bpmnElement.OutgoingById(exclusiveGateway.Default)
			targetId = outgoing.Target.Id
		}

		execution.State = engine.InstanceCompleted

		scope.ExecutionCount = scope.ExecutionCount - 1

		next, err := graph.createExecutionAt(scope, targetId)
		if err != nil {
			job.Error = pgtype.Text{String: fmt.Sprintf("failed to create execution at %s: %v", targetId, err), Valid: true}
			return nil
		}

		next.prev = execution

		executions = append(executions, &next)
	case engine.JobEvaluateInclusiveGateway:
		bpmnElement := node.bpmnElement
		if bpmnElement.Type != model.ElementInclusiveGateway {
			job.Error = pgtype.Text{String: fmt.Sprintf("expected BPMN element %s to be an inclusive gateway", execution.BpmnElementId), Valid: true}
			return nil
		}

		inclusiveGateway := bpmnElement.Model.(model.InclusiveGateway)

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
			outgoing := bpmnElement.OutgoingById(inclusiveGateway.Default)
			if !slices.Contains(targetIds, outgoing.Target.Id) {
				targetIds = append(targetIds, outgoing.Target.Id)
			}
		}

		execution.State = engine.InstanceCompleted

		scope.ExecutionCount = scope.ExecutionCount - 1

		for _, targetId := range targetIds {
			next, err := graph.createExecutionAt(scope, targetId)
			if err != nil {
				job.Error = pgtype.Text{String: fmt.Sprintf("failed to create execution at %s: %v", targetId, err), Valid: true}
				return nil
			}

			next.prev = execution

			executions = append(executions, &next)
		}
	case engine.JobExecute:
		boundaryEvents, err := ctx.ElementInstances().SelectBoundaryEvents(execution)
		if err != nil {
			return err
		}

		if jobCompletion != nil && jobCompletion.ErrorCode != "" {
			errorCode := jobCompletion.ErrorCode

			target := findErrorBoundaryEvent(boundaryEvents, errorCode)
			if target == nil {
				job.Error = pgtype.Text{String: fmt.Sprintf("failed to find boundary event for error code %s", errorCode), Valid: true}
				return nil
			}

			execution.State = engine.InstanceTerminated

			for _, boundaryEvent := range boundaryEvents {
				if boundaryEvent.Id == target.Id {
					boundaryEvent.State = engine.InstanceStarted
				} else {
					boundaryEvent.State = engine.InstanceTerminated
				}
				executions = append(executions, boundaryEvent)
			}

			event := EventEntity{
				Partition: target.Partition,

				ElementInstanceId: target.Id,

				CreatedAt: ctx.Time(),
				CreatedBy: ec.engineOrWorkerId,
				ErrorCode: pgtype.Text{String: errorCode, Valid: true},
			}

			if err := ctx.Events().Insert(&event); err != nil {
				return err
			}

			break
		}

		if jobCompletion != nil && jobCompletion.EscalationCode != "" {
			job.Error = pgtype.Text{String: "BPMN escalation code is not supported", Valid: true}
			return nil
		}

		for _, boundaryEvent := range boundaryEvents {
			boundaryEvent.State = engine.InstanceTerminated
			executions = append(executions, boundaryEvent)
		}
	case engine.JobSetErrorCode:
		attachedTo, err := ctx.ElementInstances().Select(execution.Partition, execution.PrevId.Int32)
		if err != nil {
			return fmt.Errorf("failed to select attached to element instance: %v", err)
		}

		if attachedTo.EndedAt.Valid {
			return nil // terminated or canceled
		}

		var errorCode string
		if jobCompletion != nil {
			errorCode = jobCompletion.ErrorCode
		}

		execution.Context = pgtype.Text{String: errorCode, Valid: true}

		attachedTo.ExecutionCount = attachedTo.ExecutionCount + 1
		executions = append(executions, attachedTo)
	case engine.JobSetTimer:
		if jobCompletion == nil || jobCompletion.Timer == nil {
			job.Error = pgtype.Text{String: "expected a timer", Valid: true}
			return nil
		}

		dueAt, err := evaluateTimer(*jobCompletion.Timer, ctx.Time())
		if err != nil {
			job.Error = pgtype.Text{String: fmt.Sprintf("failed to evaluate timer: %v", err), Valid: true}
			return nil
		}

		triggerEventTask := TaskEntity{
			Partition: execution.Partition,

			ElementId:         pgtype.Int4{Int32: execution.ElementId, Valid: true},
			ElementInstanceId: pgtype.Int4{Int32: execution.Id, Valid: true},
			ProcessId:         pgtype.Int4{Int32: execution.ProcessId, Valid: true},
			ProcessInstanceId: pgtype.Int4{Int32: execution.ProcessInstanceId, Valid: true},

			CreatedAt: ctx.Time(),
			CreatedBy: ec.engineOrWorkerId,
			DueAt:     dueAt,
			Type:      engine.TaskTriggerEvent,

			Instance: TriggerEventTask{Timer: jobCompletion.Timer},
		}

		if err := ctx.Tasks().Insert(&triggerEventTask); err != nil {
			return err
		}
	case engine.JobSubscribeMessage:
		if jobCompletion == nil || jobCompletion.MessageName == "" || jobCompletion.MessageCorrelationKey == "" {
			job.Error = pgtype.Text{String: "expected a message name and correlation key", Valid: true}
			return nil
		}

		bufferedMessage, err := ctx.Messages().SelectBuffered(jobCompletion.MessageName, jobCompletion.MessageCorrelationKey, ctx.Time())
		if err != nil && err != pgx.ErrNoRows {
			return err
		}

		if bufferedMessage != nil {
			triggerEventTask := TaskEntity{
				Partition: execution.Partition,

				ElementId:         pgtype.Int4{Int32: execution.ElementId, Valid: true},
				ElementInstanceId: pgtype.Int4{Int32: execution.Id, Valid: true},
				ProcessId:         pgtype.Int4{Int32: execution.ProcessId, Valid: true},
				ProcessInstanceId: pgtype.Int4{Int32: execution.ProcessInstanceId, Valid: true},

				CreatedAt: ctx.Time(),
				CreatedBy: ec.engineOrWorkerId,
				DueAt:     ctx.Time(),
				Type:      engine.TaskTriggerEvent,

				Instance: TriggerEventTask{MessageId: bufferedMessage.Id},
			}

			if err := ctx.Tasks().Insert(&triggerEventTask); err != nil {
				return err
			}

			bufferedMessage.ExpiresAt = pgtype.Timestamp{}
			bufferedMessage.IsCorrelated = true

			if err := ctx.Messages().Update(bufferedMessage); err != nil {
				return err
			}
		} else {
			messageSubscription := MessageSubscriptionEntity{
				Partition: execution.Partition,

				ElementId:         execution.ElementId,
				ElementInstanceId: execution.Id,
				ProcessId:         execution.ProcessId,
				ProcessInstanceId: execution.ProcessInstanceId,

				CorrelationKey: jobCompletion.MessageCorrelationKey,
				CreatedAt:      ctx.Time(),
				CreatedBy:      ec.engineOrWorkerId,
				Name:           jobCompletion.MessageName,
			}

			if err := ctx.MessageSubscriptions().Insert(&messageSubscription); err != nil {
				return err
			}
		}
	case engine.JobSubscribeSignal:
		if jobCompletion == nil || jobCompletion.SignalName == "" {
			job.Error = pgtype.Text{String: "expected a signal name", Valid: true}
			return nil
		}

		signalSubscription := SignalSubscriptionEntity{
			Partition: execution.Partition,

			ElementId:         execution.ElementId,
			ElementInstanceId: execution.Id,
			ProcessId:         execution.ProcessId,
			ProcessInstanceId: execution.ProcessInstanceId,

			CreatedAt: ctx.Time(),
			CreatedBy: ec.engineOrWorkerId,
			Name:      jobCompletion.SignalName,
		}

		if err := ctx.SignalSubscriptions().Insert(&signalSubscription); err != nil {
			return err
		}
	}

	if err := ec.continueExecutions(ctx, executions); err != nil {
		if _, ok := err.(engine.Error); ok {
			job.Error = pgtype.Text{String: err.Error(), Valid: true}
		} else {
			return fmt.Errorf("failed to continue executions %+v: %v", executions, err)
		}
	}

	return nil
}

func (ec executionContext) handleParallelGateway(ctx Context, task *TaskEntity) error {
	execution, err := ctx.ElementInstances().Select(task.Partition, task.ElementInstanceId.Int32)
	if err != nil {
		return err
	}

	if execution.State != engine.InstanceStarted {
		return nil // already completed by another task instance
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

	joined, err := ec.process.graph.joinParallelGateway(waiting)
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

	for i, joinedExecution := range joined {
		if i != 0 {
			// end all, but first joined execution
			joinedExecution.EndedAt = pgtype.Timestamp{Time: ctx.Time(), Valid: true}
			joinedExecution.State = engine.InstanceCompleted
		}
	}

	scope, err := ctx.ElementInstances().Select(execution.Partition, execution.ParentId.Int32)
	if err != nil {
		return err
	}

	scope.ExecutionCount = scope.ExecutionCount - (len(joined) - 1)

	joined = append(joined, scope)

	if err := ec.continueExecutions(ctx, joined); err != nil {
		if _, ok := err.(engine.Error); ok {
			return err
		} else {
			return fmt.Errorf("failed to continue executions %+v: %v", joined, err)
		}
	}

	return nil
}
