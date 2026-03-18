# NATS Client Builder Usage

This document provides a guide on how to use the NATS client builder to establish a NATS connection, configure JetStream, create streams, and set up a Key-Value (KV) store using the builder pattern.

## Installation

Ensure that you have the following dependencies in your project:
- NATS Go client (`github.com/nats-io/nats.go`)
- NATS JetStream (`github.com/nats-io/nats.go/jetstream`)
- A logger package (`github.com/homingos/flam-go-common/logger`) for logging

To install the necessary packages, you can use:

```bash
go get github.com/nats-io/nats.go
go get github.com/nats-io/nats.go/jetstream
go get github.com/homingos/flam-go-common/logger
```

###  NATS Client Builder
The NATS client is configured using a builder pattern. The builder pattern allows for easy configuration and initialization of various components of the NATS connection, such as JetStream, streams, and the KV store. Below is an example usage of the ClientBuilder.

Example Usage
```go
package main

import (
"context"
"fmt"
"log"
"your-package/nats"
"your-package/consts"
)

func main() {
        // Create a new NATS client using the builder pattern
        client, err := nats.NewClientBuilder().
        // Connect to NATS
        Connect(context.Background()).
        // Initialize JetStream
        WithJetStream(context.Background()).
        // Create or retrieve a stream
        WithStream(context.Background(), consts.WorkflowStreamName, consts.WorkflowStreamDesc, consts.WorkflowCompleteSubject).
        // Set up a Key-Value store
        WithKVStore(context.Background(), consts.WorkflowStore).
        // Build the client
        Build(context.Background())

        // Check if there was an error during the build process
        if err != nil {
            log.Fatalf("Error creating NATS client: %v", err)
        }
}
```

Steps in the Example
Create a New Builder:

nats.NewClientBuilder() initializes a new ClientBuilder.
Connect to NATS:

.Connect(context.Background()) connects to the NATS server defined by the environment variable NATS_HOST. If the connection fails, it returns an error.
Initialize JetStream:

.WithJetStream(context.Background()) initializes the JetStream context, which provides access to stream management.
Create or Retrieve a Stream:

.WithStream(context.Background(), streamName, streamDesc, subject) creates or retrieves a stream with the specified name, description, and subject.
If the stream does not exist, it will be created with the provided configurations.
Set Up Key-Value Store:

.WithKVStore(context.Background(), kvStore) creates or retrieves a Key-Value store with the specified store name. It will create the store if it doesn't exist.
Build the Client:

.Build(context.Background()) finalizes the client creation. If any error occurred during the previous steps, it will return that error.

Example Error Handling
```go
    client, err := nats.NewClientBuilder().
    Connect(context.Background()).
    WithJetStream(context.Background()).
    WithStream(context.Background(), "testStream", "Test Stream", "test.subject").
    WithKVStore(context.Background(), "testStore").
    Build(context.Background())
    
    if err != nil {
    log.Fatalf("Failed to create NATS client: %v", err)
    }
```
In this example, if any step fails (e.g., connection to NATS, stream creation), it will return an error and log a fatal message.

