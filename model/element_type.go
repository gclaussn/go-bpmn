package model

import "fmt"

// ElementType describes the different BPMN element types - especially tasks, gateways and events.
type ElementType int

const (
	ElementBusinessRuleTask ElementType = iota + 1
	ElementExclusiveGateway
	ElementInclusiveGateway
	ElementManualTask
	ElementNoneEndEvent
	ElementNoneStartEvent
	ElementNoneThrowEvent
	ElementParallelGateway
	ElementProcess
	ElementScriptTask
	ElementSendTask
	ElementServiceTask
	ElementTask
	ElementTimerCatchEvent
	ElementTimerStartEvent
)

func MapElementType(s string) ElementType {
	switch s {
	case "BUSINESS_RULE_TASK":
		return ElementBusinessRuleTask
	case "EXCLUSIVE_GATEWAY":
		return ElementExclusiveGateway
	case "INCLUSIVE_GATEWAY":
		return ElementInclusiveGateway
	case "MANUAL_TASK":
		return ElementManualTask
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
	case "TASK":
		return ElementTask
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
	case ElementExclusiveGateway:
		return "EXCLUSIVE_GATEWAY"
	case ElementInclusiveGateway:
		return "INCLUSIVE_GATEWAY"
	case ElementManualTask:
		return "MANUAL_TASK"
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
	case ElementTask:
		return "TASK"
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
