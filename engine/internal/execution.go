package internal

import (
	"fmt"
	"slices"

	"github.com/gclaussn/go-bpmn/engine"
	"github.com/gclaussn/go-bpmn/model"
	"github.com/jackc/pgx/v5/pgtype"
)

type executionContext struct {
	engineOrWorkerId string
	process          *ProcessEntity
	processInstance  *ProcessInstanceEntity
}

// continueExecutions continues each execution until a wait state is reached or no more outgoing sequence flows exist.
func (ec executionContext) continueExecutions(ctx Context, executions []*ElementInstanceEntity) error {
	graph := *ec.process.graph

	i := 0
	for i < len(executions) {
		execution := executions[i]

		node, ok := graph.nodes[execution.BpmnElementId]
		if !ok {
			return engine.Error{
				Type:   engine.ErrorBug,
				Title:  "failed to continue execution",
				Detail: fmt.Sprintf("BPMN process %s has no element %s", graph.processElement.Id, execution.BpmnElementId),
			}
		}

		i++

		if execution.State == engine.InstanceCompleted {
			continue // skip executions, that have been completed by the caller
		}
		if execution.ExecutionCount != 0 {
			continue // skip scopes
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

		scope := execution.parent
		if scope.State == engine.InstanceQueued {
			// end branch
			execution.State = scope.State
			continue
		} else if execution.State == engine.InstanceStarted {
			// continue branch
			execution.State = engine.InstanceCompleted
		} else if scope.State == engine.InstanceSuspended {
			// end branch
			execution.State = engine.InstanceSuspended
			continue
		} else {
			// end branch, if execution is waiting for a job to be completed or a task to be executed
			switch execution.BpmnElementType {
			case
				model.ElementBusinessRuleTask,
				model.ElementScriptTask,
				model.ElementSendTask,
				model.ElementServiceTask,
				model.ElementSignalCatchEvent,
				model.ElementTimerCatchEvent:
				execution.State = 0
			case
				model.ElementExclusiveGateway,
				model.ElementInclusiveGateway:
				if len(node.bpmnElement.Outgoing) > 1 {
					execution.State = 0
				} else {
					execution.State = engine.InstanceCompleted
				}
			case model.ElementParallelGateway:
				if len(node.bpmnElement.Incoming) > 1 {
					execution.State = 0
				} else {
					execution.State = engine.InstanceCompleted
				}
			default:
				// continue branch, if element has no behavior (pass through element)
				execution.State = engine.InstanceCompleted
			}
		}

		execution.StateChangedBy = ec.engineOrWorkerId

		switch execution.State {
		case engine.InstanceStarted:
			execution.StartedAt = pgtype.Timestamp{Time: ctx.Time(), Valid: true}
		case engine.InstanceCompleted, engine.InstanceTerminated:
			execution.EndedAt = pgtype.Timestamp{Time: ctx.Time(), Valid: true}
		}

		if execution.State == engine.InstanceCompleted {
			outgoing := node.bpmnElement.Outgoing
			for j := 0; j < len(outgoing); j++ {
				target := outgoing[j].Target
				targetNode := graph.nodes[target.Id]

				// start branch
				next := ElementInstanceEntity{
					Partition: scope.Partition,

					ElementId:         targetNode.id,
					ProcessId:         scope.ProcessId,
					ProcessInstanceId: scope.ProcessInstanceId,

					BpmnElementId:   target.Id,
					BpmnElementType: target.Type,

					parent: scope,
					prev:   execution,
				}

				executions = append(executions, &next)

				scope.ExecutionCount = scope.ExecutionCount + 1
			}

			scope.ExecutionCount = scope.ExecutionCount - 1

			if scope.ExecutionCount != 0 {
				continue
			}

			if scope.ParentId.Valid {
				executions = append(executions, scope)
				continue
			}

			// complete process scope
			scope.EndedAt = pgtype.Timestamp{Time: ctx.Time(), Valid: true}
			scope.State = engine.InstanceCompleted
			scope.StateChangedBy = ec.engineOrWorkerId

			// complete process instance
			ec.processInstance.EndedAt = pgtype.Timestamp{Time: ctx.Time(), Valid: true}
			ec.processInstance.State = engine.InstanceCompleted
			ec.processInstance.StateChangedBy = ec.engineOrWorkerId
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
		if execution.State != 0 {
			continue
		}

		execution.StartedAt = pgtype.Timestamp{Time: ctx.Time(), Valid: true}
		execution.State = engine.InstanceStarted

		var (
			jobType      engine.JobType
			taskType     engine.TaskType
			taskInstance Task
		)

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
		case model.ElementSignalCatchEvent:
			jobType = engine.JobSubscribeSignal
		case model.ElementTimerCatchEvent:
			jobType = engine.JobSetTimer
		default:
			return engine.Error{
				Type:   engine.ErrorBug,
				Title:  "failed to create job or task",
				Detail: fmt.Sprintf("BPMN element type %s is not supported", execution.BpmnElementType),
			}
		}

		if jobType != 0 {
			retryTimer := ctx.Options().JobRetryTimer

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
				RetryCount:     ctx.Options().JobRetryCount,
				RetryTimer:     pgtype.Text{String: retryTimer.String(), Valid: !retryTimer.IsZero()},
				Type:           jobType,
			}

			jobs = append(jobs, &job)
			jobsIdx = append(jobsIdx, i)
		} else {
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

			if execution.State == engine.InstanceCompleted { // pass through element
				execution.StartedAt = pgtype.Timestamp{Time: ctx.Time(), Valid: true}
			}

			parent := execution.parent
			if parent != nil {
				execution.ParentId = pgtype.Int4{Int32: parent.Id, Valid: true}
			}

			prev := execution.prev
			if prev != nil {
				execution.PrevElementId = pgtype.Int4{Int32: prev.ElementId, Valid: true}
				execution.PrevElementInstanceId = pgtype.Int4{Int32: prev.Id, Valid: true}
			}

			execution.parent = nil
			execution.prev = nil

			if err := ctx.ElementInstances().Insert(execution); err != nil {
				return err
			}
		} else {
			execution.parent = nil
			execution.prev = nil

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

	if err := dequeueProcessInstance(ctx, ec.processInstance); err != nil {
		return fmt.Errorf("failed to dequeue process instance: %v", err)
	}

	return nil
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

	switch job.Type {
	case engine.JobEvaluateExclusiveGateway:
		if jobCompletion == nil || jobCompletion.ExclusiveGatewayDecision == "" {
			job.Error = pgtype.Text{String: "expected an exclusive gateway decision", Valid: true}
			return nil
		}

		bpmnElementId := jobCompletion.ExclusiveGatewayDecision
		if err := ec.process.graph.ensureSequenceFlow(job.BpmnElementId, bpmnElementId); err != nil {
			job.Error = pgtype.Text{String: err.Error(), Valid: true}
			return nil
		}

		execution.EndedAt = pgtype.Timestamp{Time: ctx.Time(), Valid: true}
		execution.State = engine.InstanceCompleted
		execution.StateChangedBy = ec.engineOrWorkerId

		scope.ExecutionCount = scope.ExecutionCount - 1

		next, err := ec.process.graph.createExecutionAt(scope, bpmnElementId)
		if err != nil {
			job.Error = pgtype.Text{String: fmt.Sprintf("failed to create execution at %s: %v", bpmnElementId, err), Valid: true}
			return nil
		}

		next.prev = execution

		executions = append(executions, &next)
	case engine.JobEvaluateInclusiveGateway:
		if jobCompletion == nil || len(jobCompletion.InclusiveGatewayDecision) == 0 {
			job.Error = pgtype.Text{String: "expected an inclusive gateway decision", Valid: true}
			return nil
		}

		bpmnElementIds := jobCompletion.InclusiveGatewayDecision
		for i := range bpmnElementIds {
			for j := i + 1; j < len(bpmnElementIds); j++ {
				if bpmnElementIds[i] == bpmnElementIds[j] {
					job.Error = pgtype.Text{String: fmt.Sprintf("decision contains duplicate BPMN element ID %s", bpmnElementIds[i]), Valid: true}
					return nil
				}
			}
		}
		for i := range bpmnElementIds {
			if err := ec.process.graph.ensureSequenceFlow(job.BpmnElementId, bpmnElementIds[i]); err != nil {
				job.Error = pgtype.Text{String: err.Error(), Valid: true}
				return nil
			}
		}

		execution.EndedAt = pgtype.Timestamp{Time: ctx.Time(), Valid: true}
		execution.State = engine.InstanceCompleted
		execution.StateChangedBy = ec.engineOrWorkerId

		scope.ExecutionCount = scope.ExecutionCount - 1

		for i := range bpmnElementIds {
			next, err := ec.process.graph.createExecutionAt(scope, bpmnElementIds[i])
			if err != nil {
				job.Error = pgtype.Text{String: fmt.Sprintf("failed to create execution at %s: %v", bpmnElementIds[i], err), Valid: true}
				return nil
			}

			next.prev = execution

			executions = append(executions, &next)
		}
	case engine.JobExecute:
		if jobCompletion != nil && jobCompletion.ErrorCode != "" {
			job.Error = pgtype.Text{String: "BPMN error code is not supported", Valid: true}
			return nil
		}
		if jobCompletion != nil && jobCompletion.EscalationCode != "" {
			job.Error = pgtype.Text{String: "BPMN escalation code is not supported", Valid: true}
			return nil
		}
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

		return nil // do not continue execution
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

		return nil // do not continue execution
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
			joinedExecution.StateChangedBy = ec.engineOrWorkerId
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
