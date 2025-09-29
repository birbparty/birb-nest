# Birb-Nest Helm Chart

This Helm chart deploys Birb-Nest with instance isolation support, allowing you to run a primary instance with PostgreSQL persistence and multiple replica instances with local Redis caches.

## Architecture

```
Primary Instance                      Replica Instances
┌─────────────────────────┐          ┌─────────────────────────┐
│         API             │◀─────────│         API             │
│  - Writes to Redis      │  Write   │  - R/W local Redis      │
│  - Async to PostgreSQL  │  Forward │  - Forward writes to    │
│    via channels         │          │    primary              │
├─────────────────────────┤          ├─────────────────────────┤
│        Redis            │          │        Redis            │
├─────────────────────────┤          └─────────────────────────┘
│      PostgreSQL         │                      │
└─────────────────────────┘                      │
            ▲                                    │
            └──────── HTTP GET on cache miss ────┘
```

## Prerequisites

- Kubernetes 1.19+
- Helm 3.0+
- PV provisioner support in the underlying infrastructure (for persistence)

## Installation

### Add Helm Repository Dependencies

```bash
helm repo add bitnami https://charts.bitnami.com/bitnami
helm repo update
```

### Deploy Primary Instance

```bash
helm install birb-nest-primary ./charts/birb-nest \
  --namespace birb-nest \
  --create-namespace
```

### Deploy Replica Instance

```bash
# Using example values file
helm install birb-nest-game123 ./charts/birb-nest \
  --namespace birb-nest \
  -f ./charts/birb-nest/values-replica-example.yaml

# Or using individual parameters
helm install birb-nest-game456 ./charts/birb-nest \
  --namespace birb-nest \
  --set mode=replica \
  --set instanceId=game456 \
  --set postgresql.enabled=false \
  --set replica.primaryUrl=http://birb-nest-primary:8080
```

## Configuration

### Key Parameters

| Parameter | Description | Default |
|-----------|-------------|---------|
| `mode` | Deployment mode: "primary" or "replica" | `primary` |
| `instanceId` | Unique instance identifier | `""` |
| `replica.primaryUrl` | URL of primary instance (replicas only) | `http://birb-nest-primary:8080` |
| `primary.writeQueueSize` | Async write queue size (primary only) | `10000` |
| `primary.writeWorkers` | Number of async workers (primary only) | `5` |
| `networkPolicy.enabled` | Enable network policy | `true` |
| `networkPolicy.clientLabels` | Labels required for client access | `birb-nest-client: "true"` |

### Redis Configuration

| Parameter | Description | Default |
|-----------|-------------|---------|
| `redis.enabled` | Enable Redis subchart | `true` |
| `redis.master.persistence.enabled` | Enable Redis persistence | `true` |
| `redis.master.persistence.size` | Redis PVC size | `8Gi` |

### PostgreSQL Configuration (Primary Only)

| Parameter | Description | Default |
|-----------|-------------|---------|
| `postgresql.enabled` | Enable PostgreSQL subchart | `true` |
| `postgresql.auth.database` | Database name | `birbnest` |
| `postgresql.primary.persistence.size` | PostgreSQL PVC size | `10Gi` |
| `postgresql.backup.enabled` | Enable backup CronJob | `false` |
| `postgresql.backup.schedule` | Backup schedule | `0 2 * * *` |

## Client Access

To allow pods to access birb-nest, add the required label:

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: my-game-server
spec:
  template:
    metadata:
      labels:
        app: my-game-server
        birb-nest-client: "true"  # Required for network policy
```

## Monitoring

The chart exposes Prometheus metrics at `/metrics`:

- Request duration histogram
- Cache hit/miss rate
- Async queue depth (primary)
- Write forward success rate (replica)
- Health status gauge

## Backup and Recovery

### Enable Backups

```bash
helm upgrade birb-nest-primary ./charts/birb-nest \
  --set postgresql.backup.enabled=true \
  --set postgresql.backup.schedule="0 2 * * *" \
  --set postgresql.backup.retention=7
```

### Manual Backup

```bash
kubectl create job --from=cronjob/birb-nest-primary-backup manual-backup-$(date +%s)
```

### Restore from Backup

```bash
# List backups
kubectl exec -it birb-nest-primary-backup-pvc-pod -- ls /backup

# Restore specific backup
kubectl exec -it birb-nest-primary-postgresql-0 -- bash
PGPASSWORD=$POSTGRES_PASSWORD psql -U postgres -d birbnest < /backup/birbnest_20240125_020000.sql.gz
```

## Troubleshooting

### Check Instance Mode

```bash
kubectl exec -it <pod-name> -- env | grep MODE
```

### View Async Queue Stats (Primary)

```bash
kubectl exec -it <primary-pod> -- curl localhost:8080/metrics | jq .async_writer
```

### Test Connectivity

```bash
# From replica to primary
kubectl exec -it <replica-pod> -- curl http://birb-nest-primary:8080/health
```

### View Logs

```bash
# All pods
kubectl logs -n birb-nest -l app=birb-nest --tail=100

# Specific instance
kubectl logs -n birb-nest -l instance=game123 --tail=100
```

## Uninstallation

```bash
# Remove replica
helm uninstall birb-nest-game123 -n birb-nest

# Remove primary (will delete all data)
helm uninstall birb-nest-primary -n birb-nest

# Clean up namespace
kubectl delete namespace birb-nest
```

## Advanced Configuration

### Custom Network Policy

To restrict access to specific namespaces:

```yaml
networkPolicy:
  enabled: true
  allowedNamespaces:
    - game-servers
    - api-gateway
  clientLabels:
    authorized: "true"
```

### Resource Tuning

For high-traffic replicas:

```yaml
api:
  resources:
    requests:
      cpu: 500m
      memory: 512Mi
    limits:
      cpu: 1000m
      memory: 1Gi

redis:
  master:
    resources:
      requests:
        cpu: 200m
        memory: 256Mi
```

### Affinity Rules

To ensure primary and replicas run on different nodes:

```yaml
affinity:
  podAntiAffinity:
    preferredDuringSchedulingIgnoredDuringExecution:
    - weight: 100
      podAffinityTerm:
        labelSelector:
          matchExpressions:
          - key: app
            operator: In
            values:
            - birb-nest
        topologyKey: kubernetes.io/hostname
