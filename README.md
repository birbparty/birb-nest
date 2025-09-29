# 🐦 Birb Nest - Persistent Cache Service

A high-performance, distributed caching service built with Go, featuring automatic persistence to PostgreSQL, Redis caching, and NATS JetStream for reliable message processing.

## 🏗️ Architecture Overview

```
┌─────────────┐     ┌─────────────┐     ┌─────────────┐
│   Client    │────▶│  API Service │────▶│    Redis    │
└─────────────┘     └──────┬──────┘     └─────────────┘
                           │                     ▲
                           │                     │
                      ┌────▼─────┐               │
                      │   NATS   │               │
                      │ JetStream│               │
                      └────┬─────┘               │
                           │                     │
                      ┌────▼─────┐               │
                      │  Worker  │───────────────┘
                      │ Service  │
                      └────┬─────┘
                           │
                      ┌────▼─────┐
                      │PostgreSQL│
                      └──────────┘
```

### Key Features

- **Double-Write Pattern**: Writes go to both Redis (for fast access) and NATS queue (for persistence)
- **Read-Through Cache**: Automatic fallback from Redis → PostgreSQL → Rehydration
- **Custom DLQ**: Reliable message processing with retry logic
- **Batch Processing**: Efficient database writes through batching
- **Observability**: Full OpenTelemetry integration with traces, metrics, and logs
- **Auto-Scaling**: Worker service scales based on queue depth

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
│   ├── api/         # API service entry point
│   └── worker/      # Worker service entry point
├── internal/
│   ├── api/         # API handlers and routes
│   ├── cache/       # Redis cache implementation
│   ├── database/    # PostgreSQL layer
│   ├── queue/       # NATS JetStream implementation
│   ├── worker/      # Worker processing logic
│   └── telemetry/   # Observability setup
├── scripts/         # Helper scripts
├── tests/           # Test suites
└── docs/           # Documentation
```

### Common Commands

```bash
# Start services
make up          # Production mode
make dev         # Development mode with hot reload

# View logs
make logs        # All services
make logs-api    # API service only
make logs-worker # Worker service only

# Database access
make db-shell    # PostgreSQL shell
make redis-cli   # Redis CLI

# Testing
make test              # Run unit tests
make test-integration  # Run integration tests
make bench            # Run benchmarks

# Code quality
make lint        # Run linters
make fmt         # Format code
make vet         # Run go vet
```

### Environment Variables

Copy `.env.example` to `.env` and configure as needed. Key variables:

- `POSTGRES_*`: PostgreSQL connection settings
- `REDIS_*`: Redis connection settings
- `NATS_*`: NATS JetStream settings
- `API_*`: API service configuration
- `WORKER_*`: Worker service configuration
- `LOG_LEVEL`: Logging verbosity (debug, info, warn, error)

## 🔄 Message Flow

1. **Write Path**:
   - Client sends POST to API
   - API writes to Redis (with TTL)
   - API publishes to NATS JetStream
   - Worker consumes from NATS
   - Worker batches and writes to PostgreSQL

2. **Read Path**:
   - Client sends GET to API
   - API checks Redis (cache hit → return)
   - If miss, check PostgreSQL
   - If miss, trigger rehydration via NATS
   - Return result or 404

3. **Rehydration**:
   - Worker processes rehydration messages
   - Loads data from PostgreSQL
   - Populates Redis cache
   - Future reads hit cache

## 📊 Performance Targets

- **Read Latency**: <10ms p99 (cache hit)
- **Write Latency**: <50ms p99
- **Throughput**: 10,000+ reads/sec, 5,000+ writes/sec
- **Batch Size**: 100-1000 messages per batch
- **Cache Hit Rate**: >90% after warm-up

## 🚨 Monitoring & Alerts

The service exports metrics in Prometheus format:

- Cache hit/miss rates
- API request latency histograms
- Queue depth and processing time
- Error rates by operation
- Database connection pool stats

Pre-configured Grafana dashboards are available in `configs/grafana/dashboards/`.

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
