package worker

import (
	"context"

	"github.com/gclaussn/go-bpmn/engine"
)

type ProcessInstanceAssert2 struct {
	*engine.ProcessInstanceAssert2

	w *Worker

	partition         engine.Partition
	processInstanceId int32
}

func (a *ProcessInstanceAssert2) GetProcessVariable(name string, value any) {
	variables, err := a.w.e.GetProcessVariables(context.Background(), engine.GetProcessVariablesCmd{
		Partition:         a.partition,
		ProcessInstanceId: a.processInstanceId,
		Names:             []string{name},
	})
	if err != nil {
		a.Fatalf("failed to get process variable %s: %v", name, err)
	}

	var data *engine.Data
	for _, variable := range variables {
		if variable.Name == name {
			data = variable.Data
		}
	}
	if data == nil {
		a.Fatalf("expected process instance to have variable %s", name)
	}

	decoder := a.w.Decoder(data.Encoding)
	if decoder == nil {
		a.Fatalf("no decoder for encoding %s registered", data.Encoding)
	}

	if err := decoder.Decode(data.Value, &value); err != nil {
		a.Fatalf("failed to decode variable %s: %v", name, err)
	}
}
