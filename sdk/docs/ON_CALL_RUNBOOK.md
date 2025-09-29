# Birb-Nest SDK On-Call Runbook

**⚠️ EMERGENCY CONTACT**: If you can't resolve an issue within 30 minutes, escalate to:
- Primary: Development Team Slack Channel #birb-nest-dev
- Secondary: DevOps Lead (Phone: Update with actual number)
- Critical Issues: PagerDuty escalation policy "birb-nest-critical"

## Quick Reference - Common Issues

| Symptom | Most Likely Cause | Page # |
|---------|------------------|--------|
| "Connection refused" errors | Birb-Nest service is down | [Jump to fix](#connection-refused) |
| "Request timeout" errors | Network issues or service overload | [Jump to fix](#request-timeout) |
| "Circuit breaker is open" | Service has failed too many times | [Jump to fix](#circuit-breaker-open) |
| "Rate limited" errors | Too many requests | [Jump to fix](#rate-limited) |
| High memory usage | Connection pool exhaustion | [Jump to fix](#high-memory) |
| Slow response times | Cache miss or network latency | [Jump to fix](#slow-response) |

## Before You Start

### 1. Check Service Health
```bash
# Check if Birb-Nest API is running
curl -f http://localhost:8080/health || echo "Service is DOWN"

# Check Redis connectivity
redis-cli ping || echo "Redis is DOWN"

# Check Postgres connectivity
pg_isready -h localhost -p 5432 || echo "Postgres is DOWN"
```

### 2. Gather Information
- **When did the issue start?** Check monitoring dashboards
- **What changed?** Check recent deployments
- **How many users affected?** Check error rate metrics
- **Is it getting worse?** Check trend graphs

## Common Issues and Solutions

### <a name="connection-refused"></a>1. "Connection refused" Errors

**What this means**: The application cannot connect to the Birb-Nest service.

**Symptoms in logs**:
```
network error during dial: connection refused
Error Type: network
```

**Step-by-Step Fix**:

1. **Check if Birb-Nest API is running**:
   ```bash
   docker ps | grep birb-nest-api
   ```
   - If not listed: Service is down, go to step 2
   - If listed: Check the STATUS column

2. **Restart the Birb-Nest API** (SAFE):
   ```bash
   docker-compose restart birb-nest-api
   ```
   Wait 30 seconds, then check health:
   ```bash
   curl http://localhost:8080/health
   ```

3. **Check container logs for errors**:
   ```bash
   docker logs birb-nest-api --tail 100
   ```
   Look for:
   - "Cannot connect to Redis" → [Fix Redis](#fix-redis)
   - "Cannot connect to Postgres" → [Fix Postgres](#fix-postgres)
   - Port binding errors → Another service using port 8080

4. **If still failing**, full restart:
   ```bash
   docker-compose down
   docker-compose up -d
   ```

**When to escalate**: If service won't start after 2 restart attempts.

### <a name="request-timeout"></a>2. "Request timeout" Errors

**What this means**: Requests are taking too long to complete.

**Symptoms in logs**:
```
timeout during request
Error Type: timeout
```

**Step-by-Step Fix**:

1. **Check system resources**:
   ```bash
   # Check CPU usage
   top -n 1 | head -10
   
   # Check memory
   free -h
   
   # Check disk space
   df -h
   ```

2. **Check network connectivity**:
   ```bash
   # Test latency to Birb-Nest
   ping -c 5 localhost
   
   # Check for packet loss
   mtr --report --report-cycles 10 localhost
   ```

3. **Check service load**:
   ```bash
   # View current connections
   netstat -an | grep :8080 | wc -l
   ```
   - If > 1000: Service is overloaded

4. **Emergency load reduction** (if overloaded):
   ```bash
   # Scale up if using container orchestration
   docker-compose scale birb-nest-api=3
   ```

**When to escalate**: If timeouts persist after checking resources.

### <a name="circuit-breaker-open"></a>3. "Circuit breaker is open" Errors

**What this means**: The SDK has stopped trying to connect because too many requests have failed.

**Symptoms in logs**:
```
circuit breaker is open
Error Type: circuit_open
```

**Step-by-Step Fix**:

1. **This is protecting your system** - Don't panic!
   - The circuit breaker prevents cascading failures
   - It will automatically retry after a cooldown period (usually 30 seconds)

2. **Check the root cause**:
   ```bash
   # Look for the original errors that triggered the circuit breaker
   docker logs birb-nest-api --tail 200 | grep -B 5 "error"
   ```

3. **Fix the underlying issue**:
   - Connection refused → [See Connection Refused fix](#connection-refused)
   - Timeouts → [See Timeout fix](#request-timeout)
   - Server errors → Check API logs

4. **Wait for automatic recovery**:
   - Circuit breaker will test with a single request after 30 seconds
   - If successful, it reopens for traffic
   - Monitor logs: `tail -f application.log | grep "circuit"`

**When to escalate**: If circuit breaker stays open for > 5 minutes.

### <a name="rate-limited"></a>4. "Rate limited" Errors

**What this means**: Too many requests are being sent to the service.

**Symptoms in logs**:
```
rate limited
Error Type: rate_limit
API error (status 429): Too Many Requests
```

**Step-by-Step Fix**:

1. **Identify the source**:
   ```bash
   # Check request rates by client
   docker logs birb-nest-api --tail 1000 | grep "client_id" | sort | uniq -c | sort -rn
   ```

2. **Temporary rate limit increase** (REQUIRES APPROVAL):
   ```bash
   # Edit configuration
   vi docker-compose.yml
   # Find RATE_LIMIT_PER_SECOND and increase by 50%
   # Then restart:
   docker-compose up -d birb-nest-api
   ```

3. **Contact the high-usage client**:
   - Notify them of the issue
   - Ask them to reduce request rate
   - Suggest batching requests

**When to escalate**: If legitimate traffic is being rate limited.

### <a name="high-memory"></a>5. High Memory Usage

**What this means**: The application is using too much RAM.

**Symptoms**:
- Application becomes slow
- Out of Memory (OOM) errors
- System starts swapping

**Step-by-Step Fix**:

1. **Check memory usage**:
   ```bash
   # Check container memory
   docker stats --no-stream | grep birb-nest
   
   # Check system memory
   free -h
   ```

2. **Quick fix - Restart to clear memory** (SAFE):
   ```bash
   docker-compose restart birb-nest-api
   ```

3. **Check for connection leaks**:
   ```bash
   # Count open connections
   lsof -i :8080 | wc -l
   ```
   - If > 500: Possible connection leak

4. **Reduce connection pool size**:
   ```bash
   # Edit configuration
   vi docker-compose.yml
   # Find MAX_IDLE_CONNS and reduce from 100 to 50
   docker-compose up -d birb-nest-api
   ```

**When to escalate**: If memory issues persist after restart.

### <a name="slow-response"></a>6. Slow Response Times

**What this means**: Requests complete but take longer than expected.

**Normal response times**:
- Cache hit: < 10ms
- Cache miss: < 50ms
- Anything > 100ms is slow

**Step-by-Step Fix**:

1. **Check cache hit rate**:
   ```bash
   # View metrics endpoint
   curl http://localhost:8080/metrics | grep cache_hit_rate
   ```
   - If < 80%: Cache is not effective

2. **Check Redis performance**:
   ```bash
   redis-cli --latency
   # Press Ctrl+C after 10 seconds
   ```
   - If > 5ms: Redis is slow

3. **Check network latency**:
   ```bash
   ping -c 10 redis-server
   ```

4. **Clear cache if corrupted** (REQUIRES APPROVAL):
   ```bash
   redis-cli FLUSHDB
   ```
   ⚠️ WARNING: This deletes all cached data!

**When to escalate**: If latency > 200ms consistently.

## Fix Procedures for Dependencies

### <a name="fix-redis"></a>Fix Redis Connection Issues

1. **Check if Redis is running**:
   ```bash
   docker ps | grep redis
   redis-cli ping
   ```

2. **Restart Redis** (SAFE):
   ```bash
   docker-compose restart redis
   ```

3. **Check Redis logs**:
   ```bash
   docker logs redis --tail 50
   ```

4. **Emergency Redis recovery**:
   ```bash
   # Stop everything
   docker-compose down
   # Clear Redis data (LAST RESORT)
   rm -rf ./data/redis/*
   # Start fresh
   docker-compose up -d
   ```

### <a name="fix-postgres"></a>Fix Postgres Connection Issues

1. **Check if Postgres is running**:
   ```bash
   docker ps | grep postgres
   pg_isready
   ```

2. **Restart Postgres** (REQUIRES APPROVAL):
   ```bash
   docker-compose restart postgres
   ```

3. **Check connection limit**:
   ```bash
   docker exec postgres psql -U birb_user -c "SELECT count(*) FROM pg_stat_activity;"
   ```
   - If near 100: Connection limit reached

## Emergency Procedures

### Complete Service Restart (REQUIRES APPROVAL)

Use when individual fixes don't work:

```bash
# 1. Save current state
docker ps > ~/birb-nest-state-$(date +%s).txt

# 2. Stop everything
docker-compose down

# 3. Wait 10 seconds
sleep 10

# 4. Start everything
docker-compose up -d

# 5. Verify health
sleep 30
curl http://localhost:8080/health
```

### Rollback Procedure (CRITICAL ISSUES ONLY)

If a recent deployment caused issues:

```bash
# 1. Check current version
docker images | grep birb-nest

# 2. Rollback to previous version
docker-compose down
export BIRB_NEST_VERSION=previous_version_here
docker-compose up -d

# 3. Verify rollback
curl http://localhost:8080/version
```

## Monitoring Commands

### Quick Health Check
```bash
# Create a health check script
cat > /tmp/birb-health.sh << 'EOF'
#!/bin/bash
echo "=== Birb-Nest Health Check ==="
echo -n "API: "; curl -sf http://localhost:8080/health && echo "OK" || echo "FAIL"
echo -n "Redis: "; redis-cli ping 2>/dev/null || echo "FAIL"
echo -n "Postgres: "; pg_isready -q && echo "OK" || echo "FAIL"
echo -n "Memory: "; free -h | grep Mem | awk '{print $3"/"$2}'
echo -n "Disk: "; df -h / | tail -1 | awk '{print $5" used"}'
EOF
chmod +x /tmp/birb-health.sh
/tmp/birb-health.sh
```

### Performance Check
```bash
# Check response times
for i in {1..10}; do
  time curl -s http://localhost:8080/health > /dev/null
done 2>&1 | grep real
```

## Important Notes

### Safe Operations (No approval needed)
- Checking logs
- Viewing metrics
- Restarting individual containers
- Running health checks

### Dangerous Operations (REQUIRES APPROVAL)
- Changing configuration
- Clearing cache data
- Database operations
- Version rollbacks
- Scaling services

### When to Wake Someone Up

Call the on-call developer if:
1. Service is down for > 15 minutes
2. Data corruption is suspected
3. Security breach indicators
4. Multiple services failing simultaneously
5. Rollback doesn't fix the issue

## Useful Log Queries

```bash
# Find all errors in last hour
docker logs birb-nest-api --since 1h 2>&1 | grep -i error

# Count errors by type
docker logs birb-nest-api --tail 1000 | grep "Error Type:" | sort | uniq -c

# Find slow requests
docker logs birb-nest-api --tail 1000 | grep "duration" | awk '$NF > 100'

# Check for restarts
docker ps -a | grep birb-nest | grep -v "Up"
```

## Post-Incident Actions

After resolving any issue:

1. **Document what happened** in the incident log
2. **Note what fixed it** for future reference
3. **Check if it's happening elsewhere** in the system
4. **Set a reminder** to check again in 1 hour

Remember: Stay calm, follow the steps, and escalate when needed. You've got this! 🐦
