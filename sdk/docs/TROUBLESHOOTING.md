# Birb-Nest SDK Troubleshooting Guide

This guide explains every error message you might see, what it means in plain English, and how to fix it. No programming knowledge required!

## Quick Error Lookup

| Error Message | What It Means | Quick Fix |
|--------------|---------------|-----------|
| "key not found" | The data you're looking for doesn't exist | This is often normal - the app will create it |
| "connection refused" | Can't connect to Birb-Nest service | Check if service is running |
| "request timeout" | Request took too long | Check network/service health |
| "circuit breaker is open" | Too many failures, SDK stopped trying | Wait 30 seconds, will auto-recover |
| "rate limited" | Too many requests too fast | Wait and retry, or increase limits |
| "invalid configuration" | Settings are wrong | Check environment variables |

## Detailed Error Dictionary

### 1. "key not found"

**Full Error**: `key not found`  
**Error Type**: `client`  
**When You'll See This**: When trying to retrieve data that doesn't exist

**What This Really Means**:
- The application asked for some data that hasn't been saved yet
- Like looking for a file in an empty folder
- This is often NORMAL behavior - not always an error!

**Common Causes**:
1. First time running the application
2. Cache was cleared recently
3. Data expired and was automatically deleted
4. Someone deleted the data

**How to Fix**:
- **Usually**: Nothing to fix! The app will create the data when needed
- **If persistent**: Check if the app is saving data correctly
- **If unexpected**: Verify the data source is working

**Example Scenario**:
```
User logs in for the first time
→ App checks for user preferences
→ Gets "key not found" 
→ App creates default preferences
→ Everything works normally
```

### 2. "connection refused"

**Full Error**: `network error during dial: connection refused`  
**Error Type**: `network`  
**When You'll See This**: When the SDK can't connect to Birb-Nest

**What This Really Means**:
- It's like calling a phone number and getting "this number is not in service"
- The Birb-Nest service isn't running or isn't accepting connections
- The network path between your app and Birb-Nest is blocked

**Common Causes**:
1. Birb-Nest service is down
2. Wrong URL/port configured
3. Firewall blocking the connection
4. Docker container not running

**How to Fix**:
1. **Check if service is running**:
   ```bash
   docker ps | grep birb-nest
   ```
2. **Verify the URL is correct**:
   - Should be something like: `http://localhost:8080`
   - Check environment variable: `BIRB_NEST_URL`
3. **Test connectivity**:
   ```bash
   curl http://localhost:8080/health
   ```
4. **Restart the service**:
   ```bash
   docker-compose restart birb-nest-api
   ```

### 3. "request timeout"

**Full Error**: `timeout during request`  
**Error Type**: `timeout`  
**When You'll See This**: When a request takes longer than allowed

**What This Really Means**:
- Like waiting for a webpage to load and it never finishes
- The request started but didn't complete in time (usually 30 seconds)
- Either the network is slow or the service is overloaded

**Common Causes**:
1. Network congestion
2. Service processing too many requests
3. Database is slow
4. Large data transfer taking too long

**How to Fix**:
1. **Check network speed**:
   ```bash
   ping -c 5 localhost
   ```
2. **Check service load**:
   ```bash
   docker stats birb-nest-api
   ```
3. **Temporary fix - Increase timeout**:
   - Set environment variable: `REQUEST_TIMEOUT=60s`
4. **If persistent** - Scale the service or investigate performance

### 4. "circuit breaker is open"

**Full Error**: `circuit breaker is open`  
**Error Type**: `circuit_open`  
**When You'll See This**: After multiple consecutive failures

**What This Really Means**:
- The SDK detected the service is having problems
- To protect the system, it stopped trying temporarily
- Like a circuit breaker in your house preventing electrical damage
- This is GOOD - it's protecting your system!

**Common Causes**:
1. Birb-Nest service is down or unstable
2. Network issues causing repeated failures
3. Configuration error causing all requests to fail

**How to Fix**:
1. **Wait** - It will automatically retry after 30 seconds
2. **Check what triggered it**:
   ```bash
   # Look for the original errors
   tail -n 100 /var/log/app.log | grep -B 5 "circuit"
   ```
3. **Fix the root cause** (connection, timeout, etc.)
4. **Monitor recovery**:
   ```bash
   # Watch for "circuit closed" message
   tail -f /var/log/app.log | grep circuit
   ```

**Circuit Breaker States**:
- **Closed**: Normal operation (confusing name, but "closed" = working)
- **Open**: Not allowing requests (protecting the system)
- **Half-Open**: Testing if service recovered

### 5. "rate limited"

**Full Error**: `rate limited` or `API error (status 429): Too Many Requests`  
**Error Type**: `rate_limit`  
**When You'll See This**: When sending too many requests too quickly

**What This Really Means**:
- Like a highway on-ramp meter limiting cars entering
- The service is protecting itself from being overwhelmed
- You're sending more requests than allowed per second

**Common Causes**:
1. Batch job running too fast
2. Multiple apps using same credentials
3. Bug causing request loop
4. Rate limit set too low for your needs

**How to Fix**:
1. **Immediate**: Wait 1 minute and requests will work again
2. **Check current limits**:
   ```bash
   curl http://localhost:8080/rate-limits
   ```
3. **Find who's making requests**:
   ```bash
   docker logs birb-nest-api --tail 1000 | grep -c "client_id"
   ```
4. **Increase limits** (requires approval):
   - Update `RATE_LIMIT_PER_SECOND` in configuration
   - Restart service

### 6. "server error"

**Full Error**: `server error` or `API error (status 500)`  
**Error Type**: `server`  
**When You'll See This**: When Birb-Nest service has internal problems

**What This Really Means**:
- Something went wrong inside the Birb-Nest service
- Not your fault - it's a service problem
- Usually temporary and will self-correct

**Common Causes**:
1. Database connection lost
2. Out of memory
3. Bug in the service code
4. Disk full

**How to Fix**:
1. **Check service logs**:
   ```bash
   docker logs birb-nest-api --tail 50 | grep ERROR
   ```
2. **Check resources**:
   ```bash
   df -h  # Disk space
   free -h  # Memory
   ```
3. **Restart service** (often fixes it):
   ```bash
   docker-compose restart birb-nest-api
   ```
4. **If persistent**: Escalate to development team

### 7. "invalid configuration"

**Full Error**: `invalid configuration`  
**Error Type**: `validation`  
**When You'll See This**: When starting up with wrong settings

**What This Really Means**:
- The service can't start because settings are wrong
- Like trying to start a car with the wrong key
- Need to fix settings before it will work

**Common Causes**:
1. Missing required environment variables
2. Invalid URLs or ports
3. Wrong format for configuration values
4. Typos in configuration files

**How to Fix**:
1. **Check required variables are set**:
   ```bash
   env | grep BIRB_NEST
   ```
2. **Verify configuration format**:
   - URLs must include http:// or https://
   - Ports must be numbers (e.g., 8080)
   - Timeouts need units (e.g., "30s" not just "30")
3. **Common required variables**:
   ```bash
   BIRB_NEST_URL=http://localhost:8080
   BIRB_NEST_TIMEOUT=30s
   BIRB_NEST_MAX_RETRIES=3
   ```

### 8. "invalid response from server"

**Full Error**: `invalid response from server`  
**Error Type**: `unknown`  
**When You'll See This**: When the service returns unexpected data

**What This Really Means**:
- The service responded but with garbage data
- Like ordering pizza and receiving a bicycle
- Usually indicates version mismatch or service problem

**Common Causes**:
1. Wrong service endpoint
2. Version mismatch between SDK and service
3. Proxy or load balancer interference
4. Service returning error pages instead of data

**How to Fix**:
1. **Verify you're hitting the right service**:
   ```bash
   curl -v http://localhost:8080/health
   ```
2. **Check for proxy/load balancer issues**
3. **Verify versions match**:
   ```bash
   # Check service version
   curl http://localhost:8080/version
   ```
4. **Check response format**:
   ```bash
   # Should return JSON
   curl -s http://localhost:8080/api/cache/test | jq .
   ```

### 9. "context canceled"

**Full Error**: `context canceled`  
**Error Type**: `client`  
**When You'll See This**: When a request is cancelled before completing

**What This Really Means**:
- The application stopped waiting for the response
- Like hanging up a phone call before the other person answers
- Usually intentional, not an error

**Common Causes**:
1. User cancelled the operation
2. Application shutting down
3. Request took too long and was cancelled
4. Parent operation was cancelled

**How to Fix**:
- **Usually nothing to fix** - this is normal behavior
- If happening frequently:
  - Check if timeouts are too short
  - Verify the service is responding quickly enough

### 10. "retry budget exhausted"

**Full Error**: `retry budget exhausted`  
**Error Type**: `retry_budget`  
**When You'll See This**: After too many retry attempts

**What This Really Means**:
- The SDK tried multiple times but gave up
- Like calling someone 5 times and they never answer
- Prevents infinite retry loops

**Common Causes**:
1. Service is down for extended period
2. Persistent network issues
3. All retries failed due to same error

**How to Fix**:
1. **Check what error caused the retries**:
   ```bash
   tail -n 200 /var/log/app.log | grep -B 10 "retry"
   ```
2. **Fix the underlying issue** (see specific error type)
3. **Verify service is actually running**
4. **Increase retry budget** (if needed):
   - Set `MAX_RETRIES=5` (default is 3)

## Performance Troubleshooting

### Slow Response Times

**Normal Performance**:
- Cache hit: 1-10 milliseconds
- Cache miss: 10-50 milliseconds  
- First request: 50-100 milliseconds (connection setup)

**If Slower Than Expected**:

1. **Check Cache Hit Rate**:
   ```bash
   curl http://localhost:8080/metrics | grep hit_rate
   ```
   - Should be > 80% for good performance
   - Low hit rate = cache not effective

2. **Check Network Latency**:
   ```bash
   # To Birb-Nest service
   ping -c 10 birb-nest-api
   
   # To Redis
   redis-cli --latency
   ```

3. **Check Connection Pool**:
   ```bash
   netstat -an | grep :8080 | grep ESTABLISHED | wc -l
   ```
   - Too many = possible connection leak
   - Too few = connection pool too small

### High Memory Usage

**Symptoms**:
- Application using more RAM than expected
- Gradual memory growth over time
- Out of memory errors

**Common Causes**:
1. Connection pool too large
2. Caching too much data in SDK
3. Memory leak (rare)

**How to Fix**:
1. **Check current usage**:
   ```bash
   ps aux | grep your-app-name
   ```
2. **Reduce connection pool**:
   - Set `MAX_IDLE_CONNS=50` (default 100)
3. **Restart to clear memory**:
   ```bash
   # Safe operation
   systemctl restart your-app
   ```

## Debug Mode

When nothing else helps, enable debug logging:

1. **Enable debug logs**:
   ```bash
   export BIRB_NEST_DEBUG=true
   export LOG_LEVEL=debug
   # Restart your application
   ```

2. **What to look for**:
   - Request/response details
   - Retry attempts
   - Connection pool events
   - Circuit breaker state changes

3. **Disable when done** (debug logs are verbose):
   ```bash
   unset BIRB_NEST_DEBUG
   export LOG_LEVEL=info
   ```

## Getting Help

If you can't resolve an issue:

1. **Collect Information**:
   ```bash
   # Save debug info
   echo "=== Environment ===" > debug-info.txt
   env | grep BIRB >> debug-info.txt
   echo "=== Service Status ===" >> debug-info.txt
   docker ps | grep birb >> debug-info.txt
   echo "=== Recent Errors ===" >> debug-info.txt
   tail -n 100 /var/log/app.log | grep -i error >> debug-info.txt
   ```

2. **Contact Support** with:
   - The debug-info.txt file
   - What you were trying to do
   - When the problem started
   - What you've already tried

Remember: Most errors are temporary and will resolve themselves. The SDK is designed to handle failures gracefully and recover automatically when possible.
