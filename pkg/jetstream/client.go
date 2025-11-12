package jetstream

import (
	"context"
	"fmt"
	"time"

	"github.com/nats-io/nats.go"
	natsjs "github.com/nats-io/nats.go/jetstream"
)

const (
	CommandStreamName = "COMMANDS"
	StateKVBucketName = "EXECUTION_STATE"
)

// Client wraps NATS JetStream operations
type Client struct {
	nc *nats.Conn
	js natsjs.JetStream
}

// NewClient creates a new JetStream client
func NewClient(urls string) (*Client, error) {
	nc, err := nats.Connect(urls,
		nats.MaxReconnects(-1),
		nats.ReconnectWait(2*time.Second),
		nats.PingInterval(10*time.Second),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to NATS: %w", err)
	}

	js, err := natsjs.New(nc)
	if err != nil {
		nc.Close()
		return nil, fmt.Errorf("failed to create JetStream context: %w", err)
	}

	return &Client{
		nc: nc,
		js: js,
	}, nil
}

// Close closes the NATS connection
func (c *Client) Close() {
	if c.nc != nil {
		c.nc.Close()
	}
}

// SetupStreams creates the necessary JetStream streams
func (c *Client) SetupStreams(ctx context.Context) error {
	// Create command stream with partitioning support
	stream, err := c.js.CreateStream(ctx, natsjs.StreamConfig{
		Name:        CommandStreamName,
		Subjects:    []string{"commands.>"},
		Retention:   natsjs.WorkQueuePolicy,
		MaxAge:      24 * time.Hour,
		Storage:     natsjs.FileStorage,
		Replicas:    3,
		Duplicates:  5 * time.Minute,
		MaxMsgs:     -1,
		MaxBytes:    -1,
		NoAck:       false,
		Discard:     natsjs.DiscardOld,
	})
	if err != nil {
		// If stream already exists, try to get it
		stream, err = c.js.Stream(ctx, CommandStreamName)
		if err != nil {
			return fmt.Errorf("failed to create or get command stream: %w", err)
		}
	}
	_ = stream // Stream created or retrieved successfully

	return nil
}

// SetupKVBuckets creates the necessary KV buckets for state persistence
func (c *Client) SetupKVBuckets(ctx context.Context) error {
	kv, err := c.js.CreateKeyValue(ctx, natsjs.KeyValueConfig{
		Bucket:   StateKVBucketName,
		History:  10,
		TTL:      0, // No TTL - persist indefinitely
		Storage:  natsjs.FileStorage,
		Replicas: 3,
	})
	if err != nil {
		// If bucket already exists, try to get it
		kv, err = c.js.KeyValue(ctx, StateKVBucketName)
		if err != nil {
			return fmt.Errorf("failed to create or get state KV bucket: %w", err)
		}
	}
	_ = kv // KV bucket created or retrieved successfully

	return nil
}

// GetStateKV returns the KV bucket for execution state
func (c *Client) GetStateKV(ctx context.Context) (natsjs.KeyValue, error) {
	kv, err := c.js.KeyValue(ctx, StateKVBucketName)
	if err != nil {
		return nil, fmt.Errorf("failed to get state KV bucket: %w", err)
	}
	return kv, nil
}

// PublishCommand publishes a command to the stream
func (c *Client) PublishCommand(ctx context.Context, partitionKey string, data []byte) error {
	subject := fmt.Sprintf("commands.%s", partitionKey)

	_, err := c.js.Publish(ctx, subject, data,
		natsjs.WithExpectStream(CommandStreamName),
	)
	if err != nil {
		return fmt.Errorf("failed to publish command: %w", err)
	}

	return nil
}

// PublishDelayedCommand publishes a command with a delay
// The message will be delivered after the specified delay duration
func (c *Client) PublishDelayedCommand(ctx context.Context, partitionKey string, data []byte, delay time.Duration) error {
	subject := fmt.Sprintf("commands.%s", partitionKey)

	// Calculate scheduled delivery time
	scheduledTime := time.Now().Add(delay)

	// Create message with headers
	headers := nats.Header{}
	headers.Set("Nats-Scheduled-Time", scheduledTime.Format(time.RFC3339Nano))

	// Publish with scheduled time in headers
	_, err := c.js.PublishMsg(ctx, &nats.Msg{
		Subject: subject,
		Data:    data,
		Header:  headers,
	}, natsjs.WithExpectStream(CommandStreamName))
	if err != nil {
		return fmt.Errorf("failed to publish delayed command: %w", err)
	}

	return nil
}

// SubscribeToCommands creates a durable consumer for processing commands
// maxAckPending controls how many workflows can execute concurrently
// ackWait is the timeout for workflow completion before redelivery
func (c *Client) SubscribeToCommands(ctx context.Context, consumerName string, maxAckPending int, ackWait time.Duration) (natsjs.Consumer, error) {
	consumer, err := c.js.CreateOrUpdateConsumer(ctx, CommandStreamName, natsjs.ConsumerConfig{
		Name:          consumerName,
		Durable:       consumerName,
		AckPolicy:     natsjs.AckExplicitPolicy,
		AckWait:       ackWait,
		MaxDeliver:    3,
		MaxAckPending: maxAckPending,
		FilterSubject: "commands.>",
		ReplayPolicy:  natsjs.ReplayInstantPolicy,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create consumer: %w", err)
	}

	return consumer, nil
}

// NatsConn returns the underlying NATS connection
func (c *Client) NatsConn() *nats.Conn {
	return c.nc
}

// JetStream returns the JetStream context
func (c *Client) JetStream() natsjs.JetStream {
	return c.js
}
