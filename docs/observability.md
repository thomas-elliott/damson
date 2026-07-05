# Observability

Damson emits structured logs to **stdout** via `log/slog`. In production the
JSON handler is used; set `DAMSON_LOG_FORMAT=text` for human-readable local
output.

## Configuration

| Env var             | Values                          | Default | Effect                                   |
| ------------------- | ------------------------------- | ------- | ---------------------------------------- |
| `DAMSON_LOG_LEVEL`  | `debug` `info` `warn` `error`   | `info`  | Minimum level emitted.                   |
| `DAMSON_LOG_FORMAT` | `json` `text`                   | `json`  | `text` for local dev; `json` to ship.    |

Every line carries `service=damson`, a `version` (module version or VCS
revision), and `source` (file:line) via `AddSource`.

## Request logging

One line per HTTP request is emitted by `requestLogger`
(`cmd/server/main.go`), keyed `msg="request"`:

- `method`, `path`, `status`, `duration_ms`, `remote_ip`, `bytes_out`
- `request_id` — correlation id; honoured from an inbound `X-Request-Id`
  header, otherwise generated, and echoed on the response.
- `user_id`, `library_id` — when the request is authenticated / library-scoped.
- `err` — **the actual cause** of any 4xx/5xx. Handlers attach it via
  `respond400` / `respond500` (which call `c.Error`), so a 500 is no longer a
  bare status with no explanation. 5xx logs at `error`, 4xx at `warn`.

Panics are caught by `recoveryHandler`, which logs at `error` with a full
`stack` and the `request_id`, then returns a generic 500.

Response bodies stay generic (`{"error":"internal error"}`) — internals go to
the logs only, never to the client.

## Asynchronous work

Import pipeline failures are logged at `error` with `import_id` and
`library_id` (`internal/services/imports/worker.go`); the per-stage failure
reason is also persisted to `asset_imports.error` and streamed over SSE.

## Shipping to VictoriaLogs (Grafana Alloy)

The stdout JSON is collected by Alloy and pushed to VictoriaLogs' Loki
endpoint, which auto-parses the JSON line into queryable fields. See
[`deploy/alloy/config.alloy`](../deploy/alloy/config.alloy). The ingest URL
maps slog's `msg`/`time` onto VL's native `_msg`/`_time` and uses `service` as
the single stream field.

### Useful LogsQL queries

```logsql
# Every server error with its cause and correlation id
status:>=500 | fields _time, path, err, request_id, user_id

# Trace one request end-to-end
request_id:"<id from the X-Request-Id response header>"

# Upload failures (the POST /api/imports handler + async pipeline)
path:"/api/imports" status:>=500 | fields _time, err, request_id

# Async import pipeline failures
msg:"import pipeline failed" | fields _time, import_id, library_id, err

# Slowest requests in the last hour
_time:1h | sort by (duration_ms desc) | limit 20
```
