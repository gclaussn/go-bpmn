package cli

import (
	"fmt"
	"time"

	"github.com/gclaussn/go-bpmn/engine"
)

// instanceStateValue is a custom flag value for a instance state.
type instanceStateValue engine.InstanceState

func (v *instanceStateValue) Set(s string) error {
	instanceState := engine.MapInstanceState(s)
	if instanceState == 0 {
		return fmt.Errorf("invalid instance state %s", s)
	}

	*v = instanceStateValue(instanceState)
	return nil
}

func (v instanceStateValue) String() string {
	return engine.InstanceState(v).String()
}

func (v instanceStateValue) Type() string {
	return "instanceState"
}

// iso8601DurationValue is a custom flag value for a ISO 8601 duration.
type iso8601DurationValue engine.ISO8601Duration

func (v *iso8601DurationValue) Set(s string) error {
	d, err := engine.NewISO8601Duration(s)
	if err != nil {
		return err
	}

	*v = iso8601DurationValue(d)
	return nil
}

func (v iso8601DurationValue) String() string {
	return engine.ISO8601Duration(v).String()
}

func (v iso8601DurationValue) Type() string {
	return "iso8601Duration"
}

// partitionValue is a custom flag value for an entity partition.
type partitionValue engine.Partition

func (v *partitionValue) Set(s string) error {
	p, err := engine.NewPartition(s)
	if err != nil {
		return err
	}

	*v = partitionValue(p)
	return nil
}

func (v partitionValue) String() string {
	return engine.Partition(v).String()
}

func (v partitionValue) Type() string {
	return "partition"
}

// instanceStateValue is a custom flag value for a task type.
type taskTypeValue engine.TaskType

func (v *taskTypeValue) Set(s string) error {
	taskType := engine.MapTaskType(s)
	if taskType == 0 {
		return fmt.Errorf("invalid task type %s", s)
	}

	*v = taskTypeValue(taskType)
	return nil
}

func (v taskTypeValue) String() string {
	return engine.TaskType(v).String()
}

func (v taskTypeValue) Type() string {
	return "taskType"
}

type timeValue time.Time

func (v *timeValue) Set(s string) error {
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		return err
	}

	*v = timeValue(t)
	return nil
}

func (v timeValue) String() string {
	return time.Time(v).Format(time.RFC3339)
}

func (v timeValue) Type() string {
	return "time"
}
