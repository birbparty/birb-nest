package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/birbparty/birb-nest/internal/cache"
	"github.com/birbparty/birb-nest/internal/database"
	"github.com/birbparty/birb-nest/internal/queue"
	"github.com/nats-io/nats.go"
)

// Batch represents a batch of messages to process
type Batch struct {
	Messages  []*nats.Msg
	StartTime time.Time
	Size      int
}

// BatchProcessor handles batch processing of messages
type BatchProcessor struct {
	config    *Config
	db        *database.DB
	cache     cache.Cache
	cacheRepo *database.CacheRepository
	metrics   *Metrics
	mu        sync.Mutex
}

// NewBatchProcessor creates a new batch processor
func NewBatchProcessor(config *Config, db *database.DB, cache cache.Cache, metrics *Metrics) *BatchProcessor {
	return &BatchProcessor{
		config:    config,
		db:        db,
		cache:     cache,
		cacheRepo: database.NewCacheRepository(db),
		metrics:   metrics,
	}
}

// ProcessPersistenceBatch processes a batch of persistence messages
func (bp *BatchProcessor) ProcessPersistenceBatch(ctx context.Context, batch *Batch) error {
	if len(batch.Messages) == 0 {
		return nil
	}

	startTime := time.Now()
	successCount := 0
	errorCount := 0

	// Process messages concurrently with limited concurrency
	sem := make(chan struct{}, bp.config.ProcessingConcurrency)
	var wg sync.WaitGroup
	results := make(chan error, len(batch.Messages))

	for _, msg := range batch.Messages {
		wg.Add(1)
		go func(m *nats.Msg) {
			defer wg.Done()

			// Acquire semaphore
			sem <- struct{}{}
			defer func() { <-sem }()

			// Process individual message
			if err := bp.processPersistenceMessage(ctx, m); err != nil {
				results <- err
				errorCount++
			} else {
				results <- nil
				successCount++
			}
		}(msg)
	}

	// Wait for all messages to be processed
	go func() {
		wg.Wait()
		close(results)
	}()

	// Collect results
	var errors []error
	for err := range results {
		if err != nil {
			errors = append(errors, err)
		}
	}

	// Record metrics
	duration := time.Since(startTime)
	bp.metrics.RecordBatch("persistence", len(batch.Messages), successCount, errorCount, duration)

	if len(errors) > 0 {
		return fmt.Errorf("batch processing completed with %d errors out of %d messages", len(errors), len(batch.Messages))
	}

	return nil
}

// processPersistenceMessage processes a single persistence message
func (bp *BatchProcessor) processPersistenceMessage(ctx context.Context, msg *nats.Msg) error {
	// Parse the message
	persistMsg, err := queue.UnmarshalPersistenceMessage(msg.Data)
	if err != nil {
		bp.metrics.RecordError("unmarshal_error")
		msg.Nak() // NAK for retry
		return fmt.Errorf("failed to unmarshal persistence message: %w", err)
	}

	// Write to PostgreSQL
	err = bp.cacheRepo.Set(ctx, persistMsg.Key, persistMsg.Value, persistMsg.TTL, persistMsg.Metadata)
	if err != nil {
		bp.metrics.RecordError("persistence_error")
		msg.Nak() // NAK for retry
		return fmt.Errorf("failed to persist key %s: %w", persistMsg.Key, err)
	}

	// ACK the message
	if err := msg.Ack(); err != nil {
		return fmt.Errorf("failed to ACK message: %w", err)
	}

	bp.metrics.RecordPersistence()
	return nil
}

// ProcessRehydrationBatch processes a batch of rehydration messages
func (bp *BatchProcessor) ProcessRehydrationBatch(ctx context.Context, batch *Batch) error {
	if len(batch.Messages) == 0 {
		return nil
	}

	startTime := time.Now()
	successCount := 0
	errorCount := 0

	// Extract keys from messages
	keyMap := make(map[string]*nats.Msg)
	var keys []string

	for _, msg := range batch.Messages {
		rehydrationMsg, err := queue.UnmarshalRehydrationMessage(msg.Data)
		if err != nil {
			msg.Nak()
			errorCount++
			continue
		}
		keys = append(keys, rehydrationMsg.Key)
		keyMap[rehydrationMsg.Key] = msg
	}

	// Batch get from PostgreSQL
	entries, err := bp.cacheRepo.BatchGet(ctx, keys)
	if err != nil {
		// NAK all messages
		for _, msg := range batch.Messages {
			msg.Nak()
		}
		return fmt.Errorf("failed to batch get from database: %w", err)
	}

	// Create a map for quick lookup
	entryMap := make(map[string]*database.CacheEntry)
	for _, entry := range entries {
		entryMap[entry.Key] = entry
	}

	// Process each message
	for key, msg := range keyMap {
		entry, exists := entryMap[key]
		if !exists {
			// Key not found in database, ACK anyway to avoid retry
			msg.Ack()
			continue
		}

		// Rehydrate to Redis
		if err := bp.rehydrateEntry(ctx, entry); err != nil {
			msg.Nak()
			errorCount++
		} else {
			msg.Ack()
			successCount++
		}
	}

	// Record metrics
	duration := time.Since(startTime)
	bp.metrics.RecordBatch("rehydration", len(batch.Messages), successCount, errorCount, duration)

	return nil
}

// rehydrateEntry rehydrates a single entry to Redis
func (bp *BatchProcessor) rehydrateEntry(ctx context.Context, entry *database.CacheEntry) error {
	// Marshal entry for caching
	data, err := json.Marshal(entry)
	if err != nil {
		return fmt.Errorf("failed to marshal entry: %w", err)
	}

	// Calculate TTL
	ttl := time.Duration(0)
	if entry.TTL != nil && *entry.TTL > 0 {
		ttl = time.Duration(*entry.TTL) * time.Second
	}

	// Set in Redis
	if err := bp.cache.Set(ctx, entry.Key, data, ttl); err != nil {
		bp.metrics.RecordError("rehydration_error")
		return fmt.Errorf("failed to rehydrate key %s: %w", entry.Key, err)
	}

	bp.metrics.RecordRehydration()
	return nil
}

// RehydrateAllKeys performs bulk rehydration of all keys
func (bp *BatchProcessor) RehydrateAllKeys(ctx context.Context) error {
	fmt.Println("Starting bulk rehydration of all keys...")

	offset := 0
	totalRehydrated := 0
	totalErrors := 0

	for {
		// Get batch of keys from database
		keys, err := bp.cacheRepo.GetAllKeys(ctx, offset, bp.config.RehydrationBatchSize)
		if err != nil {
			return fmt.Errorf("failed to get keys at offset %d: %w", offset, err)
		}

		if len(keys) == 0 {
			break // No more keys
		}

		// Get entries for these keys
		entries, err := bp.cacheRepo.BatchGet(ctx, keys)
		if err != nil {
			fmt.Printf("Error getting entries for batch at offset %d: %v\n", offset, err)
			offset += bp.config.RehydrationBatchSize
			continue
		}

		// Rehydrate entries concurrently
		sem := make(chan struct{}, bp.config.ProcessingConcurrency)
		var wg sync.WaitGroup

		for _, entry := range entries {
			wg.Add(1)
			go func(e *database.CacheEntry) {
				defer wg.Done()

				sem <- struct{}{}
				defer func() { <-sem }()

				if err := bp.rehydrateEntry(ctx, e); err != nil {
					fmt.Printf("Failed to rehydrate key %s: %v\n", e.Key, err)
					totalErrors++
				} else {
					totalRehydrated++
				}
			}(entry)
		}

		wg.Wait()

		fmt.Printf("Rehydrated batch: offset=%d, count=%d, total=%d, errors=%d\n",
			offset, len(entries), totalRehydrated, totalErrors)

		offset += bp.config.RehydrationBatchSize

		// Small delay to avoid overwhelming the system
		time.Sleep(100 * time.Millisecond)
	}

	fmt.Printf("Bulk rehydration complete: total=%d, errors=%d\n", totalRehydrated, totalErrors)
	bp.metrics.RecordBulkRehydration(totalRehydrated, totalErrors)

	return nil
}
