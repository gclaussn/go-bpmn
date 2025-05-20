// Package client is used to interact with an engine via HTTP.
/*
client provides a full implementation of the [engine.Engine] interface.

Create a Client

A client requires the base URL of a HTTP server and an authorization string.

When running against a pg engine, the authorization of an API key must be used.
An API key is created via the "go-bpmn-pgd" command - for example: "go-bpmn-pgd -create-api-key -secret-id my-worker".

When running against a mem engine, basic authentication must be used.

	client, err := client.New("http://localhost:8080", "Basic dGVzdHVzZXJuYW1lOnRlc3RwYXNzd29yZA==")
	if err != nil {
		log.Fatalf("failed to create HTTP client: %v", err)
	}

	defer client.Shutdown()
*/
package client
