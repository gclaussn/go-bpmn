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
	executions       []*ElementInstanceEntity
	process          *ProcessEntity
	processInstance  *ProcessInstanceEntity
}

func (ec *executionContext) addExecution(execution *ElementInstanceEntity) {
	ec.executions = append(ec.executions, execution)
}

// continueExecutions continues each execution until a wait state is reached or no more outgoing sequence flows exist.
func (ec *executionContext) continueExecutions(ctx Context) error {
	now := pgtype.Timestamp{Time: ctx.Time(), Valid: true}

	graph := ec.process.graph

	i := 0
	for i < len(ec.executions) {
		execution := ec.executions[i]

		i++

		if execution.ExecutionCount > 0 {
			continue // skip scope
		}

		scope, err := ec.findScope(ctx, execution)
		if err != nil {
			return err
		}

		// continue execution
		executions, err := graph.continueExecution(ec.executions, execution)
		if err != nil {
			return engine.Error{
				Type:   engine.ErrorProcessModel,
				Title:  "failed to continue execution",
				Detail: err.Error(),
			}
		}

		ec.executions = executions

		switch execution.State {
		case engine.InstanceStarted:
			execution.StartedAt = now
		case engine.InstanceCompleted:
			execution.EndedAt = now

			if !execution.StartedAt.Valid {
				execution.StartedAt = now
			}
		case engine.InstanceTerminated:
			execution.EndedAt = now
		}

		if execution.State == engine.InstanceCompleted && isTaskOrScope(execution.BpmnElementType) {
			node, _ := graph.node(execution.BpmnElementId)
			if len(node.boundaryEvents) != 0 {
				// terminate attached boundary events
				boundaryEvents, err := ctx.ElementInstances().SelectBoundaryEvents(execution.Partition, execution.Id)
				if err != nil {
					return err
				}

				for _, boundaryEvent := range boundaryEvents {
					boundaryEvent.State = engine.InstanceTerminated
					ec.addExecution(boundaryEvent)
				}
			}
		}

		if scope.ExecutionCount == 0 {
			if scope.BpmnElementType == model.ElementProcess {
				scope.State = engine.InstanceCompleted
				scope.EndedAt = now

				// complete process instance
				ec.processInstance.EndedAt = now
				ec.processInstance.State = engine.InstanceCompleted
			} else if scope.State == engine.InstanceStarted {
				ec.addExecution(scope)
			}
		}
	}

	var (
		entities []any // entities to insert
		idx      []int // indices of related executions
	)

	// create jobs and tasks
	for i, execution := range ec.executions {
		if execution.State != engine.InstanceCreated && execution.State != engine.InstanceStarted {
			continue
		}

		if execution.ExecutionCount != 0 {
			continue
		}

		if execution.Context.Valid {
			continue
		}

		var (
			jobType      engine.JobType
			taskType     engine.TaskType
			taskInstance Task
		)

		node, _ := graph.node(execution.BpmnElementId)

		switch execution.BpmnElementType {
		// task
		case
			model.ElementBusinessRuleTask,
			model.ElementScriptTask,
			model.ElementSendTask,
			model.ElementServiceTask:
			if execution.State == engine.InstanceStarted {
				jobType = engine.JobExecute
			}
		case model.ElementCallActivity:
			jobType = engine.JobCallProcess

			execution.Context = ec.processInstance.CorrelationKey // needed for the [engine.JobPassVariables] job, created when the child process instance ends
		// gateway
		case model.ElementExclusiveGateway:
			jobType = engine.JobEvaluateExclusiveGateway
		case model.ElementInclusiveGateway:
			jobType = engine.JobEvaluateInclusiveGateway
		case model.ElementParallelGateway:
			taskType = engine.TaskJoinParallelGateway
			taskInstance = JoinParallelGatewayTask{}
		// event
		case model.ElementErrorBoundaryEvent:
			if node.eventDefinition == nil {
				jobType = engine.JobSetErrorCode
			} else {
				execution.Context = pgtype.Text{String: node.eventDefinition.ErrorCode.String, Valid: true}
			}
		case model.ElementErrorEndEvent:
			if node.eventDefinition == nil {
				jobType = engine.JobSetErrorCode
			} else {
				taskType = engine.TaskTriggerEvent
				taskInstance = TriggerEventTask{}

				execution.Context = pgtype.Text{String: node.eventDefinition.ErrorCode.String, Valid: true}
			}
		case model.ElementEscalationBoundaryEvent:
			if node.eventDefinition == nil {
				jobType = engine.JobSetEscalationCode
			} else {
				execution.Context = pgtype.Text{String: node.eventDefinition.EscalationCode.String, Valid: true}
			}
		case model.ElementEscalationEndEvent, model.ElementEscalationThrowEvent:
			if node.eventDefinition == nil {
				jobType = engine.JobSetEscalationCode
			} else {
				taskType = engine.TaskTriggerEvent
				taskInstance = TriggerEventTask{}

				execution.Context = pgtype.Text{String: node.eventDefinition.EscalationCode.String, Valid: true}
			}
		case model.ElementMessageBoundaryEvent, model.ElementMessageCatchEvent:
			if node.eventDefinition == nil {
				jobType = engine.JobSubscribeMessage
			} else {
				jobType = engine.JobSetMessageCorrelationKey
			}
		case model.ElementSignalBoundaryEvent, model.ElementSignalCatchEvent:
			if node.eventDefinition == nil {
				jobType = engine.JobSubscribeSignal
			} else {
				signalSubscription := SignalSubscriptionEntity{
					Partition: execution.Partition,

					ElementId:         execution.ElementId,
					ProcessId:         execution.ProcessId,
					ProcessInstanceId: execution.ProcessInstanceId,

					BpmnElementId: execution.BpmnElementId,
					CreatedAt:     ctx.Time(),
					CreatedBy:     ec.engineOrWorkerId,
					Name:          node.eventDefinition.SignalName.String,
				}

				entities = append(entities, &signalSubscription)
				idx = append(idx, i)

				execution.Context = pgtype.Text{String: node.eventDefinition.SignalName.String, Valid: true}
			}
		case model.ElementSignalEndEvent, model.ElementSignalThrowEvent:
			if node.eventDefinition == nil {
				jobType = engine.JobSetSignalName
			} else {
				taskType = engine.TaskTriggerEvent
				taskInstance = TriggerEventTask{}

				execution.Context = pgtype.Text{String: node.eventDefinition.SignalName.String, Valid: true}
			}
		case model.ElementTimerBoundaryEvent, model.ElementTimerCatchEvent:
			if node.eventDefinition == nil {
				jobType = engine.JobSetTimer
			} else {
				timer := engine.Timer{
					Time:         timeOrNil(node.eventDefinition.Time),
					TimeCycle:    node.eventDefinition.TimeCycle.String,
					TimeDuration: engine.ISO8601Duration(node.eventDefinition.TimeDuration.String),
				}

				taskType = engine.TaskTriggerEvent
				taskInstance = TriggerEventTask{Timer: &timer}

				execution.Context = pgtype.Text{String: timer.String(), Valid: true}
			}
		case
			model.ElementMessageEndEvent,
			model.ElementMessageThrowEvent:
			jobType = engine.JobExecute
		default:
			return engine.Error{
				Type:   engine.ErrorBug,
				Title:  "failed to define job or task",
				Detail: fmt.Sprintf("BPMN element type %s is not supported", execution.BpmnElementType),
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
				State:          engine.WorkCreated,
				Type:           jobType,
			}

			entities = append(entities, &job)
			idx = append(idx, i)
		} else if taskType != 0 {
			task := TaskEntity{
				Partition: execution.Partition,

				ElementId:         pgtype.Int4{Int32: execution.ElementId, Valid: true},
				ProcessId:         pgtype.Int4{Int32: execution.ProcessId, Valid: true},
				ProcessInstanceId: pgtype.Int4{Int32: execution.ProcessInstanceId, Valid: true},

				BpmnElementId: pgtype.Text{String: execution.BpmnElementId, Valid: true},
				CreatedAt:     ctx.Time(),
				CreatedBy:     ec.engineOrWorkerId,
				DueAt:         ctx.Time(),
				State:         engine.WorkCreated,
				Type:          taskType,

				Instance: taskInstance,
			}

			if isTimerEvent(node.bpmnElement.Type) {
				timer := *taskInstance.(TriggerEventTask).Timer

				dueAt, err := evaluateTimer(timer, ctx.Time())
				if err != nil {
					return fmt.Errorf("failed to evaluate timer: %v", err)
				}

				task.DueAt = dueAt
			}

			entities = append(entities, &task)
			idx = append(idx, i)
		}
	}

	// insert or update executions
	for _, execution := range ec.executions {
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

	// insert jobs, task and subscriptions
	for i, entity := range entities {
		execution := ec.executions[idx[i]]

		switch entity := entity.(type) {
		case *JobEntity:
			entity.ElementInstanceId = execution.Id

			if err := ctx.Jobs().Insert(entity); err != nil {
				return err
			}
		case *TaskEntity:
			entity.ElementInstanceId = pgtype.Int4{Int32: execution.Id, Valid: true}

			if err := ctx.Tasks().Insert(entity); err != nil {
				return err
			}
		case *SignalSubscriptionEntity:
			entity.ElementInstanceId = execution.Id

			if err := ctx.SignalSubscriptions().Insert(entity); err != nil {
				return err
			}
		}
	}

	if !ec.processInstance.EndedAt.Valid {
		return nil
	}

	if ec.processInstance.ParentId.Valid {
		var processScope *ElementInstanceEntity
		for _, execution := range ec.executions {
			if execution.BpmnElementType == model.ElementProcess {
				processScope = execution
			}
		}

		callActivityExecution, err := ctx.ElementInstances().Select(processScope.Partition, processScope.ParentId.Int32)
		if err != nil {
			return err
		}

		passVariablesJob := JobEntity{
			Partition: callActivityExecution.Partition,

			ElementId:         callActivityExecution.ElementId,
			ElementInstanceId: callActivityExecution.Id,
			ProcessId:         callActivityExecution.ProcessId,
			ProcessInstanceId: callActivityExecution.ProcessInstanceId,

			BpmnElementId:  callActivityExecution.BpmnElementId,
			CorrelationKey: callActivityExecution.Context,
			CreatedAt:      ctx.Time(),
			CreatedBy:      ec.engineOrWorkerId,
			DueAt:          ctx.Time(),
			State:          engine.WorkCreated,
			Type:           engine.JobPassVariables,
		}

		if err := ctx.Jobs().Insert(&passVariablesJob); err != nil {
			return err
		}
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

	message.ExpiresAt = now
	return ctx.Messages().Update(message)
}

// findScope finds and returns the parent execution.
func (ec *executionContext) findScope(ctx Context, execution *ElementInstanceEntity) (*ElementInstanceEntity, error) {
	if execution.parent != nil {
		return execution.parent, nil
	}

	parentId := execution.ParentId.Int32

	// find scope within executions
	for _, scope := range ec.executions {
		if scope.Id == parentId {
			execution.parent = scope
			return scope, nil
		}
	}

	// find scope within repository
	scope, err := ctx.ElementInstances().Select(execution.Partition, parentId)
	if err != nil {
		return nil, err
	}

	execution.parent = scope
	ec.addExecution(scope)
	return scope, nil
}

func (ec *executionContext) handleJob(ctx Context, job *JobEntity, cmd engine.CompleteJobCmd) error {
	jobCompletion := cmd.Completion

	continueExecution := true

	var err error
	switch job.Type {
	case engine.JobCallProcess:
		continueExecution = false
		err = ec.callProcess(ctx, job, jobCompletion)
	case engine.JobEvaluateExclusiveGateway:
		err = ec.evaluateExclusiveGateway(job, jobCompletion)
	case engine.JobEvaluateInclusiveGateway:
		err = ec.evaluateInclusiveGateway(job, jobCompletion)
	case engine.JobExecute:
		continueExecution, err = ec.execute(ctx, job, jobCompletion, cmd.RetryTimer)
	case engine.JobPassVariables:
		// nothing to do here
	case engine.JobSetErrorCode:
		err = ec.setErrorCode(ctx, job, jobCompletion)
	case engine.JobSetEscalationCode:
		err = ec.setEscalationCode(ctx, job, jobCompletion)
	case engine.JobSetSignalName:
		err = ec.setSignalName(ctx, job, jobCompletion)
	case engine.JobSetTimer:
		err = ec.setTimer(ctx, job, jobCompletion)
	case engine.JobSubscribeMessage, engine.JobSetMessageCorrelationKey:
		err = ec.subscribeMessage(ctx, job, jobCompletion)
	case engine.JobSubscribeSignal:
		err = ec.subscribeSignal(ctx, job, jobCompletion)
	}

	if err != nil {
		return err
	}
	if !continueExecution || job.Error.Valid {
		return nil
	}

	if err := ec.continueExecutions(ctx); err != nil {
		if _, ok := err.(engine.Error); ok {
			job.Error = pgtype.Text{String: err.Error(), Valid: true}
		} else {
			return fmt.Errorf("failed to continue executions %+v: %v", ec.executions, err)
		}
	}

	return nil
}

// startBoundaryEvent starts a boundary event.
// The method expects the scope of the boundary event as first and the boundary event as second execution.
//
// If the boundary event is non-interrupting, a new boundary event execution is attached and returned.
//
// If the boundary event is interrupting, all other boundary events as well as the execution, the boundary is attached to, are terminated.
func (ec *executionContext) startBoundaryEvent(ctx Context, interrupting bool) (*ElementInstanceEntity, error) {
	boundaryEventScope, boundaryEvent := ec.executions[0], ec.executions[1]

	// start boundary event
	boundaryEvent.State = engine.InstanceStarted

	if !interrupting {
		// attach new boundary event execution
		newBoundaryEvent, err := ec.process.graph.createExecutionAt(boundaryEventScope, boundaryEvent.BpmnElementId)
		if err != nil {
			return nil, err
		}

		newBoundaryEvent.ParentId = boundaryEvent.ParentId
		newBoundaryEvent.PrevElementId = boundaryEvent.PrevElementId
		newBoundaryEvent.PrevId = boundaryEvent.PrevId

		newBoundaryEvent.Context = boundaryEvent.Context
		newBoundaryEvent.State = engine.InstanceCreated

		ec.addExecution(&newBoundaryEvent)

		return &newBoundaryEvent, nil
	}

	attachedTo, err := ctx.ElementInstances().Select(boundaryEvent.Partition, boundaryEvent.PrevId.Int32)
	if err != nil {
		return nil, err
	}

	// terminate all other attached boundary events
	boundaryEvents, err := ctx.ElementInstances().SelectBoundaryEvents(attachedTo.Partition, attachedTo.Id)
	if err != nil {
		return nil, err
	}

	for _, other := range boundaryEvents {
		if other.Id == boundaryEvent.Id {
			continue
		}

		other.State = engine.InstanceTerminated
		ec.addExecution(other)
	}

	// terminate attachedTo
	attachedTo.State = engine.InstanceTerminated
	attachedTo.parent = boundaryEventScope
	ec.addExecution(attachedTo)

	i := len(ec.executions) - 1
	for i < len(ec.executions) {
		execution := ec.executions[i]

		i++

		// if execution is a call activity, terminate descending process instances recursively
		if execution.BpmnElementType == model.ElementCallActivity {
			children, err := ctx.ElementInstances().SelectActiveChildren(execution)
			if err != nil {
				return nil, err
			}

			if len(children) == 0 {
				continue // process scope already ended
			}

			processScope := children[0]

			terminateProcessInstanceTask := TaskEntity{
				Partition: processScope.Partition,

				ProcessId:         pgtype.Int4{Int32: processScope.ProcessId, Valid: true},
				ProcessInstanceId: pgtype.Int4{Int32: processScope.ProcessInstanceId, Valid: true},

				CreatedAt: ctx.Time(),
				CreatedBy: ec.engineOrWorkerId,
				DueAt:     ctx.Time(),
				State:     engine.WorkCreated,
				Type:      engine.TaskTerminateProcessInstance,

				Instance: TerminateProcessInstanceTask{},
			}

			if err := ctx.Tasks().Insert(&terminateProcessInstanceTask); err != nil {
				return nil, err
			}
		}

		if execution.ExecutionCount <= 0 {
			continue
		}

		// if attachedTo is a scope (e.g. sub-process), terminate descending executions recursively
		children, err := ctx.ElementInstances().SelectActiveChildren(execution)
		if err != nil {
			return nil, err
		}

		for _, child := range children {
			child.State = engine.InstanceTerminated
			child.parent = execution
			ec.addExecution(child)
		}
	}

	// cancel message and signal subscriptions
	var (
		messageSubscriptionIds []int32
		signalSubscriptionIds  []int32
	)
	for _, execution := range ec.executions {
		if execution.State != engine.InstanceTerminated {
			continue
		}

		switch execution.BpmnElementType {
		case model.ElementMessageBoundaryEvent, model.ElementMessageCatchEvent:
			messageSubscriptionIds = append(messageSubscriptionIds, execution.Id)
		case model.ElementSignalBoundaryEvent, model.ElementSignalCatchEvent:
			signalSubscriptionIds = append(signalSubscriptionIds, execution.Id)
		}
	}

	if len(messageSubscriptionIds) != 0 {
		messageSubscriptions, err := ctx.MessageSubscriptions().SelectByProcessInstance(ec.processInstance)
		if err != nil {
			return nil, err
		}

		for _, messageSubscription := range messageSubscriptions {
			if !slices.Contains(messageSubscriptionIds, messageSubscription.ElementInstanceId) {
				continue
			}

			if err := ctx.MessageSubscriptions().Delete(messageSubscription); err != nil {
				return nil, err
			}
		}
	}

	if len(signalSubscriptionIds) != 0 {
		signalSubscriptions, err := ctx.SignalSubscriptions().SelectByProcessInstance(ec.processInstance)
		if err != nil {
			return nil, err
		}

		for _, signalSubscription := range signalSubscriptions {
			if !slices.Contains(signalSubscriptionIds, signalSubscription.ElementInstanceId) {
				continue
			}

			if err := ctx.SignalSubscriptions().Delete(signalSubscription); err != nil {
				return nil, err
			}
		}
	}

	// reverse execution order so that scopes are executed after their children
	// this guarantees that the continuation will decrement the execution count of a terminated scope to 0
	// otherwise a terminated scope might be skipped, leaving it's parent with a wrong execution count
	slices.SortFunc(ec.executions, func(a *ElementInstanceEntity, b *ElementInstanceEntity) int {
		return int(b.Id - a.Id)
	})

	return nil, nil
}
