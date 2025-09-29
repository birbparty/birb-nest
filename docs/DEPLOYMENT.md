# 🚀 Birb Nest Deployment Guide

## Table of Contents
- [Overview](#overview)
- [Prerequisites](#prerequisites)
- [Docker Deployment](#docker-deployment)
- [Kubernetes Deployment](#kubernetes-deployment)
- [Cloud Deployments](#cloud-deployments)
  - [AWS Deployment](#aws-deployment)
  - [Google Cloud Platform](#google-cloud-platform)
  - [Azure Deployment](#azure-deployment)
- [SSL/TLS Configuration](#ssltls-configuration)
- [Load Balancing](#load-balancing)
- [Monitoring Setup](#monitoring-setup)
- [Backup Strategies](#backup-strategies)
- [Scaling Guidelines](#scaling-guidelines)
- [Security Hardening](#security-hardening)
- [Troubleshooting](#troubleshooting)

## Overview

Birb Nest is designed for cloud-native deployment with support for containerization, orchestration, and auto-scaling. This guide covers deployment options from simple Docker Compose to production-grade Kubernetes clusters.

## Prerequisites

### Required Tools
- Docker 20.10+ and Docker Compose v2.0+
- kubectl 1.24+ (for Kubernetes)
- Helm 3.0+ (for Kubernetes)
- Cloud provider CLI (AWS CLI, gcloud, az)

### System Requirements
- **Minimum (Development)**:
  - 4 CPU cores
  - 8GB RAM
  - 20GB storage

- **Recommended (Production)**:
  - 8+ CPU cores
  - 16GB+ RAM
  - 100GB+ SSD storage
  - Network bandwidth: 1Gbps+

## Docker Deployment

### Quick Start with Docker Compose

1. **Clone the repository**:
```bash
git clone https://github.com/birbparty/birb-nest.git
cd birb-nest
```

2. **Configure environment**:
```bash
cp .env.example .env
# Edit .env with your configuration
```

3. **Start services**:
```bash
# Production mode
docker-compose up -d

# Development mode with hot reload
docker-compose -f docker-compose.yml -f docker-compose.dev.yml up
```

4. **Verify deployment**:
```bash
# Check service health
curl http://localhost:8080/health

# View logs
docker-compose logs -f
```

### Production Docker Configuration

```yaml
# docker-compose.prod.yml
version: '3.8'

services:
  api:
    image: ghcr.io/birbparty/birb-nest-api:latest
    deploy:
      replicas: 3
      resources:
        limits:
          cpus: '2'
          memory: 2G
        reservations:
          cpus: '1'
          memory: 1G
      restart_policy:
        condition: any
        delay: 5s
        max_attempts: 3

  worker:
    image: ghcr.io/birbparty/birb-nest-worker:latest
    deploy:
      replicas: 5
      resources:
        limits:
          cpus: '1'
          memory: 1G
        reservations:
          cpus: '0.5'
          memory: 512M

  nginx:
    image: nginx:alpine
    ports:
      - "80:80"
      - "443:443"
    volumes:
      - ./nginx.conf:/etc/nginx/nginx.conf:ro
      - ./certs:/etc/nginx/certs:ro
    depends_on:
      - api
```

## Kubernetes Deployment

### Helm Chart Installation

1. **Add Helm repository**:
```bash
helm repo add birbparty https://charts.birbparty.com
helm repo update
```

2. **Create values file**:
```yaml
# values.yaml
global:
  storageClass: gp3

postgresql:
  enabled: true
  auth:
    postgresPassword: "secure-password"
    database: birbcache
  primary:
    persistence:
      size: 100Gi
  metrics:
    enabled: true

redis:
  enabled: true
  auth:
    enabled: true
    password: "secure-redis-password"
  master:
    persistence:
      size: 10Gi
  metrics:
    enabled: true

api:
  replicaCount: 3
  image:
    repository: ghcr.io/birbparty/birb-nest-api
    tag: latest
  resources:
    requests:
      cpu: 500m
      memory: 512Mi
    limits:
      cpu: 2000m
      memory: 2Gi
  autoscaling:
    enabled: true
    minReplicas: 3
    maxReplicas: 10
    targetCPUUtilizationPercentage: 70

worker:
  replicaCount: 5
  image:
    repository: ghcr.io/birbparty/birb-nest-worker
    tag: latest
  resources:
    requests:
      cpu: 250m
      memory: 256Mi
    limits:
      cpu: 1000m
      memory: 1Gi
  autoscaling:
    enabled: true
    minReplicas: 5
    maxReplicas: 20
    targetMemoryUtilizationPercentage: 80

ingress:
  enabled: true
  className: nginx
  annotations:
    cert-manager.io/cluster-issuer: letsencrypt-prod
  hosts:
    - host: api.birbparty.com
      paths:
        - path: /
          pathType: Prefix
  tls:
    - secretName: birb-nest-tls
      hosts:
        - api.birbparty.com
```

3. **Deploy with Helm**:
```bash
helm install birb-nest birbparty/birb-nest \
  -f values.yaml \
  --namespace birb-nest \
  --create-namespace
```

### Manual Kubernetes Deployment

```yaml
# namespace.yaml
apiVersion: v1
kind: Namespace
metadata:
  name: birb-nest

---
# configmap.yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: birb-nest-config
  namespace: birb-nest
data:
  LOG_LEVEL: "info"
  API_PORT: "8080"
  WORKER_BATCH_SIZE: "100"
  WORKER_BATCH_TIMEOUT: "1s"

---
# secret.yaml
apiVersion: v1
kind: Secret
metadata:
  name: birb-nest-secrets
  namespace: birb-nest
type: Opaque
stringData:
  POSTGRES_PASSWORD: "your-secure-password"
  REDIS_PASSWORD: "your-redis-password"
  NATS_PASSWORD: "your-nats-password"

---
# api-deployment.yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: birb-nest-api
  namespace: birb-nest
spec:
  replicas: 3
  selector:
    matchLabels:
      app: birb-nest-api
  template:
    metadata:
      labels:
        app: birb-nest-api
    spec:
      containers:
      - name: api
        image: ghcr.io/birbparty/birb-nest-api:latest
        ports:
        - containerPort: 8080
        env:
        - name: POSTGRES_HOST
          value: postgres-service
        - name: REDIS_HOST
          value: redis-service
        - name: NATS_URL
          value: nats://nats-service:4222
        envFrom:
        - configMapRef:
            name: birb-nest-config
        - secretRef:
            name: birb-nest-secrets
        livenessProbe:
          httpGet:
            path: /health
            port: 8080
          initialDelaySeconds: 30
          periodSeconds: 10
        readinessProbe:
          httpGet:
            path: /health
            port: 8080
          initialDelaySeconds: 5
          periodSeconds: 5
        resources:
          requests:
            cpu: 500m
            memory: 512Mi
          limits:
            cpu: 2000m
            memory: 2Gi

---
# api-service.yaml
apiVersion: v1
kind: Service
metadata:
  name: birb-nest-api
  namespace: birb-nest
spec:
  selector:
    app: birb-nest-api
  ports:
  - protocol: TCP
    port: 80
    targetPort: 8080
  type: LoadBalancer
```

## Cloud Deployments

### AWS Deployment

#### Using ECS Fargate

1. **Create task definition**:
```json
{
  "family": "birb-nest",
  "networkMode": "awsvpc",
  "requiresCompatibilities": ["FARGATE"],
  "cpu": "2048",
  "memory": "4096",
  "containerDefinitions": [
    {
      "name": "api",
      "image": "ghcr.io/birbparty/birb-nest-api:latest",
      "portMappings": [
        {
          "containerPort": 8080,
          "protocol": "tcp"
        }
      ],
      "environment": [
        {"name": "POSTGRES_HOST", "value": "birb-rds.region.rds.amazonaws.com"},
        {"name": "REDIS_HOST", "value": "birb-redis.region.cache.amazonaws.com"}
      ],
      "secrets": [
        {
          "name": "POSTGRES_PASSWORD",
          "valueFrom": "arn:aws:secretsmanager:region:account:secret:birb-nest/db"
        }
      ],
      "logConfiguration": {
        "logDriver": "awslogs",
        "options": {
          "awslogs-group": "/ecs/birb-nest",
          "awslogs-region": "us-east-1",
          "awslogs-stream-prefix": "ecs"
        }
      }
    }
  ]
}
```

2. **Create ECS service**:
```bash
aws ecs create-service \
  --cluster birb-nest-cluster \
  --service-name birb-nest-api \
  --task-definition birb-nest:1 \
  --desired-count 3 \
  --launch-type FARGATE \
  --network-configuration "awsvpcConfiguration={subnets=[subnet-xxx],securityGroups=[sg-xxx],assignPublicIp=ENABLED}"
```

#### Using EKS

```bash
# Create EKS cluster
eksctl create cluster \
  --name birb-nest \
  --region us-east-1 \
  --nodegroup-name standard-workers \
  --node-type t3.medium \
  --nodes 3 \
  --nodes-min 1 \
  --nodes-max 5 \
  --managed

# Install AWS Load Balancer Controller
kubectl apply -k "github.com/aws/eks-charts/stable/aws-load-balancer-controller/crds"
helm install aws-load-balancer-controller eks/aws-load-balancer-controller \
  -n kube-system \
  --set clusterName=birb-nest

# Deploy Birb Nest
helm install birb-nest ./charts/birb-nest \
  --set postgresql.enabled=false \
  --set postgresql.external.host=birb-rds.region.rds.amazonaws.com \
  --set redis.enabled=false \
  --set redis.external.host=birb-redis.region.cache.amazonaws.com
```

### Google Cloud Platform

#### Using Cloud Run

```bash
# Build and push image
gcloud builds submit --tag gcr.io/PROJECT-ID/birb-nest-api

# Deploy to Cloud Run
gcloud run deploy birb-nest-api \
  --image gcr.io/PROJECT-ID/birb-nest-api \
  --platform managed \
  --region us-central1 \
  --allow-unauthenticated \
  --set-env-vars "POSTGRES_HOST=10.x.x.x" \
  --set-env-vars "REDIS_HOST=10.x.x.x" \
  --set-secrets "POSTGRES_PASSWORD=birb-db-pass:latest" \
  --vpc-connector birb-vpc-connector \
  --min-instances 1 \
  --max-instances 10
```

#### Using GKE

```bash
# Create GKE cluster
gcloud container clusters create birb-nest \
  --zone us-central1-a \
  --num-nodes 3 \
  --enable-autoscaling \
  --min-nodes 3 \
  --max-nodes 10 \
  --enable-autorepair \
  --enable-autoupgrade

# Connect to cluster
gcloud container clusters get-credentials birb-nest --zone us-central1-a

# Deploy using Helm
helm install birb-nest ./charts/birb-nest \
  --set global.storageClass=standard-rwo
```

### Azure Deployment

#### Using Container Instances

```bash
# Create resource group
az group create --name birb-nest-rg --location eastus

# Create container instance
az container create \
  --resource-group birb-nest-rg \
  --name birb-nest-api \
  --image ghcr.io/birbparty/birb-nest-api:latest \
  --cpu 2 \
  --memory 4 \
  --ports 8080 \
  --environment-variables \
    POSTGRES_HOST=birb-postgres.postgres.database.azure.com \
    REDIS_HOST=birb-redis.redis.cache.windows.net \
  --secure-environment-variables \
    POSTGRES_PASSWORD=$DB_PASSWORD
```

#### Using AKS

```bash
# Create AKS cluster
az aks create \
  --resource-group birb-nest-rg \
  --name birb-nest-aks \
  --node-count 3 \
  --enable-addons monitoring \
  --generate-ssh-keys \
  --node-vm-size Standard_DS2_v2

# Get credentials
az aks get-credentials --resource-group birb-nest-rg --name birb-nest-aks

# Deploy
helm install birb-nest ./charts/birb-nest
```

## SSL/TLS Configuration

### Using Let's Encrypt with cert-manager

1. **Install cert-manager**:
```bash
kubectl apply -f https://github.com/cert-manager/cert-manager/releases/download/v1.12.0/cert-manager.yaml
```

2. **Create ClusterIssuer**:
```yaml
apiVersion: cert-manager.io/v1
kind: ClusterIssuer
metadata:
  name: letsencrypt-prod
spec:
  acme:
    server: https://acme-v02.api.letsencrypt.org/directory
    email: admin@birbparty.com
    privateKeySecretRef:
      name: letsencrypt-prod
    solvers:
    - http01:
        ingress:
          class: nginx
```

3. **Configure Ingress**:
```yaml
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: birb-nest-ingress
  annotations:
    cert-manager.io/cluster-issuer: letsencrypt-prod
    nginx.ingress.kubernetes.io/ssl-redirect: "true"
spec:
  tls:
  - hosts:
    - api.birbparty.com
    secretName: birb-nest-tls
  rules:
  - host: api.birbparty.com
    http:
      paths:
      - path: /
        pathType: Prefix
        backend:
          service:
            name: birb-nest-api
            port:
              number: 80
```

## Load Balancing

### NGINX Configuration

```nginx
# nginx.conf
upstream birb_api {
    least_conn;
    server api1:8080 max_fails=3 fail_timeout=30s;
    server api2:8080 max_fails=3 fail_timeout=30s;
    server api3:8080 max_fails=3 fail_timeout=30s;
}

server {
    listen 80;
    listen 443 ssl http2;
    server_name api.birbparty.com;

    ssl_certificate /etc/nginx/certs/fullchain.pem;
    ssl_certificate_key /etc/nginx/certs/privkey.pem;
    ssl_protocols TLSv1.2 TLSv1.3;
    ssl_ciphers HIGH:!aNULL:!MD5;

    # Rate limiting
    limit_req_zone $binary_remote_addr zone=api:10m rate=100r/m;
    limit_req zone=api burst=20 nodelay;

    location / {
        proxy_pass http://birb_api;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
        
        # Timeouts
        proxy_connect_timeout 5s;
        proxy_send_timeout 60s;
        proxy_read_timeout 60s;
    }

    location /health {
        access_log off;
        proxy_pass http://birb_api/health;
    }
}
```

### HAProxy Configuration

```
# haproxy.cfg
global
    maxconn 4096
    log stdout local0

defaults
    mode http
    timeout connect 5000ms
    timeout client 50000ms
    timeout server 50000ms
    option httplog

frontend birb_frontend
    bind *:80
    bind *:443 ssl crt /etc/haproxy/certs/birb-nest.pem
    redirect scheme https if !{ ssl_fc }
    
    # Rate limiting
    stick-table type ip size 100k expire 30s store http_req_rate(10s)
    http-request track-sc0 src
    http-request deny if { sc_http_req_rate(0) gt 20 }
    
    default_backend birb_backend

backend birb_backend
    balance leastconn
    option httpchk GET /health
    
    server api1 api1:8080 check inter 2000 rise 2 fall 3
    server api2 api2:8080 check inter 2000 rise 2 fall 3
    server api3 api3:8080 check inter 2000 rise 2 fall 3
```

## Monitoring Setup

### Prometheus Configuration

```yaml
# prometheus-values.yaml
prometheus:
  prometheusSpec:
    serviceMonitorSelectorNilUsesHelmValues: false
    podMonitorSelectorNilUsesHelmValues: false
    ruleSelectorNilUsesHelmValues: false
    
    additionalScrapeConfigs:
    - job_name: 'birb-nest-api'
      kubernetes_sd_configs:
      - role: pod
        namespaces:
          names:
          - birb-nest
      relabel_configs:
      - source_labels: [__meta_kubernetes_pod_label_app]
        action: keep
        regex: birb-nest-api
      - source_labels: [__meta_kubernetes_pod_name]
        target_label: pod
      - source_labels: [__meta_kubernetes_namespace]
        target_label: namespace
```

### Grafana Dashboard

```json
{
  "dashboard": {
    "title": "Birb Nest Monitoring",
    "panels": [
      {
        "title": "Cache Hit Rate",
        "targets": [
          {
            "expr": "rate(birb_cache_hits_total[5m]) / (rate(birb_cache_hits_total[5m]) + rate(birb_cache_misses_total[5m]))"
          }
        ]
      },
      {
        "title": "API Request Rate",
        "targets": [
          {
            "expr": "rate(http_requests_total{job=\"birb-nest-api\"}[5m])"
          }
        ]
      },
      {
        "title": "Queue Depth",
        "targets": [
          {
            "expr": "birb_queue_depth{queue=\"persistence\"}"
          }
        ]
      }
    ]
  }
}
```

## Backup Strategies

### PostgreSQL Backup

```bash
#!/bin/bash
# backup-postgres.sh

# Configuration
BACKUP_DIR="/backups/postgres"
RETENTION_DAYS=7
TIMESTAMP=$(date +%Y%m%d_%H%M%S)

# Create backup
pg_dump -h $POSTGRES_HOST -U $POSTGRES_USER -d $POSTGRES_DB | \
  gzip > "$BACKUP_DIR/birb-nest-$TIMESTAMP.sql.gz"

# Upload to S3
aws s3 cp "$BACKUP_DIR/birb-nest-$TIMESTAMP.sql.gz" \
  s3://birb-backups/postgres/

# Clean old backups
find $BACKUP_DIR -name "*.sql.gz" -mtime +$RETENTION_DAYS -delete
```

### Redis Backup

```yaml
# cronjob.yaml
apiVersion: batch/v1
kind: CronJob
metadata:
  name: redis-backup
spec:
  schedule: "0 2 * * *"
  jobTemplate:
    spec:
      template:
        spec:
          containers:
          - name: backup
            image: redis:7-alpine
            command:
            - /bin/sh
            - -c
            - |
              redis-cli -h redis-service --rdb /backup/dump.rdb
              aws s3 cp /backup/dump.rdb s3://birb-backups/redis/dump-$(date +%Y%m%d).rdb
            volumeMounts:
            - name: backup
              mountPath: /backup
          volumes:
          - name: backup
            emptyDir: {}
          restartPolicy: OnFailure
```

## Scaling Guidelines

### Horizontal Scaling

```yaml
# hpa.yaml
apiVersion: autoscaling/v2
kind: HorizontalPodAutoscaler
metadata:
  name: birb-nest-api-hpa
spec:
  scaleTargetRef:
    apiVersion: apps/v1
    kind: Deployment
    name: birb-nest-api
  minReplicas: 3
  maxReplicas: 20
  metrics:
  - type: Resource
    resource:
      name: cpu
      target:
        type: Utilization
        averageUtilization: 70
  - type: Resource
    resource:
      name: memory
      target:
        type: Utilization
        averageUtilization: 80
  - type: Pods
    pods:
      metric:
        name: http_requests_per_second
      target:
        type: AverageValue
        averageValue: "1000"
```

### Vertical Scaling

```yaml
# vpa.yaml
apiVersion: autoscaling.k8s.io/v1
kind: VerticalPodAutoscaler
metadata:
  name: birb-nest-api-vpa
spec:
  targetRef:
    apiVersion: apps/v1
    kind: Deployment
    name: birb-nest-api
  updatePolicy:
    updateMode: "Auto"
  resourcePolicy:
    containerPolicies:
    - containerName: api
      minAllowed:
        cpu: 500m
        memory: 512Mi
      maxAllowed:
        cpu: 4
        memory: 8Gi
```

## Security Hardening

### Network Policies

```yaml
# network-policy.yaml
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: birb-nest-network-policy
spec:
  podSelector:
    matchLabels:
      app: birb-nest-api
  policyTypes:
  - Ingress
  - Egress
  ingress:
  - from:
    - namespaceSelector:
        matchLabels:
          name: ingress-nginx
    ports:
    - protocol: TCP
      port: 8080
  egress:
  - to:
    - podSelector:
        matchLabels:
          app: postgres
    ports:
    - protocol: TCP
      port: 5432
  - to:
    - podSelector:
        matchLabels:
          app: redis
    ports:
    - protocol: TCP
      port: 6379
```

### Pod Security Standards

```yaml
# pod-security.yaml
apiVersion: v1
kind: Namespace
metadata:
  name: birb-nest
  labels:
    pod-security.kubernetes.io/enforce: restricted
    pod-security.kubernetes.io/audit: restricted
    pod-security.kubernetes.io/warn: restricted
```

## Troubleshooting

### Common Issues

1. **Service won't start**
   ```bash
   # Check logs
   kubectl logs -n birb-nest deployment/birb-nest-api
   
   # Check events
   kubectl get events -n birb-nest --sort-by='.lastTimestamp'
   ```

2. **Database connection issues**
   ```bash
   # Test connectivity
   kubectl exec -it deployment/birb-nest-api -- nc -zv postgres-service 5432
   
   # Check secrets
   kubectl get secret birb-nest-secrets -o yaml
   ```

3. **High memory usage**
   ```bash
   # Check resource usage
   kubectl top pods -n birb-nest
   
   # Get pod details
   kubectl describe pod -n birb-nest
   ```

### Health Check Endpoints

- **API Health**: `GET /health`
- **Prometheus Metrics**: `GET /metrics`
- **NATS Monitoring**: `http://nats-service:8222/`
- **Redis Info**: `redis-cli INFO`

### Rollback Procedure

```bash
# Kubernetes rollback
kubectl rollout undo deployment/birb-nest-api -n birb-nest

# Helm rollback
helm rollback birb-nest 1 -n birb-nest

# Docker Compose rollback
docker-compose down
git checkout tags/v1.0.0
docker-compose up -d
```

---

🐦 Ready to deploy? Let's get those birbs flying in production! 🚀
