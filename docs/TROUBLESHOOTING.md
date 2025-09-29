# 🔧 Birb Nest Troubleshooting Guide

## Table of Contents
- [Overview](#overview)
- [Common Issues](#common-issues)
  - [Service Won't Start](#service-wont-start)
  - [Connection Issues](#connection-issues)
  - [Performance Problems](#performance-problems)
  - [Data Consistency Issues](#data-consistency-issues)
  - [Queue Processing Issues](#queue-processing-issues)
- [Debugging Techniques](#debugging-techniques)
  - [Log Analysis](#log-analysis)
  - [Metrics Analysis](#metrics-analysis)
  - [Distributed Tracing](#distributed-tracing)
- [Error Messages](#error-messages)
- [Health Check Failures](#health-check-failures)
- [Recovery Procedures](#recovery-procedures)
- [Performance Troubleshooting](#performance-troubleshooting)
- [Emergency Procedures](#emergency-procedures)
- [FAQ](#faq)

## Overview

This guide helps diagnose and resolve common issues with Birb Nest. Start with the symptoms you're experiencing and follow the diagnostic steps.

## Common Issues

### Service Won't Start

#### API Service Startup Issues

**Symptoms:**
- API container exits immediately
- Health check fails repeatedly
- Port binding errors

**Diagnosis:**
```bash
# Check container logs
docker logs birb-nest-api

# For Kubernetes
kubectl logs -n birb-nest deployment/birb-nest-api --tail=100

# Check container status
docker inspect birb-nest-api | jq '.[0].State'
```

**Common Causes & Solutions:**

1. **Port Already in Use**
   ```bash
   # Check if port 8080 is in use
   lsof -i :8080
   # or
   netstat -tulpn | grep 8080
   
   # Solution: Change API_PORT in .env
   API_PORT=8081
   ```

2. **Database Connection Failed**
   ```
   Error: failed to connect to host=postgres user=birb database=birbcache
   ```
   Solution:
   ```bash
   # Verify PostgreSQL is running
   docker ps | grep postgres
   
   # Test connection
   docker exec -it birb-nest-postgres psql -U birb -d birbcache -c "SELECT 1"
   
   # Check credentials in .env match docker-compose.yml
   ```

3. **Invalid Configuration**
   ```
   Error: invalid value for WORKER_BATCH_SIZE: strconv.Atoi: parsing "abc": invalid syntax
   ```
   Solution: Ensure all numeric configuration values are valid numbers

#### Worker Service Startup Issues

**Symptoms:**
- Worker exits after starting
- NATS connection errors
- Consumer already exists errors

**Common Solutions:**

1. **NATS Not Ready**
   ```bash
   # Ensure NATS is healthy
   curl -s http://localhost:8222/healthz
   
   # Check NATS logs
   docker logs birb-nest-nats
   ```

2. **Consumer Name Conflict**
   ```bash
   # Delete existing consumer
   docker exec -it birb-nest-nats nats consumer rm BIRB_CACHE birb-worker
   
   # Or use unique consumer names
   WORKER_CONSUMER_GROUP=birb-workers-$(hostname)
   ```

### Connection Issues

#### PostgreSQL Connection Problems

**Symptoms:**
- "connection refused" errors
- "password authentication failed"
- Timeout errors

**Diagnostics:**
```bash
# Test direct connection
PGPASSWORD=birbpass psql -h localhost -U birb -d birbcache -c "\conninfo"

# Check PostgreSQL logs
docker logs birb-nest-postgres | tail -50

# Verify network connectivity
docker exec -it birb-nest-api nc -zv postgres 5432
```

**Solutions:**

1. **Connection Pool Exhausted**
   ```sql
   -- Check current connections
   SELECT count(*) FROM pg_stat_activity;
   
   -- Check max connections
   SHOW max_connections;
   
   -- Kill idle connections
   SELECT pg_terminate_backend(pid) 
   FROM pg_stat_activity 
   WHERE state = 'idle' 
   AND state_change < NOW() - INTERVAL '10 minutes';
   ```

2. **Network Issues**
   ```bash
   # Verify Docker network
   docker network inspect birb-net
   
   # Recreate network if needed
   docker-compose down
   docker network prune
   docker-compose up -d
   ```

#### Redis Connection Problems

**Symptoms:**
- "connection refused" on port 6379
- "NOAUTH Authentication required"
- Timeout errors

**Diagnostics:**
```bash
# Test Redis connection
docker exec -it birb-nest-redis redis-cli ping

# With authentication
docker exec -it birb-nest-redis redis-cli -a yourpassword ping

# Check Redis logs
docker logs birb-nest-redis | tail -50
```

**Solutions:**

1. **Memory Issues**
   ```bash
   # Check Redis memory
   docker exec -it birb-nest-redis redis-cli INFO memory
   
   # If maxmemory is reached
   docker exec -it birb-nest-redis redis-cli CONFIG SET maxmemory-policy allkeys-lru
   ```

2. **Persistence Issues**
   ```bash
   # Check AOF status
   docker exec -it birb-nest-redis redis-cli INFO persistence
   
   # Repair corrupted AOF
   docker exec -it birb-nest-redis redis-check-aof --fix /data/birb-cache.aof
   ```

#### NATS Connection Problems

**Symptoms:**
- "nats: no servers available for connection"
- Authentication errors
- Stream not found errors

**Diagnostics:**
```bash
# Check NATS health
curl http://localhost:8222/healthz

# View NATS monitoring
curl http://localhost:8222/varz | jq .

# Check JetStream status
docker exec -it birb-nest-nats nats-cli stream ls
```

**Solutions:**

1. **Create Missing Streams**
   ```bash
   # Create persistence stream
   docker exec -it birb-nest-nats nats stream add BIRB_CACHE \
     --subjects "cache.persist.*" \
     --storage file \
     --retention limits \
     --max-msgs=1000000
   
   # Create DLQ stream
   docker exec -it birb-nest-nats nats stream add BIRB_CACHE_DLQ \
     --subjects "cache.dlq.*" \
     --storage file \
     --retention limits \
     --max-age=7d
   ```

### Performance Problems

#### High Latency

**Symptoms:**
- API response times > 100ms
- Cache hit rate < 80%
- Database queries slow

**Diagnostics:**
```bash
# Check API metrics
curl http://localhost:8080/metrics | jq .

# Monitor Redis latency
docker exec -it birb-nest-redis redis-cli --latency

# PostgreSQL slow queries
docker exec -it birb-nest-postgres psql -U birb -d birbcache -c "
SELECT query, mean_exec_time, calls 
FROM pg_stat_statements 
ORDER BY mean_exec_time DESC 
LIMIT 10;"
```

**Solutions:**

1. **Optimize Batch Sizes**
   ```bash
   # Increase for better throughput
   WORKER_BATCH_SIZE=500
   WORKER_BATCH_TIMEOUT=2s
   
   # Decrease for lower latency
   WORKER_BATCH_SIZE=50
   WORKER_BATCH_TIMEOUT=500ms
   ```

2. **Scale Services**
   ```bash
   # Scale workers
   docker-compose up -d --scale worker=5
   
   # For Kubernetes
   kubectl scale deployment birb-nest-worker --replicas=10
   ```

#### High Memory Usage

**Symptoms:**
- Container OOM kills
- Redis evictions
- Slow garbage collection

**Diagnostics:**
```bash
# Check container memory
docker stats birb-nest-api birb-nest-worker

# Redis memory analysis
docker exec -it birb-nest-redis redis-cli MEMORY DOCTOR

# Go memory profile
curl http://localhost:8080/debug/pprof/heap > heap.prof
go tool pprof heap.prof
```

**Solutions:**

1. **Set Memory Limits**
   ```bash
   # Go memory limit
   GOMEMLIMIT=2GiB
   
   # Redis max memory
   docker exec -it birb-nest-redis redis-cli CONFIG SET maxmemory 4gb
   ```

2. **Reduce Cache Size**
   ```bash
   # Shorter TTLs
   REDIS_DEFAULT_TTL=1800  # 30 minutes
   
   # Evict old keys
   docker exec -it birb-nest-redis redis-cli FLUSHDB
   ```

### Data Consistency Issues

#### Missing Data

**Symptoms:**
- Keys exist in PostgreSQL but not Redis
- 404 errors for existing data
- Incomplete batch writes

**Diagnostics:**
```sql
-- Check version mismatches
SELECT key, version, updated_at 
FROM cache_entries 
WHERE key = 'problematic-key';

-- Find missing keys
SELECT ce.key 
FROM cache_entries ce
WHERE NOT EXISTS (
  SELECT 1 FROM redis_keys rk WHERE rk.key = ce.key
);
```

**Solutions:**

1. **Force Rehydration**
   ```bash
   # Trigger rehydration for specific key
   curl -X GET http://localhost:8080/v1/cache/missing-key
   
   # Bulk rehydration
   docker exec -it birb-nest-worker ./rehydrate --force
   ```

2. **Verify Queue Processing**
   ```bash
   # Check queue depth
   docker exec -it birb-nest-nats nats stream info BIRB_CACHE
   
   # View pending messages
   docker exec -it birb-nest-nats nats consumer info BIRB_CACHE birb-worker
   ```

### Queue Processing Issues

#### Messages Stuck in Queue

**Symptoms:**
- Increasing queue depth
- Messages not acknowledged
- DLQ filling up

**Diagnostics:**
```bash
# Check consumer status
docker exec -it birb-nest-nats nats consumer report BIRB_CACHE

# View stuck messages
docker exec -it birb-nest-nats nats stream view BIRB_CACHE
```

**Solutions:**

1. **Reset Consumer**
   ```bash
   # Delete and recreate consumer
   docker exec -it birb-nest-nats nats consumer rm BIRB_CACHE birb-worker
   docker restart birb-nest-worker
   ```

2. **Process DLQ**
   ```bash
   # View DLQ messages
   docker exec -it birb-nest-nats nats stream view BIRB_CACHE_DLQ
   
   # Reprocess DLQ
   docker exec -it birb-nest-worker ./process-dlq --retry
   ```

## Debugging Techniques

### Log Analysis

#### Enable Debug Logging

```bash
# Set debug level
LOG_LEVEL=debug
LOG_FORMAT=text  # Easier to read

# Restart services
docker-compose restart api worker
```

#### Structured Log Queries

```bash
# Find errors in JSON logs
docker logs birb-nest-api 2>&1 | jq 'select(.level == "error")'

# Track specific request
docker logs birb-nest-api 2>&1 | jq 'select(.trace_id == "abc123")'

# Count errors by type
docker logs birb-nest-api 2>&1 | \
  jq -r 'select(.level == "error") | .error' | \
  sort | uniq -c | sort -nr
```

### Metrics Analysis

#### Key Metrics to Monitor

```bash
# Cache performance
curl -s http://localhost:8080/metrics | jq '{
  hit_rate: .cache_hit_rate,
  total_requests: .total_requests,
  avg_latency: .average_latency_ms
}'

# Queue health
docker exec -it birb-nest-nats nats-cli stream report
```

#### Prometheus Queries

```promql
# Request rate
rate(http_requests_total[5m])

# Error rate
rate(http_requests_total{status=~"5.."}[5m])

# Cache hit rate
rate(cache_hits_total[5m]) / 
(rate(cache_hits_total[5m]) + rate(cache_misses_total[5m]))

# p99 latency
histogram_quantile(0.99, rate(http_request_duration_seconds_bucket[5m]))
```

### Distributed Tracing

#### View Traces in Jaeger

```bash
# Open Jaeger UI
open http://localhost:16686

# Find slow requests
# Service: birb-nest-api
# Operation: GET /v1/cache/{key}
# Min Duration: 100ms
```

#### Trace Analysis

```bash
# Export trace
curl "http://localhost:16686/api/traces/{traceID}" > trace.json

# Analyze span durations
jq '.data[0].spans[] | {operation: .operationName, duration: .duration}' trace.json
```

## Error Messages

### Common Errors and Solutions

| Error Message | Cause | Solution |
|--------------|-------|----------|
| `connection refused` | Service not running | Check service status and logs |
| `context deadline exceeded` | Timeout | Increase timeout values |
| `duplicate key value violates unique constraint` | Concurrent writes | Implement retry with backoff |
| `redis: connection pool timeout` | Pool exhausted | Increase pool size |
| `WRONGTYPE Operation against a key holding the wrong kind of value` | Redis data corruption | Delete key and retry |
| `pq: remaining connection slots are reserved` | Connection limit | Reduce pool size or increase PostgreSQL limit |
| `nats: consumer already exists` | Duplicate consumer | Use unique consumer names |
| `circuit breaker is open` | Too many failures | Check downstream service health |

## Health Check Failures

### Diagnosing Health Issues

```bash
# Detailed health check
curl -v http://localhost:8080/health | jq .

# Component-specific checks
curl http://localhost:8080/health | jq '.checks'
```

### Common Health Check Fixes

1. **Database Unhealthy**
   ```bash
   # Restart PostgreSQL
   docker restart birb-nest-postgres
   
   # Check disk space
   df -h /var/lib/postgresql/data
   ```

2. **Redis Unhealthy**
   ```bash
   # Check Redis process
   docker exec -it birb-nest-redis redis-cli ping
   
   # Restart if needed
   docker restart birb-nest-redis
   ```

3. **NATS Unhealthy**
   ```bash
   # Check NATS JetStream
   docker exec -it birb-nest-nats nats-cli account info
   
   # Restart NATS
   docker restart birb-nest-nats
   ```

## Recovery Procedures

### Data Recovery

#### Restore from PostgreSQL

```bash
# Export all data
docker exec -it birb-nest-postgres pg_dump -U birb birbcache > backup.sql

# Clear Redis
docker exec -it birb-nest-redis redis-cli FLUSHDB

# Trigger full rehydration
docker exec -it birb-nest-worker ./rehydrate --all
```

#### Restore from Backup

```bash
# Stop services
docker-compose stop api worker

# Restore PostgreSQL
docker exec -i birb-nest-postgres psql -U birb birbcache < backup.sql

# Restore Redis snapshot
docker cp redis-backup.rdb birb-nest-redis:/data/dump.rdb
docker restart birb-nest-redis

# Start services
docker-compose start api worker
```

### Service Recovery

#### Full System Restart

```bash
#!/bin/bash
# recovery.sh

echo "Starting Birb Nest recovery..."

# 1. Stop all services
docker-compose down

# 2. Clean up volumes (careful!)
# docker volume prune  # Uncomment if needed

# 3. Start infrastructure first
docker-compose up -d postgres redis nats

# 4. Wait for health
sleep 10

# 5. Start application services
docker-compose up -d api worker

# 6. Verify health
sleep 5
curl http://localhost:8080/health | jq .
```

## Performance Troubleshooting

### CPU Profiling

```bash
# Enable profiling endpoint
API_DEBUG_PORT=2345

# Capture CPU profile
curl http://localhost:2345/debug/pprof/profile?seconds=30 > cpu.prof

# Analyze
go tool pprof -http=:6060 cpu.prof
```

### Memory Profiling

```bash
# Capture heap profile
curl http://localhost:2345/debug/pprof/heap > heap.prof

# Analyze allocations
go tool pprof -alloc_space heap.prof

# Find memory leaks
go tool pprof -inuse_space heap.prof
```

### Goroutine Analysis

```bash
# Check goroutine count
curl http://localhost:2345/debug/pprof/goroutine?debug=1

# Find goroutine leaks
curl http://localhost:2345/debug/pprof/goroutine?debug=2 | grep -A 10 "goroutine profile"
```

## Emergency Procedures

### System Overload

```bash
# 1. Enable rate limiting
curl -X POST http://localhost:8080/admin/ratelimit -d '{"enabled": true, "limit": 10}'

# 2. Reduce worker batch size
docker exec -it birb-nest-worker kill -USR1 1  # Graceful config reload

# 3. Clear cache if needed
docker exec -it birb-nest-redis redis-cli FLUSHDB

# 4. Scale down if necessary
docker-compose scale worker=1
```

### Data Corruption

```bash
# 1. Stop writes
docker stop birb-nest-api

# 2. Backup current state
docker exec -it birb-nest-postgres pg_dump -U birb birbcache > emergency-backup.sql

# 3. Identify corrupted data
docker exec -it birb-nest-postgres psql -U birb -d birbcache -c "
SELECT key, version, updated_at 
FROM cache_entries 
WHERE value IS NULL OR value = '{}'::jsonb
ORDER BY updated_at DESC;"

# 4. Clean corrupted entries
# Be very careful with DELETE operations!

# 5. Restart services
docker start birb-nest-api
```

## FAQ

### Q: Why is my cache hit rate low?

**A:** Common causes:
- TTL too short
- Cache warming disabled
- Redis memory full
- Keys not following patterns

Check:
```bash
# Current hit rate
curl http://localhost:8080/metrics | jq .cache_hit_rate

# Redis memory
docker exec -it birb-nest-redis redis-cli INFO memory | grep used_memory_human

# TTL settings
echo "TTL: $REDIS_DEFAULT_TTL seconds"
```

### Q: How do I handle "duplicate key" errors?

**A:** This happens with concurrent writes. Solutions:
1. Enable optimistic locking
2. Implement retry logic
3. Use unique request IDs

### Q: Why are messages stuck in the queue?

**A:** Check:
1. Worker health
2. Database connectivity
3. Consumer acknowledgments
4. DLQ for failed messages

### Q: How do I debug intermittent failures?

**A:** Enable detailed logging and tracing:
```bash
LOG_LEVEL=debug
OTEL_TRACES_ENABLED=true
FEATURE_METRICS_DETAILED=true
```

Then correlate logs, metrics, and traces using timestamp and trace ID.

### Q: What's the maximum cache size?

**A:** Depends on:
- Redis memory: `maxmemory` setting
- Value size: Default 1MB per key
- Number of keys: Limited by memory

Calculate: `max_keys = (redis_memory * 0.8) / avg_value_size`

---

🐦 Don't panic! Most issues have simple solutions. If you're still stuck, check the logs and ask for help! 🚀
