package worker

import (
	"sync"
	"time"
)

// Metrics holds worker metrics
type Metrics struct {
	mu sync.RWMutex

	// Message processing metrics
	messagesProcessed int64
	messagesSucceeded int64
	messagesFailed    int64
	persistenceCount  int64
	rehydrationCount  int64

	// Batch metrics
	batchesProcessed    int64
	batchProcessingTime time.Duration

	// Error metrics
	errorCounts map[string]int64

	// Performance metrics
	avgBatchSize      float64
	avgProcessingTime float64

	// Bulk rehydration metrics
	bulkRehydrationCount  int64
	bulkRehydrationErrors int64

	// Worker status
	startTime       time.Time
	lastProcessedAt time.Time
	isHealthy       bool
}

// NewMetrics creates a new metrics instance
func NewMetrics() *Metrics {
	return &Metrics{
		errorCounts: make(map[string]int64),
		startTime:   time.Now(),
		isHealthy:   true,
	}
}

// RecordPersistence records a successful persistence
func (m *Metrics) RecordPersistence() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.persistenceCount++
	m.messagesProcessed++
	m.messagesSucceeded++
	m.lastProcessedAt = time.Now()
}

// RecordRehydration records a successful rehydration
func (m *Metrics) RecordRehydration() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.rehydrationCount++
	m.messagesProcessed++
	m.messagesSucceeded++
	m.lastProcessedAt = time.Now()
}

// RecordError records an error
func (m *Metrics) RecordError(errorType string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.errorCounts[errorType]++
	m.messagesFailed++
}

// RecordBatch records batch processing metrics
func (m *Metrics) RecordBatch(batchType string, size, success, errors int, duration time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.batchesProcessed++
	m.batchProcessingTime += duration

	// Update average batch size
	if m.avgBatchSize == 0 {
		m.avgBatchSize = float64(size)
	} else {
		m.avgBatchSize = (m.avgBatchSize*float64(m.batchesProcessed-1) + float64(size)) / float64(m.batchesProcessed)
	}

	// Update average processing time
	avgDuration := duration.Milliseconds() / int64(size)
	if m.avgProcessingTime == 0 {
		m.avgProcessingTime = float64(avgDuration)
	} else {
		m.avgProcessingTime = (m.avgProcessingTime*float64(m.messagesProcessed-int64(size)) + float64(avgDuration*int64(size))) / float64(m.messagesProcessed)
	}
}

// RecordBulkRehydration records bulk rehydration metrics
func (m *Metrics) RecordBulkRehydration(count, errors int) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.bulkRehydrationCount += int64(count)
	m.bulkRehydrationErrors += int64(errors)
}

// GetStats returns current metrics
func (m *Metrics) GetStats() map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()

	uptime := time.Since(m.startTime)
	timeSinceLastProcessed := time.Duration(0)
	if !m.lastProcessedAt.IsZero() {
		timeSinceLastProcessed = time.Since(m.lastProcessedAt)
	}

	return map[string]interface{}{
		"uptime_seconds":          uptime.Seconds(),
		"messages_processed":      m.messagesProcessed,
		"messages_succeeded":      m.messagesSucceeded,
		"messages_failed":         m.messagesFailed,
		"persistence_count":       m.persistenceCount,
		"rehydration_count":       m.rehydrationCount,
		"batches_processed":       m.batchesProcessed,
		"avg_batch_size":          m.avgBatchSize,
		"avg_processing_time_ms":  m.avgProcessingTime,
		"bulk_rehydration_count":  m.bulkRehydrationCount,
		"bulk_rehydration_errors": m.bulkRehydrationErrors,
		"error_counts":            m.errorCounts,
		"last_processed_ago_ms":   timeSinceLastProcessed.Milliseconds(),
		"is_healthy":              m.isHealthy,
	}
}

// SetHealthy sets the health status
func (m *Metrics) SetHealthy(healthy bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.isHealthy = healthy
}

// IsHealthy returns the health status
func (m *Metrics) IsHealthy() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.isHealthy
}
