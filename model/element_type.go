package model

import "fmt"

// ElementType describes the different BPMN element types - especially tasks, gateways and events.
type ElementType int

const (
	ElementBusinessRuleTask ElementType = iota + 1
	ElementErrorBoundaryEvent
	ElementErrorEndEvent
	ElementEscalationBoundaryEvent
	ElementEscalationEndEvent
	ElementEscalationThrowEvent
	ElementExclusiveGateway
	ElementInclusiveGateway
	ElementManualTask
	ElementMessageBoundaryEvent
	ElementMessageCatchEvent
	ElementMessageEndEvent
	ElementMessageStartEvent
	ElementMessageThrowEvent
	ElementNoneEndEvent
	ElementNoneStartEvent
	ElementNoneThrowEvent
	ElementParallelGateway
	ElementProcess
	ElementScriptTask
	ElementSendTask
	ElementServiceTask
	ElementSignalBoundaryEvent
	ElementSignalCatchEvent
	ElementSignalEndEvent
	ElementSignalStartEvent
	ElementSignalThrowEvent
	ElementSubProcess
	ElementTask
	ElementTimerBoundaryEvent
	ElementTimerCatchEvent
	ElementTimerStartEvent
)

func MapElementType(s string) ElementType {
	switch s {
	case "BUSINESS_RULE_TASK":
		return ElementBusinessRuleTask
	case "ERROR_BOUNDARY_EVENT":
		return ElementErrorBoundaryEvent
	case "ERROR_END_EVENT":
		return ElementErrorEndEvent
	case "ESCALATION_BOUNDARY_EVENT":
		return ElementEscalationBoundaryEvent
	case "ESCALATION_END_EVENT":
		return ElementEscalationEndEvent
	case "ESCALATION_THROW_EVENT":
		return ElementEscalationThrowEvent
	case "EXCLUSIVE_GATEWAY":
		return ElementExclusiveGateway
	case "INCLUSIVE_GATEWAY":
		return ElementInclusiveGateway
	case "MANUAL_TASK":
		return ElementManualTask
	case "MESSAGE_BOUNDARY_EVENT":
		return ElementMessageBoundaryEvent
	case "MESSAGE_CATCH_EVENT":
		return ElementMessageCatchEvent
	case "MESSAGE_END_EVENT":
		return ElementMessageEndEvent
	case "MESSAGE_START_EVENT":
		return ElementMessageStartEvent
	case "MESSAGE_THROW_EVENT":
		return ElementMessageThrowEvent
	case "NONE_END_EVENT":
		return ElementNoneEndEvent
	case "NONE_START_EVENT":
		return ElementNoneStartEvent
	case "NONE_THROW_EVENT":
		return ElementNoneThrowEvent
	case "PARALLEL_GATEWAY":
		return ElementParallelGateway
	case "PROCESS":
		return ElementProcess
	case "SCRIPT_TASK":
		return ElementScriptTask
	case "SEND_TASK":
		return ElementSendTask
	case "SERVICE_TASK":
		return ElementServiceTask
	case "SIGNAL_BOUNDARY_EVENT":
		return ElementSignalBoundaryEvent
	case "SIGNAL_CATCH_EVENT":
		return ElementSignalCatchEvent
	case "SIGNAL_END_EVENT":
		return ElementSignalEndEvent
	case "SIGNAL_START_EVENT":
		return ElementSignalStartEvent
	case "SIGNAL_THROW_EVENT":
		return ElementSignalThrowEvent
	case "SUB_PROCESS":
		return ElementSubProcess
	case "TASK":
		return ElementTask
	case "TIMER_BOUNDARY_EVENT":
		return ElementTimerBoundaryEvent
	case "TIMER_CATCH_EVENT":
		return ElementTimerCatchEvent
	case "TIMER_START_EVENT":
		return ElementTimerStartEvent
	default:
		return 0
	}
}

func (v ElementType) MarshalJSON() ([]byte, error) {
	s := v.String()
	if s == "" {
		return []byte("null"), nil
	}
	return []byte(fmt.Sprintf("%q", s)), nil
}

func (v ElementType) String() string {
	switch v {
	case ElementBusinessRuleTask:
		return "BUSINESS_RULE_TASK"
	case ElementErrorBoundaryEvent:
		return "ERROR_BOUNDARY_EVENT"
	case ElementErrorEndEvent:
		return "ERROR_END_EVENT"
	case ElementEscalationBoundaryEvent:
		return "ESCALATION_BOUNDARY_EVENT"
	case ElementEscalationEndEvent:
		return "ESCALATION_END_EVENT"
	case ElementEscalationThrowEvent:
		return "ESCALATION_THROW_EVENT"
	case ElementExclusiveGateway:
		return "EXCLUSIVE_GATEWAY"
	case ElementInclusiveGateway:
		return "INCLUSIVE_GATEWAY"
	case ElementManualTask:
		return "MANUAL_TASK"
	case ElementMessageBoundaryEvent:
		return "MESSAGE_BOUNDARY_EVENT"
	case ElementMessageCatchEvent:
		return "MESSAGE_CATCH_EVENT"
	case ElementMessageEndEvent:
		return "MESSAGE_END_EVENT"
	case ElementMessageStartEvent:
		return "MESSAGE_START_EVENT"
	case ElementMessageThrowEvent:
		return "MESSAGE_THROW_EVENT"
	case ElementNoneEndEvent:
		return "NONE_END_EVENT"
	case ElementNoneStartEvent:
		return "NONE_START_EVENT"
	case ElementNoneThrowEvent:
		return "NONE_THROW_EVENT"
	case ElementParallelGateway:
		return "PARALLEL_GATEWAY"
	case ElementProcess:
		return "PROCESS"
	case ElementScriptTask:
		return "SCRIPT_TASK"
	case ElementSendTask:
		return "SEND_TASK"
	case ElementServiceTask:
		return "SERVICE_TASK"
	case ElementSignalBoundaryEvent:
		return "SIGNAL_BOUNDARY_EVENT"
	case ElementSignalCatchEvent:
		return "SIGNAL_CATCH_EVENT"
	case ElementSignalEndEvent:
		return "SIGNAL_END_EVENT"
	case ElementSignalStartEvent:
		return "SIGNAL_START_EVENT"
	case ElementSignalThrowEvent:
		return "SIGNAL_THROW_EVENT"
	case ElementSubProcess:
		return "SUB_PROCESS"
	case ElementTask:
		return "TASK"
	case ElementTimerBoundaryEvent:
		return "TIMER_BOUNDARY_EVENT"
	case ElementTimerCatchEvent:
		return "TIMER_CATCH_EVENT"
	case ElementTimerStartEvent:
		return "TIMER_START_EVENT"
	default:
		return ""
	}
}

func (v *ElementType) UnmarshalJSON(data []byte) error {
	s := string(data)
	if s == "null" {
		return nil
	}
	if len(s) > 2 {
		s = s[1 : len(s)-1]
		*v = MapElementType(s)
	}
	if *v == 0 {
		return fmt.Errorf("invalid element type data %s", s)
	}
	return nil
}

func IsBoundaryEvent(elementType ElementType) bool {
	switch elementType {
	case
		ElementErrorBoundaryEvent,
		ElementEscalationBoundaryEvent,
		ElementMessageBoundaryEvent,
		ElementSignalBoundaryEvent,
		ElementTimerBoundaryEvent:
		return true
	default:
		return false
	}
}

func IsStartEvent(elementType ElementType) bool {
	switch elementType {
	case
		ElementNoneStartEvent,
		ElementMessageStartEvent,
		ElementSignalStartEvent,
		ElementTimerStartEvent:
		return true
	default:
		return false
	}
}
