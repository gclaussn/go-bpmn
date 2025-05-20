// Package worker provides a SDK to implement processes.
/*
worker offers a delegation mechanims of jobs, created within an engine to interact with running process instances.

Create a Worker

A worker requires an engine.
The engine can be an embedded engine (pg, or mem for testing) or a remote engine (HTTP client).

	w, err := worker.New(e, func(o *worker.Options) {
		o.OnJobExecutionFailure = func(job engine.Job, err error) {
			log.Printf("failed to execute job %s: %v", job, err)
		}
	})
	if err != nil {
		log.Fatalf("failed to create worker: %v", err)
	}

Implement a Process

A process must implement the [Delegate] interface.

	type exampleDelegate struct {}

	func (d exampleDelegate) CreateProcessCmd() (engine.CreateProcessCmd, error) {
		bpmnFile, err := os.Open("./example.bpmn")
		if err != nil {
			return engine.CreateProcessCmd{}, err
		}

		defer bpmnFile.Close()

		bpmnXml, err := io.ReadAll(bpmnFile)
		if err != nil {
			return engine.CreateProcessCmd{}, err
		}

		return engine.CreateProcessCmd{
			BpmnProcessId: "example",
			BpmnXml:       string(bpmnXml),
			Version:       "1",
		}, nil
	}

	func (d exampleDelegate) Delegate(delegator worker.Delegator) error {
		delegator.Execute("serviceTask", d.executeServiceTask)
		return nil
	}

	func (d exampleDelegate) executeServiceTask(jc worker.JobContext) error {
		// ...
		return nil
	}

Run a Worker

Before a worker is started, implemented delegates must be registered.

	exampleProcess, err := w.Register(exampleDelegate{})
	if err != nil {
		log.Fatalf("failed to register example delegate: %v", err)
	}

	w.Start()

	signalC := make(chan os.Signal, 1)
	signal.Notify(signalC, os.Interrupt, syscall.SIGTERM)

	<-signalC

	w.Stop()

Create a Process Instance

A registered process can be used to create a new process instance with variables.

	variables := worker.Variables{}
	variables.PutVariable("a", "value a")

	processInstance, err := exampleProcess.CreateProcessInstance(variables)
	if err != nil {
		log.Printf("failed to create process instance: %v", err)
	}
*/
package worker
