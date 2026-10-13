# Observability

Damson emits structured logs to **stdout** via `log/slog`. In production the
JSON handler is used; set `DAMSON_LOG_FORMAT=text` for human-readable local
output.

## Configuration

- `DAMSON_LOG_LEVEL` — `debug`, `info` (default), `warn` or `error`; the minimum level emitted.
- `DAMSON_LOG_FORMAT` — `json` (default, to ship) or `text` (local dev).

Every line carries `service=damson`, a `version` (module version or VCS
revision), and `source` (file:line) via `AddSource`.

### Field names

The JSON handler keeps slog's own keys: the message is `msg` and the timestamp
`time`. Those are among the message fields `ops logs` and Logs Drilldown look
for, so nothing is renamed.

## Request logging

`requestLogger` (`cmd/server/main.go`) emits one line when any request
finishes, plus a start line for requests carrying a file body.

`msg` is composed per request — `POST /api/imports → 202 in 431ms` — rather
than a static `"request"`, so the message column is readable without a
`line_format`. Filter on the structured fields, not on `msg`:

- `method`, `path`, `status`, `duration_ms`, `remote_ip`, `bytes_out`
- `bytes_in` — bytes actually **read from the request body**, not
  `Content-Length`. Present only when non-zero, so a transfer that died at
  19 GB is distinguishable from one that never delivered a byte.
- `client_aborted` — `true` when the request context was cancelled, i.e. the
  client went away rather than the server failing. Absent otherwise. It can
  under-report (the cancel may land after the handler unwinds) but does not
  false-positive.
- `request_id` — correlation id; honoured from an inbound `X-Request-Id`
  header, otherwise generated, and echoed on the response.
- `user_id`, `library_id` — when the request is authenticated / library-scoped.
  These keys are string-mirrored in `main.go`; renaming the constants in
  `auth`/`api` silently drops the fields.
- `err` — **the actual cause** of any 4xx/5xx. Handlers attach it via
  `respond400` / `respond500` (which call `c.Error`). 5xx logs at `error`, 4xx at `warn`.

Query strings are never logged — only `URL.Path` — so a presigned or
token-bearing URL cannot leak through the request line.

### Upload start lines

A request whose method is `POST`/`PUT` and whose `Content-Type` is
`multipart/*` or `application/octet-stream` also logs on **entry**, with
`phase=start`, `content_length`, and the same `request_id`:

```
POST /api/imports → upload started
```

Without it a multi-hour upload is invisible for its whole duration, and one
that never returns — proxy timeout, pod OOM-kill, client hang-up — leaves no
line at all. Pair the start line with the completion line by `request_id`; a
start with no matching completion is a request the process did not survive.

Chunked uploads log one start/completion pair per chunk
`PUT /api/imports/uploads/<id>/chunks/<n>`, each with its own `request_id`. Correlate one upload
session by the `/api/imports/uploads/<id>` path prefix.

Panics are caught by `recoveryHandler`, which logs at `error` with a full
`stack` and the `request_id`, then returns a generic 500.

Response bodies stay generic (`{"error":"internal error"}`) — internals go to
the logs only, never to the client.

## Asynchronous work

Background jobs run on River (`internal/queue`) on the `import`, `preview` and `store_lookup`
queues. River emits its own lifecycle logging through the same `slog`
handler.

Import pipeline failures are logged at `error` as `"imports: pipeline failed"` with `import_id`,
`library_id` and the cause in `reason`, not `err` (`internal/services/imports/pipeline.go`); the
reason is also persisted to `asset_imports.error` and streamed over SSE. Preview failures log at
`warn` and land against `asset_files.preview_status`.

Because the DB row is the durable source of truth in both cases, a log line
is a *diagnostic*, not the record — to ask "what failed and why", query
`asset_imports.error` / `preview_status` rather than reconstructing from logs.

## Shipping to Loki

Damson runs on the k8s cluster, where the cluster-wide Alloy collector
(the `k8s-monitoring` chart) already tails every pod's stdout and pushes it to
Loki — no per-app collector or config is needed. Loki stores the JSON line as
written; `| json` in a query turns its keys (`level`, `status`, `path`,
`duration_ms`, `request_id`, `err`, …) into fields to filter on. Loki also
detects each line's level, as `detected_level`.

Logs are scoped by k8s namespace: `damson-prod` (prod) and `damson-test`
(test).

### Useful LogQL queries

```logql
# Every server error with its cause and correlation id
{namespace="damson-prod"} | json | status >= 500 | line_format "{{.path}} {{.err}} {{.request_id}} {{.user_id}}"

# Trace one request end-to-end
{namespace="damson-prod"} | json | request_id = "<id from the X-Request-Id response header>"

# Upload failures, direct and chunked (the async pipeline is the next query)
{namespace="damson-prod"} | json | path =~ "/api/imports.*" | status >= 500 | line_format "{{.err}} {{.request_id}}"

# Async import pipeline failures
{namespace="damson-prod"} | json | msg = "imports: pipeline failed" | line_format "{{.import_id}} {{.library_id}} {{.reason}}"

# Background job failures across the queues (preview failures are warn, not error)
{namespace="damson-prod"} | json | level =~ "ERROR|WARN" | msg =~ ".*(queue|preview|imports).*"

# Slowest requests in the last hour (a metric query: set the range to 1h)
topk(20, max_over_time({namespace="damson-prod"} | json | unwrap duration_ms [1h]) by (path))

# Uploads that started and never finished — the OOM-kill / proxy-timeout shape.
# Every start line should have a completion line with the same request_id.
{namespace="damson-prod"} | json | phase = "start" | line_format "{{.request_id}} {{.path}} {{.content_length}}"

# How far a failed transfer actually got, and whether the client walked away
{namespace="damson-prod"} | json | path =~ "/api/imports.*" | status >= 400
  | line_format "{{.status}} {{.bytes_in}} {{.client_aborted}} {{.err}} {{.request_id}}"
```
