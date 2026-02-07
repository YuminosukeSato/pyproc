"""Test OpenTelemetry tracing support."""

import importlib
import importlib.abc
import os
import sys

import pytest

from pyproc_worker.tracing import (
    HAS_OTEL,
    TracingManager,
    WorkerTracing,
    extract_trace_context,
    trace_method,
)


def test_tracing_disabled_without_otel() -> None:
    """Test that tracing is disabled when OpenTelemetry is not installed."""
    if HAS_OTEL:
        manager = TracingManager(enabled=False)
        assert not manager.enabled
    else:
        manager = TracingManager(enabled=True)
        assert not manager.enabled


def test_tracing_manager_init() -> None:
    """Test TracingManager initialization."""
    if not HAS_OTEL:
        pytest.skip("OpenTelemetry not installed")

    manager = TracingManager(enabled=True, service_name="test-service")
    assert manager.enabled
    assert manager.service_name == "test-service"


def test_worker_tracing_init() -> None:
    """Test WorkerTracing initialization."""
    tracing = WorkerTracing(worker_id="test-worker")
    assert tracing.worker_id == "test-worker"
    assert tracing.manager is not None


def test_trace_request_context() -> None:
    """Test creating trace context for a request."""
    if not HAS_OTEL:
        pytest.skip("OpenTelemetry not installed")

    tracing = WorkerTracing(worker_id="test-worker")
    request = {
        "id": 123,
        "method": "test_method",
        "body": {"data": "test"},
    }

    with tracing.trace_request(request) as span:
        # Span should be None if tracing is disabled by default
        if os.environ.get("PYPROC_TRACING_ENABLED") != "true":
            assert span is None
        else:
            assert span is not None


def test_trace_method_decorator() -> None:
    """Test the trace_method decorator."""

    @trace_method
    def sample_method(request):
        return {"result": request.get("value", 0) * 2}

    request = {"value": 21}
    result = sample_method(request)
    assert result == {"result": 42}
    assert sample_method.__name__ == "sample_method"


def test_add_response_headers() -> None:
    """Test adding trace headers to response."""
    tracing = WorkerTracing()
    response = {"id": 1, "ok": True, "body": {}}

    tracing.add_response_headers(response)

    # Headers should only be added if tracing is enabled
    if os.environ.get("PYPROC_TRACING_ENABLED") == "true" and HAS_OTEL:
        assert "headers" in response
    else:
        # Headers might be added but empty if tracing is disabled
        assert "headers" not in response or response["headers"] == {}


def test_tracing_with_exception() -> None:
    """Test tracing behavior when an exception occurs."""
    if not HAS_OTEL:
        pytest.skip("OpenTelemetry not installed")

    os.environ["PYPROC_TRACING_ENABLED"] = "true"
    tracing = WorkerTracing()

    request = {"id": 1, "method": "failing_method"}

    try:
        with tracing.trace_request(request) as span:
            msg = "Test error"
            raise ValueError(msg)
    except ValueError:
        pass  # Expected

    # Clean up
    del os.environ["PYPROC_TRACING_ENABLED"]


def test_extract_inject_context() -> None:
    """Test context extraction and injection."""
    if not HAS_OTEL:
        pytest.skip("OpenTelemetry not installed")

    manager = TracingManager(enabled=True)

    # Test injection
    carrier = {}
    manager.inject_context(carrier)

    # Test extraction (should not fail even with empty carrier)
    context = manager.extract_context(carrier)
    # Context could be None or an empty context
    assert context is not None or not manager.enabled


def test_extract_trace_context_valid() -> None:
    """Test extract_trace_context with valid traceparent."""
    if not HAS_OTEL:
        pytest.skip("OpenTelemetry not installed")

    request = {
        "traceparent": "00-0af7651916cd43dd8448eb211c80319c-b7ad6b7169203331-01",
        "tracestate": "congo=t61rcWkgMzE",
    }
    context = extract_trace_context(request)
    assert context is not None


def test_extract_trace_context_missing_traceparent() -> None:
    """Test extract_trace_context with missing traceparent."""
    if not HAS_OTEL:
        pytest.skip("OpenTelemetry not installed")

    # Should not crash when traceparent is missing
    request = {}
    context = extract_trace_context(request)
    # Function should handle gracefully (return context with empty values)
    assert context is not None


def test_extract_trace_context_partial() -> None:
    """Test extract_trace_context with only traceparent (no tracestate)."""
    if not HAS_OTEL:
        pytest.skip("OpenTelemetry not installed")

    request = {
        "traceparent": "00-0af7651916cd43dd8448eb211c80319c-b7ad6b7169203331-01",
    }
    context = extract_trace_context(request)
    assert context is not None


def test_extract_trace_context_without_otel() -> None:
    """Test extract_trace_context when OpenTelemetry is not available."""
    if HAS_OTEL:
        pytest.skip("OpenTelemetry is installed, skipping negative test")

    request = {
        "traceparent": "00-0af7651916cd43dd8448eb211c80319c-b7ad6b7169203331-01",
    }
    context = extract_trace_context(request)
    assert context is None


class _OtelBlocker(importlib.abc.MetaPathFinder):
    """Block OpenTelemetry imports for negative-path testing."""

    def find_spec(self, fullname, _path, _target=None):
        if fullname.startswith("opentelemetry"):
            raise ImportError("blocked")


def test_tracing_importerror_paths() -> None:
    """ImportError paths should disable tracing cleanly."""
    from pyproc_worker import tracing

    blocker = _OtelBlocker()
    sys.meta_path.insert(0, blocker)
    for name in list(sys.modules):
        if name.startswith("opentelemetry"):
            del sys.modules[name]

    try:
        reloaded = importlib.reload(tracing)
        assert reloaded.HAS_OTEL is False
        manager = reloaded.TracingManager(enabled=True)
        assert manager.enabled is False
        assert (
            reloaded.extract_trace_context(
                {
                    "traceparent": "00-0af7651916cd43dd8448eb211c80319c-b7ad6b7169203331-01",
                },
            )
            is None
        )
    finally:
        sys.meta_path.remove(blocker)
        importlib.reload(tracing)


def test_tracing_manager_console_exporter(monkeypatch) -> None:
    """Console exporter path should initialize when enabled."""
    if not HAS_OTEL:
        pytest.skip("OpenTelemetry not installed")

    monkeypatch.setenv("PYPROC_TRACE_CONSOLE", "true")

    from pyproc_worker import tracing

    class DummyExporter:
        def export(self, _spans):
            return None

        def shutdown(self):
            return None

        def force_flush(self, _timeout_millis=None):
            return True

    monkeypatch.setattr(tracing, "ConsoleSpanExporter", DummyExporter)

    manager = TracingManager(enabled=True, service_name="console-test")
    assert manager.enabled is True
    assert manager.tracer is not None


def test_span_extracts_trace_context() -> None:
    """Span should handle traceparent and tracestate context."""
    if not HAS_OTEL:
        pytest.skip("OpenTelemetry not installed")

    manager = TracingManager(enabled=True, service_name="context-test")
    context = {
        "traceparent": "00-0af7651916cd43dd8448eb211c80319c-b7ad6b7169203331-01",
        "tracestate": "congo=t61rcWkgMzE",
    }
    with manager.span("test-span", context=context) as span:
        assert span is not None


def test_extract_context_returns_none_when_disabled() -> None:
    """extract_context should return None when disabled."""
    manager = TracingManager(enabled=False)
    assert manager.extract_context({"traceparent": "x"}) is None


def test_add_response_headers_when_enabled(monkeypatch) -> None:
    """add_response_headers should inject headers when enabled."""
    if not HAS_OTEL:
        pytest.skip("OpenTelemetry not installed")

    monkeypatch.setenv("PYPROC_TRACING_ENABLED", "true")
    tracing = WorkerTracing(worker_id="header-test")
    response = {"id": 1, "ok": True, "body": {}}

    tracing.add_response_headers(response)

    assert "headers" in response
