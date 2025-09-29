package instance

import (
	"context"
	"database/sql"
	"fmt"
)

// MonitoringStats holds instance-level statistics
type MonitoringStats struct {
	InstanceID     string  `json:"instance_id"`
	RowCount       int64   `json:"row_count"`
	DataSizeBytes  int64   `json:"data_size_bytes"`
	DataSizePretty string  `json:"data_size_pretty"`
	CacheHitRate   float64 `json:"cache_hit_rate"`
	QueryCount     int64   `json:"query_count"`
	LastActive     string  `json:"last_active"`
}

// GetInstanceStats returns statistics for a specific instance
func GetInstanceStats(ctx context.Context, db *sql.DB, instanceID string) (*MonitoringStats, error) {
	stats := &MonitoringStats{InstanceID: instanceID}

	// Get row count and data size
	err := db.QueryRowContext(ctx, `
        SELECT 
            COUNT(*) as row_count,
            COALESCE(SUM(pg_column_size(value)), 0) as data_size,
            pg_size_pretty(COALESCE(SUM(pg_column_size(value)), 0)) as size_pretty
        FROM cache_entries
        WHERE instance_id = $1
    `, instanceID).Scan(&stats.RowCount, &stats.DataSizeBytes, &stats.DataSizePretty)

	if err != nil {
		return nil, fmt.Errorf("failed to get instance stats: %w", err)
	}

	return stats, nil
}

// GetLargestInstances returns instances sorted by data size
func GetLargestInstances(ctx context.Context, db *sql.DB, limit int) ([]*MonitoringStats, error) {
	rows, err := db.QueryContext(ctx, `
        WITH instance_stats AS (
            SELECT 
                instance_id,
                COUNT(*) as row_count,
                SUM(pg_column_size(value)) as data_size
            FROM cache_entries
            GROUP BY instance_id
        )
        SELECT 
            instance_id,
            row_count,
            data_size,
            pg_size_pretty(data_size) as size_pretty
        FROM instance_stats
        ORDER BY data_size DESC
        LIMIT $1
    `, limit)

	if err != nil {
		return nil, fmt.Errorf("failed to query largest instances: %w", err)
	}
	defer rows.Close()

	var stats []*MonitoringStats
	for rows.Next() {
		s := &MonitoringStats{}
		if err := rows.Scan(&s.InstanceID, &s.RowCount, &s.DataSizeBytes, &s.DataSizePretty); err != nil {
			return nil, fmt.Errorf("failed to scan row: %w", err)
		}
		stats = append(stats, s)
	}

	return stats, nil
}
