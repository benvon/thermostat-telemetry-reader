# TTR Architecture Documentation

## Overview

The Thermostat Telemetry Reader (TTR) is built with a pluggable architecture that decouples data collection from storage, enabling support for multiple thermostat providers and data sinks.

## Core Components

### 1. Scheduler (`internal/core/scheduler.go`)

The scheduler orchestrates the entire data collection process:

- **Polling Loop**: Runs at configurable intervals (default: 5 minutes)
- **Backfill**: On startup, backfills historical data for the configured window (default: 7 days)
- **Offset Tracking**: Maintains `last_runtime_ts` and `last_snapshot_ts` per thermostat
- **Transition Detection**: Automatically detects state changes and generates transition documents
- **Metrics Recording**: Records provider requests, errors, and sink writes

#### Transition Detection

The scheduler compares successive runtime states to detect significant changes:

- **Mode changes**: `heat` ↔ `cool` ↔ `auto` ↔ `off`
- **Climate changes**: `Home` ↔ `Away` ↔ `Sleep` ↔ `Vacation`
- **Setpoint changes**: Heat/cool temperature adjustments (with 0.1°C tolerance)

When a transition is detected, a `transition` document is generated with:
- Previous and next states
- Event classification (hold/vacation/resume/schedule/manual)
- Timestamp and thermostat identification

### 2. Normalizer (`internal/core/normalizer.go`)

Converts provider-specific data to the canonical format:

- **Temperature Normalization**: All temperatures converted to Celsius
- **Mode Mapping**: Standardizes mode strings (`heating` → `heat`, etc.)
- **Climate Mapping**: Standardizes climate names
- **Equipment Normalization**: Consistent equipment key naming
- **Event Classification**: Maps provider events to canonical event kinds

Provider-specific data is preserved under `provider.<name>` namespace.

### 3. Providers

#### Interface (`pkg/model/interfaces.go`)

```go
type Provider interface {
    Info() ProviderInfo
    ListThermostats(ctx) ([]ThermostatRef, error)
    GetSummary(ctx, tr ThermostatRef) (Summary, error)
    GetSnapshot(ctx, tr ThermostatRef, since time.Time) (Snapshot, error)
    GetRuntime(ctx, tr ThermostatRef, from, to time.Time) ([]RuntimeRow, error)
    Auth() AuthManager
}
```

#### Ecobee Provider (`internal/providers/ecobee/`)

- **Authentication**: OAuth 2.0 with automatic token refresh
- **Retry Logic**: Exponential backoff with jitter (max 3 retries)
- **Rate Limit Handling**: Respects `Retry-After` headers
- **Temperature Conversion**: Converts from tenths of Fahrenheit to Celsius
- **API Endpoints**:
  - `/thermostatSummary`: Change detection
  - `/thermostat`: Current state snapshots
  - `/runtimeReport`: Historical 5-minute data

### 4. Sinks

#### Interface (`pkg/model/interfaces.go`)

```go
type Sink interface {
    Info() SinkInfo
    Open(ctx) error
    Write(ctx, docs []Doc) (WriteResult, error)
    Close(ctx) error
}
```

#### Elasticsearch Sink (`internal/sinks/elasticsearch/`)

- **Bulk Operations**: Uses `_bulk` API for efficient writes
- **Index Naming**: `ttr-<doctype>-YYYY.MM.DD` (daily indices)
- **Index Templates**: Automatically created for optimal time-series storage
- **Deterministic IDs**: Prevents duplicate documents on retry
- **Error Handling**: Graceful handling of partial failures

### 5. Offset Store

#### Interface (`internal/core/scheduler.go`)

```go
type OffsetStore interface {
    GetLastRuntimeTime(ctx, thermostatID string) (time.Time, error)
    SetLastRuntimeTime(ctx, thermostatID string, timestamp time.Time) error
    GetLastSnapshotTime(ctx, thermostatID string) (time.Time, error)
    SetLastSnapshotTime(ctx, thermostatID string, timestamp time.Time) error
}
```

#### SQLite Implementation (`internal/core/offset_sqlite.go`)

- **Persistent Storage**: Survives application restarts
- **External Dependency**: Uses `github.com/mattn/go-sqlite3`
- **Database Location**: `./data/offsets.db` by default
- **Schema**: Single table with thermostat_id as primary key
- **Fallback**: Automatically falls back to in-memory store if SQLite unavailable

**Note**: The application gracefully handles SQLite unavailability and falls back to an in-memory offset store. This ensures the application can run even if SQLite is not available, though offset state will not persist across restarts.

### 6. ID Generator (`pkg/model/id_generator.go`)

Generates deterministic document IDs to ensure idempotency:

- **runtime_5m**: `thermostat_id:event_time:type:hash(body)`
- **transition**: `thermostat_id:event_time:hash(prev,next)`
- **device_snapshot**: `thermostat_id:collected_at`

Hash uses SHA-256 (first 16 characters) for collision avoidance while keeping IDs manageable.

### 7. Retry/Backoff (`pkg/retry/`)

Reusable retry logic with:

- **Exponential Backoff**: Delay increases with each retry
- **Jitter**: Random variance to prevent thundering herd
- **Max Delay Cap**: Prevents excessive wait times
- **Retry-After Support**: Respects server-provided retry delays
- **Context Cancellation**: Properly handles context timeouts
- **Retriable Error Detection**: Identifies transient vs permanent failures

**Default Configuration**:
- Max Retries: 3
- Initial Delay: 1 second
- Max Delay: 30 seconds
- Multiplier: 2.0
- Jitter: Enabled (0-25% variance)

## Data Flow

```
┌─────────────┐
│  Scheduler  │ ◄─── Poll Interval (5 min)
└──────┬──────┘
       │
       ├──► Provider.GetSummary() ──► Check revision changes
       │
       ├──► Provider.GetSnapshot() ──► Device state
       │         │
       │         └──► Normalizer ──► DeviceSnapshot doc
       │                   │
       │                   └──► Sink.Write()
       │
       └──► Provider.GetRuntime() ──► Historical data
                 │
                 └──► Normalizer ──► Runtime5m docs
                           │              + Transition docs
                           │
                           └──► Sink.Write()
```

## Error Handling Strategy

### Provider Errors

1. **Transient Errors** (network, timeout):
   - Retry with exponential backoff
   - Log at debug level
   - Record metrics

2. **Rate Limit Errors**:
   - Respect `Retry-After` header
   - Back off with jitter
   - Continue processing other thermostats

3. **Authentication Errors**:
   - Automatic token refresh
   - Retry once with new token
   - Fatal if refresh fails

### Sink Errors

1. **Partial Write Failures**:
   - Log individual failures
   - Record metrics
   - Continue processing

2. **Complete Write Failures**:
   - Do not advance offsets
   - Log at error level
   - Retry on next poll cycle

### Offset Store Errors

- Non-fatal: Uses zero time and re-fetches
- Logged at warn level
- Does not halt processing

## Metrics and Observability

### Health Checks (`/healthz`)

Returns:
- Overall status (healthy/degraded/unhealthy)
- Per-component checks (providers, sinks)
- Check duration and last checked time

### Metrics (`/metrics`)

Tracks:
- Provider request counts and errors
- Sink write counts and errors
- Documents written count
- Last request/write timestamps
- Application uptime

### Logging

Uses structured logging (slog) with levels:
- **Debug**: Detailed operational info
- **Info**: Normal operations, state changes
- **Warn**: Non-fatal issues, fallbacks
- **Error**: Failures requiring attention

**Security**: Sensitive data (tokens, API keys) never logged.

## Configuration

### Environment Variables

Priority: ENV vars > Config file > Defaults

Core settings:
- `TTR_TIMEZONE`: Timezone for local reference
- `TTR_LOG_LEVEL`: Logging verbosity
- `TTR_POLL_INTERVAL`: Polling frequency
- `TTR_BACKFILL_WINDOW`: Historical backfill period

Provider/Sink settings:
- `PROVIDERS_N_SETTINGS_KEY`: Override provider config
- `SINKS_N_SETTINGS_KEY`: Override sink config

### Docker Deployment

The docker-compose.yml configures:
- Persistent volume for SQLite database (`ttr-data`)
- Health check on port 8080
- Metrics endpoint on port 9090
- Environment variable injection

## Extensibility

### Adding a New Provider

1. Implement `model.Provider` interface
2. Create provider-specific authentication
3. Map provider data to canonical format
4. Handle provider-specific retry logic
5. Add to provider initialization in `main.go`

### Adding a New Sink

1. Implement `model.Sink` interface
2. Handle bulk write operations
3. Implement error handling and metrics
4. Add to sink initialization in `main.go`

### Adding New Document Types

1. Define struct in `pkg/model/canonical.go`
2. Add ID generation method to `IDGenerator`
3. Add normalizer method
4. Update sink implementations if needed

## Performance Considerations

### Resource Usage

Target: <150MB RAM, <1 vCPU

Achieved through:
- Streaming data processing
- Bounded retry attempts
- Efficient batch writes
- Minimal data buffering

### Scalability

Current design supports:
- Hundreds of thermostats
- 5-minute polling intervals
- Multiple concurrent providers/sinks

For larger deployments, consider:
- Sharding by thermostat groups
- Independent scheduler instances
- Load balancing across sinks

## Security

### Credentials Management

- Tokens stored only in memory
- Environment variable injection
- Config file values redacted in logs
- No credentials in application logs

### Network Security

- HTTPS for all provider APIs
- TLS for sink connections
- Configurable timeout values
- No credential exposure in errors

### Data Privacy

- Only necessary telemetry collected
- No PII beyond thermostat names
- Provider payloads not logged at info level
- Sensitive config values redacted

## Testing Strategy

### Unit Tests

- All exported functions tested
- Table-driven tests for multiple cases
- Mock external dependencies
- Parallel test execution

### Integration Tests

- Provider authentication flows
- Sink write operations
- Offset persistence
- Error recovery scenarios

### Acceptance Criteria

1. ✅ Discovers thermostats with valid credentials
2. ✅ Emits all three document types
3. ✅ Writes successfully with deterministic IDs
4. ✅ Backfills historical data on startup
5. ✅ Handles transient errors gracefully
6. ✅ Health and metrics endpoints functional
7. ✅ Persistent offset tracking across restarts

