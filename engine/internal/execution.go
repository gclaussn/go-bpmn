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
				boundaryEvents, err := ctx.ElementInstances().SelectBoundaryEvents(execution)
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
	scope := ec.executions[0]
	execution := ec.executions[1]

	graph := ec.process.graph

	node, err := graph.node(execution.BpmnElementId)
	if err != nil {
		job.Error = pgtype.Text{String: err.Error(), Valid: true}
		return nil
	}

	jobCompletion := cmd.Completion

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

		next, err := graph.createExecutionAt(scope, targetId)
		if err != nil {
			job.Error = pgtype.Text{String: fmt.Sprintf("failed to create execution at %s: %v", targetId, err), Valid: true}
			return nil
		}

		ec.addExecution(&next)
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

		for _, targetId := range targetIds {
			next, err := graph.createExecutionAt(scope, targetId)
			if err != nil {
				job.Error = pgtype.Text{String: fmt.Sprintf("failed to create execution at %s: %v", targetId, err), Valid: true}
				return nil
			}

			ec.addExecution(&next)
		}
	case engine.JobExecute:
		var (
			errorCodeSet      bool
			escalationCodeSet bool
		)
		if jobCompletion != nil {
			if jobCompletion.ErrorCode != "" {
				errorCodeSet = true
			}

			if jobCompletion.EscalationCode != "" {
				escalationCodeSet = true
			}

			if errorCodeSet && escalationCodeSet {
				job.Error = pgtype.Text{String: "expected error code or escalation code to be set", Valid: true}
				return nil
			}
		}

		if !errorCodeSet && !escalationCodeSet {
			break
		}

		boundaryEvents, err := ctx.ElementInstances().SelectBoundaryEvents(execution)
		if err != nil {
			return err
		}

		var (
			boundaryEvent *ElementInstanceEntity
			interrupting  = true
		)
		if errorCodeSet {
			errorCode := jobCompletion.ErrorCode

			boundaryEvent = findErrorBoundaryEvent(boundaryEvents, errorCode)
			if boundaryEvent == nil {
				job.Error = pgtype.Text{String: fmt.Sprintf("failed to find boundary event for error code %s", errorCode), Valid: true}
				return nil
			}

			event := EventEntity{
				Partition: boundaryEvent.Partition,

				ElementInstanceId: boundaryEvent.Id,

				CreatedAt: ctx.Time(),
				CreatedBy: ec.engineOrWorkerId,
				ErrorCode: pgtype.Text{String: errorCode, Valid: true},
			}

			if err := ctx.Events().Insert(&event); err != nil {
				return err
			}
		} else {
			escalationCode := jobCompletion.EscalationCode

			boundaryEvent = findEscalationBoundaryEvent(boundaryEvents, escalationCode)
			if boundaryEvent == nil {
				job.Error = pgtype.Text{String: fmt.Sprintf("failed to find boundary event for escalation code %s", escalationCode), Valid: true}
				return nil
			}

			boundaryEventNode, err := graph.node(boundaryEvent.BpmnElementId)
			if err != nil {
				job.Error = pgtype.Text{String: err.Error(), Valid: true}
				return nil
			}

			interrupting = boundaryEventNode.bpmnElement.Model.(model.BoundaryEvent).CancelActivity

			event := EventEntity{
				Partition: boundaryEvent.Partition,

				ElementInstanceId: boundaryEvent.Id,

				CreatedAt:      ctx.Time(),
				CreatedBy:      ec.engineOrWorkerId,
				EscalationCode: pgtype.Text{String: escalationCode, Valid: true},
			}

			if err := ctx.Events().Insert(&event); err != nil {
				return err
			}
		}

		// overwrite executions to fulfill startBoundaryEvent contract
		ec.executions = []*ElementInstanceEntity{scope, boundaryEvent}

		newBoundaryEvent, err := ec.startBoundaryEvent(ctx, interrupting)
		if err != nil {
			return err
		}

		if newBoundaryEvent != nil {
			// retry job
			retryTimer := engine.ISO8601Duration(cmd.RetryTimer)

			retry := JobEntity{
				Partition: job.Partition,

				ElementId:         job.ElementId,
				ElementInstanceId: job.ElementInstanceId,
				ProcessId:         job.ProcessId,
				ProcessInstanceId: job.ProcessInstanceId,

				BpmnElementId:  job.BpmnElementId,
				CorrelationKey: job.CorrelationKey,
				CreatedAt:      ctx.Time(),
				CreatedBy:      ec.engineOrWorkerId,
				DueAt:          retryTimer.Calculate(ctx.Time()),
				RetryCount:     job.RetryCount,
				State:          engine.WorkCreated,
				Type:           job.Type,
			}

			if err := ctx.Jobs().Insert(&retry); err != nil {
				return err
			}

			// overwrite executions to avoid completion of execution (e.g. service task)
			ec.executions = []*ElementInstanceEntity{scope, newBoundaryEvent, boundaryEvent}
		}
	case engine.JobSetErrorCode, engine.JobSetEscalationCode:
		var context string
		if jobCompletion != nil {
			switch job.Type {
			case engine.JobSetErrorCode:
				context = jobCompletion.ErrorCode
			case engine.JobSetEscalationCode:
				context = jobCompletion.EscalationCode
			}
		}

		switch execution.BpmnElementType {
		case model.ElementErrorBoundaryEvent, model.ElementEscalationBoundaryEvent:
			attachedTo, err := ctx.ElementInstances().Select(execution.Partition, execution.PrevId.Int32)
			if err != nil {
				return fmt.Errorf("failed to select attached to element instance: %v", err)
			}

			if attachedTo.EndedAt.Valid {
				return nil // terminated or canceled
			}

			attachedTo.ExecutionCount++
			ec.addExecution(attachedTo)
		case model.ElementErrorEndEvent:
			if context == "" {
				job.Error = pgtype.Text{String: "expected an error code", Valid: true}
				return nil
			}

			triggerEventTask := TaskEntity{
				Partition: execution.Partition,

				ElementId:         pgtype.Int4{Int32: execution.ElementId, Valid: true},
				ElementInstanceId: pgtype.Int4{Int32: execution.Id, Valid: true},
				ProcessId:         pgtype.Int4{Int32: execution.ProcessId, Valid: true},
				ProcessInstanceId: pgtype.Int4{Int32: execution.ProcessInstanceId, Valid: true},

				BpmnElementId: pgtype.Text{String: execution.BpmnElementId, Valid: true},
				CreatedAt:     ctx.Time(),
				CreatedBy:     ec.engineOrWorkerId,
				DueAt:         ctx.Time(),
				State:         engine.WorkCreated,
				Type:          engine.TaskTriggerEvent,

				Instance: TriggerEventTask{},
			}

			if err := ctx.Tasks().Insert(&triggerEventTask); err != nil {
				return err
			}
		case model.ElementEscalationEndEvent, model.ElementEscalationThrowEvent:
			if context == "" {
				job.Error = pgtype.Text{String: "expected an escalation code", Valid: true}
				return nil
			}

			triggerEventTask := TaskEntity{
				Partition: execution.Partition,

				ElementId:         pgtype.Int4{Int32: execution.ElementId, Valid: true},
				ElementInstanceId: pgtype.Int4{Int32: execution.Id, Valid: true},
				ProcessId:         pgtype.Int4{Int32: execution.ProcessId, Valid: true},
				ProcessInstanceId: pgtype.Int4{Int32: execution.ProcessInstanceId, Valid: true},

				BpmnElementId: pgtype.Text{String: execution.BpmnElementId, Valid: true},
				CreatedAt:     ctx.Time(),
				CreatedBy:     ec.engineOrWorkerId,
				DueAt:         ctx.Time(),
				State:         engine.WorkCreated,
				Type:          engine.TaskTriggerEvent,

				Instance: TriggerEventTask{},
			}

			if err := ctx.Tasks().Insert(&triggerEventTask); err != nil {
				return err
			}
		}

		execution.Context = pgtype.Text{String: context, Valid: true}
	case engine.JobSetSignalName:
		if jobCompletion == nil || jobCompletion.SignalName == "" {
			job.Error = pgtype.Text{String: "expected a signal name", Valid: true}
			return nil
		}

		triggerEventTask := TaskEntity{
			Partition: execution.Partition,

			ElementId:         pgtype.Int4{Int32: execution.ElementId, Valid: true},
			ElementInstanceId: pgtype.Int4{Int32: execution.Id, Valid: true},
			ProcessId:         pgtype.Int4{Int32: execution.ProcessId, Valid: true},
			ProcessInstanceId: pgtype.Int4{Int32: execution.ProcessInstanceId, Valid: true},

			BpmnElementId: pgtype.Text{String: execution.BpmnElementId, Valid: true},
			CreatedAt:     ctx.Time(),
			CreatedBy:     ec.engineOrWorkerId,
			DueAt:         ctx.Time(),
			State:         engine.WorkCreated,
			Type:          engine.TaskTriggerEvent,

			Instance: TriggerEventTask{},
		}

		if err := ctx.Tasks().Insert(&triggerEventTask); err != nil {
			return err
		}

		execution.Context = pgtype.Text{String: jobCompletion.SignalName, Valid: true}
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

		if node.bpmnElement.Type == model.ElementTimerBoundaryEvent {
			attachedTo, err := ctx.ElementInstances().Select(execution.Partition, execution.PrevId.Int32)
			if err != nil {
				return fmt.Errorf("failed to select attached to element instance: %v", err)
			}

			if attachedTo.EndedAt.Valid {
				return nil // terminated or canceled
			}

			attachedTo.ExecutionCount++
			ec.addExecution(attachedTo)
		}

		triggerEventTask := TaskEntity{
			Partition: execution.Partition,

			ElementId:         pgtype.Int4{Int32: execution.ElementId, Valid: true},
			ElementInstanceId: pgtype.Int4{Int32: execution.Id, Valid: true},
			ProcessId:         pgtype.Int4{Int32: execution.ProcessId, Valid: true},
			ProcessInstanceId: pgtype.Int4{Int32: execution.ProcessInstanceId, Valid: true},

			BpmnElementId: pgtype.Text{String: execution.BpmnElementId, Valid: true},
			CreatedAt:     ctx.Time(),
			CreatedBy:     ec.engineOrWorkerId,
			DueAt:         dueAt,
			State:         engine.WorkCreated,
			Type:          engine.TaskTriggerEvent,

			Instance: TriggerEventTask{Timer: jobCompletion.Timer},
		}

		if err := ctx.Tasks().Insert(&triggerEventTask); err != nil {
			return err
		}

		execution.Context = pgtype.Text{String: jobCompletion.Timer.String(), Valid: true}
	case engine.JobSubscribeMessage, engine.JobSetMessageCorrelationKey:
		if job.Type == engine.JobSubscribeMessage {
			if jobCompletion == nil || jobCompletion.MessageName == "" || jobCompletion.MessageCorrelationKey == "" {
				job.Error = pgtype.Text{String: "expected a message name and correlation key", Valid: true}
				return nil
			}
		} else {
			if jobCompletion == nil || jobCompletion.MessageCorrelationKey == "" {
				job.Error = pgtype.Text{String: "expected a message correlation key", Valid: true}
				return nil
			}

			jobCompletion.MessageName = node.eventDefinition.MessageName.String
		}

		var (
			attachedTo   *ElementInstanceEntity
			interrupting bool
		)
		if node.bpmnElement.Type == model.ElementMessageBoundaryEvent {
			selectedAttachedTo, err := ctx.ElementInstances().Select(execution.Partition, execution.PrevId.Int32)
			if err != nil {
				return fmt.Errorf("failed to select attached to element instance: %v", err)
			}

			if selectedAttachedTo.EndedAt.Valid {
				return nil // terminated or canceled
			}

			attachedTo = selectedAttachedTo

			interrupting = node.bpmnElement.Model.(model.BoundaryEvent).CancelActivity
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

				BpmnElementId: pgtype.Text{String: execution.BpmnElementId, Valid: true},
				CreatedAt:     ctx.Time(),
				CreatedBy:     ec.engineOrWorkerId,
				DueAt:         ctx.Time(),
				State:         engine.WorkCreated,
				Type:          engine.TaskTriggerEvent,

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

			if attachedTo != nil && !interrupting {
				attachedTo.ExecutionCount++
				ec.addExecution(attachedTo)
			}
		} else {
			messageSubscription := MessageSubscriptionEntity{
				Partition: execution.Partition,

				ElementId:         execution.ElementId,
				ElementInstanceId: execution.Id,
				ProcessId:         execution.ProcessId,
				ProcessInstanceId: execution.ProcessInstanceId,

				BpmnElementId:  execution.BpmnElementId,
				CorrelationKey: jobCompletion.MessageCorrelationKey,
				CreatedAt:      ctx.Time(),
				CreatedBy:      ec.engineOrWorkerId,
				Name:           jobCompletion.MessageName,
			}

			if err := ctx.MessageSubscriptions().Insert(&messageSubscription); err != nil {
				return err
			}

			if attachedTo != nil {
				attachedTo.ExecutionCount++
				ec.addExecution(attachedTo)
			}
		}

		execution.Context = pgtype.Text{String: jobCompletion.MessageName, Valid: true}
	case engine.JobSubscribeSignal:
		if jobCompletion == nil || jobCompletion.SignalName == "" {
			job.Error = pgtype.Text{String: "expected a signal name", Valid: true}
			return nil
		}

		if node.bpmnElement.Type == model.ElementSignalBoundaryEvent {
			attachedTo, err := ctx.ElementInstances().Select(execution.Partition, execution.PrevId.Int32)
			if err != nil {
				return fmt.Errorf("failed to select attached to element instance: %v", err)
			}

			if attachedTo.EndedAt.Valid {
				return nil // terminated or canceled
			}

			attachedTo.ExecutionCount++
			ec.addExecution(attachedTo)
		}

		signalSubscription := SignalSubscriptionEntity{
			Partition: execution.Partition,

			ElementId:         execution.ElementId,
			ElementInstanceId: execution.Id,
			ProcessId:         execution.ProcessId,
			ProcessInstanceId: execution.ProcessInstanceId,

			BpmnElementId: execution.BpmnElementId,
			CreatedAt:     ctx.Time(),
			CreatedBy:     ec.engineOrWorkerId,
			Name:          jobCompletion.SignalName,
		}

		if err := ctx.SignalSubscriptions().Insert(&signalSubscription); err != nil {
			return err
		}

		execution.Context = pgtype.Text{String: jobCompletion.SignalName, Valid: true}
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

func (ec *executionContext) handleParallelGateway(ctx Context, execution *ElementInstanceEntity) error {
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

	scope, err := ctx.ElementInstances().Select(execution.Partition, execution.ParentId.Int32)
	if err != nil {
		return err
	}

	ec.addExecution(scope)

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

// startBoundaryEvent starts a boundary event.
// The method expects the scope of the boundary event as first and the boundary event as second execution.
//
// If the boundary event is non-interrupting, a new boundary event execution is attached.
//
// If the boundary event is interrupting, all other boundary events as well as the execution, the boundary is attached to, are terminated.
func (ec *executionContext) startBoundaryEvent(ctx Context, interrupting bool) (*ElementInstanceEntity, error) {
	scope := ec.executions[0]
	boundaryEvent := ec.executions[1]

	// start boundary event
	boundaryEvent.State = engine.InstanceStarted

	if !interrupting {
		// attach new boundary event execution
		newBoundaryEvent, err := ec.process.graph.createExecutionAt(scope, boundaryEvent.BpmnElementId)
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
	boundaryEvents, err := ctx.ElementInstances().SelectBoundaryEvents(attachedTo)
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

	// terminate attachedTo and it's children recursively, if attachedTo is a scope (e.g. sub-process)
	i := len(ec.executions)

	attachedTo.parent = scope
	ec.addExecution(attachedTo)

	for i < len(ec.executions) {
		execution := ec.executions[i]
		execution.State = engine.InstanceTerminated

		i++

		if execution.ExecutionCount <= 0 {
			continue
		}

		children, err := ctx.ElementInstances().SelectActiveChildren(execution)
		if err != nil {
			return nil, err
		}

		for _, child := range children {
			child.parent = execution
			ec.addExecution(child)
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
