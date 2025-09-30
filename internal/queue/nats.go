package queue

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/nats-io/nats.go"
)

// Client represents a NATS JetStream client
type Client struct {
	nc     *nats.Conn
	js     nats.JetStreamContext
	config *Config
}

// NewClient creates a new NATS JetStream client
func NewClient(config *Config) (*Client, error) {
	// Set up connection options
	opts := []nats.Option{
		nats.Name(config.Name),
		nats.MaxReconnects(-1),
		nats.ReconnectWait(2 * time.Second),
		nats.DisconnectErrHandler(func(nc *nats.Conn, err error) {
			if err != nil {
				fmt.Printf("NATS disconnected: %v\n", err)
			}
		}),
		nats.ReconnectHandler(func(nc *nats.Conn) {
			fmt.Printf("NATS reconnected to %s\n", nc.ConnectedUrl())
		}),
		nats.ErrorHandler(func(nc *nats.Conn, sub *nats.Subscription, err error) {
			fmt.Printf("NATS error: %v\n", err)
		}),
	}

	// Add authentication if provided
	if config.User != "" && config.Password != "" {
		opts = append(opts, nats.UserInfo(config.User, config.Password))
	}

	// Connect to NATS
	nc, err := nats.Connect(config.URL, opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to NATS: %w", err)
	}

	// Create JetStream context
	js, err := nc.JetStream()
	if err != nil {
		nc.Close()
		return nil, fmt.Errorf("failed to create JetStream context: %w", err)
	}

	client := &Client{
		nc:     nc,
		js:     js,
		config: config,
	}

	// Initialize streams
	if err := client.initializeStreams(); err != nil {
		nc.Close()
		return nil, fmt.Errorf("failed to initialize streams: %w", err)
	}

	return client, nil
}

// initializeStreams creates the necessary JetStream streams
func (c *Client) initializeStreams() error {
	// Main cache stream
	mainStreamConfig := &nats.StreamConfig{
		Name:        c.config.StreamName,
		Description: "Birb Nest cache persistence stream",
		Subjects:    []string{SubjectPersistence, SubjectRehydration},
		Retention:   nats.LimitsPolicy,
		MaxAge:      c.config.StreamMaxAge,
		MaxBytes:    c.config.StreamMaxBytes,
		MaxMsgs:     c.config.StreamMaxMsgs,
		MaxMsgSize:  c.config.StreamMaxMsgSize,
		Replicas:    c.config.StreamReplicas,
		Duplicates:  5 * time.Minute,
		NoAck:       false,
		Storage:     nats.FileStorage,
	}

	// Create or update main stream
	_, err := c.js.AddStream(mainStreamConfig)
	if err != nil {
		// Try to update if stream exists
		_, err = c.js.UpdateStream(mainStreamConfig)
		if err != nil {
			return fmt.Errorf("failed to create/update main stream: %w", err)
		}
	}

	// DLQ stream
	dlqStreamConfig := &nats.StreamConfig{
		Name:        c.config.DLQStreamName,
		Description: "Birb Nest cache DLQ stream",
		Subjects:    []string{SubjectDLQ},
		Retention:   nats.LimitsPolicy,
		MaxAge:      7 * 24 * time.Hour,           // Keep DLQ messages for 7 days
		MaxBytes:    c.config.StreamMaxBytes / 10, // 10% of main stream size
		MaxMsgs:     c.config.StreamMaxMsgs / 10,
		MaxMsgSize:  c.config.StreamMaxMsgSize,
		Replicas:    c.config.StreamReplicas,
		NoAck:       false,
		Storage:     nats.FileStorage,
	}

	// Create or update DLQ stream
	_, err = c.js.AddStream(dlqStreamConfig)
	if err != nil {
		// Try to update if stream exists
		_, err = c.js.UpdateStream(dlqStreamConfig)
		if err != nil {
			return fmt.Errorf("failed to create/update DLQ stream: %w", err)
		}
	}

	return nil
}

// PublishPersistence publishes a persistence message
func (c *Client) PublishPersistence(ctx context.Context, msg *PersistenceMessage) error {
	data, err := msg.Marshal()
	if err != nil {
		return fmt.Errorf("failed to marshal persistence message: %w", err)
	}

	// Add message ID for deduplication
	msgOpts := []nats.PubOpt{
		nats.MsgId(msg.ID),
	}

	// Publish with context
	pubAck, err := c.js.PublishAsync(SubjectPersistence, data, msgOpts...)
	if err != nil {
		return fmt.Errorf("failed to publish persistence message: %w", err)
	}

	// Wait for acknowledgment
	select {
	case <-pubAck.Ok():
		return nil
	case err := <-pubAck.Err():
		return fmt.Errorf("persistence message publish failed: %w", err)
	case <-ctx.Done():
		return ctx.Err()
	}
}

// PublishRehydration publishes a rehydration message
func (c *Client) PublishRehydration(ctx context.Context, msg *RehydrationMessage) error {
	data, err := msg.Marshal()
	if err != nil {
		return fmt.Errorf("failed to marshal rehydration message: %w", err)
	}

	// Add message ID for deduplication
	msgOpts := []nats.PubOpt{
		nats.MsgId(msg.ID),
	}

	// Publish with context
	pubAck, err := c.js.PublishAsync(SubjectRehydration, data, msgOpts...)
	if err != nil {
		return fmt.Errorf("failed to publish rehydration message: %w", err)
	}

	// Wait for acknowledgment
	select {
	case <-pubAck.Ok():
		return nil
	case err := <-pubAck.Err():
		return fmt.Errorf("rehydration message publish failed: %w", err)
	case <-ctx.Done():
		return ctx.Err()
	}
}

// CreateConsumer creates a durable consumer for processing messages
func (c *Client) CreateConsumer(streamName, consumerName string) (*nats.ConsumerInfo, error) {
	consumerConfig := &nats.ConsumerConfig{
		Durable:         consumerName,
		AckPolicy:       nats.AckExplicitPolicy,
		AckWait:         c.config.ConsumerAckWait,
		MaxDeliver:      c.config.ConsumerMaxDeliver,
		MaxAckPending:   c.config.ConsumerMaxAckPending,
		ReplayPolicy:    nats.ReplayInstantPolicy,
		DeliverPolicy:   nats.DeliverAllPolicy,
		FilterSubject:   "", // Subscribe to all subjects in the stream
		SampleFrequency: "100%",
	}

	info, err := c.js.AddConsumer(streamName, consumerConfig)
	if err != nil {
		// Try to update if consumer exists
		info, err = c.js.UpdateConsumer(streamName, consumerConfig)
		if err != nil {
			return nil, fmt.Errorf("failed to create/update consumer: %w", err)
		}
	}

	return info, nil
}

// Subscribe creates a subscription to consume messages
func (c *Client) Subscribe(streamName, consumerName string, handler nats.MsgHandler) (*nats.Subscription, error) {
	// Create pull subscription
	sub, err := c.js.PullSubscribe(
		"", // Empty subject means subscribe to the consumer
		consumerName,
		nats.ManualAck(),
		nats.Bind(streamName, consumerName),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create subscription: %w", err)
	}

	// Start a goroutine to fetch messages
	go func() {
		for {
			msgs, err := sub.Fetch(c.config.BatchSize, nats.MaxWait(c.config.BatchTimeout))
			if err != nil {
				if !errors.Is(err, nats.ErrTimeout) && !errors.Is(err, nats.ErrConnectionClosed) {
					fmt.Printf("Error fetching messages: %v\n", err)
				}
				continue
			}

			// Process messages
			for _, msg := range msgs {
				handler(msg)
			}
		}
	}()

	return sub, nil
}

// Health checks the NATS connection health
func (c *Client) Health() error {
	if !c.nc.IsConnected() {
		return fmt.Errorf("NATS is not connected")
	}

	// Check JetStream API access
	_, err := c.js.AccountInfo()
	if err != nil {
		return fmt.Errorf("JetStream health check failed: %w", err)
	}

	return nil
}

// Close closes the NATS connection
func (c *Client) Close() error {
	if c.nc != nil {
		c.nc.Close()
	}
	return nil
}

// StreamInfo returns information about a stream
func (c *Client) StreamInfo(streamName string) (*nats.StreamInfo, error) {
	return c.js.StreamInfo(streamName)
}

// ConsumerInfo returns information about a consumer
func (c *Client) ConsumerInfo(streamName, consumerName string) (*nats.ConsumerInfo, error) {
	return c.js.ConsumerInfo(streamName, consumerName)
}

// DeleteMessage deletes a message from a stream
func (c *Client) DeleteMessage(streamName string, seq uint64) error {
	return c.js.DeleteMsg(streamName, seq)
}

// GetMessage retrieves a message from a stream by sequence number
func (c *Client) GetMessage(streamName string, seq uint64) (*nats.RawStreamMsg, error) {
	return c.js.GetMsg(streamName, seq)
}

// GetConfig returns the client configuration
func (c *Client) GetConfig() *Config {
	return c.config
}

// GetNC returns the underlying NATS connection
func (c *Client) GetNC() *nats.Conn {
	return c.nc
}
