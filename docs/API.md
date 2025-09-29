# 📚 Birb Nest API Reference

## Table of Contents
- [Overview](#overview)
- [Base URL](#base-url)
- [Authentication](#authentication)
- [Rate Limiting](#rate-limiting)
- [Request Format](#request-format)
- [Response Format](#response-format)
- [Error Handling](#error-handling)
- [Endpoints](#endpoints)
  - [Cache Operations](#cache-operations)
  - [Batch Operations](#batch-operations)
  - [Health & Monitoring](#health--monitoring)
- [Examples](#examples)
- [Postman Collection](#postman-collection)

## Overview

The Birb Nest API provides a simple, RESTful interface for interacting with the distributed caching service. All responses are in JSON format, and the API follows standard HTTP status codes.

## Base URL

```
http://localhost:8080
```

In production, replace with your deployment URL.

## Authentication

Authentication is optional and configured via the `API_KEY` environment variable. When enabled, include the API key in the request header:

```
X-API-Key: your-api-key-here
```

## Rate Limiting

Default rate limit: **100 requests per minute** per IP address.

When rate limited, the API returns:
- Status Code: `429 Too Many Requests`
- Headers: `X-RateLimit-Limit`, `X-RateLimit-Remaining`, `X-RateLimit-Reset`

## Request Format

All POST/PUT requests must include:
- `Content-Type: application/json` header
- Valid JSON body

## Response Format

All responses return JSON with appropriate HTTP status codes:
- `200 OK` - Successful GET request
- `201 Created` - Successful POST request
- `204 No Content` - Successful DELETE request
- `400 Bad Request` - Invalid request format
- `404 Not Found` - Resource not found
- `429 Too Many Requests` - Rate limit exceeded
- `500 Internal Server Error` - Server error

## Error Handling

Error responses follow a consistent format:

```json
{
  "error": "Human-readable error message",
  "code": "ERROR_CODE",
  "details": "Additional context (optional)"
}
```

### Error Codes

| Code | Description |
|------|-------------|
| `NOT_FOUND` | The requested resource was not found |
| `INVALID_REQUEST` | The request format or parameters are invalid |
| `INTERNAL_ERROR` | An internal server error occurred |
| `VERSION_MISMATCH` | Optimistic locking version conflict |
| `TIMEOUT` | Operation timed out |
| `RATE_LIMITED` | Rate limit exceeded |

## Endpoints

### Cache Operations

#### Create/Update Cache Entry

```
POST /v1/cache/:key
PUT /v1/cache/:key
```

Creates a new cache entry or updates an existing one.

**Parameters:**
- `key` (path parameter, required): The cache key (alphanumeric, hyphens, underscores, dots allowed)

**Request Body:**
```json
{
  "value": {},
  "ttl": 3600,
  "metadata": {
    "source": "api",
    "user_id": "12345"
  }
}
```

**Fields:**
- `value` (required): The value to cache (can be any valid JSON)
- `ttl` (optional): Time-to-live in seconds (if not specified, uses default or no expiration)
- `metadata` (optional): Additional metadata as key-value pairs

**Response:**
```json
{
  "key": "user:12345",
  "value": {
    "id": "12345",
    "name": "John Doe",
    "email": "john@example.com"
  },
  "version": 1,
  "ttl": 3600,
  "metadata": {
    "source": "api",
    "user_id": "12345"
  },
  "created_at": "2025-05-27T20:00:00Z",
  "updated_at": "2025-05-27T20:00:00Z"
}
```

**Example:**
```bash
curl -X POST http://localhost:8080/v1/cache/user:12345 \
  -H "Content-Type: application/json" \
  -d '{
    "value": {
      "id": "12345",
      "name": "John Doe",
      "email": "john@example.com"
    },
    "ttl": 3600,
    "metadata": {
      "source": "api"
    }
  }'
```

#### Get Cache Entry

```
GET /v1/cache/:key
```

Retrieves a cache entry by key.

**Parameters:**
- `key` (path parameter, required): The cache key to retrieve

**Response:**
```json
{
  "key": "user:12345",
  "value": {
    "id": "12345",
    "name": "John Doe",
    "email": "john@example.com"
  },
  "version": 2,
  "ttl": 3600,
  "metadata": {
    "source": "api",
    "user_id": "12345"
  },
  "created_at": "2025-05-27T20:00:00Z",
  "updated_at": "2025-05-27T20:05:00Z"
}
```

**Cache Miss Behavior:**
1. First checks Redis (cache hit → return immediately)
2. If not in Redis, checks PostgreSQL
3. If found in PostgreSQL, returns value and asynchronously rehydrates to Redis
4. If not found anywhere, returns 404 and triggers rehydration attempt

**Example:**
```bash
curl http://localhost:8080/v1/cache/user:12345
```

#### Delete Cache Entry

```
DELETE /v1/cache/:key
```

Deletes a cache entry from both Redis and PostgreSQL.

**Parameters:**
- `key` (path parameter, required): The cache key to delete

**Response:**
- Status: `204 No Content` on success
- Status: `404 Not Found` if key doesn't exist

**Example:**
```bash
curl -X DELETE http://localhost:8080/v1/cache/user:12345
```

### Batch Operations

#### Batch Get

```
POST /v1/cache/batch/get
```

Retrieves multiple cache entries in a single request.

**Request Body:**
```json
{
  "keys": ["user:12345", "user:67890", "user:11111"]
}
```

**Fields:**
- `keys` (required): Array of keys to retrieve (max 100 keys)

**Response:**
```json
{
  "entries": {
    "user:12345": {
      "key": "user:12345",
      "value": {
        "id": "12345",
        "name": "John Doe"
      },
      "version": 1,
      "created_at": "2025-05-27T20:00:00Z",
      "updated_at": "2025-05-27T20:00:00Z"
    },
    "user:67890": {
      "key": "user:67890",
      "value": {
        "id": "67890",
        "name": "Jane Smith"
      },
      "version": 2,
      "created_at": "2025-05-27T19:00:00Z",
      "updated_at": "2025-05-27T20:00:00Z"
    }
  },
  "missing": ["user:11111"]
}
```

**Example:**
```bash
curl -X POST http://localhost:8080/v1/cache/batch/get \
  -H "Content-Type: application/json" \
  -d '{
    "keys": ["user:12345", "user:67890", "user:11111"]
  }'
```

### Health & Monitoring

#### Health Check

```
GET /health
```

Returns the health status of the service and its dependencies.

**Response:**
```json
{
  "status": "healthy",
  "service": "birb-nest-api",
  "version": "1.0.0",
  "uptime": "2h15m30s",
  "checks": {
    "database": "healthy",
    "redis": "healthy",
    "nats": "healthy"
  }
}
```

**Status Codes:**
- `200 OK` - All components healthy
- `503 Service Unavailable` - One or more components unhealthy

**Example:**
```bash
curl http://localhost:8080/health
```

#### Metrics

```
GET /metrics
```

Returns cache performance metrics.

**Response:**
```json
{
  "cache_hits": 45678,
  "cache_misses": 1234,
  "cache_hit_rate": 0.9737,
  "total_requests": 46912,
  "total_errors": 23,
  "average_latency_ms": 12.5
}
```

**Example:**
```bash
curl http://localhost:8080/metrics
```

#### Service Info

```
GET /
```

Returns basic service information and available endpoints.

**Response:**
```json
{
  "service": "birb-nest-api",
  "version": "1.0.0",
  "status": "running",
  "endpoints": {
    "cache": {
      "get": "GET /v1/cache/:key",
      "create": "POST /v1/cache/:key",
      "update": "PUT /v1/cache/:key",
      "delete": "DELETE /v1/cache/:key",
      "batch": "POST /v1/cache/batch/get"
    },
    "health": "GET /health",
    "metrics": "GET /metrics"
  }
}
```

## Examples

### Store User Profile

```bash
# Store a user profile with 1-hour TTL
curl -X POST http://localhost:8080/v1/cache/user:profile:12345 \
  -H "Content-Type: application/json" \
  -d '{
    "value": {
      "id": "12345",
      "username": "birdlover",
      "email": "bird@example.com",
      "preferences": {
        "theme": "dark",
        "notifications": true
      }
    },
    "ttl": 3600,
    "metadata": {
      "source": "user-service",
      "version": "v2"
    }
  }'
```

### Cache Session Data

```bash
# Store session data with 30-minute TTL
curl -X POST http://localhost:8080/v1/cache/session:abc123 \
  -H "Content-Type: application/json" \
  -d '{
    "value": {
      "user_id": "12345",
      "roles": ["user", "admin"],
      "expires_at": "2025-05-27T21:00:00Z"
    },
    "ttl": 1800
  }'
```

### Store Configuration

```bash
# Store application configuration (no TTL)
curl -X POST http://localhost:8080/v1/cache/config:app:features \
  -H "Content-Type: application/json" \
  -d '{
    "value": {
      "feature_flags": {
        "new_ui": true,
        "beta_features": false,
        "maintenance_mode": false
      },
      "limits": {
        "max_upload_size": 10485760,
        "rate_limit": 1000
      }
    },
    "metadata": {
      "updated_by": "admin",
      "environment": "production"
    }
  }'
```

### Batch Retrieve Multiple Users

```bash
# Get multiple user profiles at once
curl -X POST http://localhost:8080/v1/cache/batch/get \
  -H "Content-Type: application/json" \
  -d '{
    "keys": [
      "user:profile:12345",
      "user:profile:67890",
      "user:profile:11111"
    ]
  }'
```

### Check Service Health

```bash
# Full health check
curl http://localhost:8080/health

# Quick health check (just status code)
curl -f -s http://localhost:8080/health > /dev/null && echo "Healthy" || echo "Unhealthy"
```

### Monitor Performance

```bash
# Get current metrics
curl http://localhost:8080/metrics | jq .

# Watch cache hit rate
watch -n 1 'curl -s http://localhost:8080/metrics | jq .cache_hit_rate'
```

## Postman Collection

Import this collection into Postman for easy API testing:

```json
{
  "info": {
    "name": "Birb Nest API",
    "description": "Distributed caching service API",
    "schema": "https://schema.getpostman.com/json/collection/v2.1.0/collection.json"
  },
  "item": [
    {
      "name": "Cache Operations",
      "item": [
        {
          "name": "Create Cache Entry",
          "request": {
            "method": "POST",
            "header": [
              {
                "key": "Content-Type",
                "value": "application/json"
              }
            ],
            "body": {
              "mode": "raw",
              "raw": "{\n  \"value\": {\n    \"data\": \"example\"\n  },\n  \"ttl\": 3600\n}"
            },
            "url": {
              "raw": "{{baseUrl}}/v1/cache/{{key}}",
              "host": ["{{baseUrl}}"],
              "path": ["v1", "cache", "{{key}}"]
            }
          }
        },
        {
          "name": "Get Cache Entry",
          "request": {
            "method": "GET",
            "url": {
              "raw": "{{baseUrl}}/v1/cache/{{key}}",
              "host": ["{{baseUrl}}"],
              "path": ["v1", "cache", "{{key}}"]
            }
          }
        },
        {
          "name": "Update Cache Entry",
          "request": {
            "method": "PUT",
            "header": [
              {
                "key": "Content-Type",
                "value": "application/json"
              }
            ],
            "body": {
              "mode": "raw",
              "raw": "{\n  \"value\": {\n    \"data\": \"updated\"\n  },\n  \"ttl\": 7200\n}"
            },
            "url": {
              "raw": "{{baseUrl}}/v1/cache/{{key}}",
              "host": ["{{baseUrl}}"],
              "path": ["v1", "cache", "{{key}}"]
            }
          }
        },
        {
          "name": "Delete Cache Entry",
          "request": {
            "method": "DELETE",
            "url": {
              "raw": "{{baseUrl}}/v1/cache/{{key}}",
              "host": ["{{baseUrl}}"],
              "path": ["v1", "cache", "{{key}}"]
            }
          }
        }
      ]
    },
    {
      "name": "Batch Operations",
      "item": [
        {
          "name": "Batch Get",
          "request": {
            "method": "POST",
            "header": [
              {
                "key": "Content-Type",
                "value": "application/json"
              }
            ],
            "body": {
              "mode": "raw",
              "raw": "{\n  \"keys\": [\"key1\", \"key2\", \"key3\"]\n}"
            },
            "url": {
              "raw": "{{baseUrl}}/v1/cache/batch/get",
              "host": ["{{baseUrl}}"],
              "path": ["v1", "cache", "batch", "get"]
            }
          }
        }
      ]
    },
    {
      "name": "Health & Monitoring",
      "item": [
        {
          "name": "Health Check",
          "request": {
            "method": "GET",
            "url": {
              "raw": "{{baseUrl}}/health",
              "host": ["{{baseUrl}}"],
              "path": ["health"]
            }
          }
        },
        {
          "name": "Metrics",
          "request": {
            "method": "GET",
            "url": {
              "raw": "{{baseUrl}}/metrics",
              "host": ["{{baseUrl}}"],
              "path": ["metrics"]
            }
          }
        }
      ]
    }
  ],
  "variable": [
    {
      "key": "baseUrl",
      "value": "http://localhost:8080",
      "type": "default"
    },
    {
      "key": "key",
      "value": "test-key",
      "type": "default"
    }
  ]
}
```

## Performance Tips

1. **Batch Operations**: Use batch endpoints when retrieving multiple keys to reduce network overhead
2. **TTL Strategy**: Set appropriate TTLs to balance memory usage and cache effectiveness
3. **Key Naming**: Use hierarchical key names (e.g., `user:profile:12345`) for better organization
4. **Metadata**: Use metadata for tracking and debugging without affecting cache values
5. **Connection Pooling**: Reuse HTTP connections for better performance

## SDK Support

While the API is simple enough to use directly, client SDKs are available for:
- Go: `github.com/birbparty/birb-nest-go-client`
- Python: `pip install birb-nest`
- Node.js: `npm install @birbparty/birb-nest`
- Java: Coming soon

## Webhooks (Future)

Future releases will support webhooks for:
- Cache invalidation events
- TTL expiration notifications
- Rehydration completion events

---

🐦 Happy caching with Birb Nest!
