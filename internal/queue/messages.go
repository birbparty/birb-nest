package queue

import (
	"encoding/json"
	"time"
)

// MessageType represents the type of queue message
type MessageType string

const (
	// MessageTypePersistence indicates a message to persist data to PostgreSQL
	MessageTypePersistence MessageType = "persistence"
	// MessageTypeRehydration indicates a message to rehydrate data to Redis
	MessageTypeRehydration MessageType = "rehydration"
)

// Subject names for different message types
const (
	SubjectPersistence = "cache.persist"
	SubjectRehydration = "cache.rehydrate"
	SubjectDLQ         = "cache.dlq"
)

// BaseMessage contains common fields for all messages
type BaseMessage struct {
	ID        string      `json:"id"`
	Type      MessageType `json:"type"`
	Timestamp time.Time   `json:"timestamp"`
	Retries   int         `json:"retries,omitempty"`
}

// PersistenceMessage represents a message to persist cache data
type PersistenceMessage struct {
	BaseMessage
	Key      string          `json:"key"`
	Value    json.RawMessage `json:"value"`
	Version  int             `json:"version"`
	TTL      *int            `json:"ttl,omitempty"`
	Metadata json.RawMessage `json:"metadata,omitempty"`
}

// RehydrationMessage represents a message to rehydrate cache data
type RehydrationMessage struct {
	BaseMessage
	Key      string `json:"key"`
	Priority int    `json:"priority"` // Higher priority processed first
}

// DLQMessage represents a dead letter queue message
type DLQMessage struct {
	OriginalMessage json.RawMessage `json:"original_message"`
	OriginalSubject string          `json:"original_subject"`
	Error           string          `json:"error"`
	FailedAt        time.Time       `json:"failed_at"`
	Retries         int             `json:"retries"`
	MaxRetries      int             `json:"max_retries"`
}

// NewPersistenceMessage creates a new persistence message
func NewPersistenceMessage(key string, value json.RawMessage, version int, ttl *int, metadata json.RawMessage) *PersistenceMessage {
	return &PersistenceMessage{
		BaseMessage: BaseMessage{
			ID:        generateMessageID(),
			Type:      MessageTypePersistence,
			Timestamp: time.Now().UTC(),
		},
		Key:      key,
		Value:    value,
		Version:  version,
		TTL:      ttl,
		Metadata: metadata,
	}
}

// NewRehydrationMessage creates a new rehydration message
func NewRehydrationMessage(key string, priority int) *RehydrationMessage {
	return &RehydrationMessage{
		BaseMessage: BaseMessage{
			ID:        generateMessageID(),
			Type:      MessageTypeRehydration,
			Timestamp: time.Now().UTC(),
		},
		Key:      key,
		Priority: priority,
	}
}

// Marshal converts the message to JSON bytes
func (m *PersistenceMessage) Marshal() ([]byte, error) {
	return json.Marshal(m)
}

// Marshal converts the message to JSON bytes
func (m *RehydrationMessage) Marshal() ([]byte, error) {
	return json.Marshal(m)
}

// Marshal converts the message to JSON bytes
func (m *DLQMessage) Marshal() ([]byte, error) {
	return json.Marshal(m)
}

// UnmarshalPersistenceMessage unmarshals a persistence message from JSON
func UnmarshalPersistenceMessage(data []byte) (*PersistenceMessage, error) {
	var msg PersistenceMessage
	if err := json.Unmarshal(data, &msg); err != nil {
		return nil, err
	}
	return &msg, nil
}

// UnmarshalRehydrationMessage unmarshals a rehydration message from JSON
func UnmarshalRehydrationMessage(data []byte) (*RehydrationMessage, error) {
	var msg RehydrationMessage
	if err := json.Unmarshal(data, &msg); err != nil {
		return nil, err
	}
	return &msg, nil
}

// UnmarshalDLQMessage unmarshals a DLQ message from JSON
func UnmarshalDLQMessage(data []byte) (*DLQMessage, error) {
	var msg DLQMessage
	if err := json.Unmarshal(data, &msg); err != nil {
		return nil, err
	}
	return &msg, nil
}

// generateMessageID generates a unique message ID
func generateMessageID() string {
	// In production, you might want to use a UUID library
	return time.Now().Format("20060102150405") + "-" + generateRandomString(8)
}

// generateRandomString generates a random string of specified length
func generateRandomString(length int) string {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	b := make([]byte, length)
	for i := range b {
		b[i] = charset[time.Now().UnixNano()%int64(len(charset))]
	}
	return string(b)
}

// Priority levels for rehydration
const (
	PriorityLow    = 0
	PriorityNormal = 1
	PriorityHigh   = 2
	PriorityUrgent = 3
)
