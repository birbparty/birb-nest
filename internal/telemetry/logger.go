package telemetry

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"sync"

	"github.com/sirupsen/logrus"
	"go.opentelemetry.io/otel/trace"
)

var (
	logger     *logrus.Logger
	loggerOnce sync.Once
	fileLogger *FileLogger
)

// FileLogger handles writing logs to file for local-otel integration
type FileLogger struct {
	mu       sync.Mutex
	file     *os.File
	encoder  *json.Encoder
	filePath string
}

// InitLogger initializes the logger with the given configuration
func InitLogger(cfg *Config) error {
	var err error
	loggerOnce.Do(func() {
		logger = logrus.New()

		// Set log level
		level, parseErr := logrus.ParseLevel(cfg.LogLevel)
		if parseErr != nil {
			level = logrus.InfoLevel
		}
		logger.SetLevel(level)

		// Set formatter
		logger.SetFormatter(&logrus.JSONFormatter{
			TimestampFormat: "2006-01-02T15:04:05.000Z07:00",
			FieldMap: logrus.FieldMap{
				logrus.FieldKeyTime:  "@timestamp",
				logrus.FieldKeyLevel: "level",
				logrus.FieldKeyMsg:   "message",
			},
		})

		// Add standard fields
		logger = logger.WithFields(logrus.Fields{
			"service.name":    cfg.ServiceName,
			"service.version": cfg.ServiceVersion,
			"environment":     cfg.Environment,
		}).Logger

		// If file export is enabled, create file logger
		if cfg.ExportToFile && cfg.LogsFilePath != "" {
			fileLogger, err = NewFileLogger(cfg.LogsFilePath)
			if err != nil {
				logger.WithError(err).Error("Failed to create file logger")
			} else {
				// Add hook to write to file
				logger.AddHook(fileLogger)
			}
		}
	})
	return err
}

// NewFileLogger creates a new file logger
func NewFileLogger(filePath string) (*FileLogger, error) {
	// Ensure directory exists
	dir := filepath.Dir(filePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, err
	}

	file, err := os.OpenFile(filePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return nil, err
	}

	return &FileLogger{
		file:     file,
		encoder:  json.NewEncoder(file),
		filePath: filePath,
	}, nil
}

// Levels returns the log levels this hook is interested in
func (f *FileLogger) Levels() []logrus.Level {
	return logrus.AllLevels
}

// Fire is called when a log event is fired
func (f *FileLogger) Fire(entry *logrus.Entry) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	// Convert entry to a map for JSON encoding
	data := make(map[string]interface{})
	data["@timestamp"] = entry.Time.Format("2006-01-02T15:04:05.000Z07:00")
	data["level"] = entry.Level.String()
	data["message"] = entry.Message

	// Add all fields
	for k, v := range entry.Data {
		data[k] = v
	}

	return f.encoder.Encode(data)
}

// Close closes the file logger
func (f *FileLogger) Close() error {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.file.Close()
}

// L returns the global logger instance
func L() *logrus.Logger {
	if logger == nil {
		// Return a default logger if not initialized
		return logrus.StandardLogger()
	}
	return logger
}

// WithContext adds trace information to the logger
func WithContext(ctx context.Context) *logrus.Entry {
	entry := L().WithContext(ctx)

	// Add trace information if available
	span := trace.SpanFromContext(ctx)
	if span.SpanContext().IsValid() {
		entry = entry.WithFields(logrus.Fields{
			"trace.id": span.SpanContext().TraceID().String(),
			"span.id":  span.SpanContext().SpanID().String(),
		})
	}

	return entry
}

// WithFields adds fields to the logger
func WithFields(fields logrus.Fields) *logrus.Entry {
	return L().WithFields(fields)
}

// WithError adds an error to the logger
func WithError(err error) *logrus.Entry {
	return L().WithError(err)
}

// CloseLogger closes any open resources
func CloseLogger() error {
	if fileLogger != nil {
		return fileLogger.Close()
	}
	return nil
}
