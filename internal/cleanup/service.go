// Package cleanup provides instance cleanup functionality
package cleanup

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/birbparty/birb-nest/internal/instance"
	"github.com/birbparty/birb-nest/internal/operations"
	"github.com/birbparty/birb-nest/internal/storage"
	"github.com/nats-io/nats.go"
)

// CleanupService manages automatic instance cleanup based on inactivity
type CleanupService struct {
	registry   *instance.Registry
	operations *operations.InstanceOperations
	storage    *storage.SpacesClient
	queue      *nats.Conn
	config     CleanupConfig
}

// CleanupConfig contains configuration for the cleanup service
type CleanupConfig struct {
	InactivityTimeout   time.Duration
	MinimumAge          time.Duration
	CleanupInterval     time.Duration
	DryRun              bool
	ArchiveBeforeDelete bool
}

// CleanupNotification represents a cleanup event notification
type CleanupNotification struct {
	InstanceID  string    `json:"instance_id"`
	CleanupTime time.Time `json:"cleanup_time"`
	Reason      string    `json:"reason"`
	Archived    bool      `json:"archived"`
	ArchivePath string    `json:"archive_path,omitempty"`
}

// NewCleanupService creates a new cleanup service
func NewCleanupService(
	registry *instance.Registry,
	ops *operations.InstanceOperations,
	storage *storage.SpacesClient,
	queue *nats.Conn,
	config CleanupConfig,
) *CleanupService {
	// Set defaults
	if config.InactivityTimeout == 0 {
		config.InactivityTimeout = 30 * time.Minute
	}
	if config.MinimumAge == 0 {
		config.MinimumAge = 30 * time.Minute
	}
	if config.CleanupInterval == 0 {
		config.CleanupInterval = 5 * time.Minute
	}

	return &CleanupService{
		registry:   registry,
		operations: ops,
		storage:    storage,
		queue:      queue,
		config:     config,
	}
}

// Start begins the cleanup service's monitoring loop
func (c *CleanupService) Start(ctx context.Context) {
	ticker := time.NewTicker(c.config.CleanupInterval)
	defer ticker.Stop()

	log.Printf("Cleanup service started (DryRun: %v, Interval: %v)",
		c.config.DryRun, c.config.CleanupInterval)

	// Run immediately on start
	c.performCleanup(ctx)

	for {
		select {
		case <-ticker.C:
			c.performCleanup(ctx)
		case <-ctx.Done():
			log.Println("Cleanup service stopped")
			return
		}
	}
}

// performCleanup executes a cleanup cycle
func (c *CleanupService) performCleanup(ctx context.Context) {
	log.Println("Starting cleanup cycle")

	// Get all instances
	instances, err := c.registry.List(ctx, instance.ListFilter{})
	if err != nil {
		log.Printf("Failed to list instances: %v", err)
		return
	}

	cleanupCount := 0
	for _, inst := range instances {
		if c.shouldCleanup(inst) {
			if err := c.cleanupInstance(ctx, inst); err != nil {
				log.Printf("Failed to cleanup instance %s: %v", inst.InstanceID, err)
			} else {
				cleanupCount++
			}
		}
	}

	log.Printf("Cleanup cycle completed: %d instances cleaned", cleanupCount)
}

// shouldCleanup determines if an instance should be cleaned up
func (c *CleanupService) shouldCleanup(inst *instance.Context) bool {
	// Check if instance can be auto-deleted
	if !inst.CanBeAutoDeleted() {
		return false
	}

	// Check inactivity timeout
	inactiveDuration := time.Since(inst.LastActive)
	if inactiveDuration < c.config.InactivityTimeout {
		return false
	}

	// Check minimum age
	age := time.Since(inst.CreatedAt)
	if age < c.config.MinimumAge {
		return false
	}

	return true
}

// cleanupInstance archives and deletes a single instance
func (c *CleanupService) cleanupInstance(ctx context.Context, inst *instance.Context) error {
	log.Printf("Cleaning up instance %s (inactive for %v)",
		inst.InstanceID, time.Since(inst.LastActive))

	notification := CleanupNotification{
		InstanceID:  inst.InstanceID,
		CleanupTime: time.Now(),
		Reason:      fmt.Sprintf("Inactive for %v", time.Since(inst.LastActive)),
	}

	// Archive if enabled
	if c.config.ArchiveBeforeDelete && !c.config.DryRun {
		archivePath, err := c.archiveInstance(ctx, inst.InstanceID)
		if err != nil {
			return fmt.Errorf("failed to archive: %w", err)
		}
		notification.Archived = true
		notification.ArchivePath = archivePath
	}

	// Delete instance (skip in dry-run mode)
	if !c.config.DryRun {
		if err := c.operations.DeleteInstance(ctx, inst.InstanceID); err != nil {
			return fmt.Errorf("failed to delete: %w", err)
		}
	}

	// Send notification
	if err := c.sendNotification(ctx, notification); err != nil {
		log.Printf("Failed to send cleanup notification: %v", err)
	}

	if c.config.DryRun {
		log.Printf("DRY RUN: Would have deleted instance %s", inst.InstanceID)
	} else {
		log.Printf("Successfully cleaned up instance %s", inst.InstanceID)
	}

	return nil
}

// archiveInstance creates a backup and uploads it to storage
func (c *CleanupService) archiveInstance(ctx context.Context, instanceID string) (string, error) {
	// Create a buffer to store the backup
	var buf bytes.Buffer

	// Use existing backup functionality
	if err := c.operations.BackupInstance(ctx, instanceID, &buf); err != nil {
		return "", fmt.Errorf("backup failed: %w", err)
	}

	// Upload to Digital Ocean Spaces
	archivePath, err := c.storage.UploadArchive(instanceID, &buf)
	if err != nil {
		return "", fmt.Errorf("upload failed: %w", err)
	}

	log.Printf("Archived instance %s to %s", instanceID, archivePath)
	return archivePath, nil
}

// sendNotification publishes a cleanup notification to the message queue
func (c *CleanupService) sendNotification(ctx context.Context, notification CleanupNotification) error {
	data, err := json.Marshal(notification)
	if err != nil {
		return err
	}

	// Create NATS message with headers
	msg := &nats.Msg{
		Subject: "instance.cleanup",
		Data:    data,
		Header: nats.Header{
			"instance-id":  []string{notification.InstanceID},
			"cleanup-time": []string{notification.CleanupTime.Format(time.RFC3339)},
		},
	}

	// Publish with timeout
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	return c.queue.PublishMsg(msg)
}
