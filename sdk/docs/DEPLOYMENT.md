# Birb-Nest SDK Deployment Guide

This guide provides step-by-step instructions for deploying applications that use the Birb-Nest SDK. Written for operators with no programming experience.

## Pre-Deployment Checklist

Before you begin, ensure you have:

- [ ] Docker installed (version 20.10+)
- [ ] Docker Compose installed (version 2.0+)
- [ ] 4GB RAM available
- [ ] 10GB disk space available
- [ ] Network access to download Docker images
- [ ] Application package/files
- [ ] Configuration values (URLs, passwords, etc.)

## Deployment Options

Choose your deployment method:

1. **[Docker Compose](#docker-compose)** (Recommended) - All-in-one deployment
2. **[Kubernetes](#kubernetes)** - For cloud/cluster deployments
3. **[Manual Docker](#manual-docker)** - Step-by-step with individual containers

## <a name="docker-compose"></a>Option 1: Docker Compose Deployment (Recommended)

### Step 1: Prepare the Environment

```bash
# Create deployment directory
mkdir -p ~/birb-nest-deployment
cd ~/birb-nest-deployment

# Create required directories
mkdir -p data/redis data/postgres logs backups
```

### Step 2: Create Configuration Files

Create `.env` file:
```bash
cat > .env << 'EOF'
# === REQUIRED SETTINGS ===
# Change these for your environment

# Application Settings
APP_NAME=my-birb-app
APP_PORT=3000
APP_ENV=production

# Birb-Nest Connection
BIRB_NEST_URL=http://birb-nest-api:8080
BIRB_NEST_TIMEOUT=30s

# Database Passwords (CHANGE THESE!)
POSTGRES_PASSWORD=changeme123
REDIS_PASSWORD=changeme456

# === OPTIONAL SETTINGS ===
# These have good defaults

# Performance
BIRB_NEST_MAX_RETRIES=3
BIRB_NEST_MAX_IDLE_CONNS=100

# Logging
LOG_LEVEL=info
LOG_FORMAT=json

# Monitoring
ENABLE_METRICS=true
METRICS_PORT=9090
EOF
```

### Step 3: Create Docker Compose File

Create `docker-compose.yml`:
```yaml
version: '3.8'

services:
  # Your application
  app:
    image: ${APP_IMAGE:-your-app:latest}
    container_name: ${APP_NAME:-birb-app}
    ports:
      - "${APP_PORT:-3000}:3000"
    environment:
      - BIRB_NEST_URL=${BIRB_NEST_URL}
      - BIRB_NEST_TIMEOUT=${BIRB_NEST_TIMEOUT}
      - LOG_LEVEL=${LOG_LEVEL}
    depends_on:
      birb-nest-api:
        condition: service_healthy
    restart: unless-stopped
    healthcheck:
      test: ["CMD", "wget", "--quiet", "--tries=1", "--spider", "http://localhost:3000/health"]
      interval: 30s
      timeout: 10s
      retries: 3
      start_period: 40s
    volumes:
      - ./logs/app:/var/log/app
    networks:
      - birb-net

  # Birb-Nest API
  birb-nest-api:
    image: birbparty/birb-nest:latest
    container_name: birb-nest-api
    ports:
      - "8080:8080"
    environment:
      - REDIS_URL=redis://:${REDIS_PASSWORD}@redis:6379/0
      - DATABASE_URL=postgres://birb_user:${POSTGRES_PASSWORD}@postgres:5432/birb_nest
      - LOG_LEVEL=${LOG_LEVEL}
      - ENABLE_METRICS=${ENABLE_METRICS}
    depends_on:
      redis:
        condition: service_healthy
      postgres:
        condition: service_healthy
    restart: unless-stopped
    healthcheck:
      test: ["CMD", "wget", "--quiet", "--tries=1", "--spider", "http://localhost:8080/health"]
      interval: 30s
      timeout: 10s
      retries: 3
      start_period: 40s
    volumes:
      - ./logs/birb-nest:/var/log/birb-nest
    networks:
      - birb-net

  # Redis Cache
  redis:
    image: redis:7-alpine
    container_name: birb-redis
    command: >
      redis-server
      --requirepass ${REDIS_PASSWORD}
      --appendonly yes
      --appendfsync everysec
    volumes:
      - ./data/redis:/data
    restart: unless-stopped
    healthcheck:
      test: ["CMD", "redis-cli", "--pass", "${REDIS_PASSWORD}", "ping"]
      interval: 30s
      timeout: 10s
      retries: 3
    networks:
      - birb-net

  # PostgreSQL Database
  postgres:
    image: postgres:15-alpine
    container_name: birb-postgres
    environment:
      - POSTGRES_DB=birb_nest
      - POSTGRES_USER=birb_user
      - POSTGRES_PASSWORD=${POSTGRES_PASSWORD}
      - PGDATA=/var/lib/postgresql/data/pgdata
    volumes:
      - ./data/postgres:/var/lib/postgresql/data
      - ./scripts/init-db.sql:/docker-entrypoint-initdb.d/init.sql:ro
    restart: unless-stopped
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U birb_user -d birb_nest"]
      interval: 30s
      timeout: 10s
      retries: 3
    networks:
      - birb-net

networks:
  birb-net:
    driver: bridge
```

### Step 4: Deploy the Application

```bash
# Pull latest images
docker-compose pull

# Start all services
docker-compose up -d

# Watch logs during startup
docker-compose logs -f

# Press Ctrl+C to stop watching logs (services keep running)
```

### Step 5: Verify Deployment

Run this verification script:
```bash
#!/bin/bash
echo "=== Deployment Verification ==="

# Check if containers are running
echo -n "Checking containers... "
RUNNING=$(docker-compose ps --services --filter "status=running" | wc -l)
if [ "$RUNNING" -eq "4" ]; then
    echo "✓ All containers running"
else
    echo "✗ Only $RUNNING/4 containers running"
    docker-compose ps
    exit 1
fi

# Check application health
echo -n "Checking application health... "
APP_HEALTH=$(curl -s -o /dev/null -w "%{http_code}" http://localhost:3000/health)
if [ "$APP_HEALTH" = "200" ]; then
    echo "✓ Application is healthy"
else
    echo "✗ Application health check failed (HTTP $APP_HEALTH)"
fi

# Check Birb-Nest API
echo -n "Checking Birb-Nest API... "
API_HEALTH=$(curl -s -o /dev/null -w "%{http_code}" http://localhost:8080/health)
if [ "$API_HEALTH" = "200" ]; then
    echo "✓ Birb-Nest API is healthy"
else
    echo "✗ Birb-Nest API health check failed (HTTP $API_HEALTH)"
fi

# Check Redis
echo -n "Checking Redis... "
REDIS_CHECK=$(docker exec birb-redis redis-cli --pass ${REDIS_PASSWORD} ping 2>/dev/null)
if [ "$REDIS_CHECK" = "PONG" ]; then
    echo "✓ Redis is responding"
else
    echo "✗ Redis check failed"
fi

# Check PostgreSQL
echo -n "Checking PostgreSQL... "
PG_CHECK=$(docker exec birb-postgres pg_isready -U birb_user 2>/dev/null | grep -c "accepting connections")
if [ "$PG_CHECK" -eq "1" ]; then
    echo "✓ PostgreSQL is accepting connections"
else
    echo "✗ PostgreSQL check failed"
fi

echo ""
echo "=== Deployment Status ==="
if [ "$APP_HEALTH" = "200" ] && [ "$API_HEALTH" = "200" ] && [ "$REDIS_CHECK" = "PONG" ] && [ "$PG_CHECK" -eq "1" ]; then
    echo "✓ DEPLOYMENT SUCCESSFUL!"
    echo ""
    echo "Your application is running at: http://localhost:3000"
    echo "Birb-Nest API is available at: http://localhost:8080"
else
    echo "✗ DEPLOYMENT INCOMPLETE - Check the logs:"
    echo "  docker-compose logs"
fi
```

## <a name="kubernetes"></a>Option 2: Kubernetes Deployment

### Prerequisites

- Kubernetes cluster (1.19+)
- kubectl configured
- Helm 3 (optional but recommended)

### Step 1: Create Namespace

```bash
kubectl create namespace birb-nest
```

### Step 2: Create Secrets

```bash
# Create secrets for passwords
kubectl create secret generic birb-secrets \
  --namespace=birb-nest \
  --from-literal=postgres-password=changeme123 \
  --from-literal=redis-password=changeme456
```

### Step 3: Deploy with Manifests

Create `birb-nest-k8s.yaml`:
```yaml
# ConfigMap for configuration
apiVersion: v1
kind: ConfigMap
metadata:
  name: birb-config
  namespace: birb-nest
data:
  BIRB_NEST_URL: "http://birb-nest-api:8080"
  BIRB_NEST_TIMEOUT: "30s"
  LOG_LEVEL: "info"
---
# Redis Deployment
apiVersion: apps/v1
kind: Deployment
metadata:
  name: redis
  namespace: birb-nest
spec:
  replicas: 1
  selector:
    matchLabels:
      app: redis
  template:
    metadata:
      labels:
        app: redis
    spec:
      containers:
      - name: redis
        image: redis:7-alpine
        command:
          - redis-server
          - --requirepass
          - $(REDIS_PASSWORD)
          - --appendonly
          - yes
        env:
        - name: REDIS_PASSWORD
          valueFrom:
            secretKeyRef:
              name: birb-secrets
              key: redis-password
        ports:
        - containerPort: 6379
        volumeMounts:
        - name: redis-data
          mountPath: /data
      volumes:
      - name: redis-data
        persistentVolumeClaim:
          claimName: redis-pvc
---
# Redis Service
apiVersion: v1
kind: Service
metadata:
  name: redis
  namespace: birb-nest
spec:
  selector:
    app: redis
  ports:
  - port: 6379
    targetPort: 6379
---
# PostgreSQL Deployment
apiVersion: apps/v1
kind: Deployment
metadata:
  name: postgres
  namespace: birb-nest
spec:
  replicas: 1
  selector:
    matchLabels:
      app: postgres
  template:
    metadata:
      labels:
        app: postgres
    spec:
      containers:
      - name: postgres
        image: postgres:15-alpine
        env:
        - name: POSTGRES_DB
          value: birb_nest
        - name: POSTGRES_USER
          value: birb_user
        - name: POSTGRES_PASSWORD
          valueFrom:
            secretKeyRef:
              name: birb-secrets
              key: postgres-password
        ports:
        - containerPort: 5432
        volumeMounts:
        - name: postgres-data
          mountPath: /var/lib/postgresql/data
      volumes:
      - name: postgres-data
        persistentVolumeClaim:
          claimName: postgres-pvc
---
# PostgreSQL Service
apiVersion: v1
kind: Service
metadata:
  name: postgres
  namespace: birb-nest
spec:
  selector:
    app: postgres
  ports:
  - port: 5432
    targetPort: 5432
---
# Birb-Nest API Deployment
apiVersion: apps/v1
kind: Deployment
metadata:
  name: birb-nest-api
  namespace: birb-nest
spec:
  replicas: 2
  selector:
    matchLabels:
      app: birb-nest-api
  template:
    metadata:
      labels:
        app: birb-nest-api
    spec:
      containers:
      - name: birb-nest-api
        image: birbparty/birb-nest:latest
        env:
        - name: REDIS_PASSWORD
          valueFrom:
            secretKeyRef:
              name: birb-secrets
              key: redis-password
        - name: POSTGRES_PASSWORD
          valueFrom:
            secretKeyRef:
              name: birb-secrets
              key: postgres-password
        - name: REDIS_URL
          value: "redis://:$(REDIS_PASSWORD)@redis:6379/0"
        - name: DATABASE_URL
          value: "postgres://birb_user:$(POSTGRES_PASSWORD)@postgres:5432/birb_nest"
        envFrom:
        - configMapRef:
            name: birb-config
        ports:
        - containerPort: 8080
        livenessProbe:
          httpGet:
            path: /health
            port: 8080
          initialDelaySeconds: 30
          periodSeconds: 30
        readinessProbe:
          httpGet:
            path: /ready
            port: 8080
          initialDelaySeconds: 10
          periodSeconds: 10
---
# Birb-Nest API Service
apiVersion: v1
kind: Service
metadata:
  name: birb-nest-api
  namespace: birb-nest
spec:
  selector:
    app: birb-nest-api
  ports:
  - port: 8080
    targetPort: 8080
---
# PersistentVolumeClaims
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: redis-pvc
  namespace: birb-nest
spec:
  accessModes:
  - ReadWriteOnce
  resources:
    requests:
      storage: 5Gi
---
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: postgres-pvc
  namespace: birb-nest
spec:
  accessModes:
  - ReadWriteOnce
  resources:
    requests:
      storage: 10Gi
```

Deploy:
```bash
kubectl apply -f birb-nest-k8s.yaml
```

### Step 4: Verify Kubernetes Deployment

```bash
# Check pods
kubectl get pods -n birb-nest

# Check services
kubectl get svc -n birb-nest

# Check logs
kubectl logs -n birb-nest -l app=birb-nest-api

# Port forward to test
kubectl port-forward -n birb-nest svc/birb-nest-api 8080:8080
```

## <a name="manual-docker"></a>Option 3: Manual Docker Deployment

For when you need step-by-step control:

### Step 1: Create Network

```bash
docker network create birb-net
```

### Step 2: Deploy Redis

```bash
# Create data directory
mkdir -p ~/birb-data/redis

# Run Redis
docker run -d \
  --name birb-redis \
  --network birb-net \
  -v ~/birb-data/redis:/data \
  -e REDIS_PASSWORD=changeme456 \
  redis:7-alpine \
  redis-server --requirepass changeme456 --appendonly yes
```

### Step 3: Deploy PostgreSQL

```bash
# Create data directory
mkdir -p ~/birb-data/postgres

# Run PostgreSQL
docker run -d \
  --name birb-postgres \
  --network birb-net \
  -v ~/birb-data/postgres:/var/lib/postgresql/data \
  -e POSTGRES_DB=birb_nest \
  -e POSTGRES_USER=birb_user \
  -e POSTGRES_PASSWORD=changeme123 \
  postgres:15-alpine
```

### Step 4: Deploy Birb-Nest API

```bash
# Wait for databases to be ready
sleep 30

# Run Birb-Nest API
docker run -d \
  --name birb-nest-api \
  --network birb-net \
  -p 8080:8080 \
  -e REDIS_URL=redis://:changeme456@birb-redis:6379/0 \
  -e DATABASE_URL=postgres://birb_user:changeme123@birb-postgres:5432/birb_nest \
  birbparty/birb-nest:latest
```

### Step 5: Deploy Your Application

```bash
# Run your application
docker run -d \
  --name my-app \
  --network birb-net \
  -p 3000:3000 \
  -e BIRB_NEST_URL=http://birb-nest-api:8080 \
  your-app:latest
```

## Post-Deployment Tasks

### 1. Configure Backups

Create backup script `backup.sh`:
```bash
#!/bin/bash
BACKUP_DIR=~/birb-backups/$(date +%Y%m%d_%H%M%S)
mkdir -p $BACKUP_DIR

echo "Starting backup to $BACKUP_DIR"

# Backup Redis
docker exec birb-redis redis-cli --pass ${REDIS_PASSWORD} BGSAVE
sleep 5
docker cp birb-redis:/data/dump.rdb $BACKUP_DIR/redis-dump.rdb

# Backup PostgreSQL
docker exec birb-postgres pg_dump -U birb_user birb_nest > $BACKUP_DIR/postgres-dump.sql

# Backup configurations
cp .env $BACKUP_DIR/
cp docker-compose.yml $BACKUP_DIR/

echo "Backup completed!"
```

Schedule with cron:
```bash
# Add to crontab
crontab -e

# Add this line for daily backups at 2 AM
0 2 * * * /home/user/birb-nest-deployment/backup.sh
```

### 2. Set Up Monitoring

Create `monitor.sh`:
```bash
#!/bin/bash
while true; do
    clear
    echo "=== Birb-Nest Monitor - $(date) ==="
    echo ""
    
    # Container status
    echo "CONTAINERS:"
    docker ps --format "table {{.Names}}\t{{.Status}}\t{{.Ports}}" | grep birb
    echo ""
    
    # Resource usage
    echo "RESOURCES:"
    docker stats --no-stream --format "table {{.Name}}\t{{.CPUPerc}}\t{{.MemUsage}}" | grep birb
    echo ""
    
    # Health checks
    echo "HEALTH CHECKS:"
    echo -n "App: "
    curl -s http://localhost:3000/health > /dev/null && echo "✓ OK" || echo "✗ FAIL"
    echo -n "API: "
    curl -s http://localhost:8080/health > /dev/null && echo "✓ OK" || echo "✗ FAIL"
    
    sleep 5
done
```

### 3. Configure Alerts

Set up basic email alerts:
```bash
# Install mail utilities
sudo apt-get install -y mailutils

# Create alert script
cat > alert.sh << 'EOF'
#!/bin/bash
RECIPIENT="ops-team@example.com"

# Check services
if ! curl -s http://localhost:3000/health > /dev/null; then
    echo "Application health check failed" | mail -s "ALERT: Birb-Nest App Down" $RECIPIENT
fi

if ! curl -s http://localhost:8080/health > /dev/null; then
    echo "API health check failed" | mail -s "ALERT: Birb-Nest API Down" $RECIPIENT
fi
EOF

# Schedule checks every 5 minutes
crontab -e
# Add: */5 * * * * /home/user/birb-nest-deployment/alert.sh
```

## Rollback Procedures

If deployment fails or causes issues:

### Docker Compose Rollback

```bash
# Stop current deployment
docker-compose down

# Restore previous version
export APP_VERSION=v1.2.3  # Previous version
docker-compose pull
docker-compose up -d
```

### Database Rollback

```bash
# Restore from backup
docker-compose stop app birb-nest-api

# Restore Redis
docker cp backup/redis-dump.rdb birb-redis:/data/dump.rdb
docker-compose restart redis

# Restore PostgreSQL
docker exec -i birb-postgres psql -U birb_user birb_nest < backup/postgres-dump.sql

# Restart services
docker-compose start app birb-nest-api
```

## Deployment Troubleshooting

### Common Deployment Issues

1. **"Cannot connect to Docker daemon"**
   ```bash
   # Start Docker service
   sudo systemctl start docker
   
   # Add user to docker group
   sudo usermod -aG docker $USER
   # Log out and back in
   ```

2. **"Port already in use"**
   ```bash
   # Find what's using the port
   sudo lsof -i :3000
   
   # Change port in .env file
   APP_PORT=3001
   ```

3. **"No space left on device"**
   ```bash
   # Clean up Docker
   docker system prune -a
   
   # Check disk space
   df -h
   ```

4. **Services not starting**
   ```bash
   # Check logs
   docker-compose logs service-name
   
   # Restart in order
   docker-compose restart redis
   sleep 10
   docker-compose restart postgres
   sleep 10
   docker-compose restart birb-nest-api
   sleep 10
   docker-compose restart app
   ```

## Security Hardening

After successful deployment:

1. **Change default passwords** in `.env`
2. **Restrict network access**:
   ```bash
   # Only expose necessary ports
   # Edit docker-compose.yml to bind to localhost only:
   ports:
     - "127.0.0.1:3000:3000"
   ```
3. **Enable firewall**:
   ```bash
   sudo ufw allow 22/tcp  # SSH
   sudo ufw allow 3000/tcp  # App
   sudo ufw enable
   ```

## Next Steps

1. Test the application thoroughly
2. Set up regular backups
3. Configure monitoring alerts
4. Document any custom configurations
5. Train team on operations procedures

For ongoing operations, see:
- [Operations Guide](./OPERATIONS.md)
- [Troubleshooting Guide](./TROUBLESHOOTING.md)
- [On-Call Runbook](./ON_CALL_RUNBOOK.md)
