-- scripts/migrations/001_partitioned_cache_entries.sql

-- Drop existing table if migrating from non-partitioned
-- WARNING: This will delete all data! Only for dev/test environments
-- For production, use a migration strategy that preserves data

-- Create partitioned table
CREATE TABLE IF NOT EXISTS cache_entries (
    instance_id VARCHAR(255) NOT NULL,
    key VARCHAR(255) NOT NULL,
    value JSONB NOT NULL,
    version INTEGER DEFAULT 1,
    ttl INTEGER,
    metadata JSONB,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    
    PRIMARY KEY (instance_id, key)
) PARTITION BY LIST (instance_id);

-- Create default partition that catches all instances
CREATE TABLE IF NOT EXISTS cache_entries_default PARTITION OF cache_entries DEFAULT;

-- Create indexes on the parent table (inherited by all partitions)
CREATE INDEX IF NOT EXISTS idx_cache_entries_instance_id ON cache_entries(instance_id);
CREATE INDEX IF NOT EXISTS idx_cache_entries_instance_key ON cache_entries(instance_id, key);
CREATE INDEX IF NOT EXISTS idx_cache_entries_updated_at ON cache_entries(instance_id, updated_at);

-- Create a function to track table sizes (for monitoring)
CREATE OR REPLACE FUNCTION get_partition_sizes() 
RETURNS TABLE(
    partition_name TEXT,
    size_pretty TEXT,
    row_count BIGINT
) AS $$
BEGIN
    RETURN QUERY
    SELECT 
        schemaname||'.'||tablename AS partition_name,
        pg_size_pretty(pg_total_relation_size(schemaname||'.'||tablename)) AS size_pretty,
        n_live_tup AS row_count
    FROM pg_stat_user_tables
    WHERE tablename LIKE 'cache_entries_%'
    ORDER BY pg_total_relation_size(schemaname||'.'||tablename) DESC;
END;
$$ LANGUAGE plpgsql;
