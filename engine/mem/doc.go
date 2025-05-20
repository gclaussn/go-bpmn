// Package mem implements an in-memory process engine, used for testing purposes.
/*
mem provides a full implementation of the [engine.Engine] interface.

Create an Engine

Since testing must be deterministic, a mem engine is created with a disabled task executor, not running a goroutine.
If a task executor is needed, it can be configured via [mem.Options].Common.TaskExecutorEnabled.

	e, err := mem.New(func(o *mem.Options) {
		o.Common.EngineId = "my-mem-engine"
	})
	if err != nil {
		log.Fatalf("failed to create mem engine: %v", err)
	}

	defer e.Shutdown()
*/
package mem
