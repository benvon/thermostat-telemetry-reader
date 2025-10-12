# Thermostat Telemetry Reader (TTR)

A Go application that continuously ingests thermostat state and history data with minimal coupling to specific vendors or datastores. TTR normalizes data to a canonical document model suitable for time-series analytics.

## Features

- **Pluggable Providers**: Currently supports Ecobee, with extensible architecture for future providers (Nest, Honeywell, etc.)
- **Pluggable Sinks**: Currently supports Elasticsearch, with extensible architecture for future sinks (MongoDB, S3 NDJSON, Kafka, etc.)
- **Canonical Data Model**: Normalizes all data to consistent format with UTC timestamps
- **Resilient Design**: Exponential backoff with jitter, retry-after header support, and intelligent error handling
- **Persistent Offset Tracking**: SQLite-based offset storage maintains state across restarts (with in-memory fallback)
- **Automatic Transition Detection**: Identifies and tracks state changes (mode, climate, setpoints)
- **Deterministic IDs**: Hash-based document IDs ensure idempotency
- **Health Monitoring**: Built-in health checks and metrics endpoints
- **Container Ready**: Single binary with Docker support and persistent volumes

## Architecture

```
[Scheduler] → [Provider Client] → [Normalizer] → [Sink]
```

### Core Components

1. **Scheduler**: Manages polling intervals and offset tracking
2. **Providers**: Interface with thermostat APIs (Ecobee, Nest, etc.)
3. **Normalizer**: Converts provider-specific data to canonical format
4. **Sinks**: Writes data to storage systems (Elasticsearch, MongoDB, etc.)

## Data Model

TTR emits three types of documents:

### `runtime_5m` (Time-series Data)
- 5-minute runtime telemetry
- Temperature settings, current temps, outdoor conditions
- Equipment status (heat/cool/fan)
- Sensor readings

### `transition` (State Changes)
- Mode changes (heat/cool/auto/off)
- Temperature setting changes
- Climate changes (Home/Away/Sleep/Vacation)
- Event information (hold/vacation/resume/schedule/manual)

### `device_snapshot` (Current State)
- Current thermostat state
- Active events and holds
- Program information

## Quick Start

### Prerequisites

- Go 1.23+ (latest patch release)
- Ecobee account with API access
- Elasticsearch cluster (optional, can use other sinks)
- SQLite3 development libraries (for persistent offset storage)

### Installation

1. Clone the repository:
```bash
git clone https://github.com/benvon/thermostat-telemetry-reader.git
cd thermostat-telemetry-reader
```

2. Build the application:
```bash
make build
```

3. Create a configuration file:
```bash
cp config.yaml.example config.yaml
```

4. Configure your providers and sinks in `config.yaml`

5. Run the application:
```bash
./bin/thermostat-telemetry-reader -config config.yaml
```

### Configuration

Example configuration:

```yaml
ttr:
  timezone: "America/Chicago"
  poll_interval: "5m"
  backfill_window: "168h"
  log_level: "info"
  health_port: 8080
  metrics_port: 9090

providers:
  - name: "ecobee"
    enabled: true
    settings:
      client_id: "${ECOBEE_CLIENT_ID}"
      refresh_token: "${ECOBEE_REFRESH_TOKEN}"

sinks:
  - name: "elasticsearch"
    enabled: true
    settings:
      url: "https://es.example:9200"
      api_key: "${ELASTIC_API_KEY}"
      index_prefix: "ttr"
      create_templates: true
```

### Environment Variables

Set the following environment variables:

```bash
export ECOBEE_CLIENT_ID="your_ecobee_client_id"
export ECOBEE_REFRESH_TOKEN="your_ecobee_refresh_token"
export ELASTIC_API_KEY="your_elastic_api_key"
```

## Ecobee Setup

1. Create an Ecobee developer account at https://www.ecobee.com/developers/
2. Create a new application with `smartRead` scope
3. Obtain your `client_id` and `refresh_token`
4. Configure the provider in your `config.yaml`

## Elasticsearch Setup

TTR automatically creates index templates for optimal time-series storage:

- `ttr-runtime_5m-YYYY.MM.DD`
- `ttr-transition-YYYY.MM.DD`
- `ttr-device_snapshot-YYYY.MM.DD`

## Health and Metrics

TTR provides HTTP endpoints for monitoring:

- **Health Check**: `GET /healthz` - Returns overall system health
- **Metrics**: `GET /metrics` - Returns operational metrics

Example health response:
```json
{
  "status": "healthy",
  "timestamp": "2024-01-15T10:30:00Z",
  "checks": {
    "provider_ecobee": {
      "status": "pass",
      "message": "Provider is healthy",
      "duration_ms": 150,
      "last_checked": "2024-01-15T10:30:00Z"
    },
    "sink_elasticsearch": {
      "status": "pass",
      "message": "Sink is healthy",
      "duration_ms": 25,
      "last_checked": "2024-01-15T10:30:00Z"
    }
  }
}
```

## Development

### Project Structure

```
cmd/ttr/                    # Main application
internal/
  core/                     # Core scheduling and normalization logic
    scheduler.go            # Polling orchestration and transition detection
    normalizer.go           # Data normalization
    offset_sqlite.go        # Persistent offset storage
    health.go               # Health checks and metrics
  providers/ecobee/         # Ecobee provider implementation
  sinks/elasticsearch/      # Elasticsearch sink implementation
pkg/
  config/                   # Configuration management
  model/                    # Data models and interfaces
    id_generator.go         # Deterministic document ID generation
  retry/                    # Retry logic with exponential backoff
  temperature/              # Temperature conversion utilities
```

### Running Tests

```bash
make test
```

### Building

```bash
make build
```

### Linting and Security

```bash
make lint
make security
make vulnerability-check
```

## Docker

### Build Image

```bash
make docker-build
```

### Run Container

```bash
docker run -p 8080:8080 -p 9090:9090 \
  -e ECOBEE_CLIENT_ID=your_client_id \
  -e ECOBEE_REFRESH_TOKEN=your_refresh_token \
  -e ELASTIC_API_KEY=your_api_key \
  thermostat-telemetry-reader:latest
```

### Docker Compose

```bash
make docker-compose-up
```

**Note**: Docker deployment includes a persistent volume (`ttr-data`) for the SQLite offset database, ensuring state is maintained across container restarts.

## Operational Requirements

- **Resource Usage**: <150MB RAM, <1 vCPU
- **Resilience**: Automatic retries with exponential backoff and jitter
- **Time Handling**: All timestamps in UTC
- **Security**: Tokens via environment variables, never logged
- **Monitoring**: Built-in metrics and health checks
- **Persistence**: SQLite database for offset tracking (requires persistent volume in Docker)

## Error Handling

TTR handles various error conditions:

- **Transport Errors**: Network connectivity issues with retry logic
- **Rate Limits**: Respects API rate limits with exponential backoff and Retry-After headers
- **Authentication**: Automatic token refresh with retry
- **Schema Errors**: Graceful handling of data format changes
- **Provider Lag**: Handles delayed data gracefully
- **Partial Failures**: Continues processing even when individual operations fail

## Extensibility

### Adding New Providers

1. Implement the `Provider` interface in `internal/providers/`
2. Add authentication logic
3. Map provider data to canonical format
4. Add configuration support

### Adding New Sinks

1. Implement the `Sink` interface in `internal/sinks/`
2. Handle bulk write operations
3. Implement deterministic ID generation
4. Add configuration support

## Security and Privacy

- No PII is stored beyond necessary telemetry data
- Provider payloads are not logged at info level
- Tokens can be rotated and hot-reloaded
- All communications use HTTPS in production

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

## Contributing

Please read [CONTRIBUTING.md](CONTRIBUTING.md) for details on our code of conduct and the process for submitting pull requests.

## Documentation

- **[Architecture](docs/ARCHITECTURE.md)**: Detailed system architecture and design decisions
- **[Requirements](`.cursor/instructions/project_requirements.md`)**: Original project requirements and specifications
- **[Contributing](CONTRIBUTING.md)**: Guidelines for contributing to the project
- **[Release Guide](RELEASE_GUIDE.md)**: Release process and version history
- **[Security](SECURITY.md)**: Security policies and reporting

## External Dependencies

### Runtime Dependencies

- **SQLite3**: Persistent offset tracking
  - Package: `github.com/mattn/go-sqlite3` v1.14.22
  - Purpose: Maintains polling state across restarts
  - Fallback: Automatically uses in-memory storage if unavailable
  - Database location: `./data/offsets.db`

### Optional Dependencies

- **Elasticsearch**: Time-series data storage (8.x recommended)
- **Docker**: Container deployment

## Support

For support and questions:

- Create an issue on GitHub
- Check the documentation in the `docs/` directory
- Review the example configurations in `examples/`
- Read the architecture documentation for design details