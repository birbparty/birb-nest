# Birb-Nest SDK Logging Guide

This guide explains how to manage logs from applications using the Birb-Nest SDK, understand log messages, and set up log aggregation. Written for operators with no coding experience.

## Understanding Log Levels

### What Each Level Means

| Level | When It's Used | Should You Worry? | Example |
|-------|----------------|-------------------|---------|
| **DEBUG** | Detailed info for developers | No - Normal during troubleshooting | "Checking cache for key: user_123" |
| **INFO** | Normal operations | No - This is good | "Successfully connected to Birb-Nest" |
| **WARN** | Something unusual but handled | Maybe - Monitor if frequent | "Retry attempt 2 of 3" |
| **ERROR** | Something failed | Yes - Needs investigation | "Failed to connect to Redis" |
| **FATAL** | Application can't continue | YES - Immediate action! | "Cannot start: invalid configuration" |

### Quick Reference

```
DEBUG → Too detailed for normal use
INFO  → Everything is working ✓
WARN  → Keep an eye on this
ERROR → Something needs fixing
FATAL → Application stopped!
```

## Configuring Logging

### Basic Configuration

Set logging level via environment variables:

```bash
# Show only important messages (recommended for production)
LOG_LEVEL=info

# Show warnings and above
LOG_LEVEL=warn

# Show everything (for troubleshooting)
LOG_LEVEL=debug

# Format options
LOG_FORMAT=json    # For log aggregation systems
LOG_FORMAT=text    # Human-readable
```

### Docker Compose Example

```yaml
services:
  app:
    image: your-app:latest
    environment:
      - LOG_LEVEL=info
      - LOG_FORMAT=json
      - LOG_OUTPUT=stdout
    volumes:
      - ./logs:/var/log/app
```

## Important Log Patterns

### Startup Logs (Normal)

What you should see when starting:
```
INFO: Starting Birb-Nest SDK client
INFO: Connecting to http://birb-nest-api:8080
INFO: Connection pool initialized (max_idle=100)
INFO: Circuit breaker configured (threshold=5)
INFO: Successfully connected to Birb-Nest API
INFO: Health check passed
INFO: Application ready to serve requests
```

### Connection Issues

```
ERROR: Failed to connect to Birb-Nest API: connection refused
WARN: Retrying connection (attempt 1/3)
WARN: Retrying connection (attempt 2/3)
ERROR: All retry attempts failed
ERROR: Circuit breaker opened after 5 consecutive failures
```

**What to do**: Check if Birb-Nest service is running

### Performance Issues

```
WARN: Request took 250ms (threshold: 100ms)
WARN: High latency detected: avg=180ms, p95=450ms
INFO: Cache hit rate: 45% (below optimal 80%)
WARN: Connection pool exhausted, waiting for available connection
```

**What to do**: Check system resources and network

### Cache Operations

```
DEBUG: Cache lookup for key: session_xyz
INFO: Cache miss for key: session_xyz
DEBUG: Fetching from source and caching
INFO: Cached value for key: session_xyz (TTL: 3600s)
INFO: Cache hit for key: session_xyz
```

**Normal pattern**: Some misses are expected, especially after restart

## Log Collection Setup

### Option 1: Simple File Logging

Create logging directories:
```bash
mkdir -p /var/log/birb-nest/{app,api,nginx}
chmod 755 /var/log/birb-nest
```

Configure log rotation `/etc/logrotate.d/birb-nest`:
```
/var/log/birb-nest/*/*.log {
    daily
    rotate 7
    compress
    missingok
    notifempty
    create 0644 root root
    sharedscripts
    postrotate
        # Reload services to reopen log files
        docker-compose kill -s USR1 app
    endscript
}
```

### Option 2: Centralized Logging with ELK Stack

Create `logging-stack.yml`:
```yaml
version: '3.8'

services:
  # Elasticsearch for storage
  elasticsearch:
    image: docker.elastic.co/elasticsearch/elasticsearch:8.11.0
    container_name: birb-elasticsearch
    environment:
      - discovery.type=single-node
      - "ES_JAVA_OPTS=-Xms512m -Xmx512m"
      - xpack.security.enabled=false
    volumes:
      - es_data:/usr/share/elasticsearch/data
    ports:
      - "9200:9200"

  # Logstash for processing
  logstash:
    image: docker.elastic.co/logstash/logstash:8.11.0
    container_name: birb-logstash
    volumes:
      - ./logstash.conf:/usr/share/logstash/pipeline/logstash.conf
    depends_on:
      - elasticsearch

  # Kibana for visualization
  kibana:
    image: docker.elastic.co/kibana/kibana:8.11.0
    container_name: birb-kibana
    ports:
      - "5601:5601"
    environment:
      - ELASTICSEARCH_HOSTS=http://elasticsearch:9200
    depends_on:
      - elasticsearch

  # Filebeat for collection
  filebeat:
    image: docker.elastic.co/beats/filebeat:8.11.0
    container_name: birb-filebeat
    user: root
    volumes:
      - ./filebeat.yml:/usr/share/filebeat/filebeat.yml
      - /var/lib/docker/containers:/var/lib/docker/containers:ro
      - /var/run/docker.sock:/var/run/docker.sock:ro
    depends_on:
      - logstash

volumes:
  es_data:
```

### Logstash Configuration

Create `logstash.conf`:
```ruby
input {
  beats {
    port => 5044
  }
}

filter {
  # Parse JSON logs
  if [message] =~ /^\{/ {
    json {
      source => "message"
    }
  }
  
  # Parse log level
  grok {
    match => {
      "message" => "%{LOGLEVEL:log_level}"
    }
  }
  
  # Add tags based on content
  if [message] =~ /error|ERROR/ {
    mutate {
      add_tag => ["error"]
    }
  }
  
  if [message] =~ /circuit.breaker|Circuit breaker/ {
    mutate {
      add_tag => ["circuit_breaker"]
    }
  }
}

output {
  elasticsearch {
    hosts => ["elasticsearch:9200"]
    index => "birb-logs-%{+YYYY.MM.dd}"
  }
}
```

### Filebeat Configuration

Create `filebeat.yml`:
```yaml
filebeat.inputs:
- type: container
  paths:
    - '/var/lib/docker/containers/*/*.log'
  processors:
    - add_docker_metadata:
        host: "unix:///var/run/docker.sock"
    - decode_json_fields:
        fields: ["message"]
        target: "json"
        overwrite_keys: true

output.logstash:
  hosts: ["logstash:5044"]

logging.level: info
```

## Log Analysis Patterns

### Finding Errors Quickly

```bash
# Last 100 errors
docker logs your-app 2>&1 | grep -i error | tail -100

# Errors in time range
docker logs --since 1h your-app 2>&1 | grep -i error

# Count errors by type
docker logs your-app 2>&1 | grep -i error | cut -d' ' -f4- | sort | uniq -c | sort -rn
```

### Performance Analysis

```bash
# Find slow requests
docker logs your-app | grep "duration" | awk '$NF > 100' 

# Average response time
docker logs your-app | grep "duration" | awk '{sum+=$NF; count++} END {print sum/count}'

# Cache hit rate over time
docker logs --since 1h your-app | grep "cache" | grep -c "hit"
docker logs --since 1h your-app | grep "cache" | grep -c "miss"
```

### Connection Issues

```bash
# Connection failures
docker logs your-app | grep -E "connection|Connection" | grep -i "failed\|refused\|timeout"

# Circuit breaker events
docker logs your-app | grep -i "circuit"

# Retry attempts
docker logs your-app | grep -i "retry"
```

## Common Log Messages Explained

### Normal Operations

| Log Message | What It Means | Action |
|-------------|---------------|--------|
| "Health check passed" | Service is healthy | None - this is good |
| "Cache hit for key: X" | Found data in cache | None - this is optimal |
| "Connection pool: 10/100 active" | Using 10 connections | None - normal load |
| "Request completed in 25ms" | Fast response time | None - excellent |

### Warning Signs

| Log Message | What It Means | Action |
|-------------|---------------|--------|
| "Retry attempt 2 of 3" | First attempt failed | Monitor - may indicate issues |
| "Cache miss for key: X" | Data not in cache | Normal unless frequent |
| "Connection pool: 95/100 active" | Near connection limit | Consider scaling |
| "Request completed in 250ms" | Slow response | Investigate if frequent |

### Error Conditions

| Log Message | What It Means | Action |
|-------------|---------------|--------|
| "Connection refused" | Can't reach service | Check if service is running |
| "Circuit breaker open" | Too many failures | Fix underlying issue |
| "Timeout after 30s" | Request too slow | Check service health |
| "Invalid configuration" | Bad settings | Fix configuration |

## Debug Mode

### Enabling Debug Logs

When troubleshooting, enable debug mode:

```bash
# Temporarily enable debug logging
export LOG_LEVEL=debug
docker-compose up -d app

# Watch debug logs
docker logs -f your-app | grep DEBUG

# IMPORTANT: Disable when done (debug logs are verbose!)
export LOG_LEVEL=info
docker-compose up -d app
```

### What Debug Logs Show

```
DEBUG: Preparing request to /api/cache/user_123
DEBUG: Request headers: {Content-Type: application/json, X-Request-ID: abc-123}
DEBUG: Connection pool status: {active: 5, idle: 95, wait: 0}
DEBUG: Sending request...
DEBUG: Response received in 23ms
DEBUG: Response status: 200 OK
DEBUG: Parsing response body...
DEBUG: Cache write: key=user_123, size=1.2KB, ttl=3600s
```

## Log Monitoring Scripts

### Log Health Check

Create `check-logs.sh`:
```bash
#!/bin/bash
# Quick log health check

echo "=== Log Health Check ==="
echo ""

# Check error rate
ERROR_COUNT=$(docker logs --since 10m your-app 2>&1 | grep -c ERROR)
TOTAL_LINES=$(docker logs --since 10m your-app 2>&1 | wc -l)
ERROR_RATE=$(echo "scale=2; $ERROR_COUNT * 100 / $TOTAL_LINES" | bc)

echo "Error Rate: ${ERROR_RATE}%"
if (( $(echo "$ERROR_RATE > 5" | bc -l) )); then
    echo "⚠️  WARNING: High error rate!"
fi

# Check for circuit breaker
CIRCUIT_OPEN=$(docker logs --since 10m your-app 2>&1 | grep -c "circuit.*open")
if [ "$CIRCUIT_OPEN" -gt 0 ]; then
    echo "⚠️  Circuit breaker triggered $CIRCUIT_OPEN times"
fi

# Check response times
AVG_TIME=$(docker logs --since 10m your-app 2>&1 | grep "duration" | awk '{sum+=$NF; count++} END {if(count>0) print sum/count; else print 0}')
echo "Average Response Time: ${AVG_TIME}ms"

# Check cache performance
CACHE_HITS=$(docker logs --since 10m your-app 2>&1 | grep -c "cache hit")
CACHE_MISSES=$(docker logs --since 10m your-app 2>&1 | grep -c "cache miss")
if [ $((CACHE_HITS + CACHE_MISSES)) -gt 0 ]; then
    HIT_RATE=$(echo "scale=2; $CACHE_HITS * 100 / ($CACHE_HITS + $CACHE_MISSES)" | bc)
    echo "Cache Hit Rate: ${HIT_RATE}%"
fi
```

### Real-time Log Monitoring

Create `monitor-logs.sh`:
```bash
#!/bin/bash
# Real-time log monitoring with alerts

tail -f /var/log/birb-nest/app/app.log | while read line; do
    # Check for errors
    if echo "$line" | grep -q "ERROR"; then
        echo "🔴 ERROR: $line"
        # Send alert (customize as needed)
        # echo "$line" | mail -s "Birb-Nest Error" ops@example.com
    fi
    
    # Check for warnings
    if echo "$line" | grep -q "WARN"; then
        echo "🟡 WARNING: $line"
    fi
    
    # Check for circuit breaker
    if echo "$line" | grep -q "circuit.*open"; then
        echo "⚡ CIRCUIT BREAKER: $line"
        # Immediate alert for circuit breaker
    fi
done
```

## Log Retention Policy

### Recommended Settings

| Log Type | Retention | Compression | Reason |
|----------|-----------|-------------|---------|
| ERROR logs | 30 days | Yes | For investigation |
| INFO logs | 7 days | Yes | Normal operations |
| DEBUG logs | 1 day | No | Large volume |
| Audit logs | 1 year | Yes | Compliance |

### Implementing Retention

```bash
# Daily cleanup script
cat > /etc/cron.daily/birb-log-cleanup << 'EOF'
#!/bin/bash
# Clean old logs

# Delete debug logs older than 1 day
find /var/log/birb-nest -name "*.debug.log" -mtime +1 -delete

# Delete info logs older than 7 days
find /var/log/birb-nest -name "*.info.log" -mtime +7 -delete

# Compress error logs older than 2 days
find /var/log/birb-nest -name "*.error.log" -mtime +2 -exec gzip {} \;

# Delete compressed logs older than 30 days
find /var/log/birb-nest -name "*.gz" -mtime +30 -delete
EOF

chmod +x /etc/cron.daily/birb-log-cleanup
```

## Troubleshooting Logging Issues

### No Logs Appearing

1. Check log level:
   ```bash
   docker exec your-app env | grep LOG
   ```

2. Check log destination:
   ```bash
   docker inspect your-app | grep -A 5 "Mounts"
   ```

3. Check permissions:
   ```bash
   ls -la /var/log/birb-nest/
   ```

### Logs Too Verbose

1. Change log level:
   ```bash
   export LOG_LEVEL=warn
   docker-compose up -d
   ```

2. Filter in real-time:
   ```bash
   docker logs -f your-app 2>&1 | grep -v DEBUG
   ```

### Disk Full from Logs

1. Check disk usage:
   ```bash
   du -sh /var/log/birb-nest/*
   ```

2. Emergency cleanup:
   ```bash
   # Remove old logs
   find /var/log/birb-nest -name "*.log" -mtime +7 -delete
   
   # Truncate active log
   truncate -s 0 /var/log/birb-nest/app/current.log
   ```

## Quick Reference

```
=== LOGGING QUICK REFERENCE ===

Log Levels:
DEBUG < INFO < WARN < ERROR < FATAL

Key Commands:
- View logs: docker logs your-app
- Follow logs: docker logs -f your-app
- Last hour: docker logs --since 1h your-app
- Errors only: docker logs your-app 2>&1 | grep ERROR

Important Patterns:
- "ERROR" = Something failed
- "circuit.*open" = Service protection activated
- "retry" = Temporary issue, attempting recovery
- "timeout" = Slow response or network issue

Quick Checks:
- Error rate: Should be < 1%
- Cache hits: Should be > 80%
- Response time: Should be < 100ms
- Circuit breaker: Should be closed

Log Locations:
- Docker: /var/lib/docker/containers/*/\*.log
- Application: /var/log/birb-nest/app/
- System: /var/log/syslog
```

## Next Steps

1. Start with basic file logging
2. Set up log rotation
3. Create monitoring scripts
4. Consider ELK stack for scale
5. Document your logging strategy

For more information, see:
- [Monitoring Guide](./MONITORING.md)
- [Troubleshooting Guide](./TROUBLESHOOTING.md)
- [Operations Guide](./OPERATIONS.md)
