# Thermostat Telemetry Reader (TTR) — Requirements

## 1) Objectives

- Continuously ingest thermostat state and history with **minimal coupling** to a specific vendor or datastore.  
- Normalize data to a **canonical document model** suitable for time-series analytics.  
- Be polite to vendor APIs (rate limits, backoff) and resilient to laggy data.  
- Run headless in containers with simple config.  

**Non-goals**: writing settings back to devices, dashboards, correlation logic, alerting, orchestration.

---

## 2) Architecture

**Core Flow**  
Scheduler → Provider Client → Normalizer → Sink

**Pluggable Providers**  
- `ecobee` (first)  
- Future: Nest, Honeywell, etc.  

**Pluggable Sinks**  
- `elasticsearch` (first)  
- Future: MongoDB, S3 NDJSON, Kafka, stdout, etc.  

---

## 3) Canonical Data Model

All timestamps in UTC, with `tz_local` for reference.

### `runtime_5m` (time-series rows)
- `type`: `"runtime_5m"`  
- `thermostat_id`, `thermostat_name`, `household_id`  
- `event_time` (bin start)  
- `mode` (heat/cool/auto/off)  
- `climate` (Home/Away/Sleep/…)  
- `set_heat_c`, `set_cool_c`, `avg_temp_c`  
- `outdoor_temp_c`, `outdoor_humidity_pct`  
- `equip`: `{compHeat1, compHeat2, compCool1, compCool2, fan}`  
- `sensors`: optional map of `{sensor_id: temp_c}`  

### `transition` (state changes)
- `type`: `"transition"`  
- `event_time`  
- `thermostat_id`, `thermostat_name`  
- `prev`: `{mode, set_heat_c, set_cool_c, climate}`  
- `next`: `{mode, set_heat_c, set_cool_c, climate}`  
- `event`: `{kind, name?, data?}`  

### `device_snapshot` (current state)
- `type`: `"device_snapshot"`  
- `collected_at`  
- `thermostat_id`, `thermostat_name`  
- `program`: provider metadata  
- `events_active[]`: active holds/vacations  

Provider-specific details go under `provider.<name>.*`.

---

## 4) Provider Plugin Interface

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

**Ecobee provider (first):**  
- Auth: PIN/OAuth, `smartRead` scope.  
- Summary: `/thermostatSummary` for revisions.  
- Snapshot: `/thermostat` with runtime/program/events/alerts.  
- Runtime: `/runtimeReport` (≤31d).  
- Includes throttling, backoff, and retry logic.

---

## 5) Storage Sink Plugin Interface

```go
type Sink interface {
    Info() SinkInfo
    Open(ctx) error
    Write(ctx, docs []Doc) (Result, error)
    Close(ctx) error
}
```

**Elasticsearch (first sink):**  
- Bulk `_bulk` with retry/backoff.  
- Index naming: `ttr-<doctype>-YYYY.MM.DD`.  
- Deterministic IDs:  
  - runtime_5m: `thermostat_id:event_time:type:hash(body)`  
  - transition: `thermostat_id:event_time:hash(prev,next)`  
  - device_snapshot: `thermostat_id:collected_at`  

**Future sinks:** MongoDB, S3 NDJSON, stdout.

---

## 6) Scheduler & Offsets

- Poll every 5 min (default).  
- Maintain `last_runtime_ts` + `last_snapshot_ts` per thermostat in local store (BoltDB/SQLite).  
- On startup: backfill last 7 days.  
- Trigger fetch when summary revision changes or snapshot age ≥15 min.

---

## 7) Normalization Rules

- Temperatures in °C.  
- Map provider climates to `climate`.  
- Event mapping: hold/vacation/resume/schedule/manual; unknown if ambiguous.  
- Provider-specifics preserved under `provider.<name>`.  

---

## 8) Idempotency & De-dup

- Deterministic IDs (see above).  
- Upserts only, no deletes.  
- Late-arriving data re-emitted safely.  

---

## 9) Configuration

Example YAML:

```yaml
ttr:
  timezone: "America/Chicago"
  poll_interval: "5m"
  backfill_window: "168h"
  log_level: "info"

providers:
  - name: "ecobee"
    enabled: true
    client_id: "${ECOBEE_CLIENT_ID}"
    refresh_token: "${ECOBEE_REFRESH_TOKEN}"

sinks:
  - name: "elasticsearch"
    enabled: true
    url: "https://es.example:9200"
    api_key: "${ELASTIC_API_KEY}"
    index_prefix: "ttr"
    create_templates: true
```

---

## 10) Operational Requirements

- Resource footprint: <150MB RAM, <1 vCPU.  
- Resilient: retries, backoff, jitter.  
- Time: UTC everywhere.  
- Security: tokens via env/secret, never logged.  
- Metrics: counters for provider requests, errors, emitted docs, retries.  

---

## 11) Error Handling

- Error classes: transport, rate_limit, auth, schema, provider_lag.  
- Honor `Retry-After` headers.  
- If auth fails: refresh; fatal if still failing.  
- Don’t advance offsets on write failure.  

---

## 12) Build & Packaging

- Language: Go (static binary).  
- Single container image `ttr`.  
- Repo structure:  
  - `cmd/ttr/`  
  - `internal/core/`  
  - `internal/providers/`  
  - `internal/sinks/`  
  - `pkg/model/`  

---

## 13) Acceptance Criteria

1. Valid Ecobee credentials → discovers thermostats and polls.  
2. Emits `runtime_5m`, `device_snapshot`, `transition` docs.  
3. Elasticsearch sink writes successfully with dedup IDs.  
4. Backfills last 7 days on first run.  
5. Handles transient API errors gracefully.  
6. `/healthz` and `/metrics` endpoints available.  

---

## 14) Extensibility Guidelines

- Add providers by implementing `Provider`.  
- Add sinks by implementing `Sink`.  
- Schema changes: additive only, provider-specific fields namespaced.  

---

## 15) Security & Privacy

- Store only necessary telemetry, avoid PII.  
- Do not log provider payloads at info level.  
- Tokens rotated and hot-reload supported.  

---

## 16) Sequence (ASCII)

```
[Scheduler]-->[Provider.Summary]
    if changed or stale
        |
        v
  [Provider.Snapshot + Runtime]
        |
   [Normalizer]--docs-->[Sink.Write]
        |
   update offsets
```
