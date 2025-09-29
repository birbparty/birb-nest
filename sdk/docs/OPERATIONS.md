# Birb-Nest SDK Operations Guide

This guide covers everything you need to deploy, configure, and operate applications using the Birb-Nest SDK. No programming experience required!

## Table of Contents

1. [Quick Start Deployment](#quick-start)
2. [Configuration Guide](#configuration)
3. [Health Monitoring](#health-monitoring)
4. [Common Operations](#common-operations)
5. [Capacity Planning](#capacity-planning)
6. [Security Considerations](#security)

## <a name="quick-start"></a>Quick Start Deployment

### Prerequisites

Before you begin, ensure you have:
- Docker installed (version 20.10 or higher)
- Docker Compose installed (version 2.0 or higher)
- At least 4GB of free RAM
- 10GB of free disk space

### Step 1: Get the Application

```bash
# Clone or download your application
# (Replace with your actual repository)
git clone https://github.com/your-org/your-app.git
cd your-app
```

### Step 2: Configure Environment

Create a `.env` file with these settings:

```bash
# Birb-Nest Connection
BIRB_NEST_URL=http://birb-nest-api:8080
BIRB_NEST_TIMEOUT=30s
BIRB_NEST_MAX_RETRIES=3

# Connection Pool Settings
BIRB_NEST_MAX_IDLE_CONNS=100
BIRB_NEST_MAX_CONNS_PER_HOST=10
BIRB_NEST_IDLE_TIMEOUT=90s

# Circuit Breaker Settings
BIRB_NEST_CIRCUIT_THRESHOLD=5
BIRB_NEST_CIRCUIT_TIMEOUT=30s

# Logging
LOG_LEVEL=info
LOG_FORMAT=json

# Application Settings
APP_PORT=3000
APP_ENV=production
```

### Step 3: Deploy with Docker Compose

Create a `docker-compose.yml` file:

```yaml
version: '3.8'

services:
  # Your application using Birb-Nest SDK
  app:
    image: your-app:latest
    ports:
      - "3000:3000"
    environment:
      - BIRB_NEST_URL=http://birb-nest-api:8080
      - LOG_LEVEL=info
    depends_on:
      - birb-nest-api
    restart: unless-stopped
    healthcheck:
      test: ["CMD", "curl", "-f", "http://localhost:3000/health"]
      interval: 30s
      timeout: 10s
      retries: 3

  # Birb-Nest API Service
  birb-nest-api:
    image: birbparty/birb-nest:latest
    ports:
      - "8080:8080"
    environment:
      - REDIS_URL=redis://redis:6379
      - DATABASE_URL=postgres://birb_user:birb_pass@postgres:5432/birb_nest
    depends_on:
      - redis
      - postgres
    restart: unless-stopped

  # Redis for caching
  redis:
    image: redis:7-alpine
    volumes:
      - redis_data:/data
    restart: unless-stopped
    command: redis-server --appendonly yes

  # PostgreSQL for persistence
  postgres:
    image: postgres:15-alpine
    environment:
      - POSTGRES_DB=birb_nest
      - POSTGRES_USER=birb_user
      - POSTGRES_PASSWORD=birb_pass
    volumes:
      - postgres_data:/var/lib/postgresql/data
    restart: unless-stopped

volumes:
  redis_data:
  postgres_data:
```

### Step 4: Start Everything

```bash
# Start all services
docker-compose up -d

# Wait for services to be ready (about 30 seconds)
sleep 30

# Verify everything is running
docker-compose ps

# Check health
curl http://localhost:3000/health
curl http://localhost:8080/health
```

## <a name="configuration"></a>Configuration Guide

### Essential Configuration

These are the minimum settings required:

| Setting | Description | Default | Example |
|---------|-------------|---------|---------|
| `BIRB_NEST_URL` | Birb-Nest API endpoint | - | `http://localhost:8080` |
| `BIRB_NEST_TIMEOUT` | Request timeout | `30s` | `60s` |
| `BIRB_NEST_MAX_RETRIES` | Retry attempts | `3` | `5` |

### Connection Pool Configuration

Controls how connections are managed:

| Setting | Description | Default | When to Change |
|---------|-------------|---------|----------------|
| `BIRB_NEST_MAX_IDLE_CONNS` | Max idle connections | `100` | Reduce if memory limited |
| `BIRB_NEST_MAX_CONNS_PER_HOST` | Max connections per host | `10` | Increase for high traffic |
| `BIRB_NEST_IDLE_TIMEOUT` | Idle connection timeout | `90s` | Reduce for dynamic IPs |

### Circuit Breaker Configuration

Protects against cascading failures:

| Setting | Description | Default | When to Change |
|---------|-------------|---------|----------------|
| `BIRB_NEST_CIRCUIT_THRESHOLD` | Failures before opening | `5` | Lower for critical apps |
| `BIRB_NEST_CIRCUIT_TIMEOUT` | Time before retry | `30s` | Increase for unstable networks |

### Performance Tuning

For different workloads:

**High Traffic Applications**:
```bash
BIRB_NEST_MAX_IDLE_CONNS=200
BIRB_NEST_MAX_CONNS_PER_HOST=50
BIRB_NEST_TIMEOUT=10s
```

**Low Latency Requirements**:
```bash
BIRB_NEST_MAX_RETRIES=1
BIRB_NEST_TIMEOUT=5s
BIRB_NEST_CIRCUIT_THRESHOLD=3
```

**Unstable Networks**:
```bash
BIRB_NEST_MAX_RETRIES=5
BIRB_NEST_TIMEOUT=60s
BIRB_NEST_CIRCUIT_TIMEOUT=60s
```

## <a name="health-monitoring"></a>Health Monitoring

### Health Check Endpoints

Your application should expose:
- `/health` - Basic health check
- `/ready` - Readiness check (can serve traffic)
- `/metrics` - Prometheus metrics (optional)

### What to Monitor

1. **Application Health**:
   ```bash
   # Basic health check
   curl http://localhost:3000/health
   
   # Expected response:
   # {"status":"healthy","version":"1.0.0"}
   ```

2. **Birb-Nest Connection**:
   ```bash
   # Check SDK can reach Birb-Nest
   curl http://localhost:3000/ready
   
   # Expected response:
   # {"ready":true,"birb_nest":"connected"}
   ```

3. **Key Metrics**:
   - Request rate
   - Error rate
   - Response time
   - Cache hit rate
   - Connection pool usage

### Setting Up Monitoring

1. **Simple Monitoring Script**:
   ```bash
   #!/bin/bash
   # Save as monitor.sh
   
   while true; do
     echo "=== Health Check $(date) ==="
     
     # Check app health
     APP_STATUS=$(curl -s -o /dev/null -w "%{http_code}" http://localhost:3000/health)
     echo "App Status: $APP_STATUS"
     
     # Check Birb-Nest health
     BIRB_STATUS=$(curl -s -o /dev/null -w "%{http_code}" http://localhost:8080/health)
     echo "Birb-Nest Status: $BIRB_STATUS"
     
     # Check Redis
     REDIS_STATUS=$(docker exec your-app_redis_1 redis-cli ping 2>/dev/null || echo "FAIL")
     echo "Redis: $REDIS_STATUS"
     
     # Alert if any service is down
     if [ "$APP_STATUS" != "200" ] || [ "$BIRB_STATUS" != "200" ] || [ "$REDIS_STATUS" != "PONG" ]; then
       echo "⚠️  ALERT: Service degradation detected!"
       # Add your alert mechanism here (email, Slack, etc.)
     fi
     
     sleep 60
   done
   ```

2. **Docker Healthchecks**:
   Already included in the docker-compose.yml above

3. **Prometheus Metrics** (if available):
   ```yaml
   # prometheus.yml
   scrape_configs:
     - job_name: 'birb-nest-app'
       static_configs:
         - targets: ['localhost:3000']
   ```

## <a name="common-operations"></a>Common Operations

### Starting and Stopping

**Start all services**:
```bash
docker-compose up -d
```

**Stop all services**:
```bash
docker-compose stop
```

**Restart a specific service**:
```bash
docker-compose restart app
```

**Complete shutdown**:
```bash
docker-compose down
```

### Viewing Logs

**All services**:
```bash
docker-compose logs -f
```

**Specific service**:
```bash
docker-compose logs -f app
```

**Last 100 lines**:
```bash
docker-compose logs --tail=100 app
```

**Errors only**:
```bash
docker-compose logs app 2>&1 | grep -i error
```

### Scaling Services

**Scale horizontally**:
```bash
# Run 3 instances of your app
docker-compose up -d --scale app=3
```

**Behind a load balancer**:
```yaml
# Add to docker-compose.yml
nginx:
  image: nginx:alpine
  ports:
    - "80:80"
  volumes:
    - ./nginx.conf:/etc/nginx/nginx.conf
  depends_on:
    - app
```

### Backup and Recovery

**Backup Redis data**:
```bash
# Create backup
docker exec your-app_redis_1 redis-cli BGSAVE
docker cp your-app_redis_1:/data/dump.rdb ./backup/redis-$(date +%Y%m%d).rdb
```

**Backup Postgres data**:
```bash
# Create backup
docker exec your-app_postgres_1 pg_dump -U birb_user birb_nest > ./backup/postgres-$(date +%Y%m%d).sql
```

**Restore from backup**:
```bash
# Restore Redis
docker cp ./backup/redis-20240101.rdb your-app_redis_1:/data/dump.rdb
docker-compose restart redis

# Restore Postgres
docker exec -i your-app_postgres_1 psql -U birb_user birb_nest < ./backup/postgres-20240101.sql
```

### Maintenance Mode

**Enable maintenance mode**:
```bash
# Set environment variable
docker-compose exec app sh -c 'export MAINTENANCE_MODE=true'

# Or update .env and restart
echo "MAINTENANCE_MODE=true" >> .env
docker-compose up -d
```

## <a name="capacity-planning"></a>Capacity Planning

### Resource Requirements

**Minimum Requirements**:
- CPU: 2 cores
- RAM: 4GB
- Disk: 10GB
- Network: 100Mbps

**Recommended for Production**:
- CPU: 4+ cores
- RAM: 8GB+
- Disk: 50GB+ (SSD preferred)
- Network: 1Gbps

### Sizing Guide

| Users | App Instances | Redis Memory | Postgres Storage |
|-------|---------------|--------------|------------------|
| < 100 | 1 | 1GB | 5GB |
| 100-1000 | 2 | 2GB | 20GB |
| 1000-10000 | 4 | 4GB | 50GB |
| 10000+ | 8+ | 8GB+ | 100GB+ |

### Performance Expectations

**Single Instance Performance**:
- Requests per second: 1000-5000
- Average latency: 10-50ms
- Cache hit rate: 80-95%
- Concurrent connections: 100-500

### Scaling Strategies

1. **Vertical Scaling** (bigger machines):
   - Easier to manage
   - Good for < 10,000 users
   - Limited by hardware

2. **Horizontal Scaling** (more machines):
   - Better for high availability
   - No upper limit
   - Requires load balancer

3. **Cache Optimization**:
   - Increase Redis memory
   - Tune eviction policies
   - Monitor hit rates

## <a name="security"></a>Security Considerations

### Network Security

1. **Use Internal Networks**:
   ```yaml
   # docker-compose.yml
   networks:
     internal:
       driver: bridge
       internal: true
   ```

2. **Limit Port Exposure**:
   - Only expose necessary ports
   - Use reverse proxy for public access
   - Enable firewalls

3. **Enable TLS/SSL**:
   ```bash
   # For production, use HTTPS
   BIRB_NEST_URL=https://birb-nest-api:8443
   ```

### Authentication and Authorization

1. **API Keys**:
   ```bash
   # Set in environment
   BIRB_NEST_API_KEY=your-secret-key-here
   ```

2. **Rotate Credentials**:
   - Change passwords regularly
   - Use secrets management
   - Monitor access logs

### Data Protection

1. **Encryption at Rest**:
   - Enable for Redis
   - Enable for Postgres
   - Encrypt backups

2. **Encryption in Transit**:
   - Use TLS between services
   - Validate certificates
   - Disable weak ciphers

### Security Checklist

- [ ] Change default passwords
- [ ] Enable authentication on Redis
- [ ] Use strong Postgres passwords
- [ ] Limit network exposure
- [ ] Enable audit logging
- [ ] Regular security updates
- [ ] Monitor for suspicious activity
- [ ] Backup encryption keys

## Troubleshooting Quick Reference

| Problem | Check This First | See Also |
|---------|------------------|----------|
| Can't connect | Is Birb-Nest running? | [Troubleshooting Guide](./TROUBLESHOOTING.md) |
| Slow performance | Check cache hit rate | [Performance Section](#capacity-planning) |
| High memory usage | Check connection pool | [Configuration](#configuration) |
| Errors in logs | Check error type | [Error Dictionary](./TROUBLESHOOTING.md) |
| Service won't start | Check configuration | [Quick Start](#quick-start) |

## Getting Help

1. **Check Documentation**:
   - [Troubleshooting Guide](./TROUBLESHOOTING.md)
   - [On-Call Runbook](./ON_CALL_RUNBOOK.md)

2. **Collect Diagnostics**:
   ```bash
   # Run diagnostic script
   ./scripts/collect-diagnostics.sh
   ```

3. **Contact Support**:
   - Slack: #birb-nest-support
   - Email: support@example.com

Remember: Most issues are configuration-related. Double-check your settings before escalating!
