# Instance Context API Documentation

## Overview

The birb-nest API supports multi-tenant isolation through instance contexts. Each request can specify an instance ID, and all data operations will be scoped to that instance.

## Instance Context Headers

### X-Instance-ID Header

The primary method for specifying an instance context is through the `X-Instance-ID` header:

```bash
curl -X GET http://localhost:8080/v1/cache/mykey \
  -H "X-Instance-ID: game-instance-123"
```

### Query Parameter Alternative

You can also specify the instance ID as a query parameter:

```bash
curl -X GET http://localhost:8080/v1/cache/mykey?instance_id=game-instance-123
```

## Instance Lifecycle

### Instance Creation

Instances are automatically created when first accessed. The default configuration includes:

- **Status**: Active
- **GameType**: default
- **Region**: default
- **ResourceQuota**: 8GB memory, 100GB storage, 4 CPU cores

### Instance States

- **active**: Instance is running and accepting requests
- **inactive**: Instance is not currently running
- **paused**: Instance is temporarily paused
- **migrating**: Instance is being migrated
- **deleting**: Instance is being deleted

Only instances in `active` or `migrating` states can accept requests.

## API Endpoints

### Cache Operations

All cache operations require an instance context:

#### Set Value
```bash
PUT /v1/cache/{key}
Headers:
  X-Instance-ID: <instance-id>
  Content-Type: application/octet-stream
Body: <raw-value>
```

#### Get Value
```bash
GET /v1/cache/{key}
Headers:
  X-Instance-ID: <instance-id>
```

#### Delete Value
```bash
DELETE /v1/cache/{key}
Headers:
  X-Instance-ID: <instance-id>
```

#### Batch Get
```bash
POST /v1/cache/batch/get
Headers:
  X-Instance-ID: <instance-id> (optional)
  Content-Type: application/json
Body:
{
  "keys": ["key1", "key2", "key3"]
}
```

### Health Check

Health checks do not require instance context:

```bash
GET /health
```

Response includes instance information if provided:
```json
{
  "status": "healthy",
  "mode": "primary",
  "instance_id": "default",
  "timestamp": 1735238400
}
```

## Error Responses

### Missing Instance ID
```json
{
  "error": "instance ID is required",
  "code": "MISSING_INSTANCE_ID"
}
```

### Instance Not Found
```json
{
  "error": "instance not found",
  "code": "INSTANCE_NOT_FOUND"
}
```

### Instance Not Active
```json
{
  "error": "instance is inactive",
  "code": "INSTANCE_INACTIVE",
  "instance_id": "game-instance-123",
  "status": "inactive"
}
```

## Data Isolation

- Each instance has its own isolated namespace for cache keys
- Data stored by one instance cannot be accessed by another
- Keys are internally prefixed with the instance ID

## Activity Tracking

- Instance last active time is automatically updated on each request
- Updates are throttled to once per minute to reduce overhead
- Activity tracking is performed asynchronously

## Example Usage

### Create and Use an Instance

```bash
# First request creates the instance automatically
curl -X PUT http://localhost:8080/v1/cache/player-score \
  -H "X-Instance-ID: minecraft-server-001" \
  -H "Content-Type: application/octet-stream" \
  -d "1500"

# Retrieve the value
curl -X GET http://localhost:8080/v1/cache/player-score \
  -H "X-Instance-ID: minecraft-server-001"
# Output: 1500

# Different instance cannot see the data
curl -X GET http://localhost:8080/v1/cache/player-score \
  -H "X-Instance-ID: minecraft-server-002"
# Output: 404 Not Found
```

### Batch Operations with Instance Context

```bash
# Store multiple values
curl -X PUT http://localhost:8080/v1/cache/player1 \
  -H "X-Instance-ID: game-001" \
  -d "Alice"

curl -X PUT http://localhost:8080/v1/cache/player2 \
  -H "X-Instance-ID: game-001" \
  -d "Bob"

# Batch retrieve
curl -X POST http://localhost:8080/v1/cache/batch/get \
  -H "X-Instance-ID: game-001" \
  -H "Content-Type: application/json" \
  -d '{"keys": ["player1", "player2", "player3"]}'

# Response:
{
  "entries": {
    "player1": "Alice",
    "player2": "Bob"
  },
  "missing": ["player3"]
}
