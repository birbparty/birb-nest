package api

import (
	"context"
	"log"
	"time"

	"github.com/DataDog/dd-trace-go/v2/ddtrace/tracer"
	"github.com/birbparty/birb-nest/internal/database"
)

// WriteRequest represents an async write to PostgreSQL
type WriteRequest struct {
	Ctx        context.Context // Traced context for span propagation
	Key        string
	Value      []byte
	Timestamp  time.Time // Used for last-write-wins
	Retries    int
	InstanceID string // Instance ID for key namespacing
}

// AsyncWriterStats provides statistics about the async writer
type AsyncWriterStats struct {
	QueueDepth    int `json:"queue_depth"`
	QueueCapacity int `json:"queue_capacity"`
	WorkerCount   int `json:"worker_count"`
}

// AsyncWriter handles background writes to PostgreSQL
type AsyncWriter struct {
	db       database.Interface
	queue    chan WriteRequest
	workers  int
	maxRetry int
}

// NewAsyncWriter creates a new async writer with worker pool
func NewAsyncWriter(db database.Interface, queueSize, workers int) *AsyncWriter {
	aw := &AsyncWriter{
		db:       db,
		queue:    make(chan WriteRequest, queueSize),
		workers:  workers,
		maxRetry: 3,
	}

	// Start worker goroutines
	for i := 0; i < workers; i++ {
		go aw.worker(i)
	}

	return aw
}

// Write queues a write request with traced context
func (aw *AsyncWriter) Write(ctx context.Context, key string, value []byte, instanceID string) {
	select {
	case aw.queue <- WriteRequest{
		Ctx:        ctx,
		Key:        key,
		Value:      value,
		Timestamp:  time.Now(),
		InstanceID: instanceID,
	}:
		// Queued successfully
		if asyncQueueDepth != nil {
			asyncQueueDepth.WithLabelValues(instanceID).Set(float64(len(aw.queue)))
		}
	default:
		// Queue full, log and continue (Redis still has it)
		log.Printf("Write queue full, dropping write for key: %s from instance: %s", key, instanceID)
		if asyncWriteErrors != nil {
			asyncWriteErrors.WithLabelValues(instanceID, "queue_full").Inc()
		}
	}
}

// worker processes write requests from the queue
func (aw *AsyncWriter) worker(id int) {
	for req := range aw.queue {
		// Create a child span for async PostgreSQL write
		span, ctx := tracer.StartSpanFromContext(req.Ctx, "postgresql.async_write",
			tracer.ServiceName("birb-nest-async-writer"),
			tracer.ResourceName("SetWithInstance"),
			tracer.SpanType("db"),
			tracer.Tag("db.instance", req.InstanceID),
			tracer.Tag("db.key", req.Key),
		)

		// Add timeout but preserve trace context
		ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
		err := aw.db.SetWithInstance(ctx, req.Key, req.InstanceID, req.Value)
		cancel()

		if err != nil {
			span.SetTag("error", err)
		}
		span.Finish()

		if err != nil {
			if req.Retries < aw.maxRetry {
				req.Retries++
				// Re-queue with exponential backoff
				time.Sleep(time.Duration(req.Retries) * time.Second)
				select {
				case aw.queue <- req:
					log.Printf("Worker %d: Requeued write for key: %s, retry: %d", id, req.Key, req.Retries)
				default:
					log.Printf("Worker %d: Failed to requeue write for key: %s", id, req.Key)
					if asyncWriteErrors != nil {
						asyncWriteErrors.WithLabelValues(req.InstanceID, "requeue_failed").Inc()
					}
				}
			} else {
				log.Printf("Worker %d: Max retries exceeded for key: %s, error: %v", id, req.Key, err)
				if asyncWriteErrors != nil {
					asyncWriteErrors.WithLabelValues(req.InstanceID, "max_retries_exceeded").Inc()
				}
			}
		}

		// Update queue depth metric
		if asyncQueueDepth != nil {
			asyncQueueDepth.WithLabelValues(req.InstanceID).Set(float64(len(aw.queue)))
		}
	}
}

// QueueDepth returns the current queue depth
func (aw *AsyncWriter) QueueDepth() int {
	return len(aw.queue)
}

// Stats returns current statistics
func (aw *AsyncWriter) Stats() AsyncWriterStats {
	return AsyncWriterStats{
		QueueDepth:    len(aw.queue),
		QueueCapacity: cap(aw.queue),
		WorkerCount:   aw.workers,
	}
}

// Shutdown gracefully stops the async writer
func (aw *AsyncWriter) Shutdown() {
	close(aw.queue)
}
