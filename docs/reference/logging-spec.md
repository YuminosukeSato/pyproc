---
title: Logging Specification
description: Field definitions, log level usage, and format specification for pyproc log output
keywords: logging, structured logs, slog, log levels, json, text
---

# Logging Specification

This document defines the required fields for structured logs emitted by pyproc, log level usage, and format specifications.

## Fields

All log entries include the required fields. Conditional fields are attached via context-specific APIs.

| Field | Presence | Type | Description |
|-------|----------|------|-------------|
| time | Required | string (RFC3339Nano) | Log entry timestamp with nanosecond precision |
| level | Required | string | Log level (ERROR / WARN / INFO / DEBUG) |
| msg | Required | string | Log message body |
| trace_id | Conditional | uint64 | Request trace ID. Present only when TraceEnabled is true |
| worker_id | Conditional | string | Worker process identifier. Present when WithWorker is used |
| method | Conditional | string | Target Python function name. Present when WithMethod is used |
| request_id | Conditional | uint64 | Individual request identifier. Present when WithRequestID is used |
| error_category | Conditional | string | Error classification (timeout / crash / protocol / user). Present only on errors |

## Log Level Usage

### ERROR

Used for events requiring immediate attention, such as request processing failures or worker process crashes.

- Request failures (timeout, panic, protocol error)
- Worker process abnormal termination
- Socket connection loss

### WARN

Used for events that are operationally normal but require attention.

- Approaching timeout thresholds
- Retry occurrences
- Worker pool degraded operation

### INFO

Used for normal operational events.

- Pool / worker startup and shutdown
- Health check results
- Configuration loading

### DEBUG

Used for detailed information needed during development or troubleshooting.

- Individual request send/receive contents
- Protocol frame details
- Codec encode / decode processing

## Format

### JSON (Production)

Set `LoggingConfig.Format` to `"json"`. Uses slog.JSONHandler to output one JSON entry per line.

```json
{"time":"2025-01-15T12:00:00.123456789Z","level":"INFO","msg":"worker started","worker_id":"w-1","method":"predict"}
```

### Text (Development)

Set `LoggingConfig.Format` to `"text"`. Uses slog.TextHandler to output in a human-readable format.

```
time=2025-01-15T12:00:00.123+09:00 level=INFO msg="worker started" worker_id=w-1 method=predict
```

## LoggingConfig Mapping

Log settings are controlled by the `LoggingConfig` struct in `pkg/pyproc/config.go`.

```go
type LoggingConfig struct {
    Level        string `mapstructure:"level"`         // Log level (debug/info/warn/error)
    Format       string `mapstructure:"format"`        // Output format (json/text)
    TraceEnabled bool   `mapstructure:"trace_enabled"` // Enable trace_id in log output
}
```

| Field | Default Value | Description |
|-------|---------------|-------------|
| Level | info | Minimum log level to output |
| Format | json | Output format: json or text |
| TraceEnabled | true | When true, adds trace_id to log entries |

## Field Attachment API

Use `Logger`'s With methods to attach fields to log entries.

| Method | Attached Field | Usage |
|--------|---------------|-------|
| WithWorker(workerID) | worker_id | Per-worker logging |
| WithMethod(method) | method | Per-method logging |
| WithRequestID(requestID) | request_id | Per-request logging |
