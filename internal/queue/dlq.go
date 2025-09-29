package queue

import (
	"context"
	"fmt"
	"time"

	"github.com/nats-io/nats.go"
)

// DLQHandler handles dead letter queue operations
type DLQHandler struct {
	client *Client
	config *Config
}

// NewDLQHandler creates a new DLQ handler
func NewDLQHandler(client *Client) *DLQHandler {
	return &DLQHandler{
		client: client,
		config: client.config,
	}
}

// SendToDLQ sends a failed message to the dead letter queue
func (h *DLQHandler) SendToDLQ(ctx context.Context, originalMsg *nats.Msg, err error) error {
	// Extract retry count from headers
	retries := 0
	if retriesStr := originalMsg.Header.Get("X-Retries"); retriesStr != "" {
		fmt.Sscanf(retriesStr, "%d", &retries)
	}

	// Create DLQ message
	dlqMsg := &DLQMessage{
		OriginalMessage: originalMsg.Data,
		OriginalSubject: originalMsg.Subject,
		Error:           err.Error(),
		FailedAt:        time.Now().UTC(),
		Retries:         retries,
		MaxRetries:      h.config.DLQMaxRetries,
	}

	// Marshal DLQ message
	data, err := dlqMsg.Marshal()
	if err != nil {
		return fmt.Errorf("failed to marshal DLQ message: %w", err)
	}

	// Publish to DLQ with headers
	headers := nats.Header{}
	headers.Set("X-Original-Subject", originalMsg.Subject)
	headers.Set("X-Failed-At", dlqMsg.FailedAt.Format(time.RFC3339))
	headers.Set("X-Retries", fmt.Sprintf("%d", retries))

	msg := &nats.Msg{
		Subject: SubjectDLQ,
		Data:    data,
		Header:  headers,
	}

	// Publish to DLQ stream
	pubAck, err := h.client.js.PublishMsgAsync(msg)
	if err != nil {
		return fmt.Errorf("failed to publish to DLQ: %w", err)
	}

	// Wait for acknowledgment
	select {
	case <-pubAck.Ok():
		return nil
	case err := <-pubAck.Err():
		return fmt.Errorf("DLQ publish failed: %w", err)
	case <-ctx.Done():
		return ctx.Err()
	}
}

// ProcessDLQ processes messages from the dead letter queue
func (h *DLQHandler) ProcessDLQ(ctx context.Context) error {
	// Create DLQ consumer
	consumerName := fmt.Sprintf("%s-dlq", h.config.ConsumerName)
	_, err := h.client.CreateConsumer(h.config.DLQStreamName, consumerName)
	if err != nil {
		return fmt.Errorf("failed to create DLQ consumer: %w", err)
	}

	// Subscribe to DLQ messages
	sub, err := h.client.js.PullSubscribe(
		SubjectDLQ,
		consumerName,
		nats.ManualAck(),
		nats.Bind(h.config.DLQStreamName, consumerName),
	)
	if err != nil {
		return fmt.Errorf("failed to subscribe to DLQ: %w", err)
	}

	// Process messages
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			msgs, err := sub.Fetch(10, nats.MaxWait(5*time.Second))
			if err != nil {
				if err == nats.ErrTimeout {
					continue
				}
				return fmt.Errorf("failed to fetch DLQ messages: %w", err)
			}

			for _, msg := range msgs {
				if err := h.processDLQMessage(ctx, msg); err != nil {
					fmt.Printf("Failed to process DLQ message: %v\n", err)
					// NAK the message to retry later
					msg.Nak()
				} else {
					// ACK the message
					msg.Ack()
				}
			}
		}
	}
}

// processDLQMessage handles a single DLQ message
func (h *DLQHandler) processDLQMessage(ctx context.Context, msg *nats.Msg) error {
	// Unmarshal DLQ message
	dlqMsg, err := UnmarshalDLQMessage(msg.Data)
	if err != nil {
		return fmt.Errorf("failed to unmarshal DLQ message: %w", err)
	}

	// Check if max retries exceeded
	if dlqMsg.Retries >= dlqMsg.MaxRetries {
		// Log and move to permanent failure storage
		fmt.Printf("Message exceeded max retries (%d): %s\n", dlqMsg.MaxRetries, dlqMsg.OriginalSubject)
		// In production, you might want to store this in a database or send alerts
		return nil
	}

	// Check if enough time has passed for retry
	nextRetryTime := dlqMsg.FailedAt.Add(h.config.DLQRetryInterval * time.Duration(dlqMsg.Retries+1))
	if time.Now().Before(nextRetryTime) {
		// Not ready for retry yet, NAK without delay
		return fmt.Errorf("not ready for retry, next retry at %v", nextRetryTime)
	}

	// Attempt to reprocess the original message
	return h.retryOriginalMessage(ctx, dlqMsg)
}

// retryOriginalMessage attempts to reprocess the original message
func (h *DLQHandler) retryOriginalMessage(ctx context.Context, dlqMsg *DLQMessage) error {
	// Determine the original message type and reprocess
	var messageType MessageType

	// Try to detect message type from subject
	switch dlqMsg.OriginalSubject {
	case SubjectPersistence:
		messageType = MessageTypePersistence
	case SubjectRehydration:
		messageType = MessageTypeRehydration
	default:
		return fmt.Errorf("unknown original subject: %s", dlqMsg.OriginalSubject)
	}

	// Create headers with incremented retry count
	headers := nats.Header{}
	headers.Set("X-Retries", fmt.Sprintf("%d", dlqMsg.Retries+1))
	headers.Set("X-DLQ-Retry", "true")

	// Republish the original message
	msg := &nats.Msg{
		Subject: dlqMsg.OriginalSubject,
		Data:    dlqMsg.OriginalMessage,
		Header:  headers,
	}

	pubAck, err := h.client.js.PublishMsgAsync(msg)
	if err != nil {
		return fmt.Errorf("failed to retry message: %w", err)
	}

	// Wait for acknowledgment
	select {
	case <-pubAck.Ok():
		fmt.Printf("Successfully retried %s message (attempt %d)\n", messageType, dlqMsg.Retries+1)
		return nil
	case err := <-pubAck.Err():
		return fmt.Errorf("retry publish failed: %w", err)
	case <-ctx.Done():
		return ctx.Err()
	}
}

// GetDLQStats returns statistics about the DLQ
func (h *DLQHandler) GetDLQStats() (*DLQStats, error) {
	streamInfo, err := h.client.StreamInfo(h.config.DLQStreamName)
	if err != nil {
		return nil, fmt.Errorf("failed to get DLQ stream info: %w", err)
	}

	consumerName := fmt.Sprintf("%s-dlq", h.config.ConsumerName)
	consumerInfo, err := h.client.ConsumerInfo(h.config.DLQStreamName, consumerName)
	if err != nil {
		return nil, fmt.Errorf("failed to get DLQ consumer info: %w", err)
	}

	return &DLQStats{
		TotalMessages:   streamInfo.State.Msgs,
		PendingMessages: consumerInfo.NumPending,
		StreamBytes:     streamInfo.State.Bytes,
		OldestMessage:   streamInfo.State.FirstTime,
		NewestMessage:   streamInfo.State.LastTime,
	}, nil
}

// PurgeDLQ removes all messages from the DLQ (use with caution!)
func (h *DLQHandler) PurgeDLQ(ctx context.Context) error {
	err := h.client.js.PurgeStream(h.config.DLQStreamName)
	if err != nil {
		return fmt.Errorf("failed to purge DLQ: %w", err)
	}
	return nil
}

// DLQStats represents statistics about the DLQ
type DLQStats struct {
	TotalMessages   uint64    `json:"total_messages"`
	PendingMessages uint64    `json:"pending_messages"`
	StreamBytes     uint64    `json:"stream_bytes"`
	OldestMessage   time.Time `json:"oldest_message"`
	NewestMessage   time.Time `json:"newest_message"`
}

// IsRetryable determines if an error is retryable
func IsRetryable(err error) bool {
	// Add logic to determine if an error should be retried
	// For now, we'll consider most errors retryable except for specific cases

	// Non-retryable errors might include:
	// - Invalid message format
	// - Authentication errors
	// - Business logic violations

	return true // Default to retryable
}

// ExtractMessageKey attempts to extract the key from a message
func ExtractMessageKey(data []byte, subject string) (string, error) {
	switch subject {
	case SubjectPersistence:
		msg, err := UnmarshalPersistenceMessage(data)
		if err != nil {
			return "", err
		}
		return msg.Key, nil
	case SubjectRehydration:
		msg, err := UnmarshalRehydrationMessage(data)
		if err != nil {
			return "", err
		}
		return msg.Key, nil
	default:
		return "", fmt.Errorf("unknown subject: %s", subject)
	}
}
