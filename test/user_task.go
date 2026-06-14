package main

import (
	"io"
	"os"

	"github.com/gclaussn/go-bpmn/engine"
	"github.com/gclaussn/go-bpmn/worker"
)

type userTask struct {
}

func (h userTask) CreateProcessCmd() (engine.CreateProcessCmd, error) {
	bpmnFile, err := os.Open("./bpmn/user-task/start-end.bpmn")
	if err != nil {
		return engine.CreateProcessCmd{}, err
	}

	defer bpmnFile.Close()

	bpmnXml, err := io.ReadAll(bpmnFile)
	if err != nil {
		return engine.CreateProcessCmd{}, err
	}

	return engine.CreateProcessCmd{
		BpmnProcessId: "userTaskStartEndTest",
		BpmnXml:       string(bpmnXml),
		Version:       "1",
	}, nil
}

func (h userTask) Handle(mux worker.JobMux) error {
	return nil
}
