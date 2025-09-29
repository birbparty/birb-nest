#!/bin/bash
# partition-monitor.sh - Run daily to identify partition candidates

# Database connection parameters (customize these)
DB_NAME="${BIRBNEST_DB_NAME:-birbnest}"
DB_USER="${BIRBNEST_DB_USER:-birbnest}"
DB_HOST="${BIRBNEST_DB_HOST:-localhost}"
DB_PORT="${BIRBNEST_DB_PORT:-5432}"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

echo "================================================"
echo "Birb-Nest Instance Partition Monitor"
echo "Database: $DB_NAME @ $DB_HOST:$DB_PORT"
echo "Date: $(date)"
echo "================================================"
echo

# Check for instances that need partitioning
echo "Checking for instances that may need dedicated partitions..."
echo

psql -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER" -d "$DB_NAME" << EOF
SELECT 
    instance_id,
    COUNT(*) as rows,
    pg_size_pretty(SUM(pg_column_size(value))) as size,
    CASE 
        WHEN COUNT(*) > 100000 THEN 'NEEDS PARTITION'
        WHEN SUM(pg_column_size(value)) > 1073741824 THEN 'NEEDS PARTITION'
        ELSE 'OK'
    END as status
FROM cache_entries
GROUP BY instance_id
HAVING COUNT(*) > 50000  -- Show instances approaching threshold
ORDER BY COUNT(*) DESC;
EOF

echo
echo "================================================"
echo "Current Partition Sizes"
echo "================================================"
echo

# Show current partition structure and sizes
psql -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER" -d "$DB_NAME" << EOF
SELECT * FROM get_partition_sizes();
EOF

echo
echo "================================================"
echo "Partition Performance Statistics"
echo "================================================"
echo

# Check query performance by partition
psql -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER" -d "$DB_NAME" << EOF
SELECT 
    schemaname,
    tablename,
    n_tup_ins as inserts,
    n_tup_upd as updates,
    n_tup_del as deletes,
    n_live_tup as live_rows,
    n_dead_tup as dead_rows,
    CASE 
        WHEN n_dead_tup > n_live_tup * 0.2 THEN 'NEEDS VACUUM'
        ELSE 'OK'
    END as vacuum_status
FROM pg_stat_user_tables
WHERE tablename LIKE 'cache_entries_%'
ORDER BY n_tup_ins + n_tup_upd DESC;
EOF

echo
echo "================================================"
echo "Recommendations"
echo "================================================"
echo

# Generate recommendations
psql -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER" -d "$DB_NAME" -t << EOF
SELECT 
    CASE 
        WHEN COUNT(*) > 0 THEN 
            E'${YELLOW}WARNING:${NC} ' || COUNT(*) || ' instance(s) need dedicated partitions!'
        ELSE 
            E'${GREEN}OK:${NC} No instances currently need partitioning.'
    END
FROM (
    SELECT instance_id
    FROM cache_entries
    GROUP BY instance_id
    HAVING COUNT(*) > 100000 
        OR SUM(pg_column_size(value)) > 1073741824
) AS large_instances;
EOF

echo
echo "To create a partition for a large instance, run:"
echo "  CREATE TABLE cache_entries_<instance_id> PARTITION OF cache_entries FOR VALUES IN ('<instance_id>');"
echo
echo "Replace <instance_id> with the actual instance ID, replacing special characters with underscores."
echo
