// Package server implements the engine's HTTP API.
/*
server implements handlers for each engine command and query, using the [net/http] package.

Run a Server

A server requires an engine.
Moreover for authentication, either a [pg.ApiKeyManager] (implemented by a pg engine) or basic auth username and password must be set.

A server is listening on "127.0.0.1:8080".
The TCP bind address as well as various timeouts can be configured by customizing the configuration.

	server, err := server.New(e, func(o *server.Options) {
		o.ApiKeyManager = apiKeyManager
	})
	if err != nil {
		log.Fatalf("failed to create HTTP server: %v", err)
	}

	server.ListenAndServe()

	signalC := make(chan os.Signal, 1)
	signal.Notify(signalC, os.Interrupt, syscall.SIGTERM)

	<-signalC

	server.Shutdown()
*/
package server
