package worker

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/DataDog/dd-trace-go/v2/ddtrace/tracer"
	"github.com/birbparty/birb-nest/internal/cache"
	"github.com/birbparty/birb-nest/internal/database"
	"github.com/birbparty/birb-nest/internal/queue"
	"github.com/nats-io/nats.go"
)

// Processor handles message processing for the worker
type Processor struct {
	config         *Config
	db             *database.DB
	cache          cache.Cache
	queueClient    *queue.Client
	batchProcessor *BatchProcessor
	dlqHandler     *queue.DLQHandler
	metrics        *Metrics

	// Message batching
	persistenceBatch *Batch
	rehydrationBatch *Batch
	batchMu          sync.Mutex

	// Control channels
	stopCh    chan struct{}
	stoppedCh chan struct{}
}

// NewProcessor creates a new message processor
func NewProcessor(config *Config, db *database.DB, cache cache.Cache, queueClient *queue.Client, metrics *Metrics) *Processor {
	return &Processor{
		config:         config,
		db:             db,
		cache:          cache,
		queueClient:    queueClient,
		batchProcessor: NewBatchProcessor(config, db, cache, metrics),
		dlqHandler:     queue.NewDLQHandler(queueClient),
		metrics:        metrics,
		stopCh:         make(chan struct{}),
		stoppedCh:      make(chan struct{}),
	}
}

// Start begins processing messages
func (p *Processor) Start(ctx context.Context) error {
	fmt.Printf("🚀 Worker %s starting message processing...\n", p.config.WorkerID)

	// Create consumers
	if err := p.createConsumers(); err != nil {
		return fmt.Errorf("failed to create consumers: %w", err)
	}

	// Start batch processing timers
	batchTicker := time.NewTicker(p.config.BatchTimeout)
	defer batchTicker.Stop()

	// Start metrics reporting
	metricsTicker := time.NewTicker(p.config.MetricsInterval)
	defer metricsTicker.Stop()

	// Subscribe to messages
	persistenceSub, err := p.subscribeToPersistence()
	if err != nil {
		return fmt.Errorf("failed to subscribe to persistence: %w", err)
	}
	defer persistenceSub.Unsubscribe()

	rehydrationSub, err := p.subscribeToRehydration()
	if err != nil {
		return fmt.Errorf("failed to subscribe to rehydration: %w", err)
	}
	defer rehydrationSub.Unsubscribe()

	// Start DLQ processor in background
	go func() {
		if err := p.dlqHandler.ProcessDLQ(ctx); err != nil {
			fmt.Printf("DLQ processor error: %v\n", err)
		}
	}()

	// Main processing loop
	for {
		select {
		case <-ctx.Done():
			return p.shutdown()

		case <-p.stopCh:
			return p.shutdown()

		case <-batchTicker.C:
			// Process any pending batches
			p.processPendingBatches(ctx)

		case <-metricsTicker.C:
			// Report metrics
			p.reportMetrics()
		}
	}
}

// createConsumers creates NATS consumers
func (p *Processor) createConsumers() error {
	queueConfig := p.queueClient.GetConfig()

	// Create persistence consumer
	_, err := p.queueClient.CreateConsumer(
		queueConfig.StreamName,
		fmt.Sprintf("%s-persistence", queueConfig.ConsumerName),
	)
	if err != nil {
		return fmt.Errorf("failed to create persistence consumer: %w", err)
	}

	// Create rehydration consumer
	_, err = p.queueClient.CreateConsumer(
		queueConfig.StreamName,
		fmt.Sprintf("%s-rehydration", queueConfig.ConsumerName),
	)
	if err != nil {
		return fmt.Errorf("failed to create rehydration consumer: %w", err)
	}

	return nil
}

// subscribeToPersistence subscribes to persistence messages
func (p *Processor) subscribeToPersistence() (*nats.Subscription, error) {
	queueConfig := p.queueClient.GetConfig()
	return p.queueClient.Subscribe(
		queueConfig.StreamName,
		fmt.Sprintf("%s-persistence", queueConfig.ConsumerName),
		func(msg *nats.Msg) {
			p.handlePersistenceMessage(msg)
		},
	)
}

// subscribeToRehydration subscribes to rehydration messages
func (p *Processor) subscribeToRehydration() (*nats.Subscription, error) {
	queueConfig := p.queueClient.GetConfig()
	return p.queueClient.Subscribe(
		queueConfig.StreamName,
		fmt.Sprintf("%s-rehydration", queueConfig.ConsumerName),
		func(msg *nats.Msg) {
			p.handleRehydrationMessage(msg)
		},
	)
}

// handlePersistenceMessage handles a persistence message
func (p *Processor) handlePersistenceMessage(msg *nats.Msg) {
	// Extract trace context from message headers
	carrier := tracer.HTTPHeadersCarrier(msg.Header)
	spanCtx, err := tracer.Extract(carrier)
	if err != nil && err != tracer.ErrSpanContextNotFound {
		fmt.Printf("Failed to extract trace context: %v\n", err)
	}

	// Start consumer span
	var span *tracer.Span
	if spanCtx == nil {
		span = tracer.StartSpan("nats.consume",
			tracer.ServiceName("birb-nest-worker"),
			tracer.ResourceName("consume "+queue.SubjectPersistence),
			tracer.SpanType("queue"),
			tracer.Tag("messaging.system", "nats"),
			tracer.Tag("messaging.destination", queue.SubjectPersistence),
			tracer.Tag("messaging.operation", "receive"),
		)
	} else {
		span = tracer.StartSpan("nats.consume",
			tracer.ChildOf(spanCtx),
			tracer.ServiceName("birb-nest-worker"),
			tracer.ResourceName("consume "+queue.SubjectPersistence),
			tracer.SpanType("queue"),
			tracer.Tag("messaging.system", "nats"),
			tracer.Tag("messaging.destination", queue.SubjectPersistence),
			tracer.Tag("messaging.operation", "receive"),
		)
	}
	defer span.Finish()

	p.batchMu.Lock()
	defer p.batchMu.Unlock()

	// Initialize batch if needed
	if p.persistenceBatch == nil {
		p.persistenceBatch = &Batch{
			Messages:  make([]*nats.Msg, 0, p.config.BatchSize),
			StartTime: time.Now(),
		}
	}

	// Add to batch
	p.persistenceBatch.Messages = append(p.persistenceBatch.Messages, msg)

	// Process if batch is full
	if len(p.persistenceBatch.Messages) >= p.config.BatchSize {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		if err := p.batchProcessor.ProcessPersistenceBatch(ctx, p.persistenceBatch); err != nil {
			fmt.Printf("Failed to process persistence batch: %v\n", err)
			span.SetTag("error", err)
			// Messages will be retried via NATS redelivery
		} else {
			span.SetTag("messaging.batch_size", len(p.persistenceBatch.Messages))
		}

		// Reset batch
		p.persistenceBatch = nil
	}
}

// handleRehydrationMessage handles a rehydration message
func (p *Processor) handleRehydrationMessage(msg *nats.Msg) {
	// Extract trace context from message headers
	carrier := tracer.HTTPHeadersCarrier(msg.Header)
	spanCtx, err := tracer.Extract(carrier)
	if err != nil && err != tracer.ErrSpanContextNotFound {
		fmt.Printf("Failed to extract trace context: %v\n", err)
	}

	// Start consumer span
	var span *tracer.Span
	if spanCtx == nil {
		span = tracer.StartSpan("nats.consume",
			tracer.ServiceName("birb-nest-worker"),
			tracer.ResourceName("consume "+queue.SubjectRehydration),
			tracer.SpanType("queue"),
			tracer.Tag("messaging.system", "nats"),
			tracer.Tag("messaging.destination", queue.SubjectRehydration),
			tracer.Tag("messaging.operation", "receive"),
		)
	} else {
		span = tracer.StartSpan("nats.consume",
			tracer.ChildOf(spanCtx),
			tracer.ServiceName("birb-nest-worker"),
			tracer.ResourceName("consume "+queue.SubjectRehydration),
			tracer.SpanType("queue"),
			tracer.Tag("messaging.system", "nats"),
			tracer.Tag("messaging.destination", queue.SubjectRehydration),
			tracer.Tag("messaging.operation", "receive"),
		)
	}
	defer span.Finish()

	p.batchMu.Lock()
	defer p.batchMu.Unlock()

	// Initialize batch if needed
	if p.rehydrationBatch == nil {
		p.rehydrationBatch = &Batch{
			Messages:  make([]*nats.Msg, 0, p.config.BatchSize),
			StartTime: time.Now(),
		}
	}

	// Add to batch
	p.rehydrationBatch.Messages = append(p.rehydrationBatch.Messages, msg)

	// Process if batch is full
	if len(p.rehydrationBatch.Messages) >= p.config.BatchSize {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		if err := p.batchProcessor.ProcessRehydrationBatch(ctx, p.rehydrationBatch); err != nil {
			fmt.Printf("Failed to process rehydration batch: %v\n", err)
			span.SetTag("error", err)
			// Messages will be retried via NATS redelivery
		} else {
			span.SetTag("messaging.batch_size", len(p.rehydrationBatch.Messages))
		}

		// Reset batch
		p.rehydrationBatch = nil
	}
}

// processPendingBatches processes any pending batches
func (p *Processor) processPendingBatches(ctx context.Context) {
	p.batchMu.Lock()
	defer p.batchMu.Unlock()

	// Process persistence batch
	if p.persistenceBatch != nil && len(p.persistenceBatch.Messages) > 0 {
		if err := p.batchProcessor.ProcessPersistenceBatch(ctx, p.persistenceBatch); err != nil {
			fmt.Printf("Failed to process pending persistence batch: %v\n", err)
		}
		p.persistenceBatch = nil
	}

	// Process rehydration batch
	if p.rehydrationBatch != nil && len(p.rehydrationBatch.Messages) > 0 {
		if err := p.batchProcessor.ProcessRehydrationBatch(ctx, p.rehydrationBatch); err != nil {
			fmt.Printf("Failed to process pending rehydration batch: %v\n", err)
		}
		p.rehydrationBatch = nil
	}
}

// reportMetrics reports current metrics
func (p *Processor) reportMetrics() {
	stats := p.metrics.GetStats()
	fmt.Printf("📊 Worker Metrics: processed=%d, succeeded=%d, failed=%d, persistence=%d, rehydration=%d\n",
		stats["messages_processed"],
		stats["messages_succeeded"],
		stats["messages_failed"],
		stats["persistence_count"],
		stats["rehydration_count"],
	)
}

// Stop gracefully stops the processor
func (p *Processor) Stop() {
	close(p.stopCh)
	<-p.stoppedCh
}

// shutdown performs graceful shutdown
func (p *Processor) shutdown() error {
	fmt.Println("🛑 Worker shutting down gracefully...")

	// Process any remaining batches
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	p.processPendingBatches(ctx)

	// Final metrics report
	p.reportMetrics()

	close(p.stoppedCh)
	return nil
}

// PerformStartupRehydration performs initial cache warming
func (p *Processor) PerformStartupRehydration(ctx context.Context) error {
	if !p.config.StartupRehydrationEnabled {
		fmt.Println("Startup rehydration is disabled")
		return nil
	}

	fmt.Println("🔄 Performing startup rehydration...")
	return p.batchProcessor.RehydrateAllKeys(ctx)
}
