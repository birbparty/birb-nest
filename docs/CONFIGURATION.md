# ⚙️ Birb Nest Configuration Guide

## Table of Contents
- [Overview](#overview)
- [Configuration Sources](#configuration-sources)
- [Database Configuration](#database-configuration)
- [Cache Configuration](#cache-configuration)
- [Queue Configuration](#queue-configuration)
- [API Service Configuration](#api-service-configuration)
- [Worker Service Configuration](#worker-service-configuration)
- [Observability Configuration](#observability-configuration)
- [Performance Tuning](#performance-tuning)
- [Feature Flags](#feature-flags)
- [Development Tools](#development-tools)
- [Configuration Best Practices](#configuration-best-practices)
- [Environment-Specific Configurations](#environment-specific-configurations)

## Overview

Birb Nest uses environment variables for configuration, following the 12-factor app methodology. All configuration options have sensible defaults, but can be customized for your specific deployment needs.

## Configuration Sources

Configuration is loaded in the following priority order:
1. Environment variables
2. `.env` file (for local development)
3. Default values in code

### Quick Start

```bash
# Copy the example configuration
cp .env.example .env

# Edit with your values
vim .env

# Start services
docker-compose up
```

## Database Configuration

### PostgreSQL Settings

| Variable | Default | Description |
|----------|---------|-------------|
| `POSTGRES_HOST` | `localhost` | PostgreSQL server hostname |
| `POSTGRES_PORT` | `5432` | PostgreSQL server port |
| `POSTGRES_USER` | `birb` | Database username |
| `POSTGRES_PASSWORD` | `birbpass` | Database password |
| `POSTGRES_DB` | `birbcache` | Database name |
| `POSTGRES_MAX_CONNECTIONS` | `25` | Maximum connections in pool |
| `POSTGRES_MAX_IDLE_CONNECTIONS` | `5` | Maximum idle connections |
| `POSTGRES_CONNECTION_MAX_LIFETIME` | `5m` | Maximum connection lifetime |

### Connection Pool Tuning

```bash
# High-traffic production settings
POSTGRES_MAX_CONNECTIONS=100
POSTGRES_MAX_IDLE_CONNECTIONS=25
POSTGRES_CONNECTION_MAX_LIFETIME=15m

# Low-traffic or development
POSTGRES_MAX_CONNECTIONS=10
POSTGRES_MAX_IDLE_CONNECTIONS=2
POSTGRES_CONNECTION_MAX_LIFETIME=30m
```

### Performance Considerations

- **Max Connections**: Set based on `max_connections` in PostgreSQL (typically 100)
- **Rule of thumb**: API instances × max_connections_per_instance < postgres_max_connections × 0.8
- **Idle Connections**: Keep low to reduce memory usage, but high enough to avoid connection churn

## Cache Configuration

### Redis Settings

| Variable | Default | Description |
|----------|---------|-------------|
| `REDIS_HOST` | `localhost` | Redis server hostname |
| `REDIS_PORT` | `6379` | Redis server port |
| `REDIS_PASSWORD` | `` | Redis password (empty for no auth) |
| `REDIS_DB` | `0` | Redis database number |
| `REDIS_DEFAULT_TTL` | `3600` | Default TTL in seconds (1 hour) |
| `REDIS_MAX_TTL` | `86400` | Maximum allowed TTL (24 hours) |
| `REDIS_POOL_SIZE` | `10` | Connection pool size |
| `REDIS_MIN_IDLE_CONNS` | `5` | Minimum idle connections |
| `REDIS_MAX_RETRIES` | `3` | Maximum retry attempts |

### TTL Strategy

```bash
# Short-lived data (sessions, temporary tokens)
REDIS_DEFAULT_TTL=1800    # 30 minutes
REDIS_MAX_TTL=3600        # 1 hour

# Medium-lived data (user profiles, API responses)
REDIS_DEFAULT_TTL=3600    # 1 hour
REDIS_MAX_TTL=86400       # 24 hours

# Long-lived data (configuration, reference data)
REDIS_DEFAULT_TTL=86400   # 24 hours
REDIS_MAX_TTL=604800      # 7 days
```

### Memory Management

Calculate Redis memory requirements:
```
Memory = (avg_key_size + avg_value_size) × number_of_keys × 1.5 (overhead)

Example:
- Average key: 50 bytes
- Average value: 1 KB
- Expected keys: 1 million
- Memory needed: (50 + 1024) × 1,000,000 × 1.5 = ~1.6 GB
```

## Queue Configuration

### NATS JetStream Settings

| Variable | Default | Description |
|----------|---------|-------------|
| `NATS_URL` | `nats://birb:birbpass@localhost:4222` | NATS connection URL |
| `NATS_USER` | `birb` | NATS username |
| `NATS_PASSWORD` | `birbpass` | NATS password |
| `NATS_PORT` | `4222` | NATS client port |
| `NATS_MONITOR_PORT` | `8222` | NATS monitoring port |
| `NATS_STREAM_NAME` | `BIRB_CACHE` | JetStream stream name |
| `NATS_CONSUMER_NAME` | `birb-worker` | Consumer name |
| `NATS_MAX_PENDING` | `1000` | Maximum pending messages |
| `NATS_ACK_WAIT` | `30s` | Message acknowledgment timeout |

### DLQ Configuration

| Variable | Default | Description |
|----------|---------|-------------|
| `NATS_DLQ_MAX_RETRIES` | `3` | Maximum retry attempts |
| `NATS_DLQ_STREAM_NAME` | `BIRB_CACHE_DLQ` | DLQ stream name |

### Queue Sizing

```bash
# High throughput (>10k msg/s)
NATS_MAX_PENDING=10000
NATS_ACK_WAIT=60s

# Medium throughput (1k-10k msg/s)
NATS_MAX_PENDING=5000
NATS_ACK_WAIT=30s

# Low throughput (<1k msg/s)
NATS_MAX_PENDING=1000
NATS_ACK_WAIT=15s
```

## API Service Configuration

### Server Settings

| Variable | Default | Description |
|----------|---------|-------------|
| `API_HOST` | `0.0.0.0` | API bind address |
| `API_PORT` | `8080` | API port |
| `API_DEBUG_PORT` | `2345` | Debug server port (development) |
| `API_READ_TIMEOUT` | `10s` | HTTP read timeout |
| `API_WRITE_TIMEOUT` | `10s` | HTTP write timeout |
| `API_IDLE_TIMEOUT` | `120s` | HTTP idle timeout |

### Rate Limiting

| Variable | Default | Description |
|----------|---------|-------------|
| `API_RATE_LIMIT_REQUESTS` | `100` | Requests per duration |
| `API_RATE_LIMIT_DURATION` | `1m` | Rate limit window |

### CORS Settings

| Variable | Default | Description |
|----------|---------|-------------|
| `API_CORS_ALLOWED_ORIGINS` | `*` | Allowed origins |
| `API_CORS_ALLOWED_METHODS` | `GET,POST,PUT,DELETE,OPTIONS` | Allowed methods |
| `API_CORS_ALLOWED_HEADERS` | `Content-Type,Authorization` | Allowed headers |

### API Tuning Examples

```bash
# Production API (high traffic)
API_READ_TIMEOUT=30s
API_WRITE_TIMEOUT=30s
API_IDLE_TIMEOUT=300s
API_RATE_LIMIT_REQUESTS=1000
API_RATE_LIMIT_DURATION=1m

# Internal API (trusted clients)
API_RATE_LIMIT_REQUESTS=0  # Disable rate limiting
API_CORS_ALLOWED_ORIGINS=https://internal.company.com

# Public API (strict limits)
API_RATE_LIMIT_REQUESTS=60
API_RATE_LIMIT_DURATION=1m
API_CORS_ALLOWED_ORIGINS=https://app.company.com
```

## Worker Service Configuration

### Worker Settings

| Variable | Default | Description |
|----------|---------|-------------|
| `WORKER_REPLICAS` | `2` | Number of worker instances |
| `WORKER_DEBUG_PORT` | `2346` | Debug server port |
| `WORKER_BATCH_SIZE` | `100` | Messages per batch |
| `WORKER_BATCH_TIMEOUT` | `1s` | Batch collection timeout |
| `WORKER_CONSUMER_GROUP` | `birb-workers` | Consumer group name |

### Rehydration Settings

| Variable | Default | Description |
|----------|---------|-------------|
| `WORKER_REHYDRATION_BATCH_SIZE` | `1000` | Keys per rehydration batch |
| `WORKER_REHYDRATION_INTERVAL` | `5m` | Rehydration check interval |
| `WORKER_REHYDRATION_PRIORITY_THRESHOLD` | `0.8` | Priority threshold (0-1) |

### Worker Scaling

```bash
# High throughput processing
WORKER_REPLICAS=10
WORKER_BATCH_SIZE=500
WORKER_BATCH_TIMEOUT=500ms

# Balanced processing
WORKER_REPLICAS=5
WORKER_BATCH_SIZE=100
WORKER_BATCH_TIMEOUT=1s

# Low latency processing
WORKER_REPLICAS=3
WORKER_BATCH_SIZE=10
WORKER_BATCH_TIMEOUT=100ms
```

### Batch Size Calculation

```
Optimal batch size = (Database write latency × Target throughput) / Number of workers

Example:
- DB write latency: 10ms
- Target throughput: 10,000 msg/s
- Number of workers: 5
- Optimal batch size: (0.01 × 10,000) / 5 = 20
```

## Observability Configuration

### Logging

| Variable | Default | Description |
|----------|---------|-------------|
| `LOG_LEVEL` | `info` | Log level (debug, info, warn, error) |
| `LOG_FORMAT` | `json` | Log format (json, text) |

### OpenTelemetry

| Variable | Default | Description |
|----------|---------|-------------|
| `OTEL_EXPORTER_OTLP_ENDPOINT` | `http://localhost:4317` | OTLP endpoint |
| `OTEL_SERVICE_NAME` | `birb-nest` | Service name |
| `OTEL_TRACES_ENABLED` | `true` | Enable tracing |
| `OTEL_METRICS_ENABLED` | `true` | Enable metrics |
| `OTEL_LOGS_ENABLED` | `true` | Enable log export |

### Datadog Integration (Optional)

| Variable | Default | Description |
|----------|---------|-------------|
| `DATADOG_ENABLED` | `false` | Enable Datadog integration |
| `DATADOG_API_KEY` | `` | Datadog API key |
| `DATADOG_APP_KEY` | `` | Datadog application key |
| `DATADOG_SITE` | `datadoghq.com` | Datadog site |

### Observability Profiles

```bash
# Development (verbose logging, all telemetry)
LOG_LEVEL=debug
LOG_FORMAT=text
OTEL_TRACES_ENABLED=true
OTEL_METRICS_ENABLED=true
OTEL_LOGS_ENABLED=true

# Production (optimized)
LOG_LEVEL=info
LOG_FORMAT=json
OTEL_TRACES_ENABLED=true
OTEL_METRICS_ENABLED=true
OTEL_LOGS_ENABLED=false  # Use log aggregator instead

# Troubleshooting (maximum detail)
LOG_LEVEL=debug
FEATURE_METRICS_DETAILED=true
OTEL_TRACES_ENABLED=true
```

## Performance Tuning

### Go Runtime

| Variable | Default | Description |
|----------|---------|-------------|
| `GOMAXPROCS` | `0` | Max CPU cores (0 = all cores) |
| `GOMEMLIMIT` | `0` | Memory limit (0 = no limit) |

### Circuit Breaker

| Variable | Default | Description |
|----------|---------|-------------|
| `CIRCUIT_BREAKER_TIMEOUT` | `10s` | Request timeout |
| `CIRCUIT_BREAKER_MAX_REQUESTS` | `10` | Max requests in half-open |
| `CIRCUIT_BREAKER_INTERVAL` | `1m` | Reset interval |
| `CIRCUIT_BREAKER_FAILURE_RATIO` | `0.5` | Failure threshold |

### Performance Profiles

```bash
# CPU-optimized (compute-heavy workloads)
GOMAXPROCS=0  # Use all cores
GOMEMLIMIT=0  # No memory limit
WORKER_BATCH_SIZE=1000

# Memory-optimized (large cache values)
GOMAXPROCS=4  # Limit CPU
GOMEMLIMIT=8GiB  # Set memory limit
REDIS_POOL_SIZE=5  # Smaller connection pool

# Balanced (general purpose)
GOMAXPROCS=0
GOMEMLIMIT=0
WORKER_BATCH_SIZE=100
```

## Feature Flags

| Variable | Default | Description |
|----------|---------|-------------|
| `FEATURE_BATCH_WRITES` | `true` | Enable batch database writes |
| `FEATURE_ASYNC_DELETES` | `true` | Enable async delete processing |
| `FEATURE_CACHE_WARMING` | `true` | Enable cache warming on startup |
| `FEATURE_METRICS_DETAILED` | `false` | Enable detailed metrics |

### Feature Flag Usage

```bash
# Performance testing (disable optimizations)
FEATURE_BATCH_WRITES=false
FEATURE_ASYNC_DELETES=false

# Debugging (maximum visibility)
FEATURE_METRICS_DETAILED=true
LOG_LEVEL=debug

# Production (all optimizations)
FEATURE_BATCH_WRITES=true
FEATURE_ASYNC_DELETES=true
FEATURE_CACHE_WARMING=true
FEATURE_METRICS_DETAILED=false
```

## Development Tools

### pgAdmin Configuration

| Variable | Default | Description |
|----------|---------|-------------|
| `PGADMIN_EMAIL` | `admin@birb.party` | pgAdmin login email |
| `PGADMIN_PASSWORD` | `admin` | pgAdmin password |

### Grafana Configuration

| Variable | Default | Description |
|----------|---------|-------------|
| `GRAFANA_USER` | `admin` | Grafana username |
| `GRAFANA_PASSWORD` | `admin` | Grafana password |

### Air Hot Reload

| Variable | Default | Description |
|----------|---------|-------------|
| `AIR_ENABLED` | `true` | Enable hot reload in development |

## Configuration Best Practices

### 1. **Security**
- Never commit `.env` files with real credentials
- Use secrets management in production (Kubernetes Secrets, AWS Secrets Manager, etc.)
- Rotate passwords regularly
- Use strong, unique passwords

### 2. **Environment Separation**
```bash
# Development
cp .env.example .env.development
# Edit for local development

# Staging
cp .env.example .env.staging
# Edit for staging environment

# Production
# Use secrets management, not files
```

### 3. **Validation**
```go
// Example validation in code
if os.Getenv("REDIS_DEFAULT_TTL") > os.Getenv("REDIS_MAX_TTL") {
    log.Fatal("REDIS_DEFAULT_TTL cannot exceed REDIS_MAX_TTL")
}
```

### 4. **Documentation**
- Document all custom configuration
- Include examples in `.env.example`
- Explain performance implications

## Environment-Specific Configurations

### Development Environment

```bash
# .env.development
LOG_LEVEL=debug
LOG_FORMAT=text
API_RATE_LIMIT_REQUESTS=0  # Disable rate limiting
WORKER_REPLICAS=1
FEATURE_METRICS_DETAILED=true
AIR_ENABLED=true
```

### Staging Environment

```bash
# Staging configuration (via ConfigMap/Secrets)
LOG_LEVEL=info
LOG_FORMAT=json
API_RATE_LIMIT_REQUESTS=500
WORKER_REPLICAS=3
REDIS_DEFAULT_TTL=1800  # 30 minutes for testing
FEATURE_CACHE_WARMING=false  # Faster startup
```

### Production Environment

```bash
# Production configuration (via Secrets Manager)
LOG_LEVEL=info
LOG_FORMAT=json
API_RATE_LIMIT_REQUESTS=1000
WORKER_REPLICAS=10
REDIS_DEFAULT_TTL=3600
POSTGRES_MAX_CONNECTIONS=100
CIRCUIT_BREAKER_FAILURE_RATIO=0.3  # More sensitive
FEATURE_BATCH_WRITES=true
FEATURE_ASYNC_DELETES=true
FEATURE_CACHE_WARMING=true
```

### Load Testing Environment

```bash
# Load test configuration
LOG_LEVEL=warn  # Reduce logging overhead
WORKER_REPLICAS=20
WORKER_BATCH_SIZE=1000
POSTGRES_MAX_CONNECTIONS=200
REDIS_POOL_SIZE=50
API_RATE_LIMIT_REQUESTS=0  # No limits for testing
FEATURE_METRICS_DETAILED=true  # Collect all metrics
```

## Configuration Validation Script

```bash
#!/bin/bash
# validate-config.sh

# Check required variables
required_vars=(
    "POSTGRES_HOST"
    "POSTGRES_USER"
    "POSTGRES_PASSWORD"
    "REDIS_HOST"
    "NATS_URL"
)

for var in "${required_vars[@]}"; do
    if [ -z "${!var}" ]; then
        echo "ERROR: Required variable $var is not set"
        exit 1
    fi
done

# Validate numeric ranges
if [ "$API_RATE_LIMIT_REQUESTS" -lt 0 ]; then
    echo "ERROR: API_RATE_LIMIT_REQUESTS must be >= 0"
    exit 1
fi

if [ "$REDIS_DEFAULT_TTL" -gt "$REDIS_MAX_TTL" ]; then
    echo "ERROR: REDIS_DEFAULT_TTL cannot exceed REDIS_MAX_TTL"
    exit 1
fi

echo "Configuration validation passed!"
```

## Monitoring Configuration Health

Use these queries to monitor configuration effectiveness:

```sql
-- PostgreSQL connection pool utilization
SELECT count(*) as active_connections,
       max_connections,
       (count(*) * 100.0 / max_connections) as utilization_percent
FROM pg_stat_activity, 
     (SELECT setting::int as max_connections 
      FROM pg_settings 
      WHERE name = 'max_connections') s;

-- Redis memory usage
INFO memory

-- NATS stream info
nats stream info BIRB_CACHE
```

---

🐦 Configure wisely, and your birbs will fly smoothly! 🚀
