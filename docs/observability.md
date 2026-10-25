# Observability

Damson emits structured logs to **stdout** via `log/slog`. In production the
JSON handler is used; set `DAMSON_LOG_FORMAT=text` for human-readable local
output.

## Configuration

- `DAMSON_LOG_LEVEL` — `debug`, `info` (default), `warn` or `error`; the minimum level emitted.
- `DAMSON_LOG_FORMAT` — `json` (default, to ship) or `text` (local dev).

Every line carries `service=damson`, a `version` (module version or VCS
revision), and `source` (file:line) via `AddSource`.

### Field names: `_msg` and `_time`

The JSON handler renames slog's two special keys — `msg` → `_msg` and `time` →
`_time` — via `ReplaceAttr` (`victoriaLogsKeys` in `cmd/server/main.go`). Those
are the field names VictoriaLogs treats as the native message and timestamp; a
line without `_msg` is ingested but shows up empty in VL's default view and
trips its `missing _msg field` warning.

Doing the rename at the source means it works with any collector, so the
ingest side does not need to be configured with `_msg_field=msg`. Other log
stores read the fields as ordinary JSON keys.

The rename is skipped for the text handler (local dev reads better as plain
`msg=`) and for attributes nested inside an `slog` group — a group attribute
called `msg` is data, not the record's message.

## Request logging

`requestLogger` (`cmd/server/main.go`) emits one line when any request
finishes, plus a start line for requests carrying a file body.

`_msg` is composed per request — `POST /api/imports → 202 in 431ms` — rather
than a static `"request"`, so VL's message column is readable without adding
`| fields`. Filter on the structured fields, not on `_msg`:

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
`library_id` and the cause in `reason`, not `err` (`internal/services/ingest/pipeline.go`); the
reason is also persisted to `asset_imports.error` and streamed over SSE. Preview failures log at
`warn` and land against `asset_files.preview_status`.

Because the DB row is the durable source of truth in both cases, a log line
is a *diagnostic*, not the record — to ask "what failed and why", query
`asset_imports.error` / `preview_status` rather than reconstructing from logs.

## Database

GORM logs through the same `slog` handler as `"SQL executed"`, with a `trace` group carrying
`sql`, `duration`, `rows` and, on failure, `error` (`internal/db/db.go`). A query slower than
200ms logs at `warn`; a failed one at `error`. The SQL keeps its `$1` placeholders and never
carries the bound values. A record-not-found, a unique violation (answered with 409) and a
query cancelled because its request went away are not logged.

## Shipping logs

Damson writes one JSON object per line to stdout and needs no log config of its
own. Point whatever collector you run at the container's stdout. VictoriaLogs
picks up `_msg` and `_time` as its native fields (see "Field names" above).
Any store that parses JSON can query the remaining slog keys (`level`,
`status`, `path`, `duration_ms`, `request_id`, `err`, …) as fields.

### Useful queries

These are written in LogsQL (VictoriaLogs). Add whatever stream selector your
setup uses to scope them to one deployment, such as a Kubernetes namespace or
`service:="damson"`.

```logsql
# Every server error with its cause and correlation id
status:>=500 | fields _time, path, err, request_id, user_id

# Trace one request end-to-end
request_id:"<id from the X-Request-Id response header>"

# Upload failures, direct and chunked (the async pipeline is the next query)
path:"/api/imports" status:>=500 | fields _time, err, request_id

# Async import pipeline failures
_msg:"imports: pipeline failed" | fields _time, import_id, library_id, reason

# Background job failures across the queues (preview failures are warn, not error)
level:in("ERROR", "WARN") _msg:~"queue|preview|imports" | fields _time, _msg, err, reason

# Slowest requests in the last hour
_time:1h | sort by (duration_ms desc) | limit 20

# Uploads that started and never finished — the OOM-kill / proxy-timeout shape.
# Every start line should have a completion line with the same request_id.
phase:="start" | fields _time, request_id, path, content_length

# How far a failed transfer actually got, and whether the client walked away
path:"/api/imports" status:>=400
  | fields _time, status, bytes_in, client_aborted, err, request_id
```
