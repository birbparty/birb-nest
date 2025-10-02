# 🐦 Birb Nest - Persistent Cache Service

A high-performance, distributed caching service built with Go, featuring automatic persistence to PostgreSQL with async writes, Redis caching, and multi-instance support.

## 🏗️ Architecture Overview

```
┌─────────────┐     ┌──────────────────┐     ┌─────────────┐
│   Client    │────▶│   API Service    │────▶│    Redis    │
└─────────────┘     │  (Primary/       │     └─────────────┘
                    │   Replica)       │            ▲
                    └────────┬─────────┘            │
                             │                      │
                             │ AsyncWriter          │
                             │ (Primary only)       │
                             │                      │
                        ┌────▼─────┐                │
                        │PostgreSQL│────────────────┘
                        │(Source of│    (Read on miss)
                        │  Truth)  │
                        └──────────┘
```

### Key Features

- **Async Write Pattern**: Primary writes to Redis immediately, then async writes to PostgreSQL
- **Read-Through Cache**: Automatic fallback from Redis → PostgreSQL with cache rehydration
- **Multi-Instance Support**: Isolated cache instances with instance-aware routing
- **Primary-Replica Mode**: Primary handles persistence, replicas forward writes
- **Instance Registry**: Dynamic instance management with health tracking
- **Observability**: Full Datadog APM integration with traces, metrics, and logs

## 🚀 Quick Start

### Prerequisites

- Docker & Docker Compose
- Go 1.21+ (for local development)
- Make

### Setup

1. **Clone the repository**
   ```bash
   git clone https://github.com/birbparty/birb-nest.git
   cd birb-nest
   ```

2. **Initialize the project**
   ```bash
   make init
   ```

3. **Start services in development mode**
   ```bash
   make dev
   ```

4. **Verify everything is running**
   ```bash
   make health
   ```

### Available UIs

Once running in development mode, you can access:

- **API**: http://localhost:8080
- **Jaeger** (Tracing): http://localhost:16686
- **Prometheus** (Metrics): http://localhost:9090
- **Grafana** (Dashboards): http://localhost:3000 (admin/admin)
- **pgAdmin** (PostgreSQL): http://localhost:5050 (admin@birb.party/admin)
- **Redis Commander**: http://localhost:8081

## 📚 Documentation

### Quick Links
- [🏗️ Architecture Overview](docs/ARCHITECTURE.md) - System design, components, and data flow
- [📚 API Reference](docs/API.md) - Complete API documentation with examples
- [🚀 Deployment Guide](docs/DEPLOYMENT.md) - Docker, Kubernetes, and cloud deployment
- [⚙️ Configuration Guide](docs/CONFIGURATION.md) - All configuration options explained
- [🔧 Troubleshooting](docs/TROUBLESHOOTING.md) - Common issues and solutions

## 🔌 API Usage

### Create/Update Cache Entry
```bash
curl -X POST http://localhost:8080/v1/cache/my-key \
  -H "Content-Type: application/json" \
  -d '{"value": "Hello, Birb!"}'
```

### Get Cache Entry
```bash
curl http://localhost:8080/v1/cache/my-key
```

### Delete Cache Entry
```bash
curl -X DELETE http://localhost:8080/v1/cache/my-key
```

### Health Check
```bash
curl http://localhost:8080/health
```

## 🛠️ Development

### Project Structure
```
birb-nest/
├── cmd/
│   └── api/         # API service entry point
├── internal/
│   ├── api/         # API handlers, routes, and async writer
│   ├── cache/       # Redis cache implementation
│   ├── database/    # PostgreSQL layer with instance support
│   ├── instance/    # Instance registry and management
│   └── telemetry/   # Observability setup
├── sdk/             # Go client SDK
├── scripts/         # Helper scripts
└── docs/            # Documentation
```

### Common Commands

```bash
# Start services
make up          # Production mode
make dev         # Development mode with hot reload

# View logs
make logs        # All services
make logs-api    # API service only

# Database access
make db-shell    # PostgreSQL shell
make redis-cli   # Redis CLI

# Testing
make test         # Run unit tests
make bench        # Run benchmarks

# Code quality
make lint        # Run linters
make fmt         # Format code
make vet         # Run go vet
```

### Environment Variables

Copy `.env.example` to `.env` and configure as needed. Key variables:

- `POSTGRES_*`: PostgreSQL connection settings
- `REDIS_*`: Redis connection settings
- `API_*`: API service configuration (mode, instance ID, primary URL)
- `ASYNC_WRITER_*`: Async write queue configuration
- `DD_*`: Datadog APM configuration
- `LOG_LEVEL`: Logging verbosity (debug, info, warn, error)

## 🔄 Data Flow

### Write Path (Primary Mode)
1. Client sends write request with instance context
2. API writes to Redis immediately (fast response)
3. AsyncWriter queues write for PostgreSQL persistence
4. Background workers batch and persist to PostgreSQL
5. If replica mode: forward write to primary asynchronously

### Read Path
1. Client sends GET request with instance context
2. API checks Redis (cache hit → return immediately)
3. On cache miss, check PostgreSQL by instance ID
4. If found in PostgreSQL, repopulate Redis cache
5. Return result or 404

### Instance Isolation
- Each instance has isolated cache namespace
- Writes are scoped to instance ID
- Registry tracks instance metadata and health
- Default instance for backward compatibility

## 📊 Performance Targets

- **Read Latency**: <5ms p99 (cache hit), <50ms p99 (cache miss)
- **Write Latency**: <10ms p99 (Redis), async PostgreSQL persistence
- **Throughput**: 10,000+ reads/sec, 5,000+ writes/sec per instance
- **Async Queue**: Configurable queue depth and worker count
- **Cache Hit Rate**: >95% after warm-up

## 🚨 Monitoring & Alerts

The service exports metrics in Prometheus format and Datadog APM:

- Cache hit/miss rates per instance
- API request latency histograms with traces
- Async writer queue depth and processing time
- Error rates by operation and instance
- Database connection pool stats
- Instance registry health metrics

Metrics endpoint: `GET /metrics`

## 🤝 Contributing

1. Fork the repository
2. Create your feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -m 'Add amazing feature'`)
4. Push to the branch (`git push origin feature/amazing-feature`)
5. Open a Pull Request

## 📝 License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

## 🎉 Acknowledgments

Built with love by the Birb Party team. Special thanks to all the birbs out there! 🐦

---

Ready to build something amazing? Let's crack open some cold ones and celebrate when we're done! 🍻
