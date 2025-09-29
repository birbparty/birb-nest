# Birb-Nest SDK Monitoring Guide

This guide explains how to monitor applications using the Birb-Nest SDK, set up alerts, and understand metrics. No programming knowledge required!

## Quick Start Monitoring

### What You Need to Monitor

| Component | What to Watch | Alert When |
|-----------|---------------|------------|
| Application Health | `/health` endpoint | Returns non-200 status |
| API Health | Birb-Nest API status | Connection failures |
| Response Time | Average latency | > 100ms consistently |
| Error Rate | Failed requests | > 5% of requests |
| Cache Hit Rate | Successful cache reads | < 80% hit rate |
| Memory Usage | Container RAM | > 80% of limit |

## Setting Up Basic Monitoring

### Option 1: Simple Shell Script Monitoring

Create `simple-monitor.sh`:
```bash
#!/bin/bash
# Simple monitoring script - runs every minute

LOG_FILE="/var/log/birb-monitor.log"
ALERT_EMAIL="ops-team@example.com"

# Function to log with timestamp
log_message() {
    echo "[$(date '+%Y-%m-%d %H:%M:%S')] $1" >> $LOG_FILE
}

# Check application health
check_app_health() {
    STATUS=$(curl -s -o /dev/null -w "%{http_code}" http://localhost:3000/health)
    if [ "$STATUS" -ne "200" ]; then
        log_message "ERROR: Application health check failed (HTTP $STATUS)"
        echo "Application is DOWN - HTTP Status: $STATUS" | mail -s "ALERT: Birb App Down" $ALERT_EMAIL
        return 1
    fi
    log_message "INFO: Application health check passed"
    return 0
}

# Check response time
check_response_time() {
    RESPONSE_TIME=$(curl -s -o /dev/null -w "%{time_total}" http://localhost:3000/health)
    # Convert to milliseconds
    RESPONSE_MS=$(echo "$RESPONSE_TIME * 1000" | bc | cut -d'.' -f1)
    
    if [ "$RESPONSE_MS" -gt "100" ]; then
        log_message "WARN: Slow response time: ${RESPONSE_MS}ms"
        if [ "$RESPONSE_MS" -gt "500" ]; then
            echo "Response time critical: ${RESPONSE_MS}ms" | mail -s "ALERT: Birb App Slow" $ALERT_EMAIL
        fi
    else
        log_message "INFO: Response time OK: ${RESPONSE_MS}ms"
    fi
}

# Check memory usage
check_memory() {
    # Get container memory usage
    MEMORY_PERCENT=$(docker stats --no-stream --format "{{.MemPerc}}" your-app-container | sed 's/%//')
    
    if (( $(echo "$MEMORY_PERCENT > 80" | bc -l) )); then
        log_message "ERROR: High memory usage: ${MEMORY_PERCENT}%"
        echo "Memory usage critical: ${MEMORY_PERCENT}%" | mail -s "ALERT: High Memory Usage" $ALERT_EMAIL
    else
        log_message "INFO: Memory usage OK: ${MEMORY_PERCENT}%"
    fi
}

# Main monitoring loop
main() {
    log_message "Starting monitoring check"
    
    check_app_health
    check_response_time
    check_memory
    
    log_message "Monitoring check complete"
}

# Run the main function
main
```

Schedule with cron:
```bash
# Run every minute
* * * * * /path/to/simple-monitor.sh
```

### Option 2: Docker-based Monitoring Stack

Create `monitoring-stack.yml`:
```yaml
version: '3.8'

services:
  # Prometheus for metrics collection
  prometheus:
    image: prom/prometheus:latest
    container_name: birb-prometheus
    ports:
      - "9090:9090"
    volumes:
      - ./prometheus.yml:/etc/prometheus/prometheus.yml
      - prometheus_data:/prometheus
    command:
      - '--config.file=/etc/prometheus/prometheus.yml'
      - '--storage.tsdb.path=/prometheus'
      - '--web.console.libraries=/usr/share/prometheus/console_libraries'
      - '--web.console.templates=/usr/share/prometheus/consoles'
    restart: unless-stopped

  # Grafana for visualization
  grafana:
    image: grafana/grafana:latest
    container_name: birb-grafana
    ports:
      - "3001:3000"
    environment:
      - GF_SECURITY_ADMIN_PASSWORD=admin123
      - GF_USERS_ALLOW_SIGN_UP=false
    volumes:
      - grafana_data:/var/lib/grafana
      - ./grafana-dashboards:/etc/grafana/provisioning/dashboards
      - ./grafana-datasources.yml:/etc/grafana/provisioning/datasources/datasources.yml
    restart: unless-stopped

  # AlertManager for alerts
  alertmanager:
    image: prom/alertmanager:latest
    container_name: birb-alertmanager
    ports:
      - "9093:9093"
    volumes:
      - ./alertmanager.yml:/etc/alertmanager/alertmanager.yml
      - alertmanager_data:/alertmanager
    restart: unless-stopped

volumes:
  prometheus_data:
  grafana_data:
  alertmanager_data:
```

## Prometheus Configuration

Create `prometheus.yml`:
```yaml
global:
  scrape_interval: 15s
  evaluation_interval: 15s

# Alerting configuration
alerting:
  alertmanagers:
    - static_configs:
        - targets:
          - alertmanager:9093

# Load rules
rule_files:
  - "alerts.yml"

# Scrape configurations
scrape_configs:
  # Your application metrics
  - job_name: 'birb-app'
    static_configs:
      - targets: ['your-app:3000']
    metrics_path: '/metrics'

  # Birb-Nest API metrics
  - job_name: 'birb-nest-api'
    static_configs:
      - targets: ['birb-nest-api:8080']
    metrics_path: '/metrics'

  # Docker metrics
  - job_name: 'docker'
    static_configs:
      - targets: ['docker-host:9323']
```

## Alert Rules

Create `alerts.yml`:
```yaml
groups:
  - name: birb_alerts
    interval: 30s
    rules:
      # Application Down
      - alert: ApplicationDown
        expr: up{job="birb-app"} == 0
        for: 2m
        labels:
          severity: critical
          service: birb-app
        annotations:
          summary: "Application is down"
          description: "{{ $labels.instance }} has been down for more than 2 minutes."

      # High Error Rate
      - alert: HighErrorRate
        expr: rate(http_requests_total{status=~"5.."}[5m]) > 0.05
        for: 5m
        labels:
          severity: warning
          service: birb-app
        annotations:
          summary: "High error rate detected"
          description: "Error rate is {{ $value | humanizePercentage }} over the last 5 minutes."

      # Slow Response Time
      - alert: SlowResponseTime
        expr: histogram_quantile(0.95, http_request_duration_seconds_bucket) > 0.1
        for: 5m
        labels:
          severity: warning
          service: birb-app
        annotations:
          summary: "Slow response times"
          description: "95th percentile response time is {{ $value }}s"

      # Low Cache Hit Rate
      - alert: LowCacheHitRate
        expr: (rate(cache_hits_total[5m]) / rate(cache_requests_total[5m])) < 0.8
        for: 10m
        labels:
          severity: warning
          service: birb-cache
        annotations:
          summary: "Cache hit rate below 80%"
          description: "Cache hit rate is {{ $value | humanizePercentage }}"

      # High Memory Usage
      - alert: HighMemoryUsage
        expr: container_memory_usage_bytes / container_spec_memory_limit_bytes > 0.8
        for: 5m
        labels:
          severity: warning
        annotations:
          summary: "High memory usage"
          description: "Container {{ $labels.container }} memory usage is {{ $value | humanizePercentage }}"

      # Database Connection Issues
      - alert: DatabaseConnectionFailure
        expr: pg_up == 0 or redis_up == 0
        for: 1m
        labels:
          severity: critical
        annotations:
          summary: "Database connection failure"
          description: "Cannot connect to {{ $labels.database_type }}"
```

## AlertManager Configuration

Create `alertmanager.yml`:
```yaml
global:
  # Email configuration
  smtp_smarthost: 'smtp.gmail.com:587'
  smtp_from: 'alerts@example.com'
  smtp_auth_username: 'alerts@example.com'
  smtp_auth_password: 'your-app-password'

# Route tree
route:
  group_by: ['alertname', 'service']
  group_wait: 10s
  group_interval: 10s
  repeat_interval: 1h
  receiver: 'team-email'
  
  routes:
  # Critical alerts
  - match:
      severity: critical
    receiver: 'pagerduty'
    repeat_interval: 5m
  
  # Warning alerts
  - match:
      severity: warning
    receiver: 'team-email'
    repeat_interval: 30m

# Receivers
receivers:
- name: 'team-email'
  email_configs:
  - to: 'ops-team@example.com'
    headers:
      Subject: 'Birb-Nest Alert: {{ .GroupLabels.alertname }}'

- name: 'pagerduty'
  pagerduty_configs:
  - service_key: 'your-pagerduty-key'
    description: '{{ .GroupLabels.alertname }}: {{ .CommonAnnotations.summary }}'
```

## Grafana Dashboard

Create `grafana-dashboard.json`:
```json
{
  "dashboard": {
    "title": "Birb-Nest SDK Monitoring",
    "panels": [
      {
        "title": "Request Rate",
        "targets": [
          {
            "expr": "rate(http_requests_total[5m])",
            "legendFormat": "{{ method }} {{ status }}"
          }
        ],
        "gridPos": {"h": 8, "w": 12, "x": 0, "y": 0}
      },
      {
        "title": "Response Time (95th percentile)",
        "targets": [
          {
            "expr": "histogram_quantile(0.95, http_request_duration_seconds_bucket)",
            "legendFormat": "95th percentile"
          }
        ],
        "gridPos": {"h": 8, "w": 12, "x": 12, "y": 0}
      },
      {
        "title": "Cache Hit Rate",
        "targets": [
          {
            "expr": "rate(cache_hits_total[5m]) / rate(cache_requests_total[5m]) * 100",
            "legendFormat": "Hit Rate %"
          }
        ],
        "gridPos": {"h": 8, "w": 12, "x": 0, "y": 8}
      },
      {
        "title": "Memory Usage",
        "targets": [
          {
            "expr": "container_memory_usage_bytes / 1024 / 1024",
            "legendFormat": "{{ container }} (MB)"
          }
        ],
        "gridPos": {"h": 8, "w": 12, "x": 12, "y": 8}
      }
    ]
  }
}
```

## Key Metrics Explained

### Application Metrics

| Metric | What It Means | Good Value | Action if Bad |
|--------|---------------|------------|---------------|
| `up` | Is the service running? | 1 | Check if service crashed |
| `http_requests_total` | Total requests handled | Increasing | If flat, check if app is receiving traffic |
| `http_request_duration_seconds` | How long requests take | < 0.1s | Investigate slow queries/operations |
| `error_rate` | Percentage of failed requests | < 1% | Check logs for error patterns |

### Cache Metrics

| Metric | What It Means | Good Value | Action if Bad |
|--------|---------------|------------|---------------|
| `cache_hits_total` | Successful cache reads | High | Normal if app just started |
| `cache_misses_total` | Cache reads that found nothing | Low | Check if cache is being populated |
| `cache_hit_rate` | Hits / (Hits + Misses) | > 80% | Review caching strategy |
| `cache_evictions_total` | Items removed from cache | Low | May need more cache memory |

### System Metrics

| Metric | What It Means | Good Value | Action if Bad |
|--------|---------------|------------|---------------|
| `memory_percent` | RAM usage | < 80% | Restart or scale up |
| `cpu_percent` | CPU usage | < 70% | Optimize code or scale up |
| `disk_usage_percent` | Disk space used | < 80% | Clean logs or add storage |
| `network_errors` | Network problems | 0 | Check network configuration |

## Understanding Monitoring Data

### Reading Prometheus Queries

Common queries explained:

1. **Request Rate**:
   ```
   rate(http_requests_total[5m])
   ```
   - Shows requests per second over last 5 minutes
   - Higher = more traffic
   - Sudden drop = possible issue

2. **Error Rate**:
   ```
   rate(http_requests_total{status=~"5.."}[5m]) / rate(http_requests_total[5m])
   ```
   - Shows percentage of requests that failed
   - Should be close to 0
   - Spike = something is wrong

3. **Response Time**:
   ```
   histogram_quantile(0.95, http_request_duration_seconds_bucket)
   ```
   - 95% of requests are faster than this
   - Lower is better
   - Increase = performance problem

### Interpreting Grafana Dashboards

**Request Rate Graph**:
- Steady line = consistent traffic
- Spikes = traffic bursts
- Flatline at 0 = no traffic (problem!)

**Response Time Graph**:
- Flat low line = good performance
- Gradual increase = growing load
- Sudden spike = performance issue

**Cache Hit Rate Graph**:
- Above 80% = cache working well
- Below 50% = cache not effective
- 0% = cache might be disabled

## Monitoring Best Practices

### 1. Set Realistic Thresholds

Don't alert on every small issue:
- Response time: Alert at 2x normal
- Error rate: Alert at 5% not 1%
- Memory: Alert at 80% not 50%

### 2. Monitor Business Metrics

Not just technical metrics:
- User logins per minute
- API calls per customer
- Cache value (money saved)

### 3. Create Runbooks for Alerts

For each alert, document:
- What it means
- How to investigate
- How to fix
- Who to escalate to

### 4. Regular Review

Weekly tasks:
- Review false alarms
- Adjust thresholds
- Check dashboard usage
- Update documentation

## Troubleshooting Monitoring

### Prometheus Not Scraping

1. Check target is reachable:
   ```bash
   curl http://your-app:3000/metrics
   ```

2. Check Prometheus targets:
   - Visit http://localhost:9090/targets
   - Look for "DOWN" targets

3. Check network connectivity:
   ```bash
   docker network ls
   docker network inspect birb-net
   ```

### Grafana Not Showing Data

1. Check data source:
   - Settings → Data Sources
   - Test connection

2. Check time range:
   - Top right corner
   - Set to "Last 5 minutes"

3. Check query:
   - Edit panel
   - Run query directly

### Alerts Not Firing

1. Check AlertManager:
   - Visit http://localhost:9093
   - Look for silenced alerts

2. Check email configuration:
   ```bash
   docker logs birb-alertmanager
   ```

3. Test alert:
   ```bash
   # Stop a service to trigger alert
   docker stop your-app
   # Wait 2 minutes
   # Should receive alert
   ```

## Quick Reference Card

Print and keep handy:

```
=== MONITORING QUICK REFERENCE ===

URLs:
- Prometheus: http://localhost:9090
- Grafana: http://localhost:3001 (admin/admin123)
- AlertManager: http://localhost:9093

Key Queries:
- Is app up: up{job="birb-app"}
- Error rate: rate(http_requests_total{status=~"5.."}[5m])
- Response time: histogram_quantile(0.95, http_request_duration_seconds_bucket)
- Cache hit rate: rate(cache_hits_total[5m]) / rate(cache_requests_total[5m])

Quick Checks:
- All green in Grafana = Good
- Red panels = Check that metric
- No data = Check Prometheus targets
- Too many alerts = Adjust thresholds

Emergency:
- Grafana down: Check docker ps
- No metrics: Restart Prometheus
- No alerts: Check AlertManager logs
- Everything down: docker-compose restart
```

## Next Steps

1. Start with simple monitoring
2. Add Prometheus when comfortable
3. Customize dashboards for your needs
4. Create runbooks for each alert
5. Train team on reading dashboards

For more details, see:
- [Operations Guide](./OPERATIONS.md)
- [Troubleshooting Guide](./TROUBLESHOOTING.md)
- [Logging Guide](./LOGGING.md)
