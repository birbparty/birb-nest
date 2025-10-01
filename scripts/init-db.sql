-- Birb Nest PostgreSQL initialization script

-- Create cache_entries table
CREATE TABLE IF NOT EXISTS cache_entries (
    key VARCHAR(255) NOT NULL,
    value JSONB NOT NULL,
    instance_id TEXT DEFAULT '' NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    version INTEGER DEFAULT 1,
    ttl INTEGER DEFAULT NULL,
    metadata JSONB DEFAULT '{}'::jsonb,
    PRIMARY KEY (instance_id, key)
);

-- Create indexes for performance
CREATE INDEX idx_cache_entries_created_at ON cache_entries(created_at);
CREATE INDEX idx_cache_entries_updated_at ON cache_entries(updated_at);
CREATE INDEX idx_cache_entries_ttl ON cache_entries(ttl) WHERE ttl IS NOT NULL;
CREATE INDEX idx_cache_entries_value_gin ON cache_entries USING GIN (value);
CREATE INDEX idx_cache_entries_metadata_gin ON cache_entries USING GIN (metadata);
-- Instance-aware composite index for efficient queries
CREATE INDEX idx_cache_entries_instance_key ON cache_entries(instance_id, key);

-- Create function to update updated_at timestamp
CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = CURRENT_TIMESTAMP;
    RETURN NEW;
END;
$$ language 'plpgsql';

-- Create trigger to automatically update updated_at
CREATE TRIGGER update_cache_entries_updated_at 
    BEFORE UPDATE ON cache_entries 
    FOR EACH ROW 
    EXECUTE FUNCTION update_updated_at_column();

-- Create function to increment version on update
CREATE OR REPLACE FUNCTION increment_version()
RETURNS TRIGGER AS $$
BEGIN
    NEW.version = OLD.version + 1;
    RETURN NEW;
END;
$$ language 'plpgsql';

-- Create trigger to automatically increment version
CREATE TRIGGER increment_cache_entries_version
    BEFORE UPDATE ON cache_entries
    FOR EACH ROW
    EXECUTE FUNCTION increment_version();

-- Create table for dead letter queue entries
CREATE TABLE IF NOT EXISTS dlq_entries (
    id SERIAL PRIMARY KEY,
    message_id VARCHAR(255) NOT NULL,
    key VARCHAR(255) NOT NULL,
    value JSONB NOT NULL,
    error_message TEXT,
    retry_count INTEGER DEFAULT 0,
    max_retries INTEGER DEFAULT 3,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    last_retry_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    status VARCHAR(50) DEFAULT 'pending'
);

-- Create indexes for DLQ
CREATE INDEX idx_dlq_entries_status ON dlq_entries(status);
CREATE INDEX idx_dlq_entries_key ON dlq_entries(key);
CREATE INDEX idx_dlq_entries_created_at ON dlq_entries(created_at);
CREATE INDEX idx_dlq_entries_last_retry_at ON dlq_entries(last_retry_at);

-- Create metrics table for analytics
CREATE TABLE IF NOT EXISTS cache_metrics (
    id SERIAL,
    timestamp TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP NOT NULL,
    operation VARCHAR(50) NOT NULL,
    key VARCHAR(255),
    duration_ms INTEGER,
    success BOOLEAN DEFAULT true,
    error_message TEXT,
    metadata JSONB DEFAULT '{}'::jsonb,
    PRIMARY KEY (id, timestamp)
) PARTITION BY RANGE (timestamp);

-- Create indexes for metrics
CREATE INDEX idx_cache_metrics_timestamp ON cache_metrics(timestamp);
CREATE INDEX idx_cache_metrics_operation ON cache_metrics(operation);
CREATE INDEX idx_cache_metrics_success ON cache_metrics(success);

-- Partition metrics table by month for better performance
CREATE TABLE cache_metrics_2025_01 PARTITION OF cache_metrics
    FOR VALUES FROM ('2025-01-01') TO ('2025-02-01');

CREATE TABLE cache_metrics_2025_02 PARTITION OF cache_metrics
    FOR VALUES FROM ('2025-02-01') TO ('2025-03-01');

-- Continue creating partitions as needed...

-- Grant permissions to the birb user
GRANT ALL PRIVILEGES ON ALL TABLES IN SCHEMA public TO birb;
GRANT ALL PRIVILEGES ON ALL SEQUENCES IN SCHEMA public TO birb;
GRANT EXECUTE ON ALL FUNCTIONS IN SCHEMA public TO birb;

-- Initial data for testing (optional)
INSERT INTO cache_entries (key, value, metadata) VALUES
    ('test:welcome', '{"message": "Welcome to Birb Nest!", "emoji": "🐦"}', '{"type": "system"}'),
    ('test:version', '{"version": "1.0.0", "build": "initial"}', '{"type": "system"}')
ON CONFLICT (key) DO NOTHING;

-- Vacuum and analyze for optimal performance
VACUUM ANALYZE;
