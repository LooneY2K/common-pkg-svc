package nats

import (
	"context"
	"errors"
	"fmt"
	"log"

	logger "github.com/LooneY2K/common-pkg-svc/log/logger"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
)

type Nats struct {
	NatsConnection *nats.Conn
	JetStream      jetstream.JetStream
	Stream         jetstream.Stream
	KV             jetstream.KeyValue
}

type ClientBuilder struct {
	natsConnection *nats.Conn
	jetStream      jetstream.JetStream
	stream         jetstream.Stream
	streamName     string
	streamDesc     string
	subject        []string
	kvStore        string
	kv             jetstream.KeyValue
	err            error
}

func NewClientBuilder() *ClientBuilder {
	return &ClientBuilder{}
}

func (cb *ClientBuilder) Connect(ctx context.Context, serverURL string) *ClientBuilder {
	if cb.err != nil {
		return cb
	}

	nc, err := nats.Connect(serverURL)
	if err != nil {
		cb.err = fmt.Errorf("failed to connect to NATS: %w", err)
	}
	cb.natsConnection = nc
	return cb
}

func (cb *ClientBuilder) WithJetStream(ctx context.Context) *ClientBuilder {
	if cb.err != nil {
		return cb
	}

	if cb.natsConnection == nil {
		cb.err = fmt.Errorf("nats connection is not established")
		return cb
	}

	js, err := jetstream.New(cb.natsConnection)
	if err != nil {
		cb.err = fmt.Errorf("failed to create JetStream context: %w", err)
	}
	cb.jetStream = js
	return cb
}

func (cb *ClientBuilder) WithStream(ctx context.Context, streamName, streamDesc string, subject []string) *ClientBuilder {
	if cb.err != nil {
		return cb
	}

	if cb.jetStream == nil {
		cb.err = fmt.Errorf("JetStream context is not created")
		return cb
	}

	stream, err := cb.jetStream.Stream(ctx, streamName)
	if err != nil {
		if errors.Is(err, jetstream.ErrStreamNotFound) {
			streamConfig := jetstream.StreamConfig{
				Name:        streamName,
				Description: streamDesc,
				Subjects:    subject,
				Retention:   jetstream.WorkQueuePolicy,
			}
			stream, err = cb.jetStream.CreateStream(ctx, streamConfig)
			if err != nil {
				cb.err = fmt.Errorf("failed to create stream: %w", err)
				return cb
			}
			fmt.Println("Stream created successfully")
		} else {
			cb.err = fmt.Errorf("failed to get or create stream: %w", err)
			return cb
		}
	}

	cb.streamName = streamName
	cb.streamDesc = streamDesc
	cb.subject = subject
	cb.stream = stream
	return cb
}

func (cb *ClientBuilder) WithKVStore(ctx context.Context, kvStore string) *ClientBuilder {
	if cb.err != nil {
		return cb
	}

	if cb.jetStream == nil {
		cb.err = fmt.Errorf("JetStream context is not created")
		return cb
	}

	kv, err := newStore(ctx, cb.jetStream, kvStore)
	if err != nil {
		cb.err = fmt.Errorf("failed to fetch/create store: %w", err)
		return cb
	}

	cb.kvStore = kvStore
	cb.kv = kv
	return cb
}

func (cb *ClientBuilder) Build(ctx context.Context) (*Nats, error) {
	if cb.err != nil {
		return nil, cb.err // If an error exists, return it
	}

	if cb.natsConnection == nil {
		logger.GetLogger().Errorw("NATS connection not established")
		return nil, fmt.Errorf("NATS connection not established")
	}

	if cb.jetStream == nil {
		return nil, fmt.Errorf("JetStream context not created")
	}

	if cb.stream == nil {
		log.Println("Stream not created")
	}

	if cb.kv == nil {
		log.Println("KV Store not created")
	}

	return &Nats{
		NatsConnection: cb.natsConnection,
		JetStream:      cb.jetStream,
		Stream:         cb.stream,
		KV:             cb.kv,
	}, nil
}

func newStore(ctx context.Context, js jetstream.JetStream, bucket string) (jetstream.KeyValue, error) {
	kv, err := js.KeyValue(ctx, bucket)
	if err != nil {
		if err == jetstream.ErrBucketNotFound {
			kv, err = js.CreateKeyValue(ctx, jetstream.KeyValueConfig{
				Bucket:  bucket,
				History: 5,
			})
			if err != nil {
				return nil, err
			}
			return kv, nil
		} else {
			return nil, err
		}
	}
	return kv, nil
}

func (cli *Nats) PublishMsg(subject string, payload []byte) error {
	err := cli.NatsConnection.Publish(subject, payload)
	if err != nil {
		return fmt.Errorf("publish: %w", err)
	}
	return nil
}
